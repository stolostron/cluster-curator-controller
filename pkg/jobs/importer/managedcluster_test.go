// Copyright Contributors to the Open Cluster Management project.
package importer

import (
	"context"
	"testing"
	"time"

	"github.com/open-cluster-management/api/client/cluster/clientset/versioned/fake"
	managedclusterv1 "github.com/open-cluster-management/api/cluster/v1"
	"github.com/stolostron/cluster-curator-controller/pkg/jobs/utils"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
)

const ClusterName = "my-cluster"

func TestMonitorManagedClusterMissing(t *testing.T) {
	mcset := fake.NewSimpleClientset()
	assert.NotNil(t, MonitorImport(mcset, ClusterName), "err not nil, when no ManagedCluster object is present")
}

// Uses all three stages, otherwise it loops infinitely
func TestMonitorManagedClusterConditionAvailable(t *testing.T) {

	mcset := fake.NewSimpleClientset(&managedclusterv1.ManagedCluster{
		TypeMeta: v1.TypeMeta{
			Kind:       "ManagedCluster",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: ClusterName,
		},
		Status: managedclusterv1.ManagedClusterStatus{
			Conditions: []v1.Condition{
				v1.Condition{
					Type: managedclusterv1.ManagedClusterConditionHubAccepted,
				},
				v1.Condition{
					Type: managedclusterv1.ManagedClusterConditionJoined,
				},
				v1.Condition{
					Type: managedclusterv1.ManagedClusterConditionAvailable,
				},
			},
		},
	})

	assert.Nil(t, MonitorImport(mcset, ClusterName), "err nil, when ManagedCluster is available")
}

func TestMonitorManagedClusterConditionDenied(t *testing.T) {

	mcset := fake.NewSimpleClientset(&managedclusterv1.ManagedCluster{
		TypeMeta: v1.TypeMeta{
			Kind:       "ManagedCluster",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: ClusterName,
		},
		Status: managedclusterv1.ManagedClusterStatus{
			Conditions: []v1.Condition{
				v1.Condition{
					Type: managedclusterv1.ManagedClusterConditionHubDenied,
				},
			},
		},
	})

	assert.NotNil(t, MonitorImport(mcset, ClusterName), "err not nil, when ManagedCluster join condition is denied")
}

func getManagedClusterInfos(conditionType string, conditionMessage string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "internal.open-cluster-management.io/v1beta1",
			"kind":       "ManagedClusterInfo",
			"metadata": map[string]interface{}{
				"name":      ClusterName,
				"namespace": ClusterName,
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":    conditionType,
						"message": conditionMessage,
					},
				},
			},
		},
	}
}

func TestMonitorMCInfoConditionMissingManagedClusterInfos(t *testing.T) {

	dynfake := dynfake.NewSimpleDynamicClient(runtime.NewScheme())
	assert.NotNil(t, MonitorMCInfoImport(dynfake, ClusterName), "err not nil, when ManagedClusterInfo resource is not present")
}
func TestMonitorMCInfoConditionAvailable(t *testing.T) {

	dynfake := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), getManagedClusterInfos(managedclusterv1.ManagedClusterConditionAvailable, "All good"))
	assert.Nil(t, MonitorMCInfoImport(dynfake, ClusterName), "err nil, when ManagedClusterInfos is available")
}

func TestMonitorMCInfoConditionDenied(t *testing.T) {

	dynfake := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), getManagedClusterInfos(managedclusterv1.ManagedClusterConditionHubDenied, "Not Allowed"))
	assert.NotNil(t, MonitorMCInfoImport(dynfake, ClusterName), "err not nil, when ManagedClusterInfos is denied")
}

var mciGVR = schema.GroupVersionResource{
	Group: "internal.open-cluster-management.io", Version: "v1beta1", Resource: "managedclusterinfos"}

func TestMonitorMCInfoConditionFullFlowAvailable(t *testing.T) {

	dynfake := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), getManagedClusterInfos("", ""))
	go func() {

		time.Sleep(utils.PauseFiveSeconds)
		_, err := dynfake.Resource(mciGVR).Namespace(ClusterName).Update(context.TODO(), getManagedClusterInfos(managedclusterv1.ManagedClusterConditionJoined, "Cluster has joined."), v1.UpdateOptions{})
		assert.Nil(t, err, "err is nill, when ManagedClusterConditionJoined condition is created")

		time.Sleep(utils.PauseFiveSeconds)
		_, err = dynfake.Resource(mciGVR).Namespace(ClusterName).Update(context.TODO(), getManagedClusterInfos(managedclusterv1.ManagedClusterConditionAvailable, "connected"), v1.UpdateOptions{})
		assert.Nil(t, err, "err is nill, when ManagedClusterConditionAvailable condition is updated")
	}()
	assert.Nil(t, MonitorMCInfoImport(dynfake, ClusterName), "err nil, when ManagedCluster is available")
}

// Includes a test for no-initial conditions
func TestMonitorMCInfoConditionFullFlowDenied(t *testing.T) {

	mci := getManagedClusterInfos("", "")
	mci.Object["status"].(map[string]interface{})["conditions"] = nil
	//s.AddKnownTypes(ajv1.SchemeBuilder.GroupVersion, &ajv1.AnsibleJob{}, &corev1.ConfigMap{})
	dynfake := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), mci)
	go func() {

		time.Sleep(utils.PauseFiveSeconds)
		_, err := dynfake.Resource(mciGVR).Namespace(ClusterName).Update(context.TODO(), getManagedClusterInfos(managedclusterv1.ManagedClusterConditionJoined, "Cluster has joined."), v1.UpdateOptions{})
		assert.Nil(t, err, "err is nill, when ManagedClusterConditionJoined condition is created")

		time.Sleep(utils.PauseFiveSeconds)
		_, err = dynfake.Resource(mciGVR).Namespace(ClusterName).Update(context.TODO(), getManagedClusterInfos(managedclusterv1.ManagedClusterConditionHubDenied, "connected"), v1.UpdateOptions{})
		assert.Nil(t, err, "err is nill, when ManagedClusterConditionAvailable condition is updated")
	}()
	assert.NotNil(t, MonitorMCInfoImport(dynfake, ClusterName), "err not nil, when ManagedCluster is denied")
}
