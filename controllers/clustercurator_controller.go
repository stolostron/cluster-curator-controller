// Copyright Contributors to the Open Cluster Management project.

package controllers

import (
	"context"
	"reflect"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	clustercuratorv1 "github.com/open-cluster-management/cluster-curator-controller/pkg/api/v1beta1"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/controller/launcher"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/rbac"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/utils"
)

const DeleteNamespace = "delete-cluster-namespace"

// ClusterCuratorReconciler reconciles a ClusterCurator object
type ClusterCuratorReconciler struct {
	client.Client
	Kubeset  kubernetes.Interface
	Log      logr.Logger
	Scheme   *runtime.Scheme
	ImageURI string
}

// +kubebuilder:rbac:groups=cluster.open-cluster-management.io.cluster.open-cluster-management.io,resources=clustercurators,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.open-cluster-management.io.cluster.open-cluster-management.io,resources=clustercurators/status,verbs=get;update;patch

func (r *ClusterCuratorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	//ctx := context.Background()
	log := r.Log.WithValues("clustercurator", req.NamespacedName)

	var curator clustercuratorv1.ClusterCurator
	if err := r.Get(ctx, req.NamespacedName, &curator); err != nil {
		log.V(2).Info("Resource deleted")
		return ctrl.Result{}, nil
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
