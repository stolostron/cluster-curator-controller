// Copyright Contributors to the Open Cluster Management project.

package main

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const ClusterName = "my-cluster"

func getConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: v1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
			Labels: map[string]string{
				"open-cluster-management": "curator",
			},
		},
		Data: map[string]string{},
	}
}

func TestCuratorRunNoParam(t *testing.T) {

	defer func() {
		r := recover()
		t.Log(r.(error).Error())

		if !strings.Contains(r.(error).Error(), "Command: ./curator [") &&
			!strings.Contains(r.(error).Error(), "Invalid Parameter: \"\"") {
			t.Fatal(r)
		}
		t.Log("Detected missing paramter")
	}()

	os.Args[1] = ""

	curatorRun(nil, nil, ClusterName)
}

func TestCuratorRunWrongParam(t *testing.T) {

	defer func() {
		r := recover()
		t.Log(r.(error).Error())

		if !strings.Contains(r.(error).Error(), "Command: ./curator [") &&
			!strings.Contains(r.(error).Error(), "something-wrong") {
			t.Fatal(r)
		}
		t.Log("Detected wrong paramter")
	}()

	os.Args[1] = "something-wrong"

	curatorRun(nil, nil, ClusterName)
}

func TestCuratorRunNoConfigMap(t *testing.T) {

	defer func() {
		r := recover()
		t.Log(r.(error).Error())

		if !strings.Contains(r.(error).Error(), "configmaps \"my-cluster\"") {
			t.Fatal(r)
		}
		t.Log("Detected missing ConfigMap")
	}()

	kubeset := fake.NewSimpleClientset()

	os.Args[1] = "SKIP_ALL_TESTING"

	curatorRun(kubeset, nil, ClusterName)
}

func TestCuratorRunConfigMap(t *testing.T) {

	kubeset := fake.NewSimpleClientset(getConfigMap())

	os.Args[1] = "SKIP_ALL_TESTING"

	assert.NotPanics(t, func() { curatorRun(kubeset, nil, ClusterName) }, "no panic when configmap found and skip test")
}

func TestCuratorRunNoProviderCredentialPath(t *testing.T) {

	defer func() {
		r := recover()
		t.Log(r.(error).Error())

		if !strings.Contains(r.(error).Error(), "Missing spec.data.providerCredentialPath") {
			t.Fatal(r)
		}
		t.Log("Detected missing provierCredentialPath")
	}()

	kubeset := fake.NewSimpleClientset(getConfigMap())

	os.Args[1] = "applycloudprovider-ansible"

	curatorRun(kubeset, nil, ClusterName)
}

func TestCuratorRunProviderCredentialPathEnv(t *testing.T) {

	defer func() {
		r := recover()
		t.Log(r.(error).Error())

		if !strings.Contains(r.(error).Error(), "secrets \"secretname\" not found") {
			t.Fatal(r)
		}
		t.Log("Detected missing namespace/secretName")
	}()

	os.Setenv("PROVIDER_CREDENTIAL_PATH", "namespace/secretname")
	kubeset := fake.NewSimpleClientset()

	os.Args[1] = "applycloudprovider-ansible"

	curatorRun(kubeset, nil, ClusterName)
}

func TestCuratorRunConfigMapDifferentClusterName(t *testing.T) {

	defer func() {
		r := recover()
		t.Log(r.(error).Error())

		if !strings.Contains(r.(error).Error(),
			"Cluster namespace \"my-cluster\" does not match the cluster") {
			t.Fatal(r)
		}
		t.Log("Detected ConfigMap.Data.clusterName mis-match")
	}()

	cm := getConfigMap()
	cm.Data["clusterName"] = ClusterName + "123"

	kubeset := fake.NewSimpleClientset(cm)

	os.Args[1] = "SKIP_ALL_TESTING"

	curatorRun(kubeset, nil, ClusterName)
}
