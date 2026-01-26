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

func getHostedClusterWithChannel(hcType string, channel string, hcConditions []interface{}) *unstructured.Unstructured {
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
				"channel":     channel,
				"platform": map[string]interface{}{
					"type": hcType,
				},
				"release": map[string]interface{}{
					"image": "quay.io/openshift-release-dev/ocp-release:4.13.6-multi",
				},
			},
			"status": map[string]interface{}{
				"conditions": hcConditions,
				"version": map[string]interface{}{
					"desired": map[string]interface{}{
						"channels": []interface{}{
							"candidate-4.13",
							"candidate-4.14",
							"fast-4.13",
							"fast-4.14",
							"stable-4.13",
							"stable-4.14",
						},
						"version": "4.13.6",
					},
				},
			},
		},
	}
}

func getHostedClusterWithVersion(hcType string, version string, hcConditions []interface{}) *unstructured.Unstructured {
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
					"image": "quay.io/openshift-release-dev/ocp-release:" + version + "-multi",
				},
			},
			"status": map[string]interface{}{
				"conditions": hcConditions,
				"version": map[string]interface{}{
					"history": []interface{}{
						map[string]interface{}{
							"version": version,
							"state":   "Completed",
						},
					},
					"desired": map[string]interface{}{
						"version": version,
					},
				},
			},
		},
	}
}

func getHostedClusterWithChannelAndVersion(hcType string, channel string, version string, hcConditions []interface{}) *unstructured.Unstructured {
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
				"channel":     channel,
				"platform": map[string]interface{}{
					"type": hcType,
				},
				"release": map[string]interface{}{
					"image": "quay.io/openshift-release-dev/ocp-release:" + version + "-multi",
				},
			},
			"status": map[string]interface{}{
				"conditions": hcConditions,
				"version": map[string]interface{}{
					"history": []interface{}{
						map[string]interface{}{
							"version": version,
							"state":   "Completed",
						},
					},
					"desired": map[string]interface{}{
						"channels": []interface{}{
							"candidate-4.13",
							"candidate-4.14",
							"fast-4.13",
							"fast-4.14",
							"stable-4.13",
							"stable-4.14",
						},
						"version": version,
					},
				},
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

func getNodepoolWithVersion(npName string, npNamespace string, npClusterName string, version string) *unstructured.Unstructured {
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
					"image": "quay.io/openshift-release-dev/ocp-release:" + version + "-multi",
				},
			},
			"status": map[string]interface{}{
				"version": version,
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "True",
					},
					map[string]interface{}{
						"type":   "UpdatingVersion",
						"status": "False",
					},
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

func getChannelOnlyClusterCurator(channel string) *clustercuratorv1.ClusterCurator {
	return &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterNamespace,
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "upgrade",
			Upgrade: clustercuratorv1.UpgradeHooks{
				Channel:        channel,
				MonitorTimeout: 5,
			},
			Destroy: clustercuratorv1.Hooks{
				JobMonitorTimeout: 1,
			},
		},
	}
}

func getUpgradeWithChannelClusterCurator(desiredUpdate string, channel string) *clustercuratorv1.ClusterCurator {
	return &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterNamespace,
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "upgrade",
			Upgrade: clustercuratorv1.UpgradeHooks{
				DesiredUpdate:  desiredUpdate,
				Channel:        channel,
				MonitorTimeout: 5,
			},
			Destroy: clustercuratorv1.Hooks{
				JobMonitorTimeout: 1,
			},
		},
	}
}

func getUpgradeClusterCuratorWithType(desiredUpdate string, upgradeType clustercuratorv1.UpgradeType) *clustercuratorv1.ClusterCurator {
	return &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterNamespace,
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "upgrade",
			Upgrade: clustercuratorv1.UpgradeHooks{
				DesiredUpdate:  desiredUpdate,
				UpgradeType:    upgradeType,
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
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clusterCurator).Build()

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
	clusterCurator := getClusterCurator(utils.Installing)
	dynfake := dynfake.NewSimpleDynamicClient(
		runtime.NewScheme(),
		getHostedClusterNoLabel("AWS", []interface{}{
			map[string]interface{}{
				"type":    "Degraded",
				"status":  "True",
				"message": "The hosted cluster is degraded",
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

		newHC := getHostedClusterNoLabel("AWS", []interface{}{
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
				"message": "The hosted cluster is degraded",
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
				"message": "The hosted cluster is degraded",
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
		getNodepoolWithVersion(NodepoolName, ClusterNamespace, ClusterName, "4.13.7"),
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
		getNodepoolWithVersion(NodepoolName, ClusterNamespace, ClusterName, "4.13.7"),
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

// Channel-only update tests
func TestUpgradeClusterChannelOnly(t *testing.T) {
	clusterCurator := getChannelOnlyClusterCurator("fast-4.14")
	dynfake := dynfake.NewSimpleDynamicClient(
		runtime.NewScheme(),
		getHostedClusterWithChannel("AWS", "stable-4.13", []interface{}{
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
			map[string]interface{}{
				"type":    "Progressing",
				"status":  "False",
				"message": "HostedCluster is at expected version",
			},
		}),
		getNodepool(NodepoolName, ClusterNamespace, ClusterName),
	)
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clusterCurator).Build()

	assert.Nil(
		t,
		UpgradeCluster(client, dynfake, ClusterName, clusterCurator),
		"err is nil, when channel-only update is performed",
	)

	// Verify the channel was updated
	hc, err := dynfake.Resource(utils.HCGVR).Namespace(ClusterNamespace).Get(context.TODO(), ClusterName, v1.GetOptions{})
	assert.Nil(t, err, "should be able to get HostedCluster")
	spec := hc.Object["spec"].(map[string]interface{})
	assert.Equal(t, "fast-4.14", spec["channel"], "channel should be updated to fast-4.14")
}

func TestUpgradeClusterNoVersionOrChannel(t *testing.T) {
	clusterCurator := &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterNamespace,
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "upgrade",
			Upgrade: clustercuratorv1.UpgradeHooks{
				MonitorTimeout: 5,
			},
		},
	}
	dynfake := dynfake.NewSimpleDynamicClient(
		runtime.NewScheme(),
		getHostedCluster("AWS", []interface{}{}),
		getNodepool(NodepoolName, ClusterNamespace, ClusterName),
	)
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clusterCurator).Build()

	err := UpgradeCluster(client, dynfake, ClusterName, clusterCurator)
	assert.NotNil(t, err, "err should not be nil when neither version nor channel is provided")
	assert.Contains(t, err.Error(), "Provide valid upgrade version or channel")
}

func TestUpgradeClusterWithVersionAndChannel(t *testing.T) {
	clusterCurator := getUpgradeWithChannelClusterCurator("4.13.7", "fast-4.14")
	managedClusterInfo := getManagedClusterInfo()
	dynfake := dynfake.NewSimpleDynamicClient(
		runtime.NewScheme(),
		getHostedClusterWithChannel("AWS", "stable-4.13", []interface{}{
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
		"err is nil, when both version and channel are provided",
	)

	// Verify both channel and release image were updated
	hc, err := dynfake.Resource(utils.HCGVR).Namespace(ClusterNamespace).Get(context.TODO(), ClusterName, v1.GetOptions{})
	assert.Nil(t, err, "should be able to get HostedCluster")
	spec := hc.Object["spec"].(map[string]interface{})
	assert.Equal(t, "fast-4.14", spec["channel"], "channel should be updated to fast-4.14")
	release := spec["release"].(map[string]interface{})
	assert.Equal(t, "quay.io/openshift-release-dev/ocp-release:4.13.7-multi", release["image"], "release image should be updated")
}

func TestMonitorUpgradeStatusChannelOnly(t *testing.T) {
	clusterCurator := getChannelOnlyClusterCurator("fast-4.14")
	dynfake := dynfake.NewSimpleDynamicClient(
		runtime.NewScheme(),
		getHostedClusterWithChannel("AWS", "fast-4.14", []interface{}{
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
			map[string]interface{}{
				"type":    "Progressing",
				"status":  "False",
				"message": "HostedCluster is at expected version",
			},
		}),
		getNodepool(NodepoolName, ClusterNamespace, ClusterName),
	)
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clusterCurator).Build()

	assert.Nil(
		t,
		MonitorUpgradeStatus(dynfake, client, ClusterName, clusterCurator),
		"err is nil, when channel-only update is detected",
	)
}

func TestUpgradeClusterInvalidChannel(t *testing.T) {
	clusterCurator := getChannelOnlyClusterCurator("invalid-channel")
	dynfake := dynfake.NewSimpleDynamicClient(
		runtime.NewScheme(),
		getHostedClusterWithChannel("AWS", "stable-4.13", []interface{}{
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
			map[string]interface{}{
				"type":    "Progressing",
				"status":  "False",
				"message": "HostedCluster is at expected version",
			},
		}),
		getNodepool(NodepoolName, ClusterNamespace, ClusterName),
	)
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clusterCurator).Build()

	err := UpgradeCluster(client, dynfake, ClusterName, clusterCurator)
	assert.NotNil(t, err, "err should not be nil when invalid channel is provided")
	assert.Contains(t, err.Error(), "is not valid")
	assert.Contains(t, err.Error(), "Available channels")
}

// Tests for UpgradeType: ControlPlane only
func TestUpgradeClusterControlPlaneOnly(t *testing.T) {
	clusterCurator := getUpgradeClusterCuratorWithType("4.13.7", clustercuratorv1.UpgradeTypeControlPlane)
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
		"err is nil, when ControlPlane only upgrade is performed",
	)

	// Verify the HostedCluster was upgraded
	hc, err := dynfake.Resource(utils.HCGVR).Namespace(ClusterNamespace).Get(context.TODO(), ClusterName, v1.GetOptions{})
	assert.Nil(t, err, "should be able to get HostedCluster")
	spec := hc.Object["spec"].(map[string]interface{})
	release := spec["release"].(map[string]interface{})
	assert.Equal(t, "quay.io/openshift-release-dev/ocp-release:4.13.7-multi", release["image"], "HostedCluster image should be updated")

	// Verify the NodePool was NOT upgraded
	np, err := dynfake.Resource(utils.NPGVR).Namespace(ClusterNamespace).Get(context.TODO(), NodepoolName, v1.GetOptions{})
	assert.Nil(t, err, "should be able to get NodePool")
	npSpec := np.Object["spec"].(map[string]interface{})
	npRelease := npSpec["release"].(map[string]interface{})
	assert.Equal(t, "quay.io/openshift-release-dev/ocp-release:4.13.6-multi", npRelease["image"], "NodePool image should NOT be updated")
}

// Tests for UpgradeType: NodePools only
func TestUpgradeClusterNodePoolsOnly(t *testing.T) {
	// HostedCluster is at 4.14.0, upgrading NodePools from 4.13.6 to 4.13.7 (below HC version)
	clusterCurator := getUpgradeClusterCuratorWithType("4.13.7", clustercuratorv1.UpgradeTypeNodePools)
	managedClusterInfo := getManagedClusterInfo() // NodePools at 4.13.6
	dynfake := dynfake.NewSimpleDynamicClient(
		runtime.NewScheme(),
		getHostedClusterWithVersion("AWS", "4.14.0", []interface{}{
			map[string]interface{}{
				"type":    "Degraded",
				"status":  "False",
				"message": "The hosted cluster is not degraded",
			},
			map[string]interface{}{
				"type":    "ClusterVersionAvailable",
				"status":  "True",
				"message": "Done applying 4.14.0",
			},
			map[string]interface{}{
				"type":    "Available",
				"status":  "False",
				"message": "The hosted control plane is available",
			},
			map[string]interface{}{
				"type":    "ClusterVersionProgressing",
				"status":  "False",
				"message": "Cluster version is 4.14.0",
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
		"err is nil, when NodePools only upgrade is performed",
	)

	// Verify the HostedCluster was NOT upgraded (still at 4.14.0)
	hc, err := dynfake.Resource(utils.HCGVR).Namespace(ClusterNamespace).Get(context.TODO(), ClusterName, v1.GetOptions{})
	assert.Nil(t, err, "should be able to get HostedCluster")
	spec := hc.Object["spec"].(map[string]interface{})
	release := spec["release"].(map[string]interface{})
	assert.Equal(t, "quay.io/openshift-release-dev/ocp-release:4.14.0-multi", release["image"], "HostedCluster image should NOT be updated")

	// Verify the NodePool was upgraded
	np, err := dynfake.Resource(utils.NPGVR).Namespace(ClusterNamespace).Get(context.TODO(), NodepoolName, v1.GetOptions{})
	assert.Nil(t, err, "should be able to get NodePool")
	npSpec := np.Object["spec"].(map[string]interface{})
	npRelease := npSpec["release"].(map[string]interface{})
	assert.Equal(t, "quay.io/openshift-release-dev/ocp-release:4.13.7-multi", npRelease["image"], "NodePool image should be updated")
}

// Test MonitorUpgradeStatus for ControlPlane only
func TestMonitorUpgradeStatusControlPlaneOnly(t *testing.T) {
	clusterCurator := getUpgradeClusterCuratorWithType("4.13.7", clustercuratorv1.UpgradeTypeControlPlane)
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
		// NodePool is still at old version - should not matter for ControlPlane only
		getNodepool(NodepoolName, ClusterNamespace, ClusterName),
	)
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clusterCurator).Build()

	assert.Nil(
		t,
		MonitorUpgradeStatus(dynfake, client, ClusterName, clusterCurator),
		"err is nil, when ControlPlane only upgrade monitoring succeeds",
	)
}

// Test MonitorUpgradeStatus for NodePools only
func TestMonitorUpgradeStatusNodePoolsOnly(t *testing.T) {
	clusterCurator := getUpgradeClusterCuratorWithType("4.13.7", clustercuratorv1.UpgradeTypeNodePools)
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
				"message": "Done applying 4.13.6", // Still at old version
			},
			map[string]interface{}{
				"type":    "Available",
				"status":  "True",
				"message": "The hosted control plane is available",
			},
			map[string]interface{}{
				"type":    "ClusterVersionProgressing",
				"status":  "False",
				"message": "Cluster version is 4.13.6", // Still at old version
			},
			map[string]interface{}{
				"type":    "Progressing",
				"status":  "False",
				"message": "HostedCluster is at expected version",
			},
		}),
		// NodePool is at new version with Ready status
		getNodepoolWithVersion(NodepoolName, ClusterNamespace, ClusterName, "4.13.7"),
	)
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clusterCurator).Build()

	assert.Nil(
		t,
		MonitorUpgradeStatus(dynfake, client, ClusterName, clusterCurator),
		"err is nil, when NodePools only upgrade monitoring succeeds",
	)
}

// Test that NodePools cannot be upgraded to a version higher than HostedCluster
func TestUpgradeClusterNodePoolsVersionHigherThanHostedCluster(t *testing.T) {
	// NodePools at 4.13.6, trying to upgrade to 4.14.0 which is higher than HostedCluster version
	clusterCurator := getUpgradeClusterCuratorWithType("4.14.0", clustercuratorv1.UpgradeTypeNodePools)
	managedClusterInfo := getManagedClusterInfo() // Returns 4.13.6
	dynfake := dynfake.NewSimpleDynamicClient(
		runtime.NewScheme(),
		getHostedClusterWithVersion("AWS", "4.13.6", []interface{}{
			map[string]interface{}{
				"type":    "Degraded",
				"status":  "False",
				"message": "The hosted cluster is not degraded",
			},
		}),
		getNodepool(NodepoolName, ClusterNamespace, ClusterName),
	)
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(managedclusterinfov1beta1.SchemeGroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clusterCurator, managedClusterInfo).Build()

	err := UpgradeCluster(client, dynfake, ClusterName, clusterCurator)
	assert.NotNil(t, err, "err should not be nil when NodePools version is higher than HostedCluster")
	assert.Contains(t, err.Error(), "NodePools cannot be upgraded to version")
	assert.Contains(t, err.Error(), "higher than HostedCluster version")
}

// Test that NodePools can be upgraded to the same version as HostedCluster
func TestUpgradeClusterNodePoolsSameVersionAsHostedCluster(t *testing.T) {
	// HostedCluster at 4.14.0, upgrading NodePools to 4.14.0 (same version)
	clusterCurator := getUpgradeClusterCuratorWithType("4.14.0", clustercuratorv1.UpgradeTypeNodePools)
	managedClusterInfo := &managedclusterinfov1beta1.ManagedClusterInfo{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Status: managedclusterinfov1beta1.ClusterInfoStatus{
			DistributionInfo: managedclusterinfov1beta1.DistributionInfo{
				OCP: managedclusterinfov1beta1.OCPDistributionInfo{
					Version: "4.13.6", // NodePools current version
				},
			},
		},
	}
	dynfake := dynfake.NewSimpleDynamicClient(
		runtime.NewScheme(),
		getHostedClusterWithVersion("AWS", "4.14.0", []interface{}{
			map[string]interface{}{
				"type":    "Degraded",
				"status":  "False",
				"message": "The hosted cluster is not degraded",
			},
		}),
		getNodepool(NodepoolName, ClusterNamespace, ClusterName),
	)
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(managedclusterinfov1beta1.SchemeGroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clusterCurator, managedClusterInfo).Build()

	err := UpgradeCluster(client, dynfake, ClusterName, clusterCurator)
	assert.Nil(t, err, "err should be nil when NodePools version equals HostedCluster version")
}

// Test that NodePools upgrade does not fail when ManagedClusterInfo reports control plane version
// This tests the scenario where:
// - Control plane (HostedCluster) is at 4.14.0
// - ManagedClusterInfo reports 4.14.0 (control plane version)
// - NodePools are at 4.13.6
// - Desired upgrade is 4.14.0 (to bring NodePools to control plane version)
// Before the fix, this would fail with "Cannot upgrade to the same version" because
// ManagedClusterInfo.Version (4.14.0) == desiredUpdate (4.14.0)
func TestUpgradeClusterNodePoolsWhenManagedClusterInfoReportsControlPlaneVersion(t *testing.T) {
	clusterCurator := getUpgradeClusterCuratorWithType("4.14.0", clustercuratorv1.UpgradeTypeNodePools)
	// ManagedClusterInfo reports control plane version (4.14.0), not NodePool version
	managedClusterInfo := &managedclusterinfov1beta1.ManagedClusterInfo{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Status: managedclusterinfov1beta1.ClusterInfoStatus{
			DistributionInfo: managedclusterinfov1beta1.DistributionInfo{
				OCP: managedclusterinfov1beta1.OCPDistributionInfo{
					Version: "4.14.0", // Control plane version (same as desired!)
				},
			},
		},
	}
	dynfake := dynfake.NewSimpleDynamicClient(
		runtime.NewScheme(),
		getHostedClusterWithVersion("AWS", "4.14.0", []interface{}{
			map[string]interface{}{
				"type":    "Degraded",
				"status":  "False",
				"message": "The hosted cluster is not degraded",
			},
		}),
		getNodepool(NodepoolName, ClusterNamespace, ClusterName), // NodePool at 4.13.6
	)
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(managedclusterinfov1beta1.SchemeGroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clusterCurator, managedClusterInfo).Build()

	// This should succeed - NodePools-only upgrade skips the "same version" check against ManagedClusterInfo
	err := UpgradeCluster(client, dynfake, ClusterName, clusterCurator)
	assert.Nil(t, err, "NodePools upgrade should not fail when ManagedClusterInfo reports control plane version")
}

// Test that NodePools can be upgraded to a lower version than HostedCluster
func TestUpgradeClusterNodePoolsLowerVersionThanHostedCluster(t *testing.T) {
	// HostedCluster at 4.14.0, upgrading NodePools to 4.13.7 (lower version, catching up)
	clusterCurator := getUpgradeClusterCuratorWithType("4.13.7", clustercuratorv1.UpgradeTypeNodePools)
	managedClusterInfo := getManagedClusterInfo() // Returns 4.13.6
	dynfake := dynfake.NewSimpleDynamicClient(
		runtime.NewScheme(),
		getHostedClusterWithVersion("AWS", "4.14.0", []interface{}{
			map[string]interface{}{
				"type":    "Degraded",
				"status":  "False",
				"message": "The hosted cluster is not degraded",
			},
		}),
		getNodepool(NodepoolName, ClusterNamespace, ClusterName),
	)
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(managedclusterinfov1beta1.SchemeGroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clusterCurator, managedClusterInfo).Build()

	err := UpgradeCluster(client, dynfake, ClusterName, clusterCurator)
	assert.Nil(t, err, "err should be nil when NodePools version is lower than HostedCluster version")
}

// Test that channel is not updated when upgradeType is NodePools
func TestUpgradeClusterNodePoolsOnlyIgnoresChannel(t *testing.T) {
	// HostedCluster is at 4.14.0, upgrading NodePools from 4.13.6 to 4.13.7 with channel (should be ignored)
	clusterCurator := &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterNamespace,
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "upgrade",
			Upgrade: clustercuratorv1.UpgradeHooks{
				DesiredUpdate:  "4.13.7",
				Channel:        "fast-4.14", // This should be ignored
				UpgradeType:    clustercuratorv1.UpgradeTypeNodePools,
				MonitorTimeout: 5,
			},
			Destroy: clustercuratorv1.Hooks{
				JobMonitorTimeout: 1,
			},
		},
	}
	managedClusterInfo := getManagedClusterInfo() // NodePools at 4.13.6
	dynfake := dynfake.NewSimpleDynamicClient(
		runtime.NewScheme(),
		getHostedClusterWithChannelAndVersion("AWS", "stable-4.13", "4.14.0", []interface{}{
			map[string]interface{}{
				"type":    "Degraded",
				"status":  "False",
				"message": "The hosted cluster is not degraded",
			},
			map[string]interface{}{
				"type":    "ClusterVersionAvailable",
				"status":  "True",
				"message": "Done applying 4.14.0",
			},
			map[string]interface{}{
				"type":    "Available",
				"status":  "False",
				"message": "The hosted control plane is available",
			},
			map[string]interface{}{
				"type":    "ClusterVersionProgressing",
				"status":  "False",
				"message": "Cluster version is 4.14.0",
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
		"err is nil, when NodePools only upgrade is performed with channel specified",
	)

	// Verify the channel was NOT updated on HostedCluster
	hc, err := dynfake.Resource(utils.HCGVR).Namespace(ClusterNamespace).Get(context.TODO(), ClusterName, v1.GetOptions{})
	assert.Nil(t, err, "should be able to get HostedCluster")
	spec := hc.Object["spec"].(map[string]interface{})
	assert.Equal(t, "stable-4.13", spec["channel"], "channel should NOT be updated for NodePools only upgrade")
}

// ============================================================================
// Additional tests for improved code coverage
// ============================================================================

// Test areNodePoolsReady when NodePool has no status
func TestAreNodePoolsReadyNoStatus(t *testing.T) {
	np := getNodepool(NodepoolName, ClusterNamespace, ClusterName)
	// Remove status from nodepool
	delete(np.Object, "status")

	dynfake := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), np)

	ready, err := areNodePoolsReady(dynfake, ClusterName, ClusterNamespace, "4.13.7", nil)
	assert.Nil(t, err, "should not return error")
	assert.False(t, ready, "should return false when NodePool has no status")
}

// Test areNodePoolsReady when NodePool is still updating version
func TestAreNodePoolsReadyUpdatingVersion(t *testing.T) {
	np := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "hypershift.openshift.io/v1beta1",
			"kind":       "NodePool",
			"metadata": map[string]interface{}{
				"name":      NodepoolName,
				"namespace": ClusterNamespace,
			},
			"spec": map[string]interface{}{
				"clusterName": ClusterName,
				"release": map[string]interface{}{
					"image": "quay.io/openshift-release-dev/ocp-release:4.13.7-multi",
				},
			},
			"status": map[string]interface{}{
				"version": "4.13.7",
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "True",
					},
					map[string]interface{}{
						"type":   "UpdatingVersion",
						"status": "True", // Still updating
					},
				},
			},
		},
	}

	dynfake := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), np)

	ready, err := areNodePoolsReady(dynfake, ClusterName, ClusterNamespace, "4.13.7", nil)
	assert.Nil(t, err, "should not return error")
	assert.False(t, ready, "should return false when NodePool is still updating version")
}

// Test areNodePoolsReady when NodePool Ready condition is False
func TestAreNodePoolsReadyNotReady(t *testing.T) {
	np := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "hypershift.openshift.io/v1beta1",
			"kind":       "NodePool",
			"metadata": map[string]interface{}{
				"name":      NodepoolName,
				"namespace": ClusterNamespace,
			},
			"spec": map[string]interface{}{
				"clusterName": ClusterName,
				"release": map[string]interface{}{
					"image": "quay.io/openshift-release-dev/ocp-release:4.13.7-multi",
				},
			},
			"status": map[string]interface{}{
				"version": "4.13.7",
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "False", // Not ready
					},
					map[string]interface{}{
						"type":   "UpdatingVersion",
						"status": "False",
					},
				},
			},
		},
	}

	dynfake := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), np)

	ready, err := areNodePoolsReady(dynfake, ClusterName, ClusterNamespace, "4.13.7", nil)
	assert.Nil(t, err, "should not return error")
	assert.False(t, ready, "should return false when NodePool is not ready")
}

// Test areNodePoolsReady when NodePool spec image doesn't match
func TestAreNodePoolsReadyImageMismatch(t *testing.T) {
	np := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "hypershift.openshift.io/v1beta1",
			"kind":       "NodePool",
			"metadata": map[string]interface{}{
				"name":      NodepoolName,
				"namespace": ClusterNamespace,
			},
			"spec": map[string]interface{}{
				"clusterName": ClusterName,
				"release": map[string]interface{}{
					"image": "quay.io/openshift-release-dev/ocp-release:4.13.6-multi", // Old version
				},
			},
			"status": map[string]interface{}{
				"version": "4.13.7",
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "True",
					},
					map[string]interface{}{
						"type":   "UpdatingVersion",
						"status": "False",
					},
				},
			},
		},
	}

	dynfake := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), np)

	ready, err := areNodePoolsReady(dynfake, ClusterName, ClusterNamespace, "4.13.7", nil)
	assert.Nil(t, err, "should not return error")
	assert.False(t, ready, "should return false when NodePool spec image doesn't match")
}

// Test areNodePoolsReady when NodePool status.version is nil
func TestAreNodePoolsReadyNoStatusVersion(t *testing.T) {
	np := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "hypershift.openshift.io/v1beta1",
			"kind":       "NodePool",
			"metadata": map[string]interface{}{
				"name":      NodepoolName,
				"namespace": ClusterNamespace,
			},
			"spec": map[string]interface{}{
				"clusterName": ClusterName,
				"release": map[string]interface{}{
					"image": "quay.io/openshift-release-dev/ocp-release:4.13.7-multi",
				},
			},
			"status": map[string]interface{}{
				// No version field
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "True",
					},
					map[string]interface{}{
						"type":   "UpdatingVersion",
						"status": "False",
					},
				},
			},
		},
	}

	dynfake := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), np)

	ready, err := areNodePoolsReady(dynfake, ClusterName, ClusterNamespace, "4.13.7", nil)
	assert.Nil(t, err, "should not return error")
	assert.False(t, ready, "should return false when NodePool status.version is nil")
}

// Test areNodePoolsReady when NodePool status.version doesn't match
func TestAreNodePoolsReadyVersionMismatch(t *testing.T) {
	np := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "hypershift.openshift.io/v1beta1",
			"kind":       "NodePool",
			"metadata": map[string]interface{}{
				"name":      NodepoolName,
				"namespace": ClusterNamespace,
			},
			"spec": map[string]interface{}{
				"clusterName": ClusterName,
				"release": map[string]interface{}{
					"image": "quay.io/openshift-release-dev/ocp-release:4.13.7-multi",
				},
			},
			"status": map[string]interface{}{
				"version": "4.13.6", // Doesn't match desired 4.13.7
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "True",
					},
					map[string]interface{}{
						"type":   "UpdatingVersion",
						"status": "False",
					},
				},
			},
		},
	}

	dynfake := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), np)

	ready, err := areNodePoolsReady(dynfake, ClusterName, ClusterNamespace, "4.13.7", nil)
	assert.Nil(t, err, "should not return error")
	assert.False(t, ready, "should return false when NodePool status.version doesn't match")
}

// Test getHostedClusterVersion with no status
func TestGetHostedClusterVersionNoStatus(t *testing.T) {
	hc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "hypershift.openshift.io/v1beta1",
			"kind":       "HostedCluster",
			"metadata": map[string]interface{}{
				"name":      ClusterName,
				"namespace": ClusterNamespace,
			},
			"spec": map[string]interface{}{},
			// No status
		},
	}

	dynfake := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), hc)

	version, err := getHostedClusterVersion(dynfake, ClusterName, ClusterNamespace)
	assert.NotNil(t, err, "should return error when no status")
	assert.Contains(t, err.Error(), "Unable to determine HostedCluster version")
	assert.Empty(t, version, "version should be empty")
}

// Test getHostedClusterVersion with status but no version
func TestGetHostedClusterVersionNoVersionInStatus(t *testing.T) {
	hc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "hypershift.openshift.io/v1beta1",
			"kind":       "HostedCluster",
			"metadata": map[string]interface{}{
				"name":      ClusterName,
				"namespace": ClusterNamespace,
			},
			"spec": map[string]interface{}{},
			"status": map[string]interface{}{
				"conditions": []interface{}{},
				// No version field
			},
		},
	}

	dynfake := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), hc)

	version, err := getHostedClusterVersion(dynfake, ClusterName, ClusterNamespace)
	assert.NotNil(t, err, "should return error when no version in status")
	assert.Contains(t, err.Error(), "Unable to determine HostedCluster version")
	assert.Empty(t, version, "version should be empty")
}

// Test getHostedClusterVersion with empty history but has desired.version (fallback)
func TestGetHostedClusterVersionFallbackToDesired(t *testing.T) {
	hc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "hypershift.openshift.io/v1beta1",
			"kind":       "HostedCluster",
			"metadata": map[string]interface{}{
				"name":      ClusterName,
				"namespace": ClusterNamespace,
			},
			"spec": map[string]interface{}{},
			"status": map[string]interface{}{
				"version": map[string]interface{}{
					"history": []interface{}{}, // Empty history
					"desired": map[string]interface{}{
						"version": "4.14.0", // Fallback to desired
					},
				},
			},
		},
	}

	dynfake := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), hc)

	version, err := getHostedClusterVersion(dynfake, ClusterName, ClusterNamespace)
	assert.Nil(t, err, "should not return error")
	assert.Equal(t, "4.14.0", version, "should return desired.version as fallback")
}

// Test getHostedClusterVersion with history
func TestGetHostedClusterVersionFromHistory(t *testing.T) {
	hc := getHostedClusterWithVersion("AWS", "4.14.0", []interface{}{})

	dynfake := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), hc)

	version, err := getHostedClusterVersion(dynfake, ClusterName, ClusterNamespace)
	assert.Nil(t, err, "should not return error")
	assert.Equal(t, "4.14.0", version, "should return version from history")
}

// Test MonitorUpgradeStatus for NodePools only with timeout
func TestMonitorUpgradeStatusNodePoolsOnlyTimeout(t *testing.T) {
	// NodePool is not ready - should timeout
	clusterCurator := &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterNamespace,
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "upgrade",
			Upgrade: clustercuratorv1.UpgradeHooks{
				DesiredUpdate:  "4.13.7",
				UpgradeType:    clustercuratorv1.UpgradeTypeNodePools,
				MonitorTimeout: 1, // Very short timeout (1 minute = 1 attempt)
			},
		},
	}
	// NodePool at old version, not ready
	np := getNodepool(NodepoolName, ClusterNamespace, ClusterName) // 4.13.6
	hc := getHostedClusterWithVersion("AWS", "4.14.0", []interface{}{})

	dynfake := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), hc, np)
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clusterCurator).Build()

	err := MonitorUpgradeStatus(dynfake, client, ClusterName, clusterCurator)
	assert.NotNil(t, err, "should return error on timeout")
	assert.Contains(t, err.Error(), "Timed out waiting for NodePools upgrade")
}

// Test MonitorUpgradeStatus for default upgrade type (both HC and NPs)
func TestMonitorUpgradeStatusDefaultBothReady(t *testing.T) {
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
				"status":  "False",
				"message": "HostedCluster is at expected version",
			},
		}),
		getNodepoolWithVersion(NodepoolName, ClusterNamespace, ClusterName, "4.13.7"),
	)
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clusterCurator).Build()

	err := MonitorUpgradeStatus(dynfake, client, ClusterName, clusterCurator)
	assert.Nil(t, err, "should succeed when both HC and NPs are ready")
}

// Test MonitorUpgradeStatus for default upgrade type when HC ready but NPs not ready initially
func TestMonitorUpgradeStatusDefaultNodePoolsNotReadyInitially(t *testing.T) {
	clusterCurator := getUpgradeClusterCurator("4.13.7")
	// Start with NodePool not at the right version
	npNotReady := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "hypershift.openshift.io/v1beta1",
			"kind":       "NodePool",
			"metadata": map[string]interface{}{
				"name":      NodepoolName,
				"namespace": ClusterNamespace,
			},
			"spec": map[string]interface{}{
				"clusterName": ClusterName,
				"release": map[string]interface{}{
					"image": "quay.io/openshift-release-dev/ocp-release:4.13.7-multi",
				},
			},
			"status": map[string]interface{}{
				"version": "4.13.6", // Old version
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "True",
					},
					map[string]interface{}{
						"type":   "UpdatingVersion",
						"status": "True", // Still updating
					},
				},
			},
		},
	}
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
		npNotReady,
	)
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clusterCurator).Build()

	// Update NodePool to ready state after a short time
	go func() {
		time.Sleep(utils.PauseTenSeconds)
		npReady := getNodepoolWithVersion(NodepoolName, ClusterNamespace, ClusterName, "4.13.7")
		_, err := dynfake.Resource(utils.NPGVR).Namespace(ClusterNamespace).Update(context.TODO(), npReady, v1.UpdateOptions{})
		assert.Nil(t, err, "should update NodePool")
	}()

	err := MonitorUpgradeStatus(dynfake, client, ClusterName, clusterCurator)
	assert.Nil(t, err, "should succeed after NodePool becomes ready")
}

// Test areNodePoolsReady with multiple NodePools (one for cluster, one not)
func TestAreNodePoolsReadyMultipleNodePools(t *testing.T) {
	// NodePool belonging to our cluster - ready
	np1 := getNodepoolWithVersion(NodepoolName, ClusterNamespace, ClusterName, "4.13.7")
	// NodePool belonging to another cluster - should be ignored
	np2 := getNodepoolWithVersion("other-cluster-np", ClusterNamespace, "other-cluster", "4.12.0")

	dynfake := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), np1, np2)

	ready, err := areNodePoolsReady(dynfake, ClusterName, ClusterNamespace, "4.13.7", nil)
	assert.Nil(t, err, "should not return error")
	assert.True(t, ready, "should return true - only check NodePools for our cluster")
}

// Test areNodePoolsReady with specific NodePool names filter
func TestAreNodePoolsReadyWithSpecificNames(t *testing.T) {
	// NodePool1 ready at desired version
	np1 := getNodepoolWithVersion("nodepool-1", ClusterNamespace, ClusterName, "4.13.7")
	// NodePool2 NOT ready (old version) - but should be ignored since we only specify nodepool-1
	np2 := getNodepoolWithVersion("nodepool-2", ClusterNamespace, ClusterName, "4.13.6")

	dynfake := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), np1, np2)

	// Only check nodepool-1
	ready, err := areNodePoolsReady(dynfake, ClusterName, ClusterNamespace, "4.13.7", []string{"nodepool-1"})
	assert.Nil(t, err, "should not return error")
	assert.True(t, ready, "should return true - only nodepool-1 is checked and it's ready")
}

// Test areNodePoolsReady when specified NodePool is not ready
func TestAreNodePoolsReadyWithSpecificNamesNotReady(t *testing.T) {
	// NodePool1 ready at desired version
	np1 := getNodepoolWithVersion("nodepool-1", ClusterNamespace, ClusterName, "4.13.7")
	// NodePool2 NOT ready (old version)
	np2 := getNodepoolWithVersion("nodepool-2", ClusterNamespace, ClusterName, "4.13.6")

	dynfake := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), np1, np2)

	// Check nodepool-2 which is not ready
	ready, err := areNodePoolsReady(dynfake, ClusterName, ClusterNamespace, "4.13.7", []string{"nodepool-2"})
	assert.Nil(t, err, "should not return error")
	assert.False(t, ready, "should return false - nodepool-2 is not at desired version")
}

// Test upgrading specific NodePools only
func TestUpgradeClusterSpecificNodePools(t *testing.T) {
	clusterCurator := &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterNamespace,
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "upgrade",
			Upgrade: clustercuratorv1.UpgradeHooks{
				DesiredUpdate: "4.13.7",
				UpgradeType:   clustercuratorv1.UpgradeTypeNodePools,
				NodePoolNames: []string{"nodepool-1"}, // Only upgrade nodepool-1
			},
		},
	}
	managedClusterInfo := &managedclusterinfov1beta1.ManagedClusterInfo{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Status: managedclusterinfov1beta1.ClusterInfoStatus{
			DistributionInfo: managedclusterinfov1beta1.DistributionInfo{
				OCP: managedclusterinfov1beta1.OCPDistributionInfo{
					Version: "4.13.7", // Control plane version
				},
			},
		},
	}
	hc := getHostedClusterWithVersion("AWS", "4.13.7", []interface{}{
		map[string]interface{}{
			"type":    "Degraded",
			"status":  "False",
			"message": "The hosted cluster is not degraded",
		},
	})
	np1 := getNodepool("nodepool-1", ClusterNamespace, ClusterName)
	np2 := getNodepool("nodepool-2", ClusterNamespace, ClusterName)

	dynfake := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), hc, np1, np2)
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(managedclusterinfov1beta1.SchemeGroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(clusterCurator, managedClusterInfo).Build()

	err := UpgradeCluster(client, dynfake, ClusterName, clusterCurator)
	assert.Nil(t, err, "err should be nil when upgrading specific NodePools")

	// Verify nodepool-1 was upgraded
	updatedNp1, _ := dynfake.Resource(utils.NPGVR).Namespace(ClusterNamespace).Get(context.TODO(), "nodepool-1", v1.GetOptions{})
	np1Spec := updatedNp1.Object["spec"].(map[string]interface{})
	np1Release := np1Spec["release"].(map[string]interface{})
	assert.Equal(t, "quay.io/openshift-release-dev/ocp-release:4.13.7-multi", np1Release["image"], "nodepool-1 should be upgraded")

	// Verify nodepool-2 was NOT upgraded
	updatedNp2, _ := dynfake.Resource(utils.NPGVR).Namespace(ClusterNamespace).Get(context.TODO(), "nodepool-2", v1.GetOptions{})
	np2Spec := updatedNp2.Object["spec"].(map[string]interface{})
	np2Release := np2Spec["release"].(map[string]interface{})
	assert.Equal(t, "quay.io/openshift-release-dev/ocp-release:4.13.6-multi", np2Release["image"], "nodepool-2 should NOT be upgraded")
}

// Test getHostedClusterVersion when history exists but version field in history is nil
func TestGetHostedClusterVersionHistoryNoVersion(t *testing.T) {
	hc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "hypershift.openshift.io/v1beta1",
			"kind":       "HostedCluster",
			"metadata": map[string]interface{}{
				"name":      ClusterName,
				"namespace": ClusterNamespace,
			},
			"spec": map[string]interface{}{},
			"status": map[string]interface{}{
				"version": map[string]interface{}{
					"history": []interface{}{
						map[string]interface{}{
							"state": "Completed",
							// No version field
						},
					},
					"desired": map[string]interface{}{
						"version": "4.14.0", // Fallback
					},
				},
			},
		},
	}

	dynfake := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), hc)

	version, err := getHostedClusterVersion(dynfake, ClusterName, ClusterNamespace)
	assert.Nil(t, err, "should not return error - fallback to desired")
	assert.Equal(t, "4.14.0", version, "should return desired.version as fallback")
}
