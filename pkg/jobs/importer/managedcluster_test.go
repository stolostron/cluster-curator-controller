// Copyright Contributors to the Open Cluster Management project.
package importer

import (
	"testing"

	"github.com/open-cluster-management/api/client/cluster/clientset/versioned/fake"
	managedclusterv1 "github.com/open-cluster-management/api/cluster/v1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
