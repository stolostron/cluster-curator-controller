// Copyright Contributors to the Open Cluster Management project.

package controllers

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/meta"
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
	"github.com/stolostron/cluster-curator-controller/pkg/jobs/utils"
	managedclusterinfov1beta1 "github.com/stolostron/cluster-lifecycle-api/clusterinfo/v1beta1"
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
	fakeClient := builder.Build()

	return &ClusterCuratorReconciler{
		Client:    fakeClient,
		APIReader: fakeClient,
		Kubeset:   kubeset,
		Log:       logr.Discard(),
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

// TestClusterCuratorPredicateDesiredCurationClearedWithStatusIsAllowedThrough
// reproduces the real completion event, not the artificial spec-only case
// covered above: ClusterCurator has no status subresource, so
// updateDoneClusterCurator's merge patch clears spec.desiredCuration,
// spec.curatorJob, AND status in the very same API call. That must still be
// allowed through so post-curation RBAC cleanup can run; a naive
// status-equality check placed ahead of the DesiredCuration/CuratingJob
// checks would always drop this event, since Status always differs here.
func TestClusterCuratorPredicateDesiredCurationClearedWithStatusIsAllowedThrough(t *testing.T) {
	pred := newClusterCuratorPredicate()

	oldCurator := &clustercuratorv1.ClusterCurator{
		Spec: clustercuratorv1.ClusterCuratorSpec{DesiredCuration: "install", CuratingJob: "curator-job-abc"},
		Status: clustercuratorv1.ClusterCuratorStatus{
			Conditions: []metav1.Condition{
				{
					Type:               "clustercurator-job",
					Status:             metav1.ConditionTrue,
					Reason:             "Job_has_finished",
					Message:            "curator-job-abc DesiredCuration: install",
					LastTransitionTime: metav1.Now(),
				},
			},
		},
	}
	newCurator := oldCurator.DeepCopy()
	newCurator.Spec.DesiredCuration = ""
	newCurator.Spec.CuratingJob = ""
	newCurator.Status = clustercuratorv1.ClusterCuratorStatus{}

	allowed := pred.Update(event.UpdateEvent{ObjectOld: oldCurator, ObjectNew: newCurator})
	assert.True(t, allowed,
		"reconcile should run when DesiredCuration/CuratingJob clear even though Status changed in the same patch")
}

func newUpgradeTestReconciler(t *testing.T, objs ...client.Object) (*ClusterCuratorReconciler, *fake.Clientset) {
	s := runtime.NewScheme()
	assert.Nil(t, clustercuratorv1.AddToScheme(s))
	assert.Nil(t, managedclusterinfov1beta1.AddToScheme(s))

	fakeClient := clientfake.NewClientBuilder().WithScheme(s).WithObjects(objs...).Build()
	kubeset := fake.NewSimpleClientset()

	return &ClusterCuratorReconciler{
		Client:    fakeClient,
		APIReader: fakeClient,
		Kubeset:   kubeset,
		Log:       logr.Discard(),
	}, kubeset
}

func getFirstUpgradeCurator(name string) *clustercuratorv1.ClusterCurator {
	return &clustercuratorv1.ClusterCurator{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: name},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "upgrade",
			Upgrade:         clustercuratorv1.UpgradeHooks{DesiredUpdate: "4.14.5"},
		},
	}
}

// A skipped upgrade used to leave no trace on the ClusterCurator: no job, no condition, and the
// reason only in controller logs at -v=2. The skip reason must now be recorded on the resource.
func TestReconcileRecordsUpgradeSkippedCondition(t *testing.T) {
	const name = "managed-1"
	r, kubeset := newUpgradeTestReconciler(t, getFirstUpgradeCurator(name))

	_, err := r.Reconcile(context.TODO(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: name, Namespace: name},
	})
	assert.Nil(t, err)

	stored := &clustercuratorv1.ClusterCurator{}
	assert.Nil(t, r.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: name}, stored))
	cond := meta.FindStatusCondition(stored.Status.Conditions, utils.UpgradeSkippedCondition)
	if assert.NotNil(t, cond, "skip reason must be visible on the ClusterCurator") {
		assert.Equal(t, metav1.ConditionTrue, cond.Status)
		assert.Equal(t, "ManagedClusterInfoNotFound", cond.Reason)
	}

	jobs, err := kubeset.BatchV1().Jobs(name).List(context.TODO(), metav1.ListOptions{})
	assert.Nil(t, err)
	assert.Empty(t, jobs.Items, "no curator job may be launched for a skipped upgrade")
}

// Once the upgrade can proceed, a stale skip condition must be cleared so the resource no longer
// claims the upgrade was skipped while a job is running.
func TestReconcileClearsUpgradeSkippedConditionWhenUpgradeProceeds(t *testing.T) {
	const name = "managed-1"
	curator := getFirstUpgradeCurator(name)
	curator.Status.Conditions = []metav1.Condition{{
		Type:               utils.UpgradeSkippedCondition,
		Status:             metav1.ConditionTrue,
		Reason:             "AlreadyAtDesiredVersion",
		Message:            "Cluster managed-1 is already at desired version 4.14.3, skipping upgrade",
		LastTransitionTime: metav1.Now(),
	}}
	mci := &managedclusterinfov1beta1.ManagedClusterInfo{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: name},
		Status: managedclusterinfov1beta1.ClusterInfoStatus{
			DistributionInfo: managedclusterinfov1beta1.DistributionInfo{
				OCP: managedclusterinfov1beta1.OCPDistributionInfo{Version: "4.14.3"},
			},
		},
	}
	r, kubeset := newUpgradeTestReconciler(t, curator, mci)

	_, err := r.Reconcile(context.TODO(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: name, Namespace: name},
	})
	assert.Nil(t, err)

	stored := &clustercuratorv1.ClusterCurator{}
	assert.Nil(t, r.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: name}, stored))
	assert.Nil(t, meta.FindStatusCondition(stored.Status.Conditions, utils.UpgradeSkippedCondition),
		"stale skip condition must be removed once the upgrade proceeds")

	jobs, err := kubeset.BatchV1().Jobs(name).List(context.TODO(), metav1.ListOptions{})
	assert.Nil(t, err)
	assert.Len(t, jobs.Items, 1, "the upgrade job must be launched")
}
