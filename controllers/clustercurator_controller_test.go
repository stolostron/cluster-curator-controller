// Copyright Contributors to the Open Cluster Management project.

package controllers

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"

	clustercuratorv1 "github.com/stolostron/cluster-curator-controller/pkg/api/v1beta1"
	"github.com/stolostron/cluster-curator-controller/pkg/jobs/rbac"
)

const testClusterName = "hosted-cluster-1"
const testCuratorNamespace = "clusters"

func getHypershiftUpgradeDoneCurator() *clustercuratorv1.ClusterCurator {
	return &clustercuratorv1.ClusterCurator{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testClusterName,
			Namespace: testCuratorNamespace,
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			// DesiredCuration intentionally stays "upgrade" after completion (see
			// utils.NeedToUpgrade); CuratingJob is cleared once the job finishes.
			DesiredCuration: "upgrade",
			CuratingJob:     "",
			Upgrade: clustercuratorv1.UpgradeHooks{
				DesiredUpdate: "4.11.4",
			},
		},
		Status: clustercuratorv1.ClusterCuratorStatus{
			Conditions: []metav1.Condition{
				{
					Type:               "clustercurator-job",
					Status:             metav1.ConditionTrue,
					Reason:             "JobHasFinished",
					Message:            "curator-job-abc DesiredCuration: upgrade Version (4.11.4;;;;)",
					LastTransitionTime: metav1.Now(),
				},
			},
		},
	}
}

func newTestReconciler(t *testing.T, objs ...client.Object) (*ClusterCuratorReconciler, *fake.Clientset) {
	s := runtime.NewScheme()
	assert.Nil(t, clustercuratorv1.AddToScheme(s))

	builder := clientfake.NewClientBuilder().WithScheme(s)
	for _, o := range objs {
		builder = builder.WithObjects(o)
	}

	kubeset := fake.NewSimpleClientset()

	return &ClusterCuratorReconciler{
		Client:  builder.Build(),
		Kubeset: kubeset,
		Log:     logr.Discard(),
	}, kubeset
}

// TestReconcileCleansUpCuratorCRBAfterHypershiftUpgrade covers the de-scoped fix
// for the upgrade-cleanup gap: since DesiredCuration never clears to "" for
// upgrade, the CleanupRBAC/CleanupRBACHypershift block never fires, but the
// cluster-wide curator-crb ClusterRoleBinding must still be reclaimed so it
// doesn't keep granting cluster-wide managedclusters access once the upgrade
// job is no longer running.
func TestReconcileCleansUpCuratorCRBAfterHypershiftUpgrade(t *testing.T) {
	curator := getHypershiftUpgradeDoneCurator()
	r, kubeset := newTestReconciler(t, curator)

	// Simulate the RBAC left over from the upgrade curation that just finished.
	assert.Nil(t, rbac.ApplyRBAC(kubeset, testCuratorNamespace))
	assert.Nil(t, rbac.ApplyRBACHypershift(kubeset, testClusterName, testCuratorNamespace))

	crb, err := kubeset.RbacV1().ClusterRoleBindings().Get(context.TODO(), "curator-crb", metav1.GetOptions{})
	assert.Nil(t, err, "curator-crb should exist before reconcile")
	assert.Equal(t, testCuratorNamespace, crb.Subjects[0].Namespace)

	_, err = r.Reconcile(context.TODO(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: testClusterName, Namespace: testCuratorNamespace},
	})
	assert.Nil(t, err, "err nil on reconcile")

	t.Log("curator-crb should be deleted since it still belonged to this namespace")
	_, err = kubeset.RbacV1().ClusterRoleBindings().Get(context.TODO(), "curator-crb", metav1.GetOptions{})
	assert.NotNil(t, err, "curator-crb should be gone after reconcile")

	t.Log("Accepted residue: SA and namespaced RoleBindings remain (not cleaned up for upgrade)")
	_, err = kubeset.CoreV1().ServiceAccounts(testCuratorNamespace).Get(context.TODO(), "cluster-installer", metav1.GetOptions{})
	assert.Nil(t, err, "SA should still exist (accepted residue for upgrade)")
	_, err = kubeset.RbacV1().RoleBindings(testCuratorNamespace).Get(context.TODO(), "curator", metav1.GetOptions{})
	assert.Nil(t, err, "curatorNamespace RoleBinding should still exist (accepted residue for upgrade)")
	_, err = kubeset.RbacV1().RoleBindings(testClusterName).Get(context.TODO(), "curator", metav1.GetOptions{})
	assert.Nil(t, err, "cluster-namespace RoleBinding should still exist (accepted residue for upgrade)")
}

// TestReconcileCuratorCRBCleanupPreservesOtherNamespace ensures that reconciling
// a Hypershift curator whose upgrade just completed does not revoke a different,
// still-active Hypershift curation's claim on the shared curator-crb.
func TestReconcileCuratorCRBCleanupPreservesOtherNamespace(t *testing.T) {
	curator := getHypershiftUpgradeDoneCurator()
	r, kubeset := newTestReconciler(t, curator)

	assert.Nil(t, rbac.ApplyRBAC(kubeset, testCuratorNamespace))
	assert.Nil(t, rbac.ApplyRBACHypershift(kubeset, testClusterName, testCuratorNamespace))

	// A different Hypershift curation, in a different namespace, takes over
	// curator-crb before this one's upgrade-completion reconcile runs.
	otherNamespace := "other-clusters"
	assert.Nil(t, rbac.ApplyRBACHypershift(kubeset, "another-cluster", otherNamespace))

	_, err := r.Reconcile(context.TODO(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: testClusterName, Namespace: testCuratorNamespace},
	})
	assert.Nil(t, err, "err nil on reconcile")

	crb, err := kubeset.RbacV1().ClusterRoleBindings().Get(context.TODO(), "curator-crb", metav1.GetOptions{})
	assert.Nil(t, err, "curator-crb must be preserved since it now belongs to otherNamespace")
	assert.Equal(t, otherNamespace, crb.Subjects[0].Namespace,
		"curator-crb subject namespace must remain otherNamespace")
}

// TestClusterCuratorPredicateCuratingJobClearedIsAllowedThrough verifies the
// predicate no longer drops the event where CuratingJob transitions to "" while
// DesiredCuration is unchanged (the upgrade-completion signal).
func TestClusterCuratorPredicateCuratingJobClearedIsAllowedThrough(t *testing.T) {
	pred := newClusterCuratorPredicate()

	oldCurator := getHypershiftUpgradeDoneCurator()
	oldCurator.Spec.CuratingJob = "curator-job-abc"
	// Old object's status must match new object's status; the predicate drops
	// events where Status differs, and this test targets the Spec-only case.
	newCurator := oldCurator.DeepCopy()
	newCurator.Spec.CuratingJob = ""

	allowed := pred.Update(event.UpdateEvent{ObjectOld: oldCurator, ObjectNew: newCurator})
	assert.True(t, allowed, "reconcile should run when CuratingJob clears so curator-crb cleanup can happen")
}

// TestClusterCuratorPredicateDesiredCurationClearedIsAllowedThrough is a
// regression check for the earlier install/destroy completion fix, to make sure
// it still behaves correctly alongside the new CuratingJob change.
func TestClusterCuratorPredicateDesiredCurationClearedIsAllowedThrough(t *testing.T) {
	pred := newClusterCuratorPredicate()

	oldCurator := &clustercuratorv1.ClusterCurator{
		Spec: clustercuratorv1.ClusterCuratorSpec{DesiredCuration: "install", CuratingJob: ""},
	}
	newCurator := oldCurator.DeepCopy()
	newCurator.Spec.DesiredCuration = ""

	allowed := pred.Update(event.UpdateEvent{ObjectOld: oldCurator, ObjectNew: newCurator})
	assert.True(t, allowed, "reconcile should run when DesiredCuration clears so install/destroy cleanup can happen")
}

// TestClusterCuratorPredicateStatusOnlyChangeIsDropped is a regression check
// that unrelated status-only updates are still suppressed.
func TestClusterCuratorPredicateStatusOnlyChangeIsDropped(t *testing.T) {
	pred := newClusterCuratorPredicate()

	oldCurator := getHypershiftUpgradeDoneCurator()
	newCurator := oldCurator.DeepCopy()
	newCurator.Status.Conditions[0].Message = "a different message"

	allowed := pred.Update(event.UpdateEvent{ObjectOld: oldCurator, ObjectNew: newCurator})
	assert.False(t, allowed, "status-only changes must still be dropped")
}
