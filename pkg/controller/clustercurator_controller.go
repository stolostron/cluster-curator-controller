// Copyright (c) 2020 Red Hat, Inc.

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterCuratorReconciler reconciles a ClusterCurator object
type ClusterCuratorReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=clustercurator.open-cluster-management.io,resources=clustercurators,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=clustercurator.open-cluster-management.io,resources=clustercurators/status,verbs=get;update;patch

func (r *ClusterCuratorReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("clustercurator", req.NamespacedName)

	// your logic here

	return ctrl.Result{}, nil
}

// func (r *ClusterCuratorReconciler) SetupWithManager(mgr ctrl.Manager) error {
// 	return ctrl.NewControllerManagedBy(mgr).
// 		For(clustercuratorv1.ClusterCurator{}).
// 		Complete(r)
// }
