// Copyright Contributors to the Open Cluster Management project.
package hive

import (
	"testing"

	hivev1 "github.com/openshift/hive/pkg/apis/hive/v1"
	"github.com/openshift/hive/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const ClusterName = "my-cluster"

func TestActivateDeployNoCD(t *testing.T) {

	hiveset := fake.NewSimpleClientset()

	t.Log("No ClusterDeployment")
	assert.NotNil(t, ActivateDeploy(hiveset, ClusterName),
		"err NotNil when ClusterDeployment kind does not exist")
}

func TestActivateDeployNoInstallAttemptsLimit(t *testing.T) {

	hiveset := fake.NewSimpleClientset(&hivev1.ClusterDeployment{
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
	hiveset := fake.NewSimpleClientset(&hivev1.ClusterDeployment{
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
	hiveset := fake.NewSimpleClientset(&hivev1.ClusterDeployment{
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
