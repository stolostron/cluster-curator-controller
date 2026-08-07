// Copyright Contributors to the Open Cluster Management project.
package rbac

import (
	"context"
	"errors"
	"time"

	"github.com/stolostron/cluster-curator-controller/pkg/jobs/utils"
	"k8s.io/klog/v2"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const clusterInstaller = "cluster-installer"

func getRole(clusterName string) *rbacv1.Role {
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
				APIGroups:     []string{""},
				Resources:     []string{"secrets"},
				Verbs:         []string{"get"},
				ResourceNames: []string{clusterName + "-install-config"},
			},
		},
	}
	return curatorRole
}

func getClusterRole(clusterName string) *rbacv1.ClusterRole {
	curatorClusterRole := &rbacv1.ClusterRole{
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
			// managedclusters is cluster-scoped and cannot be granted via a RoleBinding;
			// it is covered exclusively by ClusterRole/curator-cluster-scoped + curator-crb.
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
			// get secrets is intentionally unrestricted here because this ClusterRole is a
			// singleton and cannot encode per-cluster ResourceNames. It is safe because
			// this ClusterRole is only bound via namespace-scoped RoleBindings (not via
			// ClusterRoleBinding), which confines secret reads to the bound namespace.
			// The ClusterRoleBinding (curator-crb) references curator-cluster-scoped
			// instead, which carries no secrets rule at all.
			rbacv1.PolicyRule{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get"},
			},
		},
	}

	return curatorClusterRole
}

// getClusterScopedRole returns a minimal ClusterRole covering only genuinely
// cluster-scoped resources. This is used exclusively with the curator-crb
// ClusterRoleBinding so that the ServiceAccount does not receive cluster-wide
// authority over namespace-scoped resources (e.g. secrets, hostedclusters).
func getClusterScopedRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: v1.ObjectMeta{Name: "curator-cluster-scoped"},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"cluster.open-cluster-management.io"},
				Resources: []string{"managedclusters"},
				Verbs:     []string{"get", "update", "patch", "delete"},
			},
		},
	}
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
			Kind:     "ClusterRole",
			Name:     "curator",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	return clusterRoleBinding
}

// getClusterScopedRoleBinding returns a ClusterRoleBinding that binds
// cluster-installer in curatorNamespace to the minimal curator-cluster-scoped
// ClusterRole. Using the scoped role instead of the full curator ClusterRole
// prevents the ServiceAccount from obtaining cluster-wide authority over
// namespace-scoped resources such as secrets and hostedclusters.
func getClusterScopedRoleBinding(namespace string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: v1.ObjectMeta{Name: "curator-crb"},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      clusterInstaller,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "curator-cluster-scoped",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
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

	klog.V(2).Info("Check if ClusterRole curator exists")
	if _, err := kubeset.RbacV1().ClusterRoles().Get(context.TODO(), "curator", v1.GetOptions{}); err != nil {
		klog.V(2).Info(" Creating ClusterRole curator")
		_, err = kubeset.RbacV1().ClusterRoles().Create(context.TODO(), getClusterRole(namespace), v1.CreateOptions{})
		if err != nil {
			return err
		}
		klog.V(0).Info(" Created ClusterRole ✓")
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

func ApplyRBACHypershift(kubeset kubernetes.Interface, namespace string, curatorNamespace string) error {
	klog.V(2).Info("Check if RoleBinding curator exists in namespace " + namespace)
	var err error
	if _, err = kubeset.RbacV1().RoleBindings(namespace).Get(
		context.TODO(), "curator", v1.GetOptions{}); k8serrors.IsNotFound(err) {
		klog.V(2).Info(" Creating RoleBinding curator in namespace " + namespace)
		_, err = kubeset.RbacV1().RoleBindings(namespace).Create(
			context.TODO(), getRoleBinding(curatorNamespace), v1.CreateOptions{})
		if err != nil {
			return err
		}
		klog.V(0).Info(" Created RoleBinding in cluster namespace ✓")
	} else if err != nil {
		return err
	}

	klog.V(2).Info("Check if ClusterRole curator-cluster-scoped exists")
	if _, err = kubeset.RbacV1().ClusterRoles().Get(
		context.TODO(), "curator-cluster-scoped", v1.GetOptions{}); k8serrors.IsNotFound(err) {
		klog.V(2).Info(" Creating ClusterRole curator-cluster-scoped")
		_, err = kubeset.RbacV1().ClusterRoles().Create(
			context.TODO(), getClusterScopedRole(), v1.CreateOptions{})
		if err != nil {
			return err
		}
		klog.V(0).Info(" Created ClusterRole curator-cluster-scoped ✓")
	} else if err != nil {
		return err
	}

	// Upsert curator-crb: create if absent, or update the subject namespace when
	// a subsequent Hypershift cluster from a different namespace triggers this path.
	// Without the update the CRB would permanently reference the first namespace
	// that triggered its creation.
	klog.V(2).Info("Check if ClusterRoleBinding curator-crb exists")
	existingCRB, err := kubeset.RbacV1().ClusterRoleBindings().Get(
		context.TODO(), "curator-crb", v1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		klog.V(2).Info(" Creating ClusterRoleBinding curator-crb")
		_, err = kubeset.RbacV1().ClusterRoleBindings().Create(
			context.TODO(), getClusterScopedRoleBinding(curatorNamespace), v1.CreateOptions{})
		if err != nil {
			return err
		}
		klog.V(0).Info(" Created ClusterRoleBinding ✓")
	} else if err != nil {
		return err
	} else if len(existingCRB.Subjects) == 0 || existingCRB.Subjects[0].Namespace != curatorNamespace {
		klog.V(2).Info(" Updating ClusterRoleBinding curator-crb subject namespace to " + curatorNamespace)
		existingCRB.Subjects = getClusterScopedRoleBinding(curatorNamespace).Subjects
		_, err = kubeset.RbacV1().ClusterRoleBindings().Update(
			context.TODO(), existingCRB, v1.UpdateOptions{})
		if err != nil {
			return err
		}
		klog.V(0).Info(" Updated ClusterRoleBinding ✓")
	}
	return nil
}

// CleanupRBAC removes the cluster-installer ServiceAccount and curator RoleBinding
// from namespace. It is called after a curation job completes so that the
// ServiceAccount cannot be used to mint tokens between curations.
func CleanupRBAC(kubeset kubernetes.Interface, namespace string) error {
	klog.V(2).Info("Cleaning up RBAC in namespace " + namespace)

	if err := kubeset.CoreV1().ServiceAccounts(namespace).Delete(
		context.TODO(), clusterInstaller, v1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	klog.V(0).Info(" Deleted ServiceAccount cluster-installer ✓")

	if err := kubeset.RbacV1().RoleBindings(namespace).Delete(
		context.TODO(), "curator", v1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	klog.V(0).Info(" Deleted RoleBinding curator ✓")
	return nil
}

// CleanupRBACHypershift removes the curator RoleBinding from the Hypershift cluster
// namespace, and removes the shared curator-crb ClusterRoleBinding's grant if it
// still belongs to curatorNamespace.
//
// curator-crb is a cluster-wide singleton with a single subject slot that
// ApplyRBACHypershift upserts to whichever curatorNamespace most recently ran a
// Hypershift curation. If a different Hypershift curation has since taken over
// the CRB (its subject namespace no longer matches curatorNamespace), it is left
// intact so that unrelated, still-active curation's access is not revoked.
// Deleting curator-crb here (rather than leaving it as a stale reference) closes
// the escalation path where a tenant could recreate a ServiceAccount named
// cluster-installer in its own namespace and silently inherit the binding, since
// RBAC subjects are matched by name/namespace, not object identity.
func CleanupRBACHypershift(kubeset kubernetes.Interface, clusterNamespace string, curatorNamespace string) error {
	klog.V(2).Info("Cleaning up Hypershift RBAC in namespace " + clusterNamespace)

	if err := kubeset.RbacV1().RoleBindings(clusterNamespace).Delete(
		context.TODO(), "curator", v1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	klog.V(0).Info(" Deleted RoleBinding curator in cluster namespace ✓")

	crb, err := kubeset.RbacV1().ClusterRoleBindings().Get(context.TODO(), "curator-crb", v1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	if len(crb.Subjects) > 0 && crb.Subjects[0].Namespace == curatorNamespace {
		if err := kubeset.RbacV1().ClusterRoleBindings().Delete(
			context.TODO(), "curator-crb", v1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
			return err
		}
		klog.V(0).Info(" Deleted ClusterRoleBinding curator-crb ✓")
	} else {
		klog.V(2).Info(" curator-crb now belongs to another namespace; leaving intact")
	}
	return nil
}

func ExtendClusterInstallerRole(kubeset kubernetes.Interface, namespace string) error {

	klog.V(0).Infof("Extending the %v role to support curator", clusterInstaller)

	checkCount := 15 // Loop every 2s
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
