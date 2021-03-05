// Copyright Contributors to the Open Cluster Management project.
package rbac

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const ClusterName = "my-cluster"

func TestApplyRbac(t *testing.T) {

	rules := []rbacv1.PolicyRule{
		rbacv1.PolicyRule{
			APIGroups: []string{"tower.ansible.com", "", "hive.openshift.io"},
			Resources: []string{"ansiblejobs", "secrets", "clusterdeployments", "machinepools"},
			Verbs:     []string{"create"},
		},
		rbacv1.PolicyRule{
			APIGroups: []string{"", "hive.openshift.io"},
			Resources: []string{"clusterdeployments", "secrets"},
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
			Verbs:     []string{"update", "get"},
		},
	}

	subjects := []rbacv1.Subject{
		rbacv1.Subject{
			Kind:      "ServiceAccount",
			Name:      clusterInstall,
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

	_, err = kubeset.CoreV1().ServiceAccounts(ClusterName).Get(context.TODO(), clusterInstall, v1.GetOptions{})
	assert.Nil(t, err, "err nil, when service account exists")

	t.Log("Validate Role")

	role, err := kubeset.RbacV1().Roles(ClusterName).Get(context.TODO(), "curator", v1.GetOptions{})

	assert.Nil(t, err, "err nil, when Role exists")
	assert.ElementsMatch(t, rules, role.Rules, "The rules should match")

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
