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

	// This is the desired curation that occurs. The supported options are 'install', 'upgrade', or 'destroy'.
	// +kubebuilder:validation:Enum={install,scale,upgrade,destroy,delete-cluster-namespace}
	DesiredCuration string `json:"desiredCuration,omitempty"`

	// Points to the Cloud Provider or Ansible Provider secret, format: namespace/secretName
	ProviderCredentialPath string `json:"providerCredentialPath,omitempty"`

	// An install curation runs these prehooks and posthooks.
	Install Hooks `json:"install,omitempty"`

	// A scale curation run these prehooks and posthooks.
	Scale Hooks `json:"scale,omitempty"`

	// A destroy curation runs these hooks.
	// Standalone clusters only support the prehook.
	// Hosted clusters support both prehook and posthook.
	Destroy Hooks `json:"destroy,omitempty"`

	// An upgrade curation runs these hooks.
	// +kubebuilder:validation:XValidation:rule="!(has(oldSelf.intermediateUpdate) && self.intermediateUpdate != oldSelf.intermediateUpdate)",message="The intermediateUpdate cannot be modified"
	// +kubebuilder:validation:XValidation:rule="!(has(oldSelf.intermediateUpdate) && oldSelf.intermediateUpdate != '' && has(oldSelf.desiredUpdate) && self.desiredUpdate != oldSelf.desiredUpdate)",message="The desiredUpdate cannot be modified when intermediateUpdate exists"
	// +kubebuilder:validation:XValidation:rule="!has(self.intermediateUpdate) || (has(self.desiredUpdate) && self.desiredUpdate != '')",message="The intermediateUpdate cannot be created if desiredUpdate is missing or empty"
	// +kubebuilder:validation:XValidation:rule="!(has(self.intermediateUpdate) && !has(oldSelf.intermediateUpdate) && has(oldSelf.desiredUpdate) && oldSelf.desiredUpdate != '')",message="The intermediateUpdate cannot be added via update if desiredUpdate already exists"
	Upgrade UpgradeHooks `json:"upgrade,omitempty"`

	// Kubernetes job resource created for curation of a cluster.
	CuratingJob string `json:"curatorJob,omitempty"`

	// Inventory values are supplied for use with the pre/post jobs.
	Inventory string `json:"inventory,omitempty"`
}

type Hook struct {
	// Name of the Ansible Template to run in the Ansible Tower as a job.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Type of the Hook. For Job type, Ansible job template is used.
	// For Workflow type, Ansible workflow template is used.
	// If omitted, it defaults to the Job type.
	// +optional
	// +kubebuilder:default=Job
	Type HookType `json:"type,omitempty"`

	// Ansible job extra_vars is passed to the Ansible job at execution time
	// and is a known Ansible entity.
	// +kubebuilder:pruning:PreserveUnknownFields
	ExtraVars *runtime.RawExtension `json:"extra_vars,omitempty"`

	// A comma-separated list of tags to specify which sets
	// of Ansible tasks in a job should be run.
	// +optional
	JobTags string `json:"job_tags,omitempty"`

	// A comma-separated list of tags to specify which sets
	// of Ansible tasks in a job should not be run.
	// +optional
	SkipTags string `json:"skip_tags,omitempty"`
}

type Hooks struct {

	// TowerAuthSecret is an Ansible secret used in the template to provide authentication to an Ansbile tower.
	// +kubebuilder:validation:Required
	TowerAuthSecret string `json:"towerAuthSecret,omitempty"`

	// Jobs to run before the cluster deployment.
	Prehook []Hook `json:"prehook,omitempty"`

	// Jobs to run after the cluster import.
	Posthook []Hook `json:"posthook,omitempty"`

	// When provided, this is a Job specification and overrides the default flow.
	// +kubebuilder:pruning:PreserveUnknownFields
	OverrideJob *runtime.RawExtension `json:"overrideJob,omitempty"`

	// JobMonitorTimeout defines the timeout for finding a job and defines time in minutes.
	// If the job is found, the curator controller waits until the job becomes active.
	// By default, it is 5 minutes.
	// If its value is less than or equal to zero, the default is used.
	// +optional
	// +kubebuilder:default=5
	JobMonitorTimeout int `json:"jobMonitorTimeout,omitempty"`
}

type UpgradeHooks struct {

	// TowerAuthSecret is an Ansible secret used in the template to provide authentication to an Ansbile tower.
	// +kubebuilder:validation:Required
	TowerAuthSecret string `json:"towerAuthSecret,omitempty"`

	// IntermediateUpdate indicates the desired value of
	// the intermediate cluster version when performing EUS to EUS upgrades.
	// Setting both this value and DesiredUpdate triggers an EUS to EUS upgrade.
	// +kubebuilder:validation:Optional
	IntermediateUpdate string `json:"intermediateUpdate,omitempty"`

	// DesiredUpdate indicates the desired value of
	// the cluster version. Setting this value triggers an upgrade (if
	// the current version does not match the desired version). During
	// an EUS to EUS upgrade, this value becomes the final cluster version
	// (the target version that ClusterCurator upgrades the cluster to).
	// +optional
	DesiredUpdate string `json:"desiredUpdate,omitempty"`

	// Channel is an identifier for explicitly requesting that a non-default
	// set of updates be applied to this cluster. The default channel
	// contains stable updates that are appropriate for production clusters.
	// +optional
	Channel string `json:"channel,omitempty"`

	// Upstream may be used to specify the preferred update server. By default
	// it uses the appropriate update server for the cluster and region.
	// +optional
	Upstream string `json:"upstream,omitempty"`

	// Jobs to run before the cluster upgrade.
	Prehook []Hook `json:"prehook,omitempty"`

	// Jobs to run after the cluster upgrade.
	Posthook []Hook `json:"posthook,omitempty"`

	// When provided, this is a Job specification and overrides the default flow.
	// +kubebuilder:pruning:PreserveUnknownFields
	OverrideJob *runtime.RawExtension `json:"overrideJob,omitempty"`

	// MonitorTimeout defines the monitor process timeout, and defines time in minutes.
	// By default, it is 120 minutes.
	// If its value is less than or equal to zero, the default value is used.
	// +optional
	// +kubebuilder:default=120
	MonitorTimeout int `json:"monitorTimeout,omitempty"`
}

// ClusterCuratorStatus defines the observed state of ClusterCurator work.
type ClusterCuratorStatus struct {
	// Track the conditions for each step in the desired curation that is being
	// executed as a job.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// HookType indicates the type for the hook. It can be 'Job' or 'Workflow'
// +kubebuilder:validation:Enum=Job;Workflow
type HookType string

const (
	// HookTypeJob, the hook is an Ansible Job template
	HookTypeJob HookType = "Job"

	// HookTypeWorkflow, the hook is an Ansible Workflow template
	HookTypeWorkflow HookType = "Workflow"
)

// +kubebuilder:object:root=true

// Operation contains information about a requested or running operation
type Operation struct {
	// Option for retrying a failed posthook job. The supported options are 'installPosthook' or 'upgradePosthook'.
	// +kubebuilder:validation:Enum={installPosthook,upgradePosthook}
	RetryPosthook string `json:"retryPosthook,omitempty"`
}

// ClusterCurator is the custom resource for the clustercurators API.
// This kind allows you to run Ansible prehook and posthook jobs before provisioning a Hive or HyperShift cluster
// and importing a cluster. Additionally, cluster upgrade and destroy operations are supported as well.
type ClusterCurator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec      ClusterCuratorSpec   `json:"spec,omitempty"`
	Status    ClusterCuratorStatus `json:"status,omitempty"`
	Operation *Operation           `json:"operation,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterCuratorList contains a list of ClusterCurator resources.
type ClusterCuratorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterCurator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterCurator{}, &ClusterCuratorList{})
}
