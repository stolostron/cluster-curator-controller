// Copyright Contributors to the Open Cluster Management project.
package hive

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	clusterversionv1 "github.com/openshift/api/config/v1"
	hivev1 "github.com/openshift/hive/apis/hive/v1"
	clustercuratorv1 "github.com/stolostron/cluster-curator-controller/pkg/api/v1beta1"
	"github.com/stolostron/cluster-curator-controller/pkg/jobs/utils"
	managedclusteractionv1beta1 "github.com/stolostron/cluster-lifecycle-api/action/v1beta1"
	managedclusterinfov1beta1 "github.com/stolostron/cluster-lifecycle-api/clusterinfo/v1beta1"
	managedclusterviewv1beta1 "github.com/stolostron/cluster-lifecycle-api/view/v1beta1"
	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog"
	clientv1 "sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const ClusterName = "my-cluster"
const testTimeout = 6

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
					{
						Name: "Service now App Update",
						ExtraVars: &runtime.RawExtension{
							Raw: []byte(`{"variable1": "1","variable2": "2"}`),
						},
					},
				},
				Posthook: []clustercuratorv1.Hook{
					{
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
					{
						Name: "Service now App Update",
						ExtraVars: &runtime.RawExtension{
							Raw: []byte(`{"variable1": "1","variable2": "2"}`),
						},
					},
				},
				Posthook: []clustercuratorv1.Hook{
					{
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
			APIVersion: managedclusterinfov1beta1.SchemeGroupVersion.String(),
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
					Desired: managedclusterinfov1beta1.OCPVersionRelease{
						Version:  "4.5.14",
						Channels: []string{"stable-4.6", "stable-4.7"},
						URL:      "https://access.redhat.com/errata",
					},
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

	s := scheme.Scheme
	hivev1.AddToScheme(s)
	hiveset := clientfake.NewClientBuilder().WithScheme(s).Build()

	t.Log("No ClusterDeployment")
	assert.NotNil(t, ActivateDeploy(hiveset, ClusterName),
		"err NotNil when ClusterDeployment kind does not exist")
}

func TestActivateDeployNoPauseAnnotation(t *testing.T) {

	s := scheme.Scheme
	hivev1.AddToScheme(s)
	hiveset := clientfake.NewClientBuilder().WithRuntimeObjects(&hivev1.ClusterDeployment{
		TypeMeta: v1.TypeMeta{
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
	}).WithScheme(s).Build()

	t.Log("ClusterDeployment with no Pause annotation")
	assert.Nil(t, ActivateDeploy(hiveset, ClusterName),
		"err Nil when ClusterDeployment has no Pause annotation")
}

func TestActivateDeployNoneTruePauseAnnotation(t *testing.T) {
	s := scheme.Scheme
	hivev1.AddToScheme(s)
	hiveset := clientfake.NewClientBuilder().WithRuntimeObjects(&hivev1.ClusterDeployment{
		TypeMeta: v1.TypeMeta{
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:        ClusterName,
			Namespace:   ClusterName,
			Annotations: map[string]string{"hive.openshift.io/reconcile-pause": "false"},
		},
	}).WithScheme(s).Build()

	assert.Nil(t, ActivateDeploy(hiveset, ClusterName),
		"err Nil when ClusterDeployment with non true pause annotation")
}

func TestActivateDeploy(t *testing.T) {
	s := scheme.Scheme
	hivev1.AddToScheme(s)
	hiveset := clientfake.NewClientBuilder().WithRuntimeObjects(&hivev1.ClusterDeployment{
		TypeMeta: v1.TypeMeta{
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:        ClusterName,
			Namespace:   ClusterName,
			Annotations: map[string]string{"hive.openshift.io/reconcile-pause": "true"},
		},
	}).WithScheme(s).Build()

	t.Log("ClusterDeployment with true pause annotation")
	assert.Nil(t, ActivateDeploy(hiveset, ClusterName),
		"err Nil when ClusterDeployment with true pause annotation")
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

func getUninstallJob() *batchv1.Job {
	return &batchv1.Job{
		TypeMeta: v1.TypeMeta{
			Kind:       "job",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName + "-uninstall",
			Namespace: ClusterName,
		},
		Status: batchv1.JobStatus{
			Active:    0,
			Succeeded: 1,
		},
	}
}

func TestMonitorDeployStatusNoClusterDeployment(t *testing.T) {

	hiveset := clientfake.NewClientBuilder().Build()

	assert.NotNil(t, monitorClusterStatus(hiveset, ClusterName, utils.Installing, testTimeout),
		"err is not nil, when cluster provisioning has a condition")
}

func TestMonitorDeployStatusProvisionStoppedCondition(t *testing.T) {

	cd := getClusterDeployment()
	cd.Status.Conditions = []hivev1.ClusterDeploymentCondition{
		{
			Type:   hivev1.ProvisionStoppedCondition,
			Status: "True",
		},
	}

	hiveset := clientfake.NewClientBuilder().WithRuntimeObjects(cd).Build()

	assert.NotNil(t, monitorClusterStatus(hiveset, ClusterName, utils.Installing, testTimeout),
		"err is not nil, when cluster provisioning has a condition")
}

func TestMonitorDeployStatusRequirementsMetCondition(t *testing.T) {

	cd := getClusterDeployment()
	cd.Status.Conditions = []hivev1.ClusterDeploymentCondition{
		{
			Type:   hivev1.RequirementsMetCondition,
			Status: "False",
		},
	}

	hiveset := clientfake.NewClientBuilder().WithRuntimeObjects(cd).Build()

	assert.NotNil(t, monitorClusterStatus(hiveset, ClusterName, utils.Installing, testTimeout),
		"err is not nil, when cluster provisioning has a condition")
}

func TestMonitorDeployNoJobTimeout(t *testing.T) {

	cd := getClusterDeployment()

	hiveset := clientfake.NewClientBuilder().WithRuntimeObjects(cd).Build()

	assert.NotNil(t, monitorClusterStatus(hiveset, ClusterName, utils.Installing, testTimeout),
		"err is not nil, when cluster provisioning has no job created")
}
func TestMonitorDeployStatusJobFailed(t *testing.T) {

	cd := getClusterDeployment()
	cd.Status.ProvisionRef = &corev1.LocalObjectReference{
		Name: ClusterName + "-12345", // The monitor adds the -provision suffix
	}

	job := getProvisionJob()
	job.Status.Succeeded = 0

	cc := getClusterCurator()

	s := scheme.Scheme
	hivev1.AddToScheme(s)
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})

	client := clientfake.NewClientBuilder().WithRuntimeObjects(cd, cc, job).WithScheme(s).Build()

	assert.NotNil(t, monitorClusterStatus(client, ClusterName, utils.Installing, testTimeout),
		"err is not nil, when cluster provisioning has a condition")
}

func TestMonitorDeployStatusJobCompletedWithSuccess(t *testing.T) {

	cd := getClusterDeployment()
	cd.Status.Conditions = []hivev1.ClusterDeploymentCondition{
		{
			Message: "Provisioned",
			Reason:  "SuccessfulProvision",
		},
	}
	cd.Status.ProvisionRef = &corev1.LocalObjectReference{
		Name: ClusterName + "-12345", // The monitor adds the -provision suffix
	}
	cc := getClusterCurator()

	s := scheme.Scheme
	hivev1.AddToScheme(s)
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})

	client := clientfake.NewClientBuilder().WithRuntimeObjects(cd, cc).WithScheme(s).Build()

	// Put a delay to complete the ClusterDeployment to test the wait loop
	go func() {
		time.Sleep(utils.PauseFiveSeconds)
		cd.Status.WebConsoleURL = "https://my-cluster"

		client.Update(context.TODO(), cd)
		t.Log("ClusterDeployment webConsoleURL applied")
	}()

	assert.Equal(t, len(cc.Status.Conditions), 0, "Should be emtpy")
	assert.Nil(t, monitorClusterStatus(client, ClusterName, utils.Installing, testTimeout),
		"err is nil, when cluster provisioning is successful")

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

	s := scheme.Scheme
	hivev1.AddToScheme(s)
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})

	client := clientfake.NewClientBuilder().WithRuntimeObjects(cd, cc).WithScheme(s).Build()

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
		client.Update(context.TODO(), cd)
	}()

	assert.Nil(t, monitorClusterStatus(client, ClusterName, utils.Installing, testTimeout),
		"err is not nil, when cluster provisioning is successful")
}

func TestMonitorDeployStatusJobDelayedComplete(t *testing.T) {

	cd := getClusterDeployment()
	cd.Status.ProvisionRef = &corev1.LocalObjectReference{
		Name: ClusterName + "-12345", // The monitor adds the -provision suffix
	}

	s := scheme.Scheme
	hivev1.AddToScheme(s)
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})

	client := clientfake.NewClientBuilder().WithRuntimeObjects(cd, getClusterCurator()).WithScheme(s).Build()

	// Put a delay to complete the job to test the wait loop
	go func() {
		time.Sleep(utils.PauseFiveSeconds)
		cd.Status.WebConsoleURL = "https://my-cluster"
		cd.Status.Conditions = []hivev1.ClusterDeploymentCondition{
			{
				Message: "Provisioned",
				Reason:  "SuccessfulProvision",
			},
		}
		t.Log("Created the Job resource and update Cluster Deployment as complete")
		client.Create(context.Background(), getProvisionJob())
		client.Update(context.TODO(), cd)
	}()

	assert.Nil(t, monitorClusterStatus(client, ClusterName, utils.Installing, testTimeout),
		"err is nil, when cluster provisioning is successful")
}

func TestMonitorDestroyStatusJobDelayedComplete(t *testing.T) {

	cd := getClusterDeployment()
	cd.Status.ProvisionRef = &corev1.LocalObjectReference{
		Name: ClusterName + "-12345", // The monitor adds the -provision suffix
	}

	s := scheme.Scheme
	hivev1.AddToScheme(s)
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})

	client := clientfake.NewClientBuilder().WithRuntimeObjects(cd, getClusterCurator()).WithScheme(s).Build()

	// Put a delay to complete the job to test the wait loop
	go func() {
		time.Sleep(utils.PauseFiveSeconds)
		cd.Status.WebConsoleURL = "https://my-cluster"
		cd.Status.Conditions = []hivev1.ClusterDeploymentCondition{
			{
				Message: "Provisioned",
				Reason:  "SuccessfulProvision",
			},
		}
		t.Log("Created the Job resource")
		uninstallJob := getUninstallJob()
		client.Create(context.Background(), uninstallJob)
		// time.Sleep(utils.PauseTenSeconds)
		// client.Delete(context.Background(), uninstallJob)
	}()

	assert.Nil(t, monitorClusterStatus(client, ClusterName, utils.Destroying, testTimeout),
		"err is nil, when cluster uninstall is successful")
}

func TestMonitorDestroyClusterDeployementMissing(t *testing.T) {
	cd := getClusterDeployment()

	s := scheme.Scheme
	hivev1.AddToScheme(s)

	client := clientfake.NewClientBuilder().WithRuntimeObjects(cd).WithScheme(s).Build()

	// Should not return an error, just logs a warning
	assert.Nil(t, DestroyClusterDeployment(client, cd.GetName()))
}

func TestMonitorDestroyClusterDeployement(t *testing.T) {
	cd := getClusterDeployment()
	s := scheme.Scheme
	hivev1.AddToScheme(s)

	client := clientfake.NewClientBuilder().WithRuntimeObjects(cd).WithScheme(s).Build()
	// Should not return an error, just logs a warning
	assert.Nil(t, DestroyClusterDeployment(client, cd.GetName()))

	err := client.Get(context.TODO(), types.NamespacedName{
		Name:      cd.GetName(),
		Namespace: cd.GetName(),
	}, cd)
	assert.Contains(t, err.Error(), " not found")
}

func TestMonitorDestroyStatusJobDelayedDeleteOnFinish(t *testing.T) {

	cd := getClusterDeployment()
	cd.Status.ProvisionRef = &corev1.LocalObjectReference{
		Name: ClusterName + "-12345", // The monitor adds the -provision suffix
	}

	s := scheme.Scheme
	hivev1.AddToScheme(s)
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})

	client := clientfake.NewClientBuilder().WithRuntimeObjects(cd, getClusterCurator()).WithScheme(s).Build()

	// Put a delay to complete the job to test the wait loop
	go func() {
		time.Sleep(utils.PauseFiveSeconds)
		cd.Status.WebConsoleURL = "https://my-cluster"
		cd.Status.Conditions = []hivev1.ClusterDeploymentCondition{
			{
				Message: "Provisioned",
				Reason:  "SuccessfulProvision",
			},
		}
		t.Log("Created the Job resource")
		uninstallJob := getUninstallJob()
		uninstallJob.Status.Active = int32(1)
		uninstallJob.Status.Succeeded = int32(0)
		client.Create(context.Background(), uninstallJob)
		time.Sleep(utils.PauseTenSeconds)
		client.Delete(context.Background(), uninstallJob)
	}()

	assert.Nil(t, monitorClusterStatus(client, ClusterName, utils.Destroying, testTimeout),
		"err is nil, when cluster uninstall is successful")
}

func TestUpgradeClusterNonOpenshift(t *testing.T) {

	s := scheme.Scheme
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(managedclusterinfov1beta1.SchemeGroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})

	managedclusterinfo := &managedclusterinfov1beta1.ManagedClusterInfo{
		TypeMeta: v1.TypeMeta{
			APIVersion: managedclusterinfov1beta1.SchemeGroupVersion.String(),
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

	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(getUpgradeClusterCurator(), managedclusterinfo).Build()

	assert.Equal(t, UpgradeCluster(client, ClusterName, getUpgradeClusterCurator()),
		errors.New("Can not upgrade non openshift cluster"))
}

func TestUpgradeClusterNoDesiredUpdate(t *testing.T) {

	s := scheme.Scheme
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(managedclusterinfov1beta1.SchemeGroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})

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

	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clustercurator, getManagedClusterInfo()).Build()

	assert.Equal(t, UpgradeCluster(client, ClusterName, clustercurator),
		errors.New("Provide valid upgrade version or channel or upstream"))
}

func TestUpgradeClusterInValidVersion(t *testing.T) {

	s := scheme.Scheme
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(managedclusterinfov1beta1.SchemeGroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})

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

	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clustercurator, getManagedClusterInfo()).Build()

	assert.Equal(t, UpgradeCluster(client, ClusterName, clustercurator),
		errors.New("Provided version is not valid"), "Invalid Version")
}

func TestUpgradeClusterInValidChannel(t *testing.T) {

	s := scheme.Scheme
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(managedclusterinfov1beta1.SchemeGroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})

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

	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clustercurator, getManagedClusterInfo()).Build()

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
	s.AddKnownTypes(managedclusterinfov1beta1.SchemeGroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})
	s.AddKnownTypes(managedclusteractionv1beta1.SchemeGroupVersion, &managedclusteractionv1beta1.ManagedClusterAction{})
	s.AddKnownTypes(managedclusterviewv1beta1.SchemeGroupVersion, &managedclusterviewv1beta1.ManagedClusterView{})

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

	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clustercurator, getManagedClusterInfo(), managedclusterview).Build()

	assert.NotNil(t, UpgradeCluster(client, ClusterName, clustercurator), "err not nil when managedclusterview already exists")
}

func TestUpgradeCluster(t *testing.T) {

	s := scheme.Scheme
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(managedclusterinfov1beta1.SchemeGroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})
	s.AddKnownTypes(managedclusteractionv1beta1.SchemeGroupVersion, &managedclusteractionv1beta1.ManagedClusterAction{})
	s.AddKnownTypes(managedclusterviewv1beta1.SchemeGroupVersion, &managedclusterviewv1beta1.ManagedClusterView{})

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

	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clustercurator, getManagedClusterInfo()).Build()

	go func() {
		for i := 0; i < 10; i++ {
			time.Sleep(1 * time.Second)
			resultmcview := managedclusterviewv1beta1.ManagedClusterView{}
			err := client.Get(context.TODO(), types.NamespacedName{
				Namespace: ClusterName,
				Name:      ClusterName,
			}, &resultmcview)
			if err != nil {
				klog.Error("failed to get managedClusterview.", err)
				continue
			}
			resultmcview.Status.Result.Raw = b
			err = client.Update(context.TODO(), &resultmcview)
			if err != nil {
				klog.Error("failed to update managedClusterview.", err)
				continue
			}
			updatedresultmcview := managedclusterviewv1beta1.ManagedClusterView{}
			err = client.Get(context.TODO(), types.NamespacedName{
				Namespace: ClusterName,
				Name:      ClusterName,
			}, &updatedresultmcview)
			if err != nil {
				klog.Error("failed to get managedClusterview.", err)
				continue
			}
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
	s.AddKnownTypes(managedclusterinfov1beta1.SchemeGroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})
	s.AddKnownTypes(managedclusteractionv1beta1.SchemeGroupVersion, &managedclusteractionv1beta1.ManagedClusterAction{})
	s.AddKnownTypes(managedclusterviewv1beta1.SchemeGroupVersion, &managedclusterviewv1beta1.ManagedClusterView{})

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

	client := clientfake.NewClientBuilder().WithRuntimeObjects(clustercurator, getManagedClusterInfo()).WithScheme(s).Build()

	go func() {
		for i := 0; i < 10; i++ {
			time.Sleep(1 * time.Second)
			resultmcview := managedclusterviewv1beta1.ManagedClusterView{}
			err := client.Get(context.TODO(), types.NamespacedName{
				Namespace: ClusterName,
				Name:      ClusterName,
			}, &resultmcview)
			if err != nil {
				klog.Error("failed to get managedClusterview.", err)
				continue
			}
			resultmcview.Status.Result.Raw = b
			err = client.Update(context.TODO(), &resultmcview)
			if err != nil {
				klog.Error("failed to update managedClusterview.", err)
				continue
			}
			updatedresultmcview := managedclusterviewv1beta1.ManagedClusterView{}
			err = client.Get(context.TODO(), types.NamespacedName{
				Namespace: ClusterName,
				Name:      ClusterName,
			}, &updatedresultmcview)
			if err != nil {
				klog.Error("failed to get managedClusterview.", err)
				continue
			}
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
	s.AddKnownTypes(managedclusterviewv1beta1.SchemeGroupVersion, &managedclusterviewv1beta1.ManagedClusterView{})
	client := clientfake.NewClientBuilder().WithRuntimeObjects([]runtime.Object{cc, managedclusterview}...).WithScheme(s).Build()

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

	assert.Nil(t, MonitorUpgradeStatus(client, ClusterName, cc, false), "err is nil, when cluster upgrade is successful")
}

func TestUpgradeClusterForceUpgradeCSVHasDesiredUpdate(t *testing.T) {

	s := scheme.Scheme
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(managedclusterinfov1beta1.SchemeGroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})
	s.AddKnownTypes(managedclusteractionv1beta1.SchemeGroupVersion, &managedclusteractionv1beta1.ManagedClusterAction{})
	s.AddKnownTypes(managedclusterviewv1beta1.SchemeGroupVersion, &managedclusterviewv1beta1.ManagedClusterView{})

	clustercurator := &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
			Annotations: map[string]string{
				"cluster.open-cluster-management.io/upgrade-allow-not-recommended-versions": "true",
			},
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "upgrade",
			Upgrade: clustercuratorv1.UpgradeHooks{
				DesiredUpdate: "4.5.14",
			},
		},
	}

	currentClusterVersion := &clusterversionv1.ClusterVersion{
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
			DesiredUpdate: &clusterversionv1.Update{
				Image:   "quay.io/openshift-release-dev/ocp-release@sha256:71e158c6173ad6aa6e356c119a87459196bbe70e89c0db1e35c1f63a87d90676",
				Version: "4.5.10",
			},
		},
		Status: clusterversionv1.ClusterVersionStatus{
			AvailableUpdates: []clusterversionv1.Release{
				{
					Version: "4.5.10",
					Image:   "quay.io/openshift-release-dev/ocp-release@sha256:71e158c6173ad6aa6e356c119a87459196bbe70e89c0db1e35c1f63a87d90676",
				},
			},
		},
	}

	b, _ := json.Marshal(currentClusterVersion)

	client := clientfake.NewClientBuilder().WithRuntimeObjects([]runtime.Object{
		clustercurator, getManagedClusterInfo(),
	}...).WithScheme(s).Build()

	go func() {
		for i := 0; i < 10; i++ {
			time.Sleep(1 * time.Second)
			resultmcview := managedclusterviewv1beta1.ManagedClusterView{}
			err := client.Get(context.TODO(), types.NamespacedName{
				Namespace: ClusterName,
				Name:      ClusterName,
			}, &resultmcview)
			if err != nil {
				klog.Error("failed to get managedClusterview.", err)
				continue
			}
			resultmcview.Status.Result.Raw = b
			err = client.Update(context.TODO(), &resultmcview)
			if err != nil {
				klog.Error("failed to update managedClusterview.", err)
				continue
			}
			updatedresultmcview := managedclusterviewv1beta1.ManagedClusterView{}
			err = client.Get(context.TODO(), types.NamespacedName{
				Namespace: ClusterName,
				Name:      ClusterName,
			}, &updatedresultmcview)
			if err != nil {
				klog.Error("failed to get managedClusterview.", err)
				continue
			}
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

	assert.Nil(t, UpgradeCluster(client, ClusterName, clustercurator), "Upgrade started successfully to non-recommended version")
}

func TestUpgradeClusterForceUpgradeCSVNoDesiredUpdate(t *testing.T) {

	s := scheme.Scheme
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(managedclusterinfov1beta1.SchemeGroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})
	s.AddKnownTypes(managedclusteractionv1beta1.SchemeGroupVersion, &managedclusteractionv1beta1.ManagedClusterAction{})
	s.AddKnownTypes(managedclusterviewv1beta1.SchemeGroupVersion, &managedclusterviewv1beta1.ManagedClusterView{})

	clustercurator := &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
			Annotations: map[string]string{
				"cluster.open-cluster-management.io/upgrade-allow-not-recommended-versions": "true",
			},
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "upgrade",
			Upgrade: clustercuratorv1.UpgradeHooks{
				DesiredUpdate: "4.5.14",
			},
		},
	}

	currentClusterVersion := &clusterversionv1.ClusterVersion{
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
					Version: "4.5.10",
					Image:   "quay.io/openshift-release-dev/ocp-release@sha256:71e158c6173ad6aa6e356c119a87459196bbe70e89c0db1e35c1f63a87d90676",
				},
			},
		},
	}

	b, _ := json.Marshal(currentClusterVersion)

	client := clientfake.NewClientBuilder().WithRuntimeObjects([]runtime.Object{
		clustercurator, getManagedClusterInfo(),
	}...).WithScheme(s).Build()

	go func() {
		for i := 0; i < 100; i++ {
			time.Sleep(1 * time.Second)
			resultmcview := managedclusterviewv1beta1.ManagedClusterView{}
			err := client.Get(context.TODO(), types.NamespacedName{
				Namespace: ClusterName,
				Name:      ClusterName,
			}, &resultmcview)
			if err != nil {
				klog.Error("failed to get managedClusterview.", err)
				continue
			}
			resultmcview.Status.Result.Raw = b
			err = client.Update(context.TODO(), &resultmcview)
			if err != nil {
				klog.Error("failed to update managedClusterview.", err)
				continue
			}
			updatedresultmcview := managedclusterviewv1beta1.ManagedClusterView{}
			err = client.Get(context.TODO(), types.NamespacedName{
				Namespace: ClusterName,
				Name:      ClusterName,
			}, &updatedresultmcview)
			if err != nil {
				klog.Error("failed to get managedClusterview.", err)
				continue
			}
			if updatedresultmcview.Status.Result.Raw != nil {
				break
			}
		}
	}()

	go func() {
		for i := 0; i < 100; i++ {
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

	assert.Nil(t, UpgradeCluster(client, ClusterName, clustercurator), "Upgrade started successfully to non-recommended version")
}

func TestEUSIntermediateUpgrade(t *testing.T) {
	s := scheme.Scheme
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(managedclusterinfov1beta1.SchemeGroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})
	s.AddKnownTypes(managedclusteractionv1beta1.SchemeGroupVersion, &managedclusteractionv1beta1.ManagedClusterAction{})
	s.AddKnownTypes(managedclusterviewv1beta1.SchemeGroupVersion, &managedclusterviewv1beta1.ManagedClusterView{})
	s.AddKnownTypes(corev1.SchemeGroupVersion, &corev1.ConfigMap{})
	s.AddKnownTypes(managedclusteractionv1beta1.SchemeGroupVersion, &managedclusteractionv1beta1.ManagedClusterAction{})

	clustercurator := &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
			Annotations: map[string]string{
				"cluster.open-cluster-management.io/upgrade-clusterversion-backoff-limit": "10",
			},
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "upgrade",
			Upgrade: clustercuratorv1.UpgradeHooks{
				IntermediateUpdate: "4.13.37",
				DesiredUpdate:      "4.14.16",
			},
		},
	}

	ocpConfigMap := &corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name:      "admin-acks",
			Namespace: "openshift-config",
		},
		Data: map[string]string{},
	}

	configMapBytes, _ := json.Marshal(ocpConfigMap)

	ocpConfigMCV := &managedclusterviewv1beta1.ManagedClusterView{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName + "admack",
			Namespace: ClusterName,
			Labels: map[string]string{
				"cluster-curator-upgrade": "feng-managed",
			},
		},
		Spec: managedclusterviewv1beta1.ViewSpec{
			Scope: managedclusterviewv1beta1.ViewScope{
				Kind:      "ConfigMap",
				Name:      "admin-acks",
				Namespace: "openshift-config",
				Version:   "v1",
			},
		},
		Status: managedclusterviewv1beta1.ViewStatus{
			Conditions: []v1.Condition{
				{
					Message: "Watching resources successfully",
					Reason:  "GetResourceProcessing",
					Status:  "True",
					Type:    "Processing",
				},
			},
			Result: runtime.RawExtension{
				Raw: configMapBytes,
			},
		},
	}

	currentClusterVersion := &clusterversionv1.ClusterVersion{
		TypeMeta: v1.TypeMeta{
			APIVersion: "config.openshift.io/v1",
			Kind:       "ClusterVersion",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "version",
		},
		Spec: clusterversionv1.ClusterVersionSpec{
			Channel:   "stable-4.12",
			ClusterID: "201ad26c-67d6-416a",
			Upstream:  "https://api.openshift.com/api",
		},
		Status: clusterversionv1.ClusterVersionStatus{
			AvailableUpdates: []clusterversionv1.Release{
				{
					Version: "4.12.15",
					Image:   "quay.io/openshift-release-dev/ocp-release@sha256:71e158c6173ad6aa6e356c119a87459196bbe70e89c0db1e35c1f63a87d90676",
				},
			},
			Desired: clusterversionv1.Release{
				Version: "4.12.14",
			},
		},
	}

	b, _ := json.Marshal(currentClusterVersion)

	clusterVersionMCV := &managedclusterviewv1beta1.ManagedClusterView{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
			Labels: map[string]string{
				"cluster-curator-upgrade": "feng-managed",
			},
		},
		Spec: managedclusterviewv1beta1.ViewSpec{
			Scope: managedclusterviewv1beta1.ViewScope{
				Kind:    "ClusterVersion",
				Name:    "version",
				Version: "v1",
				Group:   "config.openshift.io",
			},
		},
		Status: managedclusterviewv1beta1.ViewStatus{
			Conditions: []v1.Condition{
				{
					Message: "Watching resources successfully",
					Reason:  "GetResourceProcessing",
					Status:  "True",
					Type:    "Processing",
				},
			},
			Result: runtime.RawExtension{
				Raw: b,
			},
		},
	}

	mci := &managedclusterinfov1beta1.ManagedClusterInfo{
		TypeMeta: v1.TypeMeta{
			APIVersion: managedclusterinfov1beta1.SchemeGroupVersion.String(),
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
					AvailableUpdates: []string{"4.12.15", "4.12.16", "4.12.17"},
					Desired: managedclusterinfov1beta1.OCPVersionRelease{
						Version:  "4.12.14",
						Channels: []string{"stable-4.12", "stable-4.13"},
						URL:      "https://access.redhat.com/errata",
					},
					VersionAvailableUpdates: []managedclusterinfov1beta1.OCPVersionRelease{
						{
							Version:  "4.12.15",
							Channels: []string{"stable-4.6", "stable-4.7"},
							URL:      "https://access.redhat.com/errata",
						},
					},
					Version: "4.12.14",
				},
			},
		},
	}

	client := clientfake.NewClientBuilder().WithRuntimeObjects(
		clustercurator,
		mci,
		ocpConfigMCV,
		clusterVersionMCV,
	).WithScheme(s).Build()

	go func() {
		for i := 0; i < 1000; i++ {
			resultmca := managedclusteractionv1beta1.ManagedClusterAction{}
			err := client.Get(context.TODO(), types.NamespacedName{
				Namespace: ClusterName,
				Name:      ClusterName + "admack",
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
			time.Sleep(1 * time.Second)
		}
	}()

	go func() {
		for i := 0; i < 1000; i++ {
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
			time.Sleep(1 * time.Second)
		}
	}()

	assert.Nil(
		t,
		EUSUpgradeCluster(client, ClusterName, clustercurator, true),
		"EUS Intermediate Upgrade started successfully")
}

func TestEUSFinalUpgrade(t *testing.T) {
	s := scheme.Scheme
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(managedclusterinfov1beta1.SchemeGroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})
	s.AddKnownTypes(managedclusteractionv1beta1.SchemeGroupVersion, &managedclusteractionv1beta1.ManagedClusterAction{})
	s.AddKnownTypes(managedclusterviewv1beta1.SchemeGroupVersion, &managedclusterviewv1beta1.ManagedClusterView{})
	s.AddKnownTypes(corev1.SchemeGroupVersion, &corev1.ConfigMap{})
	s.AddKnownTypes(managedclusteractionv1beta1.SchemeGroupVersion, &managedclusteractionv1beta1.ManagedClusterAction{})

	clustercurator := &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
			Annotations: map[string]string{
				"cluster.open-cluster-management.io/upgrade-clusterversion-backoff-limit": "10",
			},
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "upgrade",
			Upgrade: clustercuratorv1.UpgradeHooks{
				IntermediateUpdate: "4.13.37",
				DesiredUpdate:      "4.14.16",
			},
		},
	}

	ocpConfigMap := &corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name:      "admin-acks",
			Namespace: "openshift-config",
		},
		Data: map[string]string{},
	}

	configMapBytes, _ := json.Marshal(ocpConfigMap)

	ocpConfigMCV := &managedclusterviewv1beta1.ManagedClusterView{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName + "admack",
			Namespace: ClusterName,
			Labels: map[string]string{
				"cluster-curator-upgrade": "feng-managed",
			},
		},
		Spec: managedclusterviewv1beta1.ViewSpec{
			Scope: managedclusterviewv1beta1.ViewScope{
				Kind:      "ConfigMap",
				Name:      "admin-acks",
				Namespace: "openshift-config",
				Version:   "v1",
			},
		},
		Status: managedclusterviewv1beta1.ViewStatus{
			Conditions: []v1.Condition{
				{
					Message: "Watching resources successfully",
					Reason:  "GetResourceProcessing",
					Status:  "True",
					Type:    "Processing",
				},
			},
			Result: runtime.RawExtension{
				Raw: configMapBytes,
			},
		},
	}

	currentClusterVersion := &clusterversionv1.ClusterVersion{
		TypeMeta: v1.TypeMeta{
			APIVersion: "config.openshift.io/v1",
			Kind:       "ClusterVersion",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "version",
		},
		Spec: clusterversionv1.ClusterVersionSpec{
			Channel:   "stable-4.12",
			ClusterID: "201ad26c-67d6-416a",
			Upstream:  "https://api.openshift.com/api",
		},
		Status: clusterversionv1.ClusterVersionStatus{
			AvailableUpdates: []clusterversionv1.Release{
				{
					Version: "4.13.38",
					Image:   "quay.io/openshift-release-dev/ocp-release@sha256:71e158c6173ad6aa6e356c119a87459196bbe70e89c0db1e35c1f63a87d90676",
				},
			},
			Desired: clusterversionv1.Release{
				Version: "4.13.37",
			},
		},
	}

	b, _ := json.Marshal(currentClusterVersion)

	clusterVersionMCV := &managedclusterviewv1beta1.ManagedClusterView{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
			Labels: map[string]string{
				"cluster-curator-upgrade": "feng-managed",
			},
		},
		Spec: managedclusterviewv1beta1.ViewSpec{
			Scope: managedclusterviewv1beta1.ViewScope{
				Kind:    "ClusterVersion",
				Name:    "version",
				Version: "v1",
				Group:   "config.openshift.io",
			},
		},
		Status: managedclusterviewv1beta1.ViewStatus{
			Conditions: []v1.Condition{
				{
					Message: "Watching resources successfully",
					Reason:  "GetResourceProcessing",
					Status:  "True",
					Type:    "Processing",
				},
			},
			Result: runtime.RawExtension{
				Raw: b,
			},
		},
	}

	mci := &managedclusterinfov1beta1.ManagedClusterInfo{
		TypeMeta: v1.TypeMeta{
			APIVersion: managedclusterinfov1beta1.SchemeGroupVersion.String(),
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
					AvailableUpdates: []string{"4.13.38", "4.13.39", "4.13.40"},
					Desired: managedclusterinfov1beta1.OCPVersionRelease{
						Version:  "4.13.37",
						Channels: []string{"stable-4.12", "stable-4.13"},
						URL:      "https://access.redhat.com/errata",
					},
					VersionAvailableUpdates: []managedclusterinfov1beta1.OCPVersionRelease{
						{
							Version:  "4.13.38",
							Channels: []string{"stable-4.12", "stable-4.13"},
							URL:      "https://access.redhat.com/errata",
						},
					},
					Version: "4.13.37",
				},
			},
		},
	}

	client := clientfake.NewClientBuilder().WithRuntimeObjects(
		clustercurator,
		mci,
		ocpConfigMCV,
		clusterVersionMCV,
	).WithScheme(s).Build()

	go func() {
		for i := 0; i < 1000; i++ {
			resultmca := managedclusteractionv1beta1.ManagedClusterAction{}
			err := client.Get(context.TODO(), types.NamespacedName{
				Namespace: ClusterName,
				Name:      ClusterName + "admack",
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
			time.Sleep(1 * time.Second)
		}
	}()

	go func() {
		for i := 0; i < 1000; i++ {
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
			time.Sleep(1 * time.Second)
		}
	}()

	assert.Nil(
		t,
		EUSUpgradeCluster(client, ClusterName, clustercurator, false),
		"EUS Final Upgrade started successfully")
}

func TestUpgradeClusterForceUpgradeWithImageDigest(t *testing.T) {

	s := scheme.Scheme
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(managedclusterinfov1beta1.SchemeGroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})
	s.AddKnownTypes(managedclusteractionv1beta1.SchemeGroupVersion, &managedclusteractionv1beta1.ManagedClusterAction{})
	s.AddKnownTypes(managedclusterviewv1beta1.SchemeGroupVersion, &managedclusterviewv1beta1.ManagedClusterView{})

	clustercurator := &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
			Annotations: map[string]string{
				"cluster.open-cluster-management.io/upgrade-allow-not-recommended-versions": "true",
			},
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "upgrade",
			Upgrade: clustercuratorv1.UpgradeHooks{
				DesiredUpdate: "4.5.10",
			},
		},
	}

	currentClusterVersion := &clusterversionv1.ClusterVersion{
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
			DesiredUpdate: &clusterversionv1.Update{
				Image:   "quay.io/openshift-release-dev/ocp-release@sha256:71e158c6173ad6aa6e356c119a87459196bbe70e89c0db1e35c1f63a87d90676",
				Version: "4.5.10",
			},
		},
		Status: clusterversionv1.ClusterVersionStatus{
			AvailableUpdates: []clusterversionv1.Release{
				{
					Version: "4.5.10",
					Image:   "quay.io/openshift-release-dev/ocp-release@sha256:71e158c6173ad6aa6e356c119a87459196bbe70e89c0db1e35c1f63a87d90676",
				},
			},
			ConditionalUpdates: []clusterversionv1.ConditionalUpdate{
				{
					Release: clusterversionv1.Release{
						Channels: []string{
							"stable-4.5",
						},
						Image:   "quay.io/openshift-release-dev/ocp-release@sha256:71e158c6173ad6aa6e356c119a87459196bbe70e89c0db1e35c1f63a87d90676",
						URL:     "'https://access.redhat.com/errata/RHBA-2025:8116",
						Version: "4.5.10",
					},
				},
			},
		},
	}

	b, _ := json.Marshal(currentClusterVersion)

	client := clientfake.NewClientBuilder().WithRuntimeObjects([]runtime.Object{
		clustercurator, getManagedClusterInfo(),
	}...).WithScheme(s).Build()

	go func() {
		for i := 0; i < 10; i++ {
			time.Sleep(1 * time.Second)
			resultmcview := managedclusterviewv1beta1.ManagedClusterView{}
			err := client.Get(context.TODO(), types.NamespacedName{
				Namespace: ClusterName,
				Name:      ClusterName,
			}, &resultmcview)
			if err != nil {
				klog.Error("failed to get managedClusterview.", err)
				continue
			}
			resultmcview.Status.Result.Raw = b
			err = client.Update(context.TODO(), &resultmcview)
			if err != nil {
				klog.Error("failed to update managedClusterview.", err)
				continue
			}
			updatedresultmcview := managedclusterviewv1beta1.ManagedClusterView{}
			err = client.Get(context.TODO(), types.NamespacedName{
				Namespace: ClusterName,
				Name:      ClusterName,
			}, &updatedresultmcview)
			if err != nil {
				klog.Error("failed to get managedClusterview.", err)
				continue
			}
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

	assert.Nil(t, UpgradeCluster(client, ClusterName, clustercurator), "Upgrade started successfully to non-recommended version with image digest")
}

func TestUpgradeClusterForceUpgradeWithImageDigestInAvailableList(t *testing.T) {

	s := scheme.Scheme
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(managedclusterinfov1beta1.SchemeGroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})
	s.AddKnownTypes(managedclusteractionv1beta1.SchemeGroupVersion, &managedclusteractionv1beta1.ManagedClusterAction{})
	s.AddKnownTypes(managedclusterviewv1beta1.SchemeGroupVersion, &managedclusterviewv1beta1.ManagedClusterView{})

	clustercurator := &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
			Annotations: map[string]string{
				"cluster.open-cluster-management.io/upgrade-allow-not-recommended-versions": "true",
			},
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "upgrade",
			Upgrade: clustercuratorv1.UpgradeHooks{
				DesiredUpdate: "4.5.10",
			},
		},
	}

	currentClusterVersion := &clusterversionv1.ClusterVersion{
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
					Version: "4.5.10",
					Image:   "quay.io/openshift-release-dev/ocp-release@sha256:71e158c6173ad6aa6e356c119a87459196bbe70e89c0db1e35c1f63a87d90676",
				},
			},
		},
	}

	b, _ := json.Marshal(currentClusterVersion)

	client := clientfake.NewClientBuilder().WithRuntimeObjects([]runtime.Object{
		clustercurator, getManagedClusterInfo(),
	}...).WithScheme(s).Build()

	go func() {
		for i := 0; i < 10; i++ {
			time.Sleep(1 * time.Second)
			resultmcview := managedclusterviewv1beta1.ManagedClusterView{}
			err := client.Get(context.TODO(), types.NamespacedName{
				Namespace: ClusterName,
				Name:      ClusterName,
			}, &resultmcview)
			if err != nil {
				klog.Error("failed to get managedClusterview.", err)
				continue
			}
			resultmcview.Status.Result.Raw = b
			err = client.Update(context.TODO(), &resultmcview)
			if err != nil {
				klog.Error("failed to update managedClusterview.", err)
				continue
			}
			updatedresultmcview := managedclusterviewv1beta1.ManagedClusterView{}
			err = client.Get(context.TODO(), types.NamespacedName{
				Namespace: ClusterName,
				Name:      ClusterName,
			}, &updatedresultmcview)
			if err != nil {
				klog.Error("failed to get managedClusterview.", err)
				continue
			}
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

	assert.Nil(t, UpgradeCluster(client, ClusterName, clustercurator),
		"Upgrade started successfully to non-recommended version with image digest in available list")
}
