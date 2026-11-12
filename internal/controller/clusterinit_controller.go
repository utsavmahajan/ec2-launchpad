/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	dispatchv1 "github.com/utsavmahajan/ec2-launchpad/api/v1"
	batch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	// "k8s.io/client-go/metadata"
	"encoding/json"

	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *ClusterInitReconciler) CreateRoleBinding(ctx context.Context, cli corev1.CoreV1Interface, Namespace string) error {
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default-sa-binding",
			Namespace: Namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "default",
				Namespace: Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "admin",
		},
	}

	if err := r.Create(ctx, rb); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func CreateCM(ctx context.Context, cli corev1.CoreV1Interface, clusterSpec dispatchv1.ClusterInitSpec, Namespace string) error {
	Name := clusterSpec.UserName + "-" + clusterSpec.RandomSTR
	addons_str := strings.Join(clusterSpec.Addons, ",")
	cmDefinition := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      Name,
			Namespace: Namespace,
		},
		Data: map[string]string{
			"Control_Node_type.txt": clusterSpec.ControlNodeType,
			"No_of_Worker_Node.txt": strconv.Itoa(clusterSpec.NO_Of_Worker_Node),
			"CNI.txt":               clusterSpec.CNI,
			"addons.txt":            addons_str,
			"ingress.txt":           clusterSpec.Ingress,
			"gateway.txt":           clusterSpec.Gateway,
			"randomSTR.txt":         clusterSpec.RandomSTR,
			"storage.txt":           strconv.Itoa(int(clusterSpec.Storage)),
			"uuid.txt":              clusterSpec.UUID,
			"helm.txt":              strconv.FormatBool(clusterSpec.Helm),
			"crictl.txt":            strconv.FormatBool(clusterSpec.Crictl),
		},
	}
	_, err := cli.ConfigMaps(Namespace).Create(ctx, cmDefinition, metav1.CreateOptions{})
	return err
}

func createCMforInstanceType(ctx context.Context, cli corev1.CoreV1Interface, Namespace string, data string) error {
	Name := "instancetypes"
	datamap := make(map[string]string)
	if err := json.Unmarshal([]byte(data), &datamap); err != nil {
		return err
	}
	cmDefinition := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      Name,
			Namespace: Namespace,
		},
		Data: datamap,
	}
	_, err := cli.ConfigMaps(Namespace).Create(ctx, cmDefinition, metav1.CreateOptions{})
	return err
}

func createSecret(ctx context.Context, cli corev1.CoreV1Interface, clusterSpec dispatchv1.ClusterInitSpec, Namespace string) error {
	Name := "databaseandawssecrets"
	secretDefinition := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      Name,
			Namespace: Namespace,
		},
		Type: v1.SecretTypeOpaque,
		Data: map[string][]byte{
			"awsRegion":  []byte(clusterSpec.AWSRegion),
			"roleARN":    []byte(clusterSpec.RoleARN),
			"externalID": []byte(clusterSpec.ExternalID),
		},
	}
	_, err := cli.Secrets(Namespace).Create(ctx, secretDefinition, metav1.CreateOptions{})
	return err
}

func createPod(ctx context.Context, cli corev1.CoreV1Interface, clusterSpec dispatchv1.ClusterInitSpec, Namespace string) error {
	podCli := cli.Pods(Namespace)
	Name := strings.ToLower(clusterSpec.UserName + "-" + clusterSpec.RandomSTR)
	volumeName := "cluster-spec"
	var AWS_ACCESS_KEY_ID string = os.Getenv("AWS_ACCESS_KEY_ID")
	if AWS_ACCESS_KEY_ID == "" {
		return errors.New("AWS Access Key can't Be Empty")
	}
	var AWS_SECRET_ACCESS_KEY string = os.Getenv("AWS_SECRET_ACCESS_KEY")
	if AWS_SECRET_ACCESS_KEY == "" {
		return errors.New("AWS Access Secret Key can't Be Empty")
	}
	var DATABASE_URL string = os.Getenv("DATABASE_URL")
	if DATABASE_URL == "" {
		return errors.New("Database Conn String Can't Be Empty")
	}

	podDefinition := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      Name,
			Namespace: Namespace,
		},
		Spec: v1.PodSpec{
			Volumes: []v1.Volume{
				{
					Name: volumeName,
					VolumeSource: v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: Name,
							},
						},
					},
				},
			},
			Containers: []v1.Container{
				{
					Name:  Name,
					Image: "utsav5/ec2-launcher:v11",
					Env: []v1.EnvVar{
						{
							Name:  "DATABASE_URL",
							Value: DATABASE_URL,
						},
						{
							Name:  "AWS_ACCESS_KEY_ID",
							Value: AWS_ACCESS_KEY_ID,
						},
						{
							Name:  "AWS_SECRET_ACCESS_KEY",
							Value: AWS_SECRET_ACCESS_KEY,
						},
						{
							Name:  "ROLEARN",
							Value: clusterSpec.RoleARN,
						},
						{
							Name:  "AWSREGION",
							Value: clusterSpec.AWSRegion,
						},
						{
							Name:  "UUID",
							Value: clusterSpec.UUID,
						},
						{
							Name:  "RANDSTR",
							Value: clusterSpec.RandomSTR,
						},
						{
							Name:  "EXTERNALID",
							Value: clusterSpec.ExternalID,
						},
						{
							Name: "NAMESPACE",
							ValueFrom: &v1.EnvVarSource{
								FieldRef: &v1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
							},
						},
						{
							Name:  "DISPATCH_NAME",
							Value: Name,
						},
						{
							Name:  "CLUSTER_NAME",
							Value: Name,
						},
					},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      volumeName,
							MountPath: "/values",
						},
					},
				},
			},
		},
	}
	_, err := podCli.Create(ctx, podDefinition, metav1.CreateOptions{})
	return err
}

func createJob(ctx context.Context, client *kubernetes.Clientset, Namespace string, envList []v1.EnvVar) (error, bool) {
	jobcli := client.BatchV1().Jobs(Namespace)
	Name := "deleteinstancejob"
	backoffLimit := int32(2)

	jobDefinition := &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      Name,
			Namespace: Namespace,
		},
		Spec: batch.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  Name,
							Image: "utsav5/delete-job:v3",
							Env:   envList,
						},
					},
					RestartPolicy: v1.RestartPolicyNever,
				},
			},
		},
	}

	if _, err := jobcli.Create(ctx, jobDefinition, metav1.CreateOptions{}); err != nil {
		return err, false
	}

	err := wait.PollUntilContextTimeout(ctx, 3*time.Second, 12*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			j, err := jobcli.Get(ctx, Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			if j.Status.Succeeded >= 1 {
				return true, nil
			}
			if j.Status.Failed > backoffLimit {
				return false, errors.New("job failed")
			}
			return false, nil
		})
	if err != nil {
		return errors.New("Failed To Delete Instances Please Do it manually"), false
	}

	return nil, true
}

// ClusterInitReconciler reconciles a ClusterInit object
type ClusterInitReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=dispatch.ec2-launchpad.io,resources=clusterinits,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dispatch.ec2-launchpad.io,resources=clusterinits/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dispatch.ec2-launchpad.io,resources=clusterinits/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;delete
// +kubebuilder:subresource:status
// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ClusterInit object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.25.0/pkg/reconcile
func (r *ClusterInitReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := logf.FromContext(ctx)
	Finalizerstr := "deleteing-resources-from-aws"
	launcher := &dispatchv1.ClusterInit{}
	if err := r.Get(ctx, req.NamespacedName, launcher); err != nil {
		if apierrors.IsNotFound(err) {
			l.Info("ClusterInit resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		l.Error(err, "Failed to get ClusterInit")
		return ctrl.Result{}, err
	}
	if len(launcher.Finalizers) == 0 {
		launcher.Finalizers = append(launcher.Finalizers, Finalizerstr)
		if err := r.Update(ctx, launcher); err != nil {
			l.Error(err, "Failed To ADD The Finalizer")
			return ctrl.Result{}, err
		}
	}
	l.Info("Successfully fetched resource", "name", launcher.Name)
	clusterSpec := launcher.Spec
	clusterStatus := launcher.Status
	Namespace := launcher.Namespace
	home, _ := os.UserHomeDir()
	kubeConfigPath := filepath.Join(home, ".kube/config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		panic(err.Error())
	}
	client := kubernetes.NewForConfigOrDie(config)
	cli := client.CoreV1()
	envList := []v1.EnvVar{}
	var AWS_ACCESS_KEY_ID string = os.Getenv("AWS_ACCESS_KEY_ID")
	if AWS_ACCESS_KEY_ID == "" {
		return ctrl.Result{}, errors.New("AWS Access Key can't Be Empty")
	}
	var AWS_SECRET_ACCESS_KEY string = os.Getenv("AWS_SECRET_ACCESS_KEY")
	if AWS_SECRET_ACCESS_KEY == "" {
		return ctrl.Result{}, errors.New("AWS Access Secret Key can't Be Empty")
	}
	var DATABASE_URL string = os.Getenv("DATABASE_URL")
	if DATABASE_URL == "" {
		return ctrl.Result{}, errors.New("Database Conn String Can't Be Empty")
	}
	if !launcher.DeletionTimestamp.IsZero() {
		if launcher.Spec.ManagedBy == "k8-launcher" {
			envList = []v1.EnvVar{
				{
					Name:  "AWS_ACCESS_KEY_ID",
					Value: AWS_ACCESS_KEY_ID,
				},
				{
					Name:  "AWS_SECRET_ACCESS_KEY",
					Value: AWS_SECRET_ACCESS_KEY,
				},
				{
					Name:  "AWSREGION",
					Value: clusterSpec.AWSRegion,
				},
				{
					Name:  "DATABASE_URL",
					Value: DATABASE_URL,
				},
				{
					Name:  "UUID",
					Value: clusterSpec.UUID,
				},
				{
					Name:  "RANDOMCODE",
					Value: clusterSpec.RandomSTR,
				},
				{
					Name:  "NAMESPACE",
					Value: Namespace,
				},
				{
					Name:  "Cluster_Name",
					Value: clusterSpec.UserName + "-" + clusterSpec.RandomSTR,
				},
				{
					Name:  "ROLEARN",
					Value: clusterSpec.RoleARN,
				},
				{
					Name:  "EXTERNALID",
					Value: clusterSpec.ExternalID,
				},
			}
		} else {
			envList = []v1.EnvVar{
				{
					Name:  "DATABASE_URL",
					Value: DATABASE_URL,
				},
				{
					Name:  "UUID",
					Value: clusterSpec.UUID,
				},
				{
					Name:  "RANDOMCODE",
					Value: clusterSpec.RandomSTR,
				},
				{
					Name:  "NAMESPACE",
					Value: Namespace,
				},
				{
					Name:  "Cluster_Name",
					Value: clusterSpec.UserName + "-" + clusterSpec.RandomSTR,
				},
			}
		}
		err, status := createJob(ctx, client, Namespace, envList)
		if err != nil || status == false {
			l.Error(err, "Failed To Complete the JOB")
		}
		time.Sleep(time.Minute * 2)
		if status == true {
			if err := cli.Namespaces().Delete(ctx, Namespace, metav1.DeleteOptions{}); err != nil {
				l.Error(err, "Failed To Delete The Namespace")
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(launcher, Finalizerstr)
			if err := r.Update(ctx, launcher); err != nil {
				l.Error(err, "Failed To Delete Finalizer")
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
	}

	if clusterStatus.RoleBindingCreated == false {
		if createErr := r.CreateRoleBinding(ctx, cli, Namespace); createErr != nil {
			l.Error(err, "Failed To Create A Role Binding")
			return ctrl.Result{}, err
		}
		launcher.Status.RoleBindingCreated = true
		if err := r.Status().Update(ctx, launcher); err != nil {
			l.Error(err, "Failed To Edit the status")
		}
	}

	if clusterStatus.CMCreated == false {
		if createErr := CreateCM(ctx, cli, clusterSpec, Namespace); createErr != nil {
			l.Error(createErr, "Failed to create ConfigMap from the given spec")
			return ctrl.Result{}, createErr
		}
		launcher.Status.CMCreated = true
		if err := r.Status().Update(ctx, launcher); err != nil {
			l.Error(err, "Failed To Edit the status")
		}
	}
	if clusterStatus.SecretCreated == false {
		if createErr := createSecret(ctx, cli, clusterSpec, Namespace); createErr != nil {
			l.Error(createErr, "Failed to create ConfigMap from the given spec")
			return ctrl.Result{}, createErr
		}
		launcher.Status.SecretCreated = true
		if err := r.Status().Update(ctx, launcher); err != nil {
			l.Error(err, "Failed To Edit the status")
		}
	}
	if clusterStatus.PodCreated == false {
		if createErr := createPod(ctx, cli, clusterSpec, Namespace); createErr != nil {
			l.Error(createErr, "Failed to create ConfigMap from the given spec")
			return ctrl.Result{}, createErr
		}
		launcher.Status.PodCreated = true
		if err := r.Status().Update(ctx, launcher); err != nil {
			l.Error(err, "Failed To Edit the status")
		}
	}
	if clusterStatus.CMForIPCreated == false {
		if createErr := createCMforInstanceType(ctx, cli, Namespace, clusterSpec.WorkerNodeInstanceTypes); createErr != nil {
			l.Error(createErr, "Failed to create ConfigMap from the given spec")
			return ctrl.Result{}, createErr
		}
		launcher.Status.CMForIPCreated = true
		if err := r.Status().Update(ctx, launcher); err != nil {
			l.Error(err, "Failed To Edit the status")
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterInitReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dispatchv1.ClusterInit{}).
		Named("clusterinit").
		Complete(r)
}

// k logs -n k
// k get po -n 5aa319e3-e413-4635-8f11-6551257a0212-abcde
// kubectl patch clusterinit test-user-abcde -n 5aa319e3-e413-4635-8f11-6551257a0212-abcde   --type=merge -p '{"metadata":{"finalizers":null}}'
