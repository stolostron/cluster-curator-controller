package rbac

import (
	"context"
	"log"

	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/utils"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func getRole() *rbacv1.Role {
	curatorRole := &rbacv1.Role{
		ObjectMeta: v1.ObjectMeta{Name: "curator"},
		Rules: []rbacv1.PolicyRule{
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
				APIGroups: []string{"", "batch", "hive.openshift.io", "tower.ansible.com"},
				Resources: []string{"configmaps", "jobs", "clusterdeployments", "ansiblejobs"},
				Verbs:     []string{"get"},
			},
		},
	}
	return curatorRole
}

func getRoleBinding(namespace string) *rbacv1.RoleBinding {
	clusterRoleBinding := &rbacv1.RoleBinding{
		ObjectMeta: v1.ObjectMeta{Name: "curator"},
		Subjects: []rbacv1.Subject{
			rbacv1.Subject{
				Kind:      "ServiceAccount",
				Name:      "cluster-installer",
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     "curator",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	return clusterRoleBinding
}

func getServiceAccount() *corev1.ServiceAccount {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: v1.ObjectMeta{Name: "cluster-installer"},
	}
	return serviceAccount
}

func ApplyRBAC(config *rest.Config, namespace string) {
	kubeset, err := kubernetes.NewForConfig(config)
	utils.CheckError(err)

	log.Println("Check if serviceAccount cluster-installer exists")
	if _, err := kubeset.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), "cluster-installer", v1.GetOptions{}); err != nil {
		log.Println(" Creating serviceAccount cluster-installer")
		_, err = kubeset.CoreV1().ServiceAccounts(namespace).Create(context.TODO(), getServiceAccount(), v1.CreateOptions{})
		utils.CheckError(err)
		log.Println(" Created serviceAccount ✓")
	}

	log.Println("Check if RoleBinding cluster-installer exists")
	if _, err := kubeset.RbacV1().RoleBindings(namespace).Get(context.TODO(), "curator", v1.GetOptions{}); err != nil {
		log.Println(" Creating RoleBinding cluster-installer")
		_, err = kubeset.RbacV1().RoleBindings(namespace).Create(context.TODO(), getRoleBinding(namespace), v1.CreateOptions{})
		log.Println(" Created RoleBinding ✓")
	}

	log.Println("Check if Role cluster-installer exists")
	if _, err := kubeset.RbacV1().Roles(namespace).Get(context.TODO(), "curator", v1.GetOptions{}); err != nil {
		log.Println(" Creating Role cluster-installer")
		_, err = kubeset.RbacV1().Roles(namespace).Create(context.TODO(), getRole(), v1.CreateOptions{})
		log.Println(" Created Role ✓")
	}
}
