// Copyright (c) 2020 Red Hat, Inc.
package v1alpha1

import (
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterCurator is the Custom Resource object which holds the desired state and current status
// of an cluster. ClusterCurator is a namespace scoped resource
type ClusterCurator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	// spec holds desired configuration to create the job
	// +kubebuilder:validation:Required
	// +required
	Spec ClusterCuratorSpec `json:"spec"`

	// status holds the information about the state of a cluster curator resource, managed cluster heealth and cluster health.  It is consistent with status information across
	// the Kubernetes ecosystem.
	// +optional
	Status ClusterCuratorStatus `json:"status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterCuratorSpec defines the desired configuration to ceate a job
type ClusterCuratorSpec struct {

	// Desired action: create/import/destroy/detach/noop
	// action holds the information about create/import/destroy/detach/noop one of the action job should perform
	// +required
	Action Action `json:"action,omitempty"`

	// PreJobs holds the job specifications
	// +optional
	PreJobs JobSpec `json:"preJobs,omitempty"`

	// PostJobs holds the job specifications
	// +optional
	PostJobs JobSpec `json:"postJobs,omitempty"`

	// Jobs holds the job specifications
	// +optional
	Job JobSpec `json:"job,omitempty"`
}

// Action holds the information about create/import/destroy/detach/noop one of the action job should perform
type Action string

const (
	// Create indicates create cluster using hive
	Create Action = "create"

	// Import indicates import an existing cluster
	Import Action = "import"

	// Destroy indicates delete and destroy of cluster created by hive
	Destroy Action = "destroy"

	// Detach indiactes detach of cluster from hub
	Detach Action = "detach"

	// Noop is for migration
	Noop Action = "noop"
)

// JobSpec holds desired configuration to create the job
// +kubebuilder:validation:Required
// +optional
// +union
type JobSpec struct {

	// name represents the name of the job
	// +required
	Name string `json:"name,omitempty"`

	// type represents type of job
	// +required
	Type string `json:"type,omitempty"`

	// image represents image which will be used to run a job
	// OPTIONAL required if NOT ansible
	// +optional
	Image string `json:"image,omitempty"`

	// RunOnAction represents on which actoin this job should be executed
	// action are create/import-only/destroy/detach
	// +required
	RunOnAction Action `json:"runOnAction,omitempty"`

	// Values represents necassary fields required to exceute the job
	// +kubebuilder:validation:Optional
	Values apiextensions.JSON `json:"values,omitempty"`
}

// ClusterCuratorStatus defines the observed state of ClusterCurator
type ClusterCuratorStatus struct {
	// conditions describe the state of cluster curator resource
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	Conditions []Conditions `json:"conditions,omitempty"`

	//ClusterHealth ClusterStatus `json:"clusterhealth"`

	//ManagedClusterHealth
}

// Conditions contains details for one aspect of the current state of this API Resource.
type Conditions struct {
	// name represents the name of the job
	// +required
	JobName string `json:"jobname,omitempty"` //ansible-job-123
	// conditions describe the state of job resource
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	Condition metav1.Condition `json:"condition,omitempty"  patchStrategy:"merge" patchMergeKey:"type"`
}

// ClusterStatus represents state of cluster created using hive
// +optional
// type ClusterStatus struct {
// 	// Status  Status
// 	// Details map[string]interface{} `json:"clusterdetails,omitempty"`
// }

// type Status string

// const (
// 	Ready        Status = "Ready"
// 	Provisioning Status = "Provisioning"
// 	Destroying   Status = "Destroying"
// )

// type ManagedClusterHealth struct {
// 	ManagedClusterStatus ManagedClusterStatus `json:"managedclusterstatus,omitempty"`

// 	Details map[string]interface{} `json:"managedclusterdetails,omitempty"`
// }

// type ManagedClusterStatus string

// const (
// 	Ok ManagedClusterStatus = "OK"
// 	// (some addons down)
// 	Degraded  ManagedClusterStatus = "Degraded"
// 	Importing ManagedClusterStatus = "Importing"
// 	Detaching ManagedClusterStatus = "Detaching"
// 	Offline   ManagedClusterStatus = "Offline"
// )

// ClusterCuratorList contains a list of ClusterCurator
type ClusterCuratorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterCurator `json:"items"`
}

// func init() {
// 	SchemeBuilder.Register(&ClusterCurator{}, &ClusterCuratorList{})
// }
