// Copyright Contributors to the Open Cluster Management project.
package rbac

import (
	"context"
	"testing"
	"time"

	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/utils"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const ClusterName = "my-cluster"

func getRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		rbacv1.PolicyRule{
			APIGroups: []string{"tower.ansible.com", ""},
			Resources: []string{"ansiblejobs", "secrets", "serviceaccounts"},
			Verbs:     []string{"create"},
		},
		rbacv1.PolicyRule{
			APIGroups: []string{"hive.openshift.io"},
			Resources: []string{"clusterdeployments"},
			Verbs:     []string{"patch"},
		},
		rbacv1.PolicyRule{
			APIGroups: []string{"batch", "hive.openshift.io", "tower.ansible.com"},
			Resources: []string{"jobs", "clusterdeployments", "ansiblejobs"},
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
	}
}

func getCombinedCIRules() []rbacv1.PolicyRule {
	return append(getRules(),
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
				APIGroups: []string{"", "hive.openshift.io"},
				Resources: []string{"configmaps", "clusterdeployments"},
				Verbs:     []string{"patch"},
			},
			rbacv1.PolicyRule{
				APIGroups: []string{"internal.open-cluster-management.io"},
				Resources: []string{"managedclusterinfos"},
				Verbs:     []string{"get"},
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
		Kind:     "Role",
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

	role, err := kubeset.RbacV1().Roles(ClusterName).Get(context.TODO(), "curator", v1.GetOptions{})

	assert.Nil(t, err, "err nil, when Role exists")
	assert.ElementsMatch(t, getRules(), role.Rules, "The rules should match")

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

	testRole := getRole()
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

	testRole := getRole()
	testRole.Name = clusterInstaller

	err := ExtendClusterInstallerRole(kubeset, ClusterName)

	assert.NotNil(t, err, "err not nil, when failure or timeout")
	t.Log(err.Error())
	assert.Contains(t, err.Error(), "Timeout waiting for role", "err.Error() should contain \"Timeout waiting for role\"")
}
