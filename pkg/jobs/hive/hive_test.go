// Copyright Contributors to the Open Cluster Management project.
package hive

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	clustercuratorv1 "github.com/open-cluster-management/cluster-curator-controller/pkg/api/v1beta1"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/utils"
	managedclusteractionv1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/action/v1beta1"
	managedclusterinfov1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/internal.open-cluster-management.io/v1beta1"
	managedclusterviewv1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/view/v1beta1"
	clusterversionv1 "github.com/openshift/api/config/v1"
	hivev1 "github.com/openshift/hive/pkg/apis/hive/v1"
	hivefake "github.com/openshift/hive/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	clientv1 "sigs.k8s.io/controller-runtime/pkg/client"
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

func getManagedClusterInfo() *managedclusterinfov1beta1.ManagedClusterInfo {
	return &managedclusterinfov1beta1.ManagedClusterInfo{
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
					VersionAvailableUpdates: []managedclusterinfov1beta1.OCPVersionRelease{
						{
							Version:  "4.5.14",
							Channels: []string{"stable-4.6", "stable-4.7"},
							URL:      "https://access.redhat.com/errata",
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
		clustercurator, getManagedClusterInfo(),
	}...)

	assert.Equal(t, UpgradeCluster(client, ClusterName, clustercurator),
		errors.New("Provide valid upgrade version or channel or upstream"))
}

func TestUpgradeClusterInValidVersion(t *testing.T) {

	s := scheme.Scheme
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(managedclusterinfov1beta1.GroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})

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
		clustercurator, getManagedClusterInfo(),
	}...)

	assert.Equal(t, UpgradeCluster(client, ClusterName, clustercurator),
		errors.New("Provided version is not valid"), "Invalid Version")
}

func TestUpgradeClusterInValidChannel(t *testing.T) {

	s := scheme.Scheme
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(managedclusterinfov1beta1.GroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})

	clustercurator := &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "upgrade",
			Upgrade: clustercuratorv1.UpgradeHooks{
				DesiredUpdate: "4.5.14",
				Channel:       "stable-4.5",
			},
		},
	}

	client := clientfake.NewFakeClientWithScheme(s, []runtime.Object{
		clustercurator, getManagedClusterInfo(),
	}...)

	assert.Equal(t, UpgradeCluster(client, ClusterName, clustercurator),
		errors.New("Provided channel is not valid"), "Invalid Channel")
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

func TestUpgradeClusterMCVExists(t *testing.T) {

	s := scheme.Scheme
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(managedclusterinfov1beta1.GroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})
	s.AddKnownTypes(managedclusteractionv1beta1.GroupVersion, &managedclusteractionv1beta1.ManagedClusterAction{})
	s.AddKnownTypes(managedclusterviewv1beta1.GroupVersion, &managedclusterviewv1beta1.ManagedClusterView{})

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

	managedclusterview := &managedclusterviewv1beta1.ManagedClusterView{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Spec: managedclusterviewv1beta1.ViewSpec{
			Scope: managedclusterviewv1beta1.ViewScope{
				Group:     "config.openshift.io",
				Kind:      "ClusterVersion",
				Name:      "version",
				Namespace: "",
				Version:   "v1",
			},
		},
	}

	client := clientfake.NewFakeClientWithScheme(s, []runtime.Object{
		clustercurator, getManagedClusterInfo(), managedclusterview,
	}...)

	assert.NotNil(t, UpgradeCluster(client, ClusterName, clustercurator), "err not nil when managedclusterview already exists")
}

func TestUpgradeCluster(t *testing.T) {

	s := scheme.Scheme
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(managedclusterinfov1beta1.GroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})
	s.AddKnownTypes(managedclusteractionv1beta1.GroupVersion, &managedclusteractionv1beta1.ManagedClusterAction{})
	s.AddKnownTypes(managedclusterviewv1beta1.GroupVersion, &managedclusterviewv1beta1.ManagedClusterView{})

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
			AvailableUpdates: []clusterversionv1.Update{
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

	client := clientfake.NewFakeClientWithScheme(s, []runtime.Object{
		clustercurator, getManagedClusterInfo(),
	}...)

	go func() {
		for i := 0; i < 10; i++ {
			resultmcview := managedclusterviewv1beta1.ManagedClusterView{}
			client.Get(context.TODO(), types.NamespacedName{
				Namespace: ClusterName,
				Name:      ClusterName,
			}, &resultmcview)
			resultmcview.Status.Result.Raw = b
			client.Update(context.TODO(), &resultmcview)
			updatedresultmcview := managedclusterviewv1beta1.ManagedClusterView{}
			client.Get(context.TODO(), types.NamespacedName{
				Namespace: ClusterName,
				Name:      ClusterName,
			}, &updatedresultmcview)
			if updatedresultmcview.Status.Result.Raw != nil {
				break
			}
		}
	}()
	go func() {
		for i := 0; i < 10; i++ {
			time.Sleep(1 * time.Second)
			resultmca := managedclusteractionv1beta1.ManagedClusterAction{}
			err := client.Get(context.TODO(), types.NamespacedName{
				Namespace: ClusterName,
				Name:      ClusterName,
			}, &resultmca)

			if err == nil {
				patch := []byte(`{"status":{"conditions":[
							{
								"lastTransitionTime": "2021-04-28T16:19:38Z",
								"message": " Resource action is done.",
								"reason": "ActionDone",
								"status": "True",
								"type": "Completed"
							}]}}`)
				client.Patch(context.Background(), &resultmca, clientv1.RawPatch(types.MergePatchType, patch))
				break
			}
		}
	}()

	assert.Nil(t, UpgradeCluster(client, ClusterName, clustercurator), "Upgrade started successfully")
}

func TestUpgradeClusterWithChannelUpstream(t *testing.T) {

	s := scheme.Scheme
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(managedclusterinfov1beta1.GroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})
	s.AddKnownTypes(managedclusteractionv1beta1.GroupVersion, &managedclusteractionv1beta1.ManagedClusterAction{})
	s.AddKnownTypes(managedclusterviewv1beta1.GroupVersion, &managedclusterviewv1beta1.ManagedClusterView{})

	clustercurator := &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "upgrade",
			Upgrade: clustercuratorv1.UpgradeHooks{
				DesiredUpdate: "4.5.14",
				Channel:       "stable-4.7",
				Upstream:      "https://access.redhat.com/errata",
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
			AvailableUpdates: []clusterversionv1.Update{
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

	client := clientfake.NewFakeClientWithScheme(s, []runtime.Object{
		clustercurator, getManagedClusterInfo(),
	}...)

	go func() {
		for i := 0; i < 10; i++ {
			resultmcview := managedclusterviewv1beta1.ManagedClusterView{}
			client.Get(context.TODO(), types.NamespacedName{
				Namespace: ClusterName,
				Name:      ClusterName,
			}, &resultmcview)
			resultmcview.Status.Result.Raw = b
			client.Update(context.TODO(), &resultmcview)
			updatedresultmcview := managedclusterviewv1beta1.ManagedClusterView{}
			client.Get(context.TODO(), types.NamespacedName{
				Namespace: ClusterName,
				Name:      ClusterName,
			}, &updatedresultmcview)
			if updatedresultmcview.Status.Result.Raw != nil {
				break
			}
		}
	}()
	go func() {
		for i := 0; i < 10; i++ {
			time.Sleep(1 * time.Second)
			resultmca := managedclusteractionv1beta1.ManagedClusterAction{}
			err := client.Get(context.TODO(), types.NamespacedName{
				Namespace: ClusterName,
				Name:      ClusterName,
			}, &resultmca)

			if err == nil {
				patch := []byte(`{"status":{"conditions":[
							{
								"lastTransitionTime": "2021-04-28T16:19:38Z",
								"message": " Resource action is done.",
								"reason": "ActionDone",
								"status": "True",
								"type": "Completed"
							}]}}`)
				client.Patch(context.Background(), &resultmca, clientv1.RawPatch(types.MergePatchType, patch))
				break
			}
		}
	}()

	assert.Nil(t, UpgradeCluster(client, ClusterName, clustercurator), "Upgrade started successfully")
}

func TestMonitorUpgradeStatusJobComplete(t *testing.T) {

	cc := getUpgradeClusterCurator()

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
			Conditions: []clusterversionv1.ClusterOperatorStatusCondition{
				{
					LastTransitionTime: v1.NewTime(time.Now()),
					Message:            "Done applying 4.5.12",
					Status:             "True",
					Type:               "Available",
				},
				{
					LastTransitionTime: v1.NewTime(time.Now()),
					Message:            "Working towards 4.5.13: 28% complete",
					Status:             "True",
					Type:               "Progressing",
				},
			},
		},
	}
	b, _ := json.Marshal(clusterversion)
	managedclusterview := &managedclusterviewv1beta1.ManagedClusterView{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
			Labels: map[string]string{
				MCVUpgradeLabel: ClusterName,
			},
		},
		Spec: managedclusterviewv1beta1.ViewSpec{
			Scope: managedclusterviewv1beta1.ViewScope{
				Group:     "config.openshift.io",
				Kind:      "ClusterVersion",
				Name:      "version",
				Namespace: "",
				Version:   "v1",
			},
		},
		Status: managedclusterviewv1beta1.ViewStatus{
			Result: runtime.RawExtension{
				Raw: b,
			},
		},
	}

	updatedclusterversion := &clusterversionv1.ClusterVersion{
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
			Conditions: []clusterversionv1.ClusterOperatorStatusCondition{
				{
					LastTransitionTime: v1.NewTime(time.Now()),
					Message:            "Done applying 4.5.13",
					Status:             "True",
					Type:               "Available",
				},
				{
					LastTransitionTime: v1.NewTime(time.Now()),
					Message:            "Working towards 4.5.13: 28% complete",
					Status:             "True",
					Type:               "Progressing",
				},
			},
		},
	}
	b1, _ := json.Marshal(updatedclusterversion)

	s := scheme.Scheme
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewFakeClientWithScheme(s, []runtime.Object{
		cc, managedclusterview,
	}...)

	// Put a delay to complete the job to test the wait loop
	go func() {
		resultmcview := managedclusterviewv1beta1.ManagedClusterView{}
		client.Get(context.TODO(), types.NamespacedName{
			Namespace: ClusterName,
			Name:      ClusterName,
		}, &resultmcview)
		resultmcview.Status.Result.Raw = b1
		client.Update(context.TODO(), &resultmcview)
	}()

	assert.Nil(t, MonitorUpgradeStatus(client, ClusterName, cc), "err is nil, when cluster upgrade is successful")
}
