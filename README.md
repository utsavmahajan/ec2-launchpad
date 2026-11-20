# ec2-launchpad

A Kubernetes operator that turns a `ClusterInit` custom resource into a working kubeadm cluster on AWS EC2, from nothing to a running cluster in about 10-15 minutes.

You declare the cluster you want (node types, worker count, CNI, ingress, addons), and the operator launches the infrastructure in **your own AWS account** using cross-account STS AssumeRole, bootstraps it with Ansible, monitors node health, and cleans up every AWS resource when you delete the resource.

> **Project status: core complete, product incomplete.**
> The operator and its workloads are functional end to end. The backend and frontend that would make this a self-service product are not built yet. See [Status and roadmap](#status-and-roadmap).

Related repos: [`k8-project`](https://github.com/utsavmahajan/k8-project) (provisioner, bootstrap, watcher and teardown workloads) and [`ansible-scripts`](https://github.com/utsavmahajan/ansible-scripts) (cluster bootstrap playbooks).

## How it works

```
 ClusterInit (CR)  ──►  Operator  ──►  provisioner pod  ──►  EC2 instances in your AWS account
                          │                  │
                          │                  ├─►  Ansible Job  ──►  kubeadm cluster + CNI/ingress/addons
                          │                  └─►  watcher pods ──►  health checks, dead-node cleanup
                          │
                          └─►  finalizer  ──►  delete Job  ──►  terminates instances, key pair, security group
```

The operator adds a finalizer, creates a RoleBinding, a ConfigMap holding the spec, a Secret and the provisioner pod, and records each step in the CR status so reconciles are idempotent. When the CR is deleted, the finalizer runs a teardown Job before the namespace is removed.

## Prerequisites

- A Kubernetes cluster you can administer, with `kubectl` and `helm` installed
- An AWS account where the clusters will be created
- A PostgreSQL database reachable from the cluster, with a `clusterinfo` table (`user_id`, `random_code`, `instance_public_ip`, `role`)
- AWS credentials for the operator to use when assuming your role

## Setup

### 1. Create the IAM role in your AWS account

This repo contains a CloudFormation template (`cloudformation/` directory; use the template file in that folder). Run it in **your own AWS account** from the CloudFormation console or CLI.

It creates the IAM role that the operator assumes through STS. Once the stack finishes, note two values from the stack:

- the **Role ARN**, which goes in `roleARN` in your `ClusterInit`
- the **External ID** you chose, which goes in `externalID`

### 2. Create the operator namespace and secret

```bash
kubectl create namespace ec2-operator-system

kubectl create secret generic ec2-operator-secrets -n ec2-operator-system \
  --from-literal=AWS_ACCESS_KEY_ID=<your-access-key-id> \
  --from-literal=AWS_SECRET_ACCESS_KEY=<your-secret-access-key> \
  --from-literal=DATABASE_URL=<your-postgres-connection-string>
```

### 3. Grant permissions on the custom resource

This ClusterRole aggregates into the built-in `admin` role, so namespace admins can manage `ClusterInit` resources:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: clusterinit-aggregate-to-admin
  labels:
    rbac.authorization.k8s.io/aggregate-to-admin: "true"
rules:
- apiGroups: ["dispatch.ec2-launchpad.io"]
  resources: ["clusterinits"]
  verbs: ["*"]
- apiGroups: ["dispatch.ec2-launchpad.io"]
  resources: ["clusterinits/status"]
  verbs: ["*"]
```

Save it to a file and apply it with `kubectl apply -f`. Then bind the operator's ServiceAccount to the `admin` role:

```bash
kubectl create clusterrolebinding requiredpermisions \
  --clusterrole=admin \
  --serviceaccount=ec2-operator-system:ec2-operator-controller-manager
```

> This binding is broader than it needs to be. See [Known limitations](#known-limitations).

### 4. Install the operator

```bash
helm install ec2-operator oci://registry-1.docker.io/utsav5/ec2-operator \
  --version 0.1.0 -n ec2-operator-system
```

Check that it's running:

```bash
kubectl get pods -n ec2-operator-system
```

## Creating a cluster

Each `ClusterInit` lives in its own namespace, which the operator deletes again on teardown. Create the namespace first, then apply the resource.

```bash
kubectl create namespace 5aa319e3-e413-4635-8f11-6551257a0212-abcde
```

```yaml
apiVersion: dispatch.ec2-launchpad.io/v1
kind: ClusterInit
metadata:
  name: test-user-abcde
  namespace: 5aa319e3-e413-4635-8f11-6551257a0212-abcde
spec:
  roleARN: arn:aws:iam::XXXXXXXX:role/MyAppAccessRole
  externalID: abcde
  awsRegion: us-east-1
  controlNodeType: c7i-flex.large
  storage: 15
  workerNodeinstanceTypes: '{"worker-1":"c7i-flex.large","worker-2":"c7i-flex.large"}'
  userName: test-user
  randomSTR: abcde
  cni: cilium
  uuid: 5aa319e3-e413-4635-8f11-6551257a0212
  ingress: nginx-ingress
  gateway: ""
  helm: true
  crictl: false
  noOfWorkerNodeInstance: 1
  addons:
    - etcdctl
    - metrics-server
```

Apply it with `kubectl apply -f` and watch progress with `kubectl get clusterinit -A -w` and `kubectl get pods -n <namespace>`.

> **This example is not the intended user experience.**
> The `ClusterInit` was never meant to be written by hand. It was designed to be generated by a backend and frontend: a user would enter their Role ARN, External ID and cluster options in a form, and the backend would create the resource. That's why it has fields like `uuid`, `userName` and `randomSTR`, which identify the user and request and are meant to be filled in by a backend. When creating one manually, any unique values will do.

### Spec reference

| Field | Description |
|---|---|
| `roleARN`, `externalID` | The IAM role from the CloudFormation stack and its External ID |
| `awsRegion` | AWS region for the cluster |
| `controlNodeType` | EC2 instance type for the control-plane node |
| `workerNodeinstanceTypes` | JSON map of worker name to instance type |
| `noOfWorkerNodeInstance` | Worker count passed to the provisioner as configuration |
| `storage` | Root volume size (GB) |
| `cni` | `flannel`, `calico` or `cilium` |
| `ingress` | `nginx-ingress` or `traefik` (see `ansible-scripts` for exact names) |
| `gateway` | Gateway API implementation (`nginx-gateway`), or empty for none |
| `helm`, `crictl` | Install Helm / crictl on the nodes |
| `addons` | Extra installs, such as `metrics-server` and `etcdctl` |
| `userName`, `randomSTR`, `uuid` | Identifiers used for naming and database tracking; meant to be set by a backend |

### Deleting a cluster

```bash
kubectl delete clusterinit test-user-abcde -n 5aa319e3-e413-4635-8f11-6551257a0212-abcde
```

The finalizer runs a teardown Job that terminates the EC2 instances and deletes the key pair, security group and database rows, then removes the namespace.

## Known limitations

This is an early version, and these gaps are known:

- **Overly permissive ClusterRoleBinding.** The operator's ServiceAccount is bound to `admin`, and the default ServiceAccount in each cluster namespace gets `admin` as well.
- **No backend or frontend.** Clusters are created by applying YAML by hand, which was never the intended experience.
- **No unit tests.**
- **No proper error handling.** Some failures are only logged and don't requeue, and several waits are fixed sleeps rather than readiness checks.
- Credentials are passed to workloads as plain env vars rather than Secret references.
- The generated security group opens SSH and other ports to `0.0.0.0/0`.
- Only `us-east-1` has an AMI configured.

## Status and roadmap

The core structure is in place: the CRD, reconciliation with finalizers, AWS provisioning, Ansible bootstrap, health watchers and teardown. The next version is planned to address the limitations above:

- [ ] A backend API and frontend so users enter their details in a form instead of writing YAML
- [ ] Least-privilege RBAC in place of the broad `admin` bindings
- [ ] Unit and integration tests
- [ ] Proper error handling, requeue behavior and readiness checks
- [ ] Secrets handled through Kubernetes Secret references
- [ ] Multi-region support