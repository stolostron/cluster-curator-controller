// +kubebuilder:object:generate=true
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ClusterCuratorSpec defines the desired state of ClusterCurator
type ClusterCuratorSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// This is the desired curation flow
	DesiredCuration string `json:"desiredCuration,omitempty"`

	// Points to the Cloud Provider or Ansible Provider secret
	ProviderCredentialPath string `json:"providerCredentialPath,omitempty"`

	// During an install curation run these Pre/Post hooks
	Install Hooks `json:"install,omitempty"`

	// During an scale curation run these Pre/Post hooks
	Scale Hooks `json:"scale,omitempty"`

	// During an destroy curation run these **Pre hook ONLY**
	Destroy Hooks `json:"destroy,omitempty"`

	// During an destroy curation run these **Pre hook ONLY**
	Upgrade Hooks `json:"destroy,omitempty"`

	// Kubernetes job resource created for curation of a cluster
	CuratingJob string `json:"curatorJob,omitempty"`
}

type Hook struct {
	// Name of the Ansible Template to run in Tower as a job
	Name string `json:"name"`

	// Ansible job extra_vars is passed to the Ansible job at execution time
	// and is a known Ansible entity.
	ExtraVars *runtime.RawExtension `json:"extra_vars,omitempty"`
}

type Hooks struct {
	Prehook  []Hook `json:"prehook,omitempty"`
	Posthook []Hook `json:"posthook,omitempty"`

	// When provided, this is a Job specification
	OverrideJob *runtime.RawExtension `json:"overrideJob,omitempty"`
}

// ClusterCuratorStatus defines the observed state of ClusterCurator
type ClusterCuratorStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

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
