// +kubebuilder:object:generate=true
package v1beta1

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

	// This is the desired curation that will occur
	// +kubebuilder:validation:Enum={install,scale,upgrade,destroy}
	DesiredCuration string `json:"desiredCuration,omitempty"`

	// Points to the Cloud Provider or Ansible Provider secret, format: namespace/secretName
	ProviderCredentialPath string `json:"providerCredentialPath,omitempty"`

	// During an install curation run these Pre/Post hooks
	Install Hooks `json:"install,omitempty"`

	// During an scale curation run these Pre/Post hooks
	Scale Hooks `json:"scale,omitempty"`

	// During an destroy curation run these **Pre hook ONLY**
	Destroy Hooks `json:"destroy,omitempty"`

	// During an upgrade curation run these
	Upgrade UpgradeHooks `json:"upgrade,omitempty"`

	// Kubernetes job resource created for curation of a cluster
	CuratingJob string `json:"curatorJob,omitempty"`
}

type Hook struct {
	// Name of the Ansible Template to run in Tower as a job
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Ansible job extra_vars is passed to the Ansible job at execution time
	// and is a known Ansible entity.
	// +kubebuilder:pruning:PreserveUnknownFields
	ExtraVars *runtime.RawExtension `json:"extra_vars,omitempty"`
}

type Hooks struct {
	// Jobs to run before the cluster deployment
	Prehook []Hook `json:"prehook,omitempty"`

	// Jobs to run after the cluster import
	Posthook []Hook `json:"posthook,omitempty"`

	// When provided, this is a Job specification and overrides the default flow
	// +kubebuilder:pruning:PreserveUnknownFields
	OverrideJob *runtime.RawExtension `json:"overrideJob,omitempty"`
}

type UpgradeHooks struct {

	// DesiredUpdate indicates the desired value of
	// the cluster version. Setting this value will trigger an upgrade (if
	// the current version does not match the desired version).
	// +optional
	DesiredUpdate string `json:"desiredUpdate,omitempty"`

	// Channel is an identifier for explicitly requesting that a non-default
	// set of updates be applied to this cluster. The default channel will be
	// contain stable updates that are appropriate for production clusters.
	// +optional
	Channel string `json:"channel,omitempty"`

	// Upstream may be used to specify the preferred update server. By default
	// it will use the appropriate update server for the cluster and region.
	// +optional
	Upstream string `json:"upstream,omitempty"`

	Hooks Hooks `json:"hooks,omitempty"`
}

// ClusterCuratorStatus defines the observed state of ClusterCurator work
type ClusterCuratorStatus struct {
	// Track the conditions for each step in the desired curation that is being
	// executed as a job
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterCurator is the Schema for the clustercurators API
// This kind allows for prehook and posthook jobs to be executed prior to Hive provisioning
// and import of a cluster.
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
