// Copyright Contributors to the Open Cluster Management project.

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"
)

const ClusterName = "my-cluster"

func TestFindJobConfigMap(t *testing.T) {
	kubeset := fake.NewSimpleClientset(getConfigMap(ClusterName))

	managedCluster := getManagedClusterTemplate(ClusterName)

	configMap, err := findJobConfigMap(kubeset, managedCluster)

	assert.Nil(t, err, "err is nil when ConfigMap is found")
	assert.NotNil(t, configMap, "configMap was found")
	t.Log("ConfigMap found: " + configMap.Name)

}

func TestFindJobConfigMapMissing(t *testing.T) {
	kubeset := fake.NewSimpleClientset()

	managedCluster := getManagedClusterTemplate(ClusterName)

	configMap, err := findJobConfigMap(kubeset, managedCluster)

	assert.NotNil(t, err, "err not nil when ConfigMap is missing")
	assert.Nil(t, configMap, "configMap was not found")

}

func TestFilterConfigMaps(t *testing.T) {
	listOptions := filterConfigMaps()
	assert.NotNil(t, listOptions, "filterConfigMaps is not nil")
	assert.Equal(t, listOptions.LabelSelector, "open-cluster-management=curator", "The label open-cluster-management should be set")
	t.Log("LabelSelector: " + listOptions.LabelSelector)
}
