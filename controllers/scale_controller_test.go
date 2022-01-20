//go:build !skip
// +build !skip

// Copyright Contributors to the Open Cluster Management project.

package controllers

import (
	"context"
	"strconv"
	"testing"

	"github.com/stolostron/library-go/pkg/config"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	managedclusterclient "open-cluster-management.io/api/client/cluster/clientset/versioned"
	managedclusterv1 "open-cluster-management.io/api/cluster/v1"
)

func getManagedClusterTemplate(clusterName string) *managedclusterv1.ManagedCluster {
	return &managedclusterv1.ManagedCluster{
		TypeMeta: v1.TypeMeta{
			Kind:       "managedcluster",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      clusterName,
			Namespace: clusterName,
		},
		Spec: managedclusterv1.ManagedClusterSpec{
			HubAcceptsClient: true,
		},
	}
}

const clusterNamePrefix = "cluster-"
const ClusterTestCount = 2

func getConfigMap(clusterName string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: v1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      clusterName,
			Namespace: clusterName,
			Labels: map[string]string{
				"open-cluster-management": "curator",
			},
		},
		Data: map[string]string{
			"prehook":                jobYaml,
			"posthook":               jobYaml,
			"providerCredentialPath": "default/my-cloudprovider",
		},
	}
}

const jobYaml = "    - name: Service now App Update\n" +
	"      extra_vars:\n" +
	"        variable1: 1\n" +
	"        variable2: 2\n"

func getNamespace(clusterName string) *corev1.Namespace {
	return &corev1.Namespace{
		TypeMeta: v1.TypeMeta{
			Kind:       "namespace",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: clusterName,
		},
	}
}

func skipShort(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test in short mode.")
	}
}

func TestCreateControllerScale(t *testing.T) {

	skipShort(t)

	config, err := config.LoadConfig("", "", "")
	if err != nil {
		t.Fatal("Could not load Kube Config")
	}

	mcset, err := managedclusterclient.NewForConfig(config)
	if err != nil {
		t.Fatal("Could not create clientset")
	}

	kubeset, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatal("Could not create clientset")
	}

	t.Logf("Create %v ManagedCluster objects", ClusterTestCount)
	for i := 1; i <= ClusterTestCount; i++ {

		clusterName := clusterNamePrefix + strconv.Itoa(i)

		ns, err := kubeset.CoreV1().Namespaces().Create(context.TODO(), getNamespace(clusterName), v1.CreateOptions{})
		assert.Nil(t, err, "err nil, when namespace is created")

		_, err = kubeset.CoreV1().ConfigMaps(ns.Name).Create(context.TODO(), getConfigMap(clusterName), v1.CreateOptions{})
		assert.Nil(t, err, "err nil, when cluster ConfigMap is created")

		_, err = mcset.ClusterV1().ManagedClusters().Create(context.TODO(), getManagedClusterTemplate(clusterNamePrefix+strconv.Itoa(i)), v1.CreateOptions{})
		assert.Nil(t, err, "err nil, when ManagedCluster is created")
		t.Logf("Created ManagedCluster: %v", clusterName)
	}
}

func TestDeleteManagedClusters(t *testing.T) {

	skipShort(t)

	config, err := config.LoadConfig("", "", "")
	assert.Nil(t, err, "err nil, when kube config is found")

	mcset, err := managedclusterclient.NewForConfig(config)
	assert.Nil(t, err, "err nil, when managedCluster clientset is created")

	kubeset, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatal("Could not create clientset")
	}

	t.Logf("Delete %v ManagedCluster objects", ClusterTestCount)
	for i := 1; i <= ClusterTestCount; i++ {

		clusterName := clusterNamePrefix + strconv.Itoa(i)
		cm, err := mcset.ClusterV1().ManagedClusters().Get(context.TODO(), clusterName, v1.GetOptions{})
		if err != nil {
			continue
		}
		cm.Finalizers = nil
		mcset.ClusterV1().ManagedClusters().Update(context.TODO(), cm, v1.UpdateOptions{})
		kubeset.CoreV1().Namespaces().Delete(context.TODO(), clusterName, v1.DeleteOptions{})
		mcset.ClusterV1().ManagedClusters().Delete(context.TODO(), clusterName, v1.DeleteOptions{})
		t.Logf("Deleted ManagedCluster: %v", clusterName)
	}
}

func TestRemoveFinalizerForManagedClusters(t *testing.T) {

	skipShort(t)

	config, err := config.LoadConfig("", "", "")
	assert.Nil(t, err, "err nil, when kube config is found")

	mcset, err := managedclusterclient.NewForConfig(config)
	assert.Nil(t, err, "err nil, when managedCluster clientset is created")

	t.Logf("Delete %v ManagedCluster objects", ClusterTestCount)
	for i := 1; i <= ClusterTestCount; i++ {

		clusterName := clusterNamePrefix + strconv.Itoa(i)
		cm, err := mcset.ClusterV1().ManagedClusters().Get(context.TODO(), clusterName, v1.GetOptions{})
		if err != nil {
			continue
		}
		cm.Finalizers = nil
		mcset.ClusterV1().ManagedClusters().Update(context.TODO(), cm, v1.UpdateOptions{})
		t.Logf("Deleted ManagedCluster: %v", clusterName)
	}
}
