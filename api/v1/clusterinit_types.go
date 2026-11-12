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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ClusterInitSpec defines the desired state of ClusterInit
type ClusterInitSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

    // +kubebuilder:default:=us-east-1
	AWSRegion string `json:"awsRegion"`
    // +kubebuilder:validation:Minimum=0
    // +kubebuilder:validation:Maximum=5
	NO_Of_Worker_Node int `json:"noOfWorkerNodeInstance"`
	WorkerNodeInstanceTypes string `json:"workerNodeinstanceTypes"`
    // +kubebuilder:validation:Minimum=8
	Storage int  `json:"storage"`
    // +kubebuilder:default:=flannel
	CNI string `json:"cni"`
    // +required
	UUID string `json:"uuid"`
    // +required
	ExternalID string `json:"externalID"`
    // +required
	RoleARN string `json:"roleARN"`
    // +required
	UserName string `json:"userName"`
    // +required
	RandomSTR string `json:"randomSTR"`
	// +kubebuilder:default:=true
	Helm bool `json:"helm"`
	// +kubebuilder:default:=false
	Crictl bool `json:"crictl"`
	// +kubebuilder:default:=k8-launcher
	ManagedBy string `json:"managedBy"`
	// +kubebuilder:default:=c7i-flex.large
	ControlNodeType string `json:"controlNodeType"`
	Gateway string `json:"gateway"`
	Ingress string `json:"ingress"`
	Addons []string `json:"addons"`
}

// ClusterInitStatus defines the observed state of ClusterInit.
type ClusterInitStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the ClusterInit resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
    // +kubebuilder:default:=false
	PodCreated bool `json:"podCreated"`
    // +kubebuilder:default:=false
	RoleBindingCreated bool `json:"roleBindingCreated"`
    // +kubebuilder:default:=false
	CMCreated bool `json:"cmCreated"`
    // +kubebuilder:default:=false
	CMForIPCreated bool `json:"cmForIPCreated"`
    // +kubebuilder:default:=false
	SecretCreated bool `json:"secretCreated"`
    // +kubebuilder:default:=false
	ControlNodeCreated bool `json:"controlNodeCreated"`
    // +kubebuilder:default:=false
	SgCreated bool `json:"sgCreated"`
    // +kubebuilder:default:=false
	SshKeyCreated bool `json:"sshKeyCreated"`
    // +kubebuilder:default:=""
	SshKeyName string `json:"sshKeyName"`
    // +kubebuilder:default:=""
	ControlNodePublicIP string `json:"ControlNodePublicIP"`
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ClusterInit is the Schema for the clusterinits API
type ClusterInit struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ClusterInit
	// +required
	Spec ClusterInitSpec `json:"spec"`

	// status defines the observed state of ClusterInit
	// +optional
	Status ClusterInitStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ClusterInitList contains a list of ClusterInit
type ClusterInitList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ClusterInit `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &ClusterInit{}, &ClusterInitList{})
		return nil
	})
}
