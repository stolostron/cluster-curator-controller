// Copyright Contributors to the Open Cluster Management project.
package rbac

import (
	"context"
	"testing"
	"time"

	"github.com/stolostron/cluster-curator-controller/pkg/jobs/utils"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const ClusterName = "my-cluster"
const ClusterNamespace = "clusters"

func getRules(clusterName string) []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		rbacv1.PolicyRule{
			APIGroups: []string{"tower.ansible.com", ""},
			Resources: []string{"ansiblejobs", "secrets", "serviceaccounts"},
			Verbs:     []string{"create"},
		},
		rbacv1.PolicyRule{
			APIGroups: []string{"hive.openshift.io"},
			Resources: []string{"clusterdeployments"},
			Verbs:     []string{"patch", "delete", "update"},
		},
		rbacv1.PolicyRule{
			APIGroups: []string{"hypershift.openshift.io"},
			Resources: []string{"hostedclusters", "nodepools"},
			Verbs:     []string{"get", "patch", "delete", "update", "list"},
		},
		rbacv1.PolicyRule{
			APIGroups: []string{"batch", "hive.openshift.io", "tower.ansible.com"},
			Resources: []string{"jobs", "clusterdeployments", "ansiblejobs", "machinepools"},
			Verbs:     []string{"get"},
		},
		rbacv1.PolicyRule{
			APIGroups: []string{""},
			Resources: []string{"configmaps"},
			Verbs:     []string{"update", "get", "patch"},
		},
		rbacv1.PolicyRule{
			APIGroups: []string{"internal.open-cluster-management.io"},
			Resources: []string{"managedclusterinfos"},
			Verbs:     []string{"get"},
		},
		rbacv1.PolicyRule{
			APIGroups: []string{"cluster.open-cluster-management.io"},
			Resources: []string{"clustercurators"},
			Verbs:     []string{"get", "update", "patch", "delete"},
		},
		rbacv1.PolicyRule{
			APIGroups: []string{"view.open-cluster-management.io"},
			Resources: []string{"managedclusterviews"},
			Verbs:     []string{"get", "create", "update", "delete"},
		},
		rbacv1.PolicyRule{
			APIGroups: []string{"action.open-cluster-management.io"},
			Resources: []string{"managedclusteractions"},
			Verbs:     []string{"get", "create", "update", "delete"},
		},
		// To read the install-config secret
		rbacv1.PolicyRule{
			APIGroups: []string{""},
			Resources: []string{"secrets"},
			Verbs:     []string{"get"},
		},
	}
}

func getCombinedCIRules() []rbacv1.PolicyRule {
	return append(getRules(ClusterName),
		[]rbacv1.PolicyRule{
			rbacv1.PolicyRule{
				APIGroups: []string{"tower.ansible.com"},
				Resources: []string{"ansiblejobs"},
				Verbs:     []string{"create", "get"},
			},
			rbacv1.PolicyRule{
				APIGroups: []string{"batch"},
				Resources: []string{"jobs"},
				Verbs:     []string{"get"},
			},
			rbacv1.PolicyRule{
				APIGroups: []string{"hive.openshift.io"},
				Resources: []string{"clusterdeployments"},
				Verbs:     []string{"patch", "update"},
			},
			rbacv1.PolicyRule{
				APIGroups: []string{"internal.open-cluster-management.io"},
				Resources: []string{"managedclusterinfos"},
				Verbs:     []string{"get"},
			},
			rbacv1.PolicyRule{
				APIGroups: []string{"cluster.open-cluster-management.io"},
				Resources: []string{"clustercurators"},
				Verbs:     []string{"get", "update", "patch"},
			},
		}...)
}
func TestApplyRbac(t *testing.T) {

	subjects := []rbacv1.Subject{
		rbacv1.Subject{
			Kind:      "ServiceAccount",
			Name:      clusterInstaller,
			Namespace: ClusterName,
		},
	}

	roleRef := rbacv1.RoleRef{
		Kind:     "ClusterRole",
		Name:     "curator",
		APIGroup: "rbac.authorization.k8s.io",
	}

	kubeset := fake.NewSimpleClientset()

	err := ApplyRBAC(kubeset, ClusterName)
	assert.Nil(t, err, "err nil, when Roles and RoleBindings are created")

	t.Log("Validate ServiceaAccount")

	_, err = kubeset.CoreV1().ServiceAccounts(ClusterName).Get(context.TODO(), clusterInstaller, v1.GetOptions{})
	assert.Nil(t, err, "err nil, when service account exists")

	t.Log("Validate Role")

	role, err := kubeset.RbacV1().ClusterRoles().Get(context.TODO(), "curator", v1.GetOptions{})

	assert.Nil(t, err, "err nil, when Role exists")
	assert.ElementsMatch(t, getRules(ClusterName), role.Rules, "The rules should match")

	t.Log("Validate RoleBinding")
	roleBinding, err := kubeset.RbacV1().RoleBindings(ClusterName).
		Get(context.TODO(), "curator", v1.GetOptions{})

	assert.Nil(t, err, "err nil, when RoleBinding created")

	assert.Conditionf(t, func() bool {
		if roleRef.Kind == roleBinding.RoleRef.Kind &&
			roleRef.Name == roleBinding.RoleRef.Name &&
			roleRef.APIGroup == roleBinding.RoleRef.APIGroup {
			return true
		}
		return false
	}, "roleRef must match,\nExpected: %v\nFound: %v", &roleRef, &roleBinding.RoleRef)
	assert.ElementsMatch(t, subjects, roleBinding.Subjects, "subjects must match")
}

func TestExtendClusterInstallerRole(t *testing.T) {

	kubeset := fake.NewSimpleClientset()

	testRole := getRole(ClusterName)
	testRole.Rules = getRules(ClusterName)
	testRole.Name = clusterInstaller

	// Delay 5s so that we can see the ExtendClusterInstallerRole wait
	go func() {
		time.Sleep(utils.PauseFiveSeconds)
		_, err := kubeset.RbacV1().Roles(ClusterName).Create(context.TODO(), testRole, v1.CreateOptions{})
		assert.Nil(t, err, "err is nil, when cluster-installer role is created")
	}()

	err := ExtendClusterInstallerRole(kubeset, ClusterName)
	assert.Nil(t, err, "err is nil, when cluster-installer role is extended")

	role, err := kubeset.RbacV1().Roles(ClusterName).Get(context.TODO(), clusterInstaller, v1.GetOptions{})
	assert.Nil(t, err, "err is nil when role is found")

	assert.ElementsMatch(t, role.Rules, getCombinedCIRules(), "Rules should be equal")
}

func TestExtendClusterInstallerRoleTimeout(t *testing.T) {

	kubeset := fake.NewSimpleClientset()

	testRole := getRole(ClusterName)
	testRole.Name = clusterInstaller

	err := ExtendClusterInstallerRole(kubeset, ClusterName)

	assert.NotNil(t, err, "err not nil, when failure or timeout")
	t.Log(err.Error())
	assert.Contains(t, err.Error(), "Timeout waiting for role", "err.Error() should contain \"Timeout waiting for role\"")
}

func TestApplyRBACHypershift(t *testing.T) {
	kubeset := fake.NewSimpleClientset()

	// For Hypershift, the controller calls ApplyRBAC with the curator namespace
	// (e.g. "clusters"), not the cluster name — mirroring the real reconcile path.
	_ = ApplyRBAC(kubeset, ClusterNamespace)
	err := ApplyRBACHypershift(kubeset, ClusterName, ClusterNamespace)
	assert.Nil(t, err, "err nil, when ClusterRoles and RoleBindings are created")

	t.Log("Validate curator-cluster-scoped ClusterRole exists")
	scopedRole, err := kubeset.RbacV1().ClusterRoles().Get(context.TODO(), "curator-cluster-scoped", v1.GetOptions{})
	assert.Nil(t, err, "err nil, when curator-cluster-scoped ClusterRole exists")
	assert.Equal(t, 1, len(scopedRole.Rules), "curator-cluster-scoped should have exactly one rule")
	assert.Equal(t, []string{"managedclusters"}, scopedRole.Rules[0].Resources,
		"curator-cluster-scoped should only cover managedclusters")

	t.Log("Validate curator-crb ClusterRoleBinding references curator-cluster-scoped")
	crb, err := kubeset.RbacV1().ClusterRoleBindings().Get(context.TODO(), "curator-crb", v1.GetOptions{})
	assert.Nil(t, err, "err nil, when curator-crb exists")
	assert.Equal(t, "curator-cluster-scoped", crb.RoleRef.Name,
		"curator-crb must reference curator-cluster-scoped, not the full curator ClusterRole")
	assert.Equal(t, ClusterNamespace, crb.Subjects[0].Namespace,
		"curator-crb subject namespace must be the curator namespace")

	t.Log("Validate RoleBinding in cluster namespace")
	rb, err := kubeset.RbacV1().RoleBindings(ClusterName).Get(context.TODO(), "curator", v1.GetOptions{})
	assert.Nil(t, err, "err nil, when RoleBinding curator exists in cluster namespace")
	assert.Equal(t, ClusterNamespace, rb.Subjects[0].Namespace,
		"RoleBinding subject namespace must be the curator namespace")
}

func TestApplyRBACHypershiftCRBUpsert(t *testing.T) {
	kubeset := fake.NewSimpleClientset()

	// First Hypershift cluster in ClusterNamespace ("clusters")
	_ = ApplyRBAC(kubeset, ClusterNamespace)
	err := ApplyRBACHypershift(kubeset, ClusterName, ClusterNamespace)
	assert.Nil(t, err, "err nil for first Hypershift cluster")

	crb, err := kubeset.RbacV1().ClusterRoleBindings().Get(context.TODO(), "curator-crb", v1.GetOptions{})
	assert.Nil(t, err, "curator-crb should exist after first cluster")
	assert.Equal(t, ClusterNamespace, crb.Subjects[0].Namespace, "subject namespace should be ClusterNamespace")

	// Second Hypershift cluster from a different curator namespace
	differentNamespace := "other-clusters"
	err = ApplyRBACHypershift(kubeset, "another-cluster", differentNamespace)
	assert.Nil(t, err, "err nil for second Hypershift cluster from different namespace")

	crb, err = kubeset.RbacV1().ClusterRoleBindings().Get(context.TODO(), "curator-crb", v1.GetOptions{})
	assert.Nil(t, err, "curator-crb should still exist after second cluster")
	assert.Equal(t, differentNamespace, crb.Subjects[0].Namespace,
		"curator-crb subject namespace must be updated to the new curator namespace")
}

func TestCleanupRBAC(t *testing.T) {
	kubeset := fake.NewSimpleClientset()

	err := ApplyRBAC(kubeset, ClusterName)
	assert.Nil(t, err, "err nil when RBAC applied")

	_, err = kubeset.CoreV1().ServiceAccounts(ClusterName).Get(context.TODO(), clusterInstaller, v1.GetOptions{})
	assert.Nil(t, err, "SA should exist before cleanup")

	_, err = kubeset.RbacV1().RoleBindings(ClusterName).Get(context.TODO(), "curator", v1.GetOptions{})
	assert.Nil(t, err, "RoleBinding should exist before cleanup")

	err = CleanupRBAC(kubeset, ClusterName)
	assert.Nil(t, err, "err nil when CleanupRBAC succeeds")

	_, err = kubeset.CoreV1().ServiceAccounts(ClusterName).Get(context.TODO(), clusterInstaller, v1.GetOptions{})
	assert.NotNil(t, err, "SA should be gone after cleanup")

	_, err = kubeset.RbacV1().RoleBindings(ClusterName).Get(context.TODO(), "curator", v1.GetOptions{})
	assert.NotNil(t, err, "RoleBinding should be gone after cleanup")

	t.Log("Verify CleanupRBAC is idempotent")
	err = CleanupRBAC(kubeset, ClusterName)
	assert.Nil(t, err, "err nil when CleanupRBAC called on already-cleaned namespace")
}

func TestCleanupRBACHypershift(t *testing.T) {
	kubeset := fake.NewSimpleClientset()

	_ = ApplyRBAC(kubeset, ClusterNamespace)
	err := ApplyRBACHypershift(kubeset, ClusterName, ClusterNamespace)
	assert.Nil(t, err, "err nil when Hypershift RBAC applied")

	_, err = kubeset.RbacV1().RoleBindings(ClusterName).Get(context.TODO(), "curator", v1.GetOptions{})
	assert.Nil(t, err, "cluster-namespace RoleBinding should exist before cleanup")

	err = CleanupRBACHypershift(kubeset, ClusterName, ClusterNamespace)
	assert.Nil(t, err, "err nil when CleanupRBACHypershift succeeds")

	_, err = kubeset.RbacV1().RoleBindings(ClusterName).Get(context.TODO(), "curator", v1.GetOptions{})
	assert.NotNil(t, err, "cluster-namespace RoleBinding should be gone after cleanup")

	t.Log("Verify curator-crb ClusterRoleBinding is deleted since it still belonged to ClusterNamespace")
	_, err = kubeset.RbacV1().ClusterRoleBindings().Get(context.TODO(), "curator-crb", v1.GetOptions{})
	assert.NotNil(t, err, "curator-crb should be deleted by CleanupRBACHypershift when it still owns the CRB")

	t.Log("Verify CleanupRBACHypershift is idempotent")
	err = CleanupRBACHypershift(kubeset, ClusterName, ClusterNamespace)
	assert.Nil(t, err, "err nil when CleanupRBACHypershift called on already-cleaned namespace")
}

func TestCleanupRBACHypershiftPreservesCRBOwnedByOtherNamespace(t *testing.T) {
	kubeset := fake.NewSimpleClientset()

	// First Hypershift cluster curates in ClusterNamespace, claiming curator-crb.
	_ = ApplyRBAC(kubeset, ClusterNamespace)
	err := ApplyRBACHypershift(kubeset, ClusterName, ClusterNamespace)
	assert.Nil(t, err, "err nil for first Hypershift cluster")

	// A second Hypershift cluster from a different namespace takes over curator-crb
	// (mirrors the upsert behavior in ApplyRBACHypershift) before the first curation
	// finishes and calls cleanup.
	otherClusterName := "another-cluster"
	otherNamespace := "other-clusters"
	err = ApplyRBACHypershift(kubeset, otherClusterName, otherNamespace)
	assert.Nil(t, err, "err nil for second Hypershift cluster from different namespace")

	crb, err := kubeset.RbacV1().ClusterRoleBindings().Get(context.TODO(), "curator-crb", v1.GetOptions{})
	assert.Nil(t, err, "curator-crb should exist")
	assert.Equal(t, otherNamespace, crb.Subjects[0].Namespace, "curator-crb should now belong to otherNamespace")

	// Cleanup for the first (now stale) namespace must not revoke otherNamespace's
	// still-active grant.
	err = CleanupRBACHypershift(kubeset, ClusterName, ClusterNamespace)
	assert.Nil(t, err, "err nil when CleanupRBACHypershift succeeds")

	crb, err = kubeset.RbacV1().ClusterRoleBindings().Get(context.TODO(), "curator-crb", v1.GetOptions{})
	assert.Nil(t, err, "curator-crb must be preserved since it no longer belongs to ClusterNamespace")
	assert.Equal(t, otherNamespace, crb.Subjects[0].Namespace,
		"curator-crb subject namespace must remain otherNamespace")
}
