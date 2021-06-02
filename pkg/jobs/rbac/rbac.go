// Copyright Contributors to the Open Cluster Management project.
package rbac

import (
	"context"
	"errors"
	"time"

	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/utils"
	"k8s.io/klog/v2"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const clusterInstaller = "cluster-installer"

func getRole() *rbacv1.Role {
	curatorRole := &rbacv1.Role{
		ObjectMeta: v1.ObjectMeta{Name: "curator"},
		Rules: []rbacv1.PolicyRule{
			rbacv1.PolicyRule{
				APIGroups: []string{"tower.ansible.com", ""},
				Resources: []string{"ansiblejobs", "secrets", "serviceaccounts"},
				Verbs:     []string{"create"},
			},
			rbacv1.PolicyRule{
				APIGroups: []string{"hive.openshift.io"},
				Resources: []string{"clusterdeployments"},
				Verbs:     []string{"patch", "delete"},
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
				Verbs:     []string{"get", "update", "patch"},
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
		},
	}
	return curatorRole
}

func getClusterInstallerRules() []rbacv1.PolicyRule {
	curatorRule := []rbacv1.PolicyRule{
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
			Verbs:     []string{"patch"},
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
	}
	return curatorRule
}

func getRoleBinding(namespace string) *rbacv1.RoleBinding {
	clusterRoleBinding := &rbacv1.RoleBinding{
		ObjectMeta: v1.ObjectMeta{Name: "curator"},
		Subjects: []rbacv1.Subject{
			rbacv1.Subject{
				Kind:      "ServiceAccount",
				Name:      clusterInstaller,
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
		ObjectMeta: v1.ObjectMeta{Name: clusterInstaller},
	}
	return serviceAccount
}

func ApplyRBAC(kubeset kubernetes.Interface, namespace string) error {

	klog.V(2).Info("Check if serviceAccount cluster-installer exists")
	if _, err := kubeset.CoreV1().ServiceAccounts(namespace).Get(
		context.TODO(), "cluster-installer", v1.GetOptions{}); err != nil {

		klog.V(2).Info(" Creating serviceAccount cluster-installer")
		_, err = kubeset.CoreV1().ServiceAccounts(namespace).Create(
			context.TODO(), getServiceAccount(), v1.CreateOptions{})

		if err != nil {
			return err
		}
		klog.V(0).Info(" Created serviceAccount ✓")
	}

	klog.V(2).Info("Check if Role curator exists")
	if _, err := kubeset.RbacV1().Roles(namespace).Get(context.TODO(), "curator", v1.GetOptions{}); err != nil {
		klog.V(2).Info(" Creating Role curator")
		_, err = kubeset.RbacV1().Roles(namespace).Create(context.TODO(), getRole(), v1.CreateOptions{})
		if err != nil {
			return err
		}
		klog.V(0).Info(" Created Role ✓")
	}

	klog.V(2).Info("Check if RoleBinding cluster-installer exists")
	if _, err := kubeset.RbacV1().RoleBindings(namespace).Get(context.TODO(), "curator", v1.GetOptions{}); err != nil {
		klog.V(2).Info(" Creating RoleBinding curator")
		_, err = kubeset.RbacV1().RoleBindings(namespace).Create(context.TODO(), getRoleBinding(namespace), v1.CreateOptions{})
		if err != nil {
			return err
		}
		klog.V(0).Info(" Created RoleBinding ✓")
	}
	return nil
}

func ExtendClusterInstallerRole(kubeset kubernetes.Interface, namespace string) error {

	klog.V(0).Infof("Extending the %v role to support curator", clusterInstaller)

	checkCount := 15 //Loop every 2s
	for i := 1; i <= checkCount; i++ {
		ciRole, err := kubeset.RbacV1().Roles(namespace).Get(context.TODO(), clusterInstaller, v1.GetOptions{})
		if err != nil {
			klog.Warningf("Did not find %v Role in namespace: %v (%v/%v)", clusterInstaller, namespace, i, checkCount)
			time.Sleep(utils.PauseTwoSeconds)
		} else {
			klog.V(2).Infof(" Found %v role ✓", clusterInstaller)
			ciRole.Rules = append(ciRole.Rules, getClusterInstallerRules()...)
			_, err = kubeset.RbacV1().Roles(namespace).Update(context.TODO(), ciRole, v1.UpdateOptions{})
			if err != nil {
				return err
			}
			klog.V(0).Infof(" %v role extended with new rules ✓", clusterInstaller)
			break
		}

		if i == checkCount {
			return errors.New("Timeout waiting for role " + clusterInstaller + "to be created")
		}
	}
	return nil
}
