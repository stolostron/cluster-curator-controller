// Copyright Contributors to the Open Cluster Management project.

package controllers

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	clientsetx "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	clustercuratorv1 "github.com/stolostron/cluster-curator-controller/pkg/api/v1beta1"
	"github.com/stolostron/cluster-curator-controller/pkg/controller/launcher"
	"github.com/stolostron/cluster-curator-controller/pkg/jobs/rbac"
	"github.com/stolostron/cluster-curator-controller/pkg/jobs/utils"
)

const (
	DeleteNamespace                  = "delete-cluster-namespace"
	statusTypeInitiated              = "Initiated"
	statusReasonWorkflowSupported    = "WorkflowSupported"
	statusReasonWorkflowNotSupported = "WorkflowNotSupported"
)

// ClusterCuratorReconciler reconciles a ClusterCurator object
type ClusterCuratorReconciler struct {
	client.Client
	Kubeset  kubernetes.Interface
	KubeXset clientsetx.Interface
	Log      logr.Logger
	Scheme   *runtime.Scheme
	ImageURI string
}

// +kubebuilder:rbac:groups=cluster.open-cluster-management.io.cluster.open-cluster-management.io,resources=clustercurators,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.open-cluster-management.io.cluster.open-cluster-management.io,resources=clustercurators/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get

func (r *ClusterCuratorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	//ctx := context.Background()
	log := r.Log.WithValues("clustercurator", req.NamespacedName)

	var curator clustercuratorv1.ClusterCurator
	if err := r.Get(ctx, req.NamespacedName, &curator); err != nil {
		log.V(2).Info("Resource deleted")
		return ctrl.Result{}, nil
	}

	// If the ClusterCurator contains at least one Workflow hook type then
	// check the AnsibleJob CRD to make sure it can support Workflow invocation.
	if containsWorkflowType(curator) {
		err := r.checkWorkflowSupport(ctx, curator)
		if err != nil {
			meta.SetStatusCondition(&curator.Status.Conditions, metav1.Condition{
				Type:    statusTypeInitiated,
				Status:  metav1.ConditionFalse,
				Reason:  statusReasonWorkflowNotSupported,
				Message: "The AnsibleJob CRD does not support Workflow invocation"})
			if errStatusUpdate := r.Client.Update(ctx, &curator); errStatusUpdate != nil {
				return ctrl.Result{}, errStatusUpdate
			}
			log.Error(err, "The AnsibleJob CRD does not support Workflow invocation.")
			return ctrl.Result{Requeue: true, RequeueAfter: 1 * time.Minute}, nil
		}

		if !meta.IsStatusConditionPresentAndEqual(curator.Status.Conditions, statusTypeInitiated, metav1.ConditionTrue) {
			meta.SetStatusCondition(&curator.Status.Conditions, metav1.Condition{
				Type:    statusTypeInitiated,
				Status:  metav1.ConditionTrue,
				Reason:  statusReasonWorkflowSupported,
				Message: "The AnsibleJob CRD supports Workflow invocation"})
			if err := r.Client.Update(ctx, &curator); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	if curator.Spec.DesiredCuration == DeleteNamespace {
		log.V(0).Info("Deleting namespace " + curator.Namespace)
		err := utils.DeleteClusterNamespace(r.Kubeset, curator.Namespace)

		if err != nil {
			log.V(0).Info(" Deleted namespace ✓ " + curator.Namespace)
		}
		return ctrl.Result{}, err
	}

	log.V(3).Info("Reconcile: %v, DesiredCuration: %v, Previous CuratingJob: %v",
		req.NamespacedName, curator.Spec.DesiredCuration, curator.Spec.CuratingJob)

	// Curating work has already started OR no curation work supplied curator.Spec.CuratingJob != "" ||
	if curator.Spec.CuratingJob != "" || curator.Spec.DesiredCuration == "" {
		log.V(3).Info("No curation to do for %v", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	// Curation flow begins here
	// Apply RBAC required by the curation job
	err := rbac.ApplyRBAC(r.Kubeset, req.Namespace)
	if err := utils.LogError(err); err != nil {
		return ctrl.Result{}, err
	}

	// Launch the curation job
	jobLaunch := launcher.NewLauncher(r.Client, r.Kubeset, r.ImageURI, curator)
	if err := utils.LogError(jobLaunch.CreateJob()); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ClusterCuratorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clustercuratorv1.ClusterCurator{}).
		WithEventFilter(newClusterCuratorPredicate()).
		Complete(r)
}

func newClusterCuratorPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			newClusterCurator, okNew := e.ObjectNew.(*clustercuratorv1.ClusterCurator)
			oldClusterCurator, okOld := e.ObjectOld.(*clustercuratorv1.ClusterCurator)
			if okNew && okOld {
				if !reflect.DeepEqual(newClusterCurator.Status, oldClusterCurator.Status) {
					return false
				}
				if newClusterCurator.Spec.DesiredCuration == DeleteNamespace {
					return true
				}
				if newClusterCurator.Spec.DesiredCuration != oldClusterCurator.Spec.DesiredCuration && newClusterCurator.Spec.DesiredCuration == "" {
					return false
				}
				if newClusterCurator.Spec.CuratingJob != oldClusterCurator.Spec.CuratingJob && newClusterCurator.Spec.CuratingJob == "" {
					return false
				}
			}
			return true
		},
	}
}

// checkWorkflowSupport checks if the curator contains Workflow type
// If it does then check against the AnsibleJob CRD to make sure it contains
// field(s) that support Workflow inovcation.
// Return error if no Workflow support, nil otherwise.
func (r *ClusterCuratorReconciler) checkWorkflowSupport(ctx context.Context,
	curator clustercuratorv1.ClusterCurator) error {
	if !containsWorkflowType(curator) {
		return nil
	}

	ansibleJobCRD, err := r.KubeXset.ApiextensionsV1().CustomResourceDefinitions().Get(ctx,
		"ansiblejobs.tower.ansible.com", metav1.GetOptions{})
	if err != nil {
		return err
	}
	if ansibleJobCRD == nil {
		return errors.New("unable to get AnsibleJob CRD")
	}

	for _, version := range ansibleJobCRD.Spec.Versions {
		if version.Schema != nil && version.Schema.OpenAPIV3Schema != nil &&
			version.Schema.OpenAPIV3Schema.Properties != nil {
			ansibleJobSpec := version.Schema.OpenAPIV3Schema.Properties["spec"]
			if ansibleJobSpec.Properties != nil {
				_, ok := ansibleJobSpec.Properties["workflow_template_name"]
				if ok {
					return nil
				}
			}
		}
	}

	return errors.New("the AnsibleJob CRD does not support Workflow invocation")
}

// containsWorkflowType returns true if there are any Workflow type hook, false otherwise.
func containsWorkflowType(curator clustercuratorv1.ClusterCurator) bool {
	spec := curator.Spec

	return func(hookSlices ...[]clustercuratorv1.Hook) bool {
		for _, hookSlice := range hookSlices {
			for _, hook := range hookSlice {
				if hook.Type == clustercuratorv1.HookTypeWorkflow {
					return true
				}
			}
		}

		return false
	}(spec.Install.Prehook, spec.Install.Posthook,
		spec.Upgrade.Prehook, spec.Upgrade.Posthook,
		spec.Scale.Prehook, spec.Scale.Posthook,
		spec.Destroy.Prehook, spec.Destroy.Posthook)
}
