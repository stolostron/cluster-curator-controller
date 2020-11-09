/*


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
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ClusterCuratorSpec defines the desired state of ClusterCurator
type ClusterCuratorSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Desired action: create/import/destroy/detach/noop
	Action Action `json:"action,omitempty"`

	PreJobs []PreJob `json:"prejobspec"`

	PostJobs []PostJob `json:"postjobspec"`
}

type Action string

const (
	Create  Action = "create"
	Import  Action = "import"
	Destroy Action = "destroy"
	Detach  Action = "detach"
	NoOp    Action = "noop"
)

type PreJob struct {

	// If they put ansible we run our job
	Name string `json:"name,omitempty"`

	// ansible / image
	Type string `json:"type,omitempty"`

	// OPTIONAL required if NOT ansible
	Image string `json:"image,omitempty"`

	// This determines whether the job is executed
	RunOnAction Action `json:"runonaction,omitempty"`

	Values map[string]interface{} `json:"values,omitempty"`
}

type PostJob struct {
	Name string `json:"name,omitempty"`

	Type string `json:"type,omitempty"`

	Image string `json:"image,omitempty"`

	RunOnAction string `json:"runonaction,omitempty"`

	Values map[string]interface{} `json:"values,omitempty"`
}

type Job struct {
	Name   string                 `json:"name,omitempty"`  // "my custom job name" #If they put ansible we run our job
	Image  string                 `json:"image,omitempty"` // "" #OPTIONAL user override for future
	Values map[string]interface{} `json:"values,omitempty"`
}

// ClusterCuratorStatus defines the observed state of ClusterCurator
type ClusterCuratorStatus struct {
	Conditions []Conditions `json:"conditionStatus,omitempty"`
}

type Conditions struct {
	JobName         string `json:"jobname,omitempty"`         //ansible-job-123
	Type            string `json:"type,omitempty"`            //preJobCompleted
	ConditionStatus bool   `json:"conditionstatus,omitempty"` //False
	Message         string `json:"message,omitempty"`         //Creating AnsibleJob ANSIBLEJOB_RESOURCE_NAME / Running AnsibleJob ANSIBLEJOB_RESOURCE_NAME / Completed AnsibleJob ANSIBLEJOB_RESOURCE_NAME / Failed AnsibleJob ANSIBLEJOB_RESOURCE_NAME
	Reason          Reason `json:"reason,omitempty"`          //: Initializing / Running / Failed / Completed
}

type Reason string

const (
	Initializing Reason = "Initializing"
	Running      Reason = "Running"
	Failed       Reason = "Failed"
	Completed    Reason = "Completed"
)

type ClusterHealth struct {
	Status  Status
	Details map[string]interface{} `json:"clusterdetails,omitempty"`
}

type Status string

const (
	Ready        Status = "Ready"
	Provisioning Status = "Provisioning"
	Destroying   Status = "Destroying"
)

type ManagedClusterHealth struct {
	ManagedClusterStatus ManagedClusterStatus `json:"managedclusterstatus,omitempty"`

	Details map[string]interface{} `json:"managedclusterdetails,omitempty"`
}

type ManagedClusterStatus string

const (
	Ok ManagedClusterStatus = "OK"
	// (some addons down)
	Degraded  ManagedClusterStatus = "Degraded"
	Importing ManagedClusterStatus = "Importing"
	Detaching ManagedClusterStatus = "Detaching"
	Offline   ManagedClusterStatus = "Offline"
)

// +kubebuilder:object:root=true

// ClusterCurator is the Schema for the clustercurators API
type ClusterCurator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterCuratorSpec   `json:"spec,omitempty"`
	Status ClusterCuratorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterCuratorList contains a list of ClusterCurator
type ClusterCuratorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterCurator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterCurator{}, &ClusterCuratorList{})
}
