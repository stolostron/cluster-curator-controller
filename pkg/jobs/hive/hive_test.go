// Copyright Contributors to the Open Cluster Management project.
package hive

import (
	"context"
	"errors"
	"testing"
	"time"

	clustercuratorv1 "github.com/open-cluster-management/cluster-curator-controller/pkg/api/v1beta1"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/utils"
	managedclusterinfov1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/internal.open-cluster-management.io/v1beta1"
	managedclusterviewv1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/view/v1beta1"
	hivev1 "github.com/openshift/hive/pkg/apis/hive/v1"
	hivefake "github.com/openshift/hive/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const ClusterName = "my-cluster"

func getClusterCurator() *clustercuratorv1.ClusterCurator {
	return &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "install",
			Install: clustercuratorv1.Hooks{
				Prehook: []clustercuratorv1.Hook{
					clustercuratorv1.Hook{
						Name: "Service now App Update",
						ExtraVars: &runtime.RawExtension{
							Raw: []byte(`{"variable1": "1","variable2": "2"}`),
						},
					},
				},
				Posthook: []clustercuratorv1.Hook{
					clustercuratorv1.Hook{
						Name: "Service now App Update",
						ExtraVars: &runtime.RawExtension{
							Raw: []byte(`{"variable1": "3","variable2": "4"}`),
						},
					},
				},
			},
		},
	}
}

func getUpgradeClusterCurator() *clustercuratorv1.ClusterCurator {
	return &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "upgrade",
			Upgrade: clustercuratorv1.UpgradeHooks{
				DesiredUpdate: "4.5.13",
				Hooks: clustercuratorv1.Hooks{
					Prehook: []clustercuratorv1.Hook{
						clustercuratorv1.Hook{
							Name: "Service now App Update",
							ExtraVars: &runtime.RawExtension{
								Raw: []byte(`{"variable1": "1","variable2": "2"}`),
							},
						},
					},
					Posthook: []clustercuratorv1.Hook{
						clustercuratorv1.Hook{
							Name: "Service now App Update",
							ExtraVars: &runtime.RawExtension{
								Raw: []byte(`{"variable1": "3","variable2": "4"}`),
							},
						},
					},
				},
			},
		},
	}
}

func TestActivateDeployNoCD(t *testing.T) {

	hiveset := hivefake.NewSimpleClientset()

	t.Log("No ClusterDeployment")
	assert.NotNil(t, ActivateDeploy(hiveset, ClusterName),
		"err NotNil when ClusterDeployment kind does not exist")
}

func TestActivateDeployNoInstallAttemptsLimit(t *testing.T) {

	hiveset := hivefake.NewSimpleClientset(&hivev1.ClusterDeployment{
		TypeMeta: v1.TypeMeta{
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
	})

	t.Log("ClusterDeployment with no installAttemptsLimit")
	assert.NotNil(t, ActivateDeploy(hiveset, ClusterName),
		"err NotNil when ClusterDeployment has no installAttemptsLimit")
}

func TestActivateDeployNonZeroInstallAttemptsLimit(t *testing.T) {

	intValue := int32(1)
	hiveset := hivefake.NewSimpleClientset(&hivev1.ClusterDeployment{
		TypeMeta: v1.TypeMeta{
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Spec: hivev1.ClusterDeploymentSpec{
			InstallAttemptsLimit: &intValue,
		},
	})

	t.Log("ClusterDeployment with installAttemptsLimit int32(1)")
	assert.NotNil(t, ActivateDeploy(hiveset, ClusterName),
		"err NotNil when ClusterDeployment with installAttemptsLimit not zero")
}

func TestActivateDeploy(t *testing.T) {

	intValue := int32(0)
	hiveset := hivefake.NewSimpleClientset(&hivev1.ClusterDeployment{
		TypeMeta: v1.TypeMeta{
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Spec: hivev1.ClusterDeploymentSpec{
			InstallAttemptsLimit: &intValue,
		},
	})

	t.Log("ClusterDeployment with installAttemptsLimit zero")
	assert.Nil(t, ActivateDeploy(hiveset, ClusterName),
		"err NotNil when ClusterDeployment with installAttemptsLimit not zero")
}

func getClusterDeployment() *hivev1.ClusterDeployment {
	return &hivev1.ClusterDeployment{
		TypeMeta: v1.TypeMeta{
			Kind:       "clusterdeployment",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
	}
}

func getProvisionJob() *batchv1.Job {
	return &batchv1.Job{
		TypeMeta: v1.TypeMeta{
			Kind:       "job",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName + "-12345-provision",
			Namespace: ClusterName,
		},
		Status: batchv1.JobStatus{
			Active:    0,
			Succeeded: 1,
		},
	}
}

func TestMonitorDeployStatusNoClusterDeployment(t *testing.T) {

	hiveset := hivefake.NewSimpleClientset()

	assert.NotNil(t, monitorDeployStatus(nil, hiveset, ClusterName), "err is not nil, when cluster provisioning has a condition")
}

func TestMonitorDeployStatusCondition(t *testing.T) {

	cd := getClusterDeployment()
	cd.Status.Conditions = []hivev1.ClusterDeploymentCondition{
		hivev1.ClusterDeploymentCondition{
			Message: "ClusterImageSet img4.6.17-x86-64-appsub is not available",
			Type:    "ClusterImageSetNotFound",
			Status:  "True",
		},
	}

	hiveset := hivefake.NewSimpleClientset(cd)

	assert.NotNil(t, monitorDeployStatus(nil, hiveset, ClusterName), "err is not nil, when cluster provisioning has a condition")
}

func TestMonitorDeployNoJobTimeout(t *testing.T) {

	cd := getClusterDeployment()

	hiveset := hivefake.NewSimpleClientset(cd)

	assert.NotNil(t, monitorDeployStatus(nil, hiveset, ClusterName), "err is not nil, when cluster provisioning has no job created")
}
func TestMonitorDeployStatusJobFailed(t *testing.T) {

	cd := getClusterDeployment()
	cd.Status.ProvisionRef = &corev1.LocalObjectReference{
		Name: ClusterName + "-12345", // The monitor adds the -provision suffix
	}

	job := getProvisionJob()
	job.Status.Succeeded = 0

	hiveset := hivefake.NewSimpleClientset(cd)

	cc := getClusterCurator()

	s := scheme.Scheme
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})

	client := clientfake.NewFakeClientWithScheme(s, cc, job)

	assert.NotNil(t, monitorDeployStatus(client, hiveset, ClusterName), "err is not nil, when cluster provisioning has a condition")
}

func TestMonitorDeployStatusJobCompletedWithSuccess(t *testing.T) {

	cd := getClusterDeployment()
	cd.Status.Conditions = []hivev1.ClusterDeploymentCondition{
		hivev1.ClusterDeploymentCondition{
			Message: "Provisioned",
			Reason:  "SuccessfulProvision",
		},
	}
	cd.Status.ProvisionRef = &corev1.LocalObjectReference{
		Name: ClusterName + "-12345", // The monitor adds the -provision suffix
	}
	cc := getClusterCurator()

	hiveset := hivefake.NewSimpleClientset(cd)
	s := scheme.Scheme
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})

	client := clientfake.NewFakeClientWithScheme(s, cc)

	// Put a delay to complete the ClusterDeployment to test the wait loop
	go func() {
		time.Sleep(utils.PauseFiveSeconds)
		cd.Status.WebConsoleURL = "https://my-cluster"

		hiveset.HiveV1().ClusterDeployments(ClusterName).Update(context.TODO(), cd, v1.UpdateOptions{})
		t.Log("ClusterDeployment webConsoleURL applied")
	}()

	assert.Equal(t, len(cc.Status.Conditions), 0, "Should be emtpy")
	assert.Nil(t, monitorDeployStatus(client, hiveset, ClusterName), "err is nil, when cluster provisioning is successful")

	err := client.Get(context.Background(), types.NamespacedName{Namespace: ClusterName, Name: ClusterName}, cc)
	t.Log(err)
	assert.Greater(t, len(cc.Status.Conditions), 0, "Should be populated")
}

func TestMonitorDeployStatusJobComplete(t *testing.T) {

	cd := getClusterDeployment()
	cd.Status.ProvisionRef = &corev1.LocalObjectReference{
		Name: ClusterName + "-12345", // The monitor adds the -provision suffix
	}

	job := getProvisionJob()
	job.Status.Active = 1
	job.Status.Succeeded = 0

	cc := getClusterCurator()

	hiveset := hivefake.NewSimpleClientset(cd)
	s := scheme.Scheme
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewFakeClientWithScheme(s, cc)

	// Put a delay to complete the job to test the wait loop
	go func() {
		time.Sleep(utils.PauseFiveSeconds)
		job.Status.Active = 0
		job.Status.Succeeded = 1
		client.Update(context.Background(), job)
		cd.Status.WebConsoleURL = "https://my-cluster"
		cd.Status.Conditions = []hivev1.ClusterDeploymentCondition{
			hivev1.ClusterDeploymentCondition{
				Message: "Provisioned",
				Reason:  "SuccessfulProvision",
			},
		}
		hiveset.HiveV1().ClusterDeployments(ClusterName).Update(context.TODO(), cd, v1.UpdateOptions{})
	}()

	assert.Nil(t, monitorDeployStatus(client, hiveset, ClusterName), "err is not nil, when cluster provisioning is successful")
}

func TestMonitorDeployStatusJobDelayedComplete(t *testing.T) {

	cd := getClusterDeployment()
	cd.Status.ProvisionRef = &corev1.LocalObjectReference{
		Name: ClusterName + "-12345", // The monitor adds the -provision suffix
	}

	hiveset := hivefake.NewSimpleClientset(cd)
	s := scheme.Scheme
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewFakeClientWithScheme(s, getClusterCurator())

	// Put a delay to complete the job to test the wait loop
	go func() {
		time.Sleep(utils.PauseFiveSeconds)
		cd.Status.WebConsoleURL = "https://my-cluster"
		cd.Status.Conditions = []hivev1.ClusterDeploymentCondition{
			hivev1.ClusterDeploymentCondition{
				Message: "Provisioned",
				Reason:  "SuccessfulProvision",
			},
		}
		t.Log("Created the Job resource and update Cluster Deployment as complete")
		client.Create(context.Background(), getProvisionJob())
		hiveset.HiveV1().ClusterDeployments(ClusterName).Update(context.TODO(), cd, v1.UpdateOptions{})
	}()

	assert.Nil(t, monitorDeployStatus(client, hiveset, ClusterName), "err is not nil, when cluster provisioning is successful")
}

func TestUpgradeClusterNonOpenshift(t *testing.T) {

	s := scheme.Scheme
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(managedclusterinfov1beta1.GroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})

	managedclusterinfo := &managedclusterinfov1beta1.ManagedClusterInfo{
		TypeMeta: v1.TypeMeta{
			APIVersion: managedclusterinfov1beta1.GroupVersion.String(),
			Kind:       "ManagedClusterInfo",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Status: managedclusterinfov1beta1.ClusterInfoStatus{
			KubeVendor: managedclusterinfov1beta1.KubeVendorOther,
		},
	}

	client := clientfake.NewFakeClientWithScheme(s, []runtime.Object{
		getUpgradeClusterCurator(), managedclusterinfo,
	}...)

	assert.Equal(t, UpgradeCluster(client, ClusterName, getUpgradeClusterCurator()),
		errors.New("Can not upgrade non openshift cluster"))
}

func TestUpgradeClusterNoDesiredUpdate(t *testing.T) {

	s := scheme.Scheme
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(managedclusterinfov1beta1.GroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})

	managedclusterinfo := &managedclusterinfov1beta1.ManagedClusterInfo{
		TypeMeta: v1.TypeMeta{
			APIVersion: managedclusterinfov1beta1.GroupVersion.String(),
			Kind:       "ManagedClusterInfo",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Status: managedclusterinfov1beta1.ClusterInfoStatus{
			KubeVendor: managedclusterinfov1beta1.KubeVendorOpenShift,
		},
	}

	clustercurator := &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "upgrade",
			Upgrade:         clustercuratorv1.UpgradeHooks{},
		},
	}

	client := clientfake.NewFakeClientWithScheme(s, []runtime.Object{
		clustercurator, managedclusterinfo,
	}...)

	assert.Equal(t, UpgradeCluster(client, ClusterName, clustercurator),
		errors.New("Provide valid upgrade version"))
}

func TestUpgradeClusterNotValidVersion(t *testing.T) {

	s := scheme.Scheme
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(managedclusterinfov1beta1.GroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})

	managedclusterinfo := &managedclusterinfov1beta1.ManagedClusterInfo{
		TypeMeta: v1.TypeMeta{
			APIVersion: managedclusterinfov1beta1.GroupVersion.String(),
			Kind:       "ManagedClusterInfo",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Status: managedclusterinfov1beta1.ClusterInfoStatus{
			KubeVendor: managedclusterinfov1beta1.KubeVendorOpenShift,
			DistributionInfo: managedclusterinfov1beta1.DistributionInfo{
				OCP: managedclusterinfov1beta1.OCPDistributionInfo{
					AvailableUpdates: []string{"4.5.14", "4.5.16", "4.5.17"},
				},
			},
		},
	}

	clustercurator := &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "upgrade",
			Upgrade: clustercuratorv1.UpgradeHooks{
				DesiredUpdate: "4.5.15",
			},
		},
	}

	client := clientfake.NewFakeClientWithScheme(s, []runtime.Object{
		clustercurator, managedclusterinfo,
	}...)

	assert.Equal(t, UpgradeCluster(client, ClusterName, clustercurator),
		errors.New("Provided version is not valid"))
}

func getManagedClusterView() *managedclusterviewv1beta1.ManagedClusterView {
	return &managedclusterviewv1beta1.ManagedClusterView{
		TypeMeta: v1.TypeMeta{
			Kind:       "ManagedClusterView",
			APIVersion: "view.open-cluster-management.io/v1beta1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
	}
}

/*func TestUpgradeCluster(t *testing.T) {

	s := scheme.Scheme
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(managedclusterinfov1beta1.GroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})
	s.AddKnownTypes(managedclusteractionv1beta1.GroupVersion, &managedclusteractionv1beta1.ManagedClusterAction{})
	s.AddKnownTypes(managedclusterviewv1beta1.GroupVersion, &managedclusterviewv1beta1.ManagedClusterView{})

	managedclusterinfo := &managedclusterinfov1beta1.ManagedClusterInfo{
		TypeMeta: v1.TypeMeta{
			APIVersion: managedclusterinfov1beta1.GroupVersion.String(),
			Kind:       "ManagedClusterInfo",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Status: managedclusterinfov1beta1.ClusterInfoStatus{
			KubeVendor: managedclusterinfov1beta1.KubeVendorOpenShift,
			DistributionInfo: managedclusterinfov1beta1.DistributionInfo{
				OCP: managedclusterinfov1beta1.OCPDistributionInfo{
					AvailableUpdates: []string{"4.5.14", "4.5.16", "4.5.17"},
				},
			},
		},
	}

	clustercurator := &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "upgrade",
			Upgrade: clustercuratorv1.UpgradeHooks{
				DesiredUpdate: "4.5.14",
			},
		},
	}

	clusterversion := &clusterversionv1.ClusterVersion{
		TypeMeta: v1.TypeMeta{
			APIVersion: "config.openshift.io/v1",
			Kind:       "ClusterVersion",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "version",
		},
		Spec: clusterversionv1.ClusterVersionSpec{
			Channel:   "stable-4.5",
			ClusterID: "201ad26c-67d6-416a",
			Upstream:  "https://api.openshift.com/api",
		},
		Status: clusterversionv1.ClusterVersionStatus{
			AvailableUpdates: []clusterversionv1.Release{
				{
					Version: "4.5.14",
					Image:   "quay.io/openshift-release",
				},
				{
					Version: "4.5.15",
					Image:   "quay.io/openshift-release",
				},
			},
		},
	}
	b, _ := json.Marshal(clusterversion)
	// managedclusterview := &managedclusterviewv1beta1.ManagedClusterView{
	// 	ObjectMeta: v1.ObjectMeta{
	// 		Name:      ClusterName,
	// 		Namespace: ClusterName,
	// 	},
	// 	Spec: managedclusterviewv1beta1.ViewSpec{
	// 		Scope: managedclusterviewv1beta1.ViewScope{
	// 			Group:     "config.openshift.io",
	// 			Kind:      "ClusterVersion",
	// 			Name:      "version",
	// 			Namespace: "",
	// 			Version:   "v1",
	// 		},
	// 	},
	// 	Status: managedclusterviewv1beta1.ViewStatus{
	// 		Result: runtime.RawExtension{
	// 			Raw: b,
	// 		},
	// 	},
	// }

	client := clientfake.NewFakeClientWithScheme(s, []runtime.Object{
		clustercurator, managedclusterinfo,
	}...)

	managedclusterview := getManagedClusterView()

	go func() {
		for i := 0; i < 10; i++ {
			managedclusterview.Status.Result.Raw = b
			resultmcview := managedclusterviewv1beta1.ManagedClusterView{}
			client.Get(context.TODO(), types.NamespacedName{
				Namespace: ClusterName,
				Name:      ClusterName,
			}, &resultmcview)
			fmt.Println("mcv: ", resultmcview)
			err := client.Status().Patch(context.TODO(), managedclusterview, client.MergeFrom(resultmcview))
			fmt.Println("err: ", err)
		}
	}()

	// resultmcview := managedclusterviewv1beta1.ManagedClusterView{}
	// client.Get(context.TODO(), types.NamespacedName{
	// 	Namespace: ClusterName,
	// 	Name:      ClusterName,
	// }, &resultmcview)
	// fmt.Println("mcv: ", resultmcview)

	assert.Nil(t, UpgradeCluster(client, ClusterName, clustercurator))

}*/
