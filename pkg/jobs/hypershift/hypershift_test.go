// Copyright Contributors to the Open Cluster Management project.

package hypershift

import (
	"context"
	"testing"
	"time"

	clustercuratorv1 "github.com/stolostron/cluster-curator-controller/pkg/api/v1beta1"
	"github.com/stolostron/cluster-curator-controller/pkg/jobs/utils"
	managedclusterinfov1beta1 "github.com/stolostron/cluster-lifecycle-api/clusterinfo/v1beta1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const ClusterName = "my-cluster"
const ClusterNamespace = "clusters"
const NodepoolName = "my-cluster-us-east-2"
const testTimeout = 5

var s = scheme.Scheme

func getHostedCluster(hcType string, hcConditions []interface{}) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "hypershift.openshift.io/v1beta1",
			"kind":       "HostedCluster",
			"metadata": map[string]interface{}{
				"name":      ClusterName,
				"namespace": ClusterNamespace,
				"labels": map[string]interface{}{
					"hypershift.openshift.io/auto-created-for-infra": ClusterName + "-xyz",
				},
			},
			"spec": map[string]interface{}{
				"pausedUntil": "true",
				"platform": map[string]interface{}{
					"type": hcType,
				},
				"release": map[string]interface{}{
					"image": "quay.io/openshift-release-dev/ocp-release:4.13.6-multi",
				},
			},
			"status": map[string]interface{}{
				"conditions": hcConditions,
			},
		},
	}
}

func getHostedClusterNoLabel(hcType string, hcConditions []interface{}) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "hypershift.openshift.io/v1beta1",
			"kind":       "HostedCluster",
			"metadata": map[string]interface{}{
				"name":      ClusterName,
				"namespace": ClusterNamespace,
			},
			"spec": map[string]interface{}{
				"pausedUntil": "true",
				"platform": map[string]interface{}{
					"type": hcType,
				},
				"release": map[string]interface{}{
					"image": "quay.io/openshift-release-dev/ocp-release:4.13.6-multi",
				},
			},
			"status": map[string]interface{}{
				"conditions": hcConditions,
			},
		},
	}
}

func getNodepool(npName string, npNamespace string, npClusterName string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "hypershift.openshift.io/v1beta1",
			"kind":       "NodePool",
			"metadata": map[string]interface{}{
				"name":      npName,
				"namespace": npNamespace,
			},
			"spec": map[string]interface{}{
				"pausedUntil": "true",
				"clusterName": npClusterName,
				"release": map[string]interface{}{
					"image": "quay.io/openshift-release-dev/ocp-release:4.13.6-multi",
				},
			},
		},
	}
}

func getClusterCurator(desiredCuration string) *clustercuratorv1.ClusterCurator {
	return &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterNamespace,
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: desiredCuration,
		},
	}
}

func getUpgradeClusterCurator(desiredUpdate string) *clustercuratorv1.ClusterCurator {
	return &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterNamespace,
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "upgrade",
			Upgrade: clustercuratorv1.UpgradeHooks{
				DesiredUpdate:  desiredUpdate,
				MonitorTimeout: 5,
			},
			Destroy: clustercuratorv1.Hooks{
				JobMonitorTimeout: 1,
			},
		},
	}
}

func getManagedClusterInfo() *managedclusterinfov1beta1.ManagedClusterInfo {
	return &managedclusterinfov1beta1.ManagedClusterInfo{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Status: managedclusterinfov1beta1.ClusterInfoStatus{
			DistributionInfo: managedclusterinfov1beta1.DistributionInfo{
				OCP: managedclusterinfov1beta1.OCPDistributionInfo{
					Version: "4.13.6",
				},
			},
		},
	}
}

func getManagedCluster() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cluster.open-cluster-management.io/v1",
			"kind":       "ManagedCluster",
			"metadata": map[string]interface{}{
				"name": ClusterName,
			},
		},
	}
}

func TestActivateDeployNoHC(t *testing.T) {
	defer func() { // recover from not having a hub config object
		if r := recover(); r == nil {
			t.Fatal("expected recover, but failed")
		}
	}()
	dynfake := dynfake.NewSimpleDynamicClient(runtime.NewScheme())
	assert.NotNil(t, ActivateDeploy(
		dynfake, ClusterName, ClusterNamespace), "err not nil, when HostedCluster resource is not present")
}

func TestActivateDeployAvailable(t *testing.T) {
	dynfake := dynfake.NewSimpleDynamicClient(
		runtime.NewScheme(),
		getHostedCluster("AWS", []interface{}{}),
		getNodepool(NodepoolName, ClusterNamespace, ClusterName),
		getNodepool("other-cluster-us-east-2", ClusterNamespace, "other-cluster"))
	assert.Nil(t, ActivateDeploy(
		dynfake, ClusterName, ClusterNamespace), "err nil, when HostedCluster is available")
}

func TestMonitorClusterStatusInstallNoHC(t *testing.T) {
	clusterCurator := &clustercuratorv1.ClusterCurator{}
	dynfake := dynfake.NewSimpleDynamicClient(runtime.NewScheme())
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewFakeClientWithScheme(s, clusterCurator)

	assert.NotNil(
		t,
		MonitorClusterStatus(dynfake, client, ClusterName, ClusterNamespace, utils.Installing, testTimeout),
		"err is not nil, when HostedCluster does not exist",
	)
}

func TestMonitorClusterStatusDestroyComplete(t *testing.T) {
	clusterCurator := &clustercuratorv1.ClusterCurator{}
	dynfake := dynfake.NewSimpleDynamicClient(runtime.NewScheme())
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clusterCurator).Build()

	assert.Nil(
		t,
		MonitorClusterStatus(dynfake, client, ClusterName, ClusterNamespace, utils.Destroying, testTimeout),
		"err is nil, when HostedCluster is deleted",
	)
}

func TestMonitorClusterStatusInstallComplete(t *testing.T) {
	clusterCurator := &clustercuratorv1.ClusterCurator{}
	dynfake := dynfake.NewSimpleDynamicClient(
		runtime.NewScheme(),
		getHostedCluster("AWS", []interface{}{
			map[string]interface{}{
				"type":    "Degraded",
				"status":  "False",
				"message": "The hosted cluster is not degraded",
			},
			map[string]interface{}{
				"type":    "ClusterVersionAvailable",
				"status":  "True",
				"message": "Done applying 4.13.6",
			},
			map[string]interface{}{
				"type":    "Available",
				"status":  "True",
				"message": "The hosted control plane is available",
			},
			map[string]interface{}{
				"type":    "ClusterVersionProgressing",
				"status":  "False",
				"message": "Cluster version is 4.13.6",
			},
		},
		))
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clusterCurator).Build()

	assert.Nil(
		t,
		MonitorClusterStatus(dynfake, client, ClusterName, ClusterNamespace, utils.Installing, testTimeout),
		"err is nil, when HostedCluster install is complete",
	)
}

func TestMonitorClusterStatusInstallCompleteHCNoLabel(t *testing.T) {
	clusterCurator := &clustercuratorv1.ClusterCurator{}
	dynfake := dynfake.NewSimpleDynamicClient(
		runtime.NewScheme(),
		getHostedClusterNoLabel("AWS", []interface{}{
			map[string]interface{}{
				"type":    "Degraded",
				"status":  "False",
				"message": "The hosted cluster is not degraded",
			},
			map[string]interface{}{
				"type":    "ClusterVersionAvailable",
				"status":  "True",
				"message": "Done applying 4.13.6",
			},
			map[string]interface{}{
				"type":    "Available",
				"status":  "True",
				"message": "The hosted control plane is available",
			},
			map[string]interface{}{
				"type":    "ClusterVersionProgressing",
				"status":  "False",
				"message": "Cluster version is 4.13.6",
			},
		},
		))
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clusterCurator).Build()

	assert.Nil(
		t,
		MonitorClusterStatus(dynfake, client, ClusterName, ClusterNamespace, utils.Installing, testTimeout),
		"err is nil, when HostedCluster install is complete",
	)
}

func TestMonitorClusterStatusWaitForDestroyToComplete(t *testing.T) {
	clusterCurator := getClusterCurator(utils.Destroying)
	dynfake := dynfake.NewSimpleDynamicClient(
		runtime.NewScheme(),
		getHostedCluster("AWS", []interface{}{
			map[string]interface{}{
				"type":    "Degraded",
				"status":  "True",
				"message": "The hosted cluster is not degraded",
			},
			map[string]interface{}{
				"type":    "ClusterVersionAvailable",
				"status":  "True",
				"message": "Done applying 4.13.6",
			},
			map[string]interface{}{
				"type":    "Available",
				"status":  "True",
				"message": "The hosted control plane is available",
			},
			map[string]interface{}{
				"type":    "ClusterVersionProgressing",
				"status":  "False",
				"message": "Cluster version is 4.13.6",
			},
		},
		))
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clusterCurator).Build()

	go func() {
		time.Sleep(utils.PauseTenSeconds)

		err := dynfake.Resource(utils.HCGVR).Namespace(ClusterNamespace).Delete(context.TODO(), ClusterName, v1.DeleteOptions{})
		assert.Nil(t, err, "err is nil, when HostedCluster is deleted successfully")
	}()

	assert.Nil(
		t,
		MonitorClusterStatus(dynfake, client, ClusterName, ClusterNamespace, utils.Destroying, testTimeout),
		"err is nil, when HostedCluster is deleted successfully",
	)
}

func TestMonitorClusterStatusWaitForInstallToComplete(t *testing.T) {
	clusterCurator := getClusterCurator(utils.Installing)
	dynfake := dynfake.NewSimpleDynamicClient(
		runtime.NewScheme(),
		getHostedCluster("AWS", []interface{}{
			map[string]interface{}{
				"type":    "Degraded",
				"status":  "True",
				"message": "The hosted cluster is not degraded",
			},
			map[string]interface{}{
				"type":    "ClusterVersionAvailable",
				"status":  "False",
				"message": "Done applying 4.13.6",
			},
			map[string]interface{}{
				"type":    "Available",
				"status":  "False",
				"message": "The hosted control plane is available",
			},
			map[string]interface{}{
				"type":    "ClusterVersionProgressing",
				"status":  "False",
				"message": "Cluster version is 4.13.6",
			},
		},
		))
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clusterCurator).Build()

	go func() {
		time.Sleep(utils.PauseTenSeconds)

		newHC := getHostedCluster("AWS", []interface{}{
			map[string]interface{}{
				"type":    "Degraded",
				"status":  "False",
				"message": "The hosted cluster is not degraded",
			},
			map[string]interface{}{
				"type":    "ClusterVersionAvailable",
				"status":  "True",
				"message": "Done applying 4.13.6",
			},
			map[string]interface{}{
				"type":    "Available",
				"status":  "True",
				"message": "The hosted control plane is available",
			},
			map[string]interface{}{
				"type":    "ClusterVersionProgressing",
				"status":  "False",
				"message": "Cluster version is 4.13.6",
			},
		})
		_, err := dynfake.Resource(utils.HCGVR).Namespace(ClusterNamespace).Update(context.TODO(), newHC, v1.UpdateOptions{})
		assert.Nil(t, err, "err is nil, when HostedCluster is updated successfully")
	}()

	assert.Nil(
		t,
		MonitorClusterStatus(dynfake, client, ClusterName, ClusterNamespace, utils.Installing, testTimeout),
		"err is nil, when HostedCluster is deleted successfully",
	)
}

func TestUpgradeClusterCompleted(t *testing.T) {
	clusterCurator := getUpgradeClusterCurator("4.13.7")
	managedClusterInfo := getManagedClusterInfo()
	dynfake := dynfake.NewSimpleDynamicClient(
		runtime.NewScheme(),
		getHostedCluster("AWS", []interface{}{
			map[string]interface{}{
				"type":    "Degraded",
				"status":  "False",
				"message": "The hosted cluster is not degraded",
			},
			map[string]interface{}{
				"type":    "ClusterVersionAvailable",
				"status":  "True",
				"message": "Done applying 4.13.6",
			},
			map[string]interface{}{
				"type":    "Available",
				"status":  "False",
				"message": "The hosted control plane is available",
			},
			map[string]interface{}{
				"type":    "ClusterVersionProgressing",
				"status":  "False",
				"message": "Cluster version is 4.13.6",
			},
			map[string]interface{}{
				"type":    "Progressing",
				"status":  "False",
				"message": "HostedCluster is at expected version",
			},
		}),
		getNodepool(NodepoolName, ClusterNamespace, ClusterName),
	)
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(managedclusterinfov1beta1.SchemeGroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clusterCurator, managedClusterInfo).Build()

	assert.Nil(
		t,
		UpgradeCluster(client, dynfake, ClusterName, clusterCurator),
		"err is nil, when HC and NPs are patched successfully for upgrade",
	)
}

func TestUpgradeClusterSameVersion(t *testing.T) {
	clusterCurator := getUpgradeClusterCurator("4.13.6")
	managedClusterInfo := getManagedClusterInfo()
	dynfake := dynfake.NewSimpleDynamicClient(
		runtime.NewScheme(),
		getHostedCluster("AWS", []interface{}{
			map[string]interface{}{
				"type":    "Degraded",
				"status":  "False",
				"message": "The hosted cluster is not degraded",
			},
			map[string]interface{}{
				"type":    "ClusterVersionAvailable",
				"status":  "True",
				"message": "Done applying 4.13.6",
			},
			map[string]interface{}{
				"type":    "Available",
				"status":  "False",
				"message": "The hosted control plane is available",
			},
			map[string]interface{}{
				"type":    "ClusterVersionProgressing",
				"status":  "False",
				"message": "Cluster version is 4.13.6",
			},
			map[string]interface{}{
				"type":    "Progressing",
				"status":  "False",
				"message": "HostedCluster is at expected version",
			},
		}),
		getNodepool(NodepoolName, ClusterNamespace, ClusterName),
	)
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(managedclusterinfov1beta1.SchemeGroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clusterCurator, managedClusterInfo).Build()

	assert.NotNil(
		t,
		UpgradeCluster(client, dynfake, ClusterName, clusterCurator),
		"err is not nil, when HC and NPs are upgrading to the same version",
	)
}

func TestMonitorUpgradeStatusCompleted(t *testing.T) {
	clusterCurator := getUpgradeClusterCurator("4.13.7")
	managedClusterInfo := getManagedClusterInfo()
	dynfake := dynfake.NewSimpleDynamicClient(
		runtime.NewScheme(),
		getHostedCluster("AWS", []interface{}{
			map[string]interface{}{
				"type":    "Degraded",
				"status":  "False",
				"message": "The hosted cluster is not degraded",
			},
			map[string]interface{}{
				"type":    "ClusterVersionAvailable",
				"status":  "True",
				"message": "Done applying 4.13.7",
			},
			map[string]interface{}{
				"type":    "Available",
				"status":  "True",
				"message": "The hosted control plane is available",
			},
			map[string]interface{}{
				"type":    "ClusterVersionProgressing",
				"status":  "False",
				"message": "Cluster version is 4.13.7",
			},
			map[string]interface{}{
				"type":    "Progressing",
				"status":  "False",
				"message": "HostedCluster is at expected version",
			},
		}),
		getNodepool(NodepoolName, ClusterNamespace, ClusterName),
	)
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(managedclusterinfov1beta1.SchemeGroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clusterCurator, managedClusterInfo).Build()

	assert.Nil(
		t,
		MonitorUpgradeStatus(dynfake, client, ClusterName, clusterCurator),
		"err is nil, when HC and NPs are upgraded sucessfully",
	)
}

func TestMonitorUpgradeStatusWaitForUpdate(t *testing.T) {
	clusterCurator := getUpgradeClusterCurator("4.13.7")
	managedClusterInfo := getManagedClusterInfo()
	dynfake := dynfake.NewSimpleDynamicClient(
		runtime.NewScheme(),
		getHostedCluster("AWS", []interface{}{
			map[string]interface{}{
				"type":    "Degraded",
				"status":  "False",
				"message": "The hosted cluster is not degraded",
			},
			map[string]interface{}{
				"type":    "ClusterVersionAvailable",
				"status":  "True",
				"message": "Done applying 4.13.7",
			},
			map[string]interface{}{
				"type":    "Available",
				"status":  "True",
				"message": "The hosted control plane is available",
			},
			map[string]interface{}{
				"type":    "ClusterVersionProgressing",
				"status":  "False",
				"message": "Cluster version is 4.13.7",
			},
			map[string]interface{}{
				"type":    "Progressing",
				"status":  "True",
				"message": "HostedCluster is at expected version",
			},
		}),
		getNodepool(NodepoolName, ClusterNamespace, ClusterName),
	)
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(managedclusterinfov1beta1.SchemeGroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clusterCurator, managedClusterInfo).Build()

	go func() {
		time.Sleep(utils.PauseTenSeconds)

		newHC := getHostedCluster("AWS", []interface{}{
			map[string]interface{}{
				"type":    "Degraded",
				"status":  "False",
				"message": "The hosted cluster is not degraded",
			},
			map[string]interface{}{
				"type":    "ClusterVersionAvailable",
				"status":  "True",
				"message": "Done applying 4.13.7",
			},
			map[string]interface{}{
				"type":    "Available",
				"status":  "True",
				"message": "The hosted control plane is available",
			},
			map[string]interface{}{
				"type":    "ClusterVersionProgressing",
				"status":  "False",
				"message": "Cluster version is 4.13.7",
			},
			map[string]interface{}{
				"type":    "Progressing",
				"status":  "False",
				"message": "HostedCluster is at expected version",
			},
		})
		_, err := dynfake.Resource(utils.HCGVR).Namespace(ClusterNamespace).Update(context.TODO(), newHC, v1.UpdateOptions{})
		assert.Nil(t, err, "err is nil, when HostedCluster is updated successfully")
	}()

	assert.Nil(
		t,
		MonitorUpgradeStatus(dynfake, client, ClusterName, clusterCurator),
		"err is nil, when HC and NPs are upgraded sucessfully",
	)
}

func TestDestroyHostedCluster(t *testing.T) {
	dynfake := dynfake.NewSimpleDynamicClient(
		runtime.NewScheme(),
		getHostedCluster("AWS", []interface{}{
			map[string]interface{}{
				"type":    "Degraded",
				"status":  "False",
				"message": "The hosted cluster is not degraded",
			},
			map[string]interface{}{
				"type":    "ClusterVersionAvailable",
				"status":  "True",
				"message": "Done applying 4.13.7",
			},
			map[string]interface{}{
				"type":    "Available",
				"status":  "True",
				"message": "The hosted control plane is available",
			},
			map[string]interface{}{
				"type":    "ClusterVersionProgressing",
				"status":  "False",
				"message": "Cluster version is 4.13.7",
			},
			map[string]interface{}{
				"type":    "Progressing",
				"status":  "True",
				"message": "HostedCluster is at expected version",
			},
		}),
	)

	assert.Nil(
		t,
		DestroyHostedCluster(dynfake, ClusterName, ClusterNamespace),
		"err is nil, when HC and NPs are upgraded sucessfully",
	)
}

func TestDetachAndMonitorAWS(t *testing.T) {
	clusterCurator := getUpgradeClusterCurator("4.13.7")
	dynfake := dynfake.NewSimpleDynamicClient(
		runtime.NewScheme(),
		getHostedCluster("AWS", []interface{}{
			map[string]interface{}{
				"type":    "Degraded",
				"status":  "False",
				"message": "The hosted cluster is not degraded",
			},
			map[string]interface{}{
				"type":    "ClusterVersionAvailable",
				"status":  "True",
				"message": "Done applying 4.13.7",
			},
			map[string]interface{}{
				"type":    "Available",
				"status":  "True",
				"message": "The hosted control plane is available",
			},
			map[string]interface{}{
				"type":    "ClusterVersionProgressing",
				"status":  "False",
				"message": "Cluster version is 4.13.7",
			},
			map[string]interface{}{
				"type":    "Progressing",
				"status":  "True",
				"message": "HostedCluster is at expected version",
			},
		}),
		getManagedCluster(),
	)

	assert.NotNil(
		t,
		DetachAndMonitor(dynfake, ClusterName, clusterCurator),
		"err is not nil, when user is destroying AWS HC",
	)
}

func TestDetachAndMonitorKubeVirt(t *testing.T) {
	clusterCurator := getUpgradeClusterCurator("4.13.7")
	dynfake := dynfake.NewSimpleDynamicClient(
		runtime.NewScheme(),
		getHostedCluster("KubeVirt", []interface{}{
			map[string]interface{}{
				"type":    "Degraded",
				"status":  "False",
				"message": "The hosted cluster is not degraded",
			},
			map[string]interface{}{
				"type":    "ClusterVersionAvailable",
				"status":  "True",
				"message": "Done applying 4.13.7",
			},
			map[string]interface{}{
				"type":    "Available",
				"status":  "True",
				"message": "The hosted control plane is available",
			},
			map[string]interface{}{
				"type":    "ClusterVersionProgressing",
				"status":  "False",
				"message": "Cluster version is 4.13.7",
			},
			map[string]interface{}{
				"type":    "Progressing",
				"status":  "True",
				"message": "HostedCluster is at expected version",
			},
		}),
		getManagedCluster(),
	)

	assert.Nil(
		t,
		DetachAndMonitor(dynfake, ClusterName, clusterCurator),
		"err is nil, when user is destroying KubeVirt HC",
	)
}
