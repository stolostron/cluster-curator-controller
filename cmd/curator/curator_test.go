// Copyright Contributors to the Open Cluster Management project.

package main

import (
	"os"
	"strings"
	"testing"

	clustercuratorv1 "github.com/open-cluster-management/cluster-curator-controller/pkg/api/v1beta1"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/utils"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		},
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

func TestCuratorRunNoClusterCurator(t *testing.T) {

	defer func() {
		r := recover()
		t.Log(r.(error).Error())

		if !strings.Contains(r.(error).Error(), "clustercurators.cluster.open-cluster-management.io \"my-cluster\"") {
			t.Fatal(r)
		}
		t.Log("Detected missing ClusterCurator resource")
	}()

	s := scheme.Scheme
	s.AddKnownTypes(utils.CCGVR.GroupVersion(), &clustercuratorv1.ClusterCurator{})

	client := clientfake.NewFakeClientWithScheme(s)

	os.Args[1] = "SKIP_ALL_TESTING"

	curatorRun(nil, client, ClusterName)
}

func TestCuratorRunClusterCurator(t *testing.T) {

	s := scheme.Scheme
	s.AddKnownTypes(utils.CCGVR.GroupVersion(), &clustercuratorv1.ClusterCurator{})

	client := clientfake.NewFakeClientWithScheme(s, getClusterCurator())

	os.Args[1] = "SKIP_ALL_TESTING"

	assert.NotPanics(t, func() { curatorRun(nil, client, ClusterName) }, "no panic when ClusterCurator found and skip test")
}

func TestCuratorRunNoProviderCredentialPath(t *testing.T) {

	defer func() {
		r := recover()
		t.Log(r.(error).Error())

		if !strings.Contains(r.(error).Error(), "Missing spec.providerCredentialPath") {
			t.Fatal(r)
		}
		t.Log("Detected missing provierCredentialPath")
	}()

	s := scheme.Scheme
	s.AddKnownTypes(utils.CCGVR.GroupVersion(), &clustercuratorv1.ClusterCurator{})

	client := clientfake.NewFakeClientWithScheme(s, getClusterCurator())

	os.Args[1] = "applycloudprovider-ansible"

	curatorRun(nil, client, ClusterName)
}

func TestCuratorRunProviderCredentialPathEnv(t *testing.T) {

	defer func() {
		r := recover()
		t.Log(r.(error).Error())

		if !strings.Contains(r.(error).Error(), "secrets \"secretname\"") {
			t.Fatal(r)
		}
		t.Log("Detected missing namespace/secretName")
	}()

	os.Setenv("PROVIDER_CREDENTIAL_PATH", "namespace/secretname")
	client := clientfake.NewFakeClient()

	os.Args[1] = "applycloudprovider-ansible"

	curatorRun(nil, client, ClusterName)
}
