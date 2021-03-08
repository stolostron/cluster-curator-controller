// Copyright Contributors to the Open Cluster Management project.
package hive

import (
	"context"
	"testing"
	"time"

	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/utils"
	hivev1 "github.com/openshift/hive/pkg/apis/hive/v1"
	hivefake "github.com/openshift/hive/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
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
		Data: map[string]string{
			"prehook": "    - name: Service now App Update\n" +
				"      extra_vars:\n" +
				"        variable1: \"1\"\n" +
				"        variable2: \"2\"\n",
		},
	}
}
func TestActivateDeployNoCD(t *testing.T) {

	hiveset := hivefake.NewSimpleClientset()

	t.Log("No ClusterDeployment")
	assert.NotNil(t, ActivateDeploy(hiveset, ClusterName),
		"err NotNil when ClusterDeployment kind does not exist")
}

func TestActivateDeployNoInstallAttemptsLimit(t *testing.T) {

	hiveset := hivefake.NewSimpleClientset(&hivev1.ClusterDeployment{
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
	hiveset := hivefake.NewSimpleClientset(&hivev1.ClusterDeployment{
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
	hiveset := hivefake.NewSimpleClientset(&hivev1.ClusterDeployment{
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

func TestMonitorDeployStatusNoClusterDeployment(t *testing.T) {

	hiveset := hivefake.NewSimpleClientset()

	assert.NotNil(t, monitorDeployStatus(nil, hiveset, ClusterName), "err is not nil, when cluster provisioning has a condition")
}

func TestMonitorDeployStatusCondition(t *testing.T) {

	cd := getClusterDeployment()
	cd.Status.Conditions = []hivev1.ClusterDeploymentCondition{
		hivev1.ClusterDeploymentCondition{
			Message: "ClusterImageSet img4.6.17-x86-64-appsub is not available",
			Type:    "ClusterImageSetNotFound",
			Status:  "True",
		},
	}

	hiveset := hivefake.NewSimpleClientset(cd)

	assert.NotNil(t, monitorDeployStatus(nil, hiveset, ClusterName), "err is not nil, when cluster provisioning has a condition")
}

func TestMonitorDeployNoJobTimeout(t *testing.T) {

	cd := getClusterDeployment()

	hiveset := hivefake.NewSimpleClientset(cd)

	assert.NotNil(t, monitorDeployStatus(nil, hiveset, ClusterName), "err is not nil, when cluster provisioning has no job created")
}
func TestMonitorDeployStatusJobFailed(t *testing.T) {

	cd := getClusterDeployment()
	cd.Status.ProvisionRef = &corev1.LocalObjectReference{
		Name: ClusterName + "-12345", // The monitor adds the -provision suffix
	}

	job := getProvisionJob()
	job.Status.Succeeded = 0

	hiveset := hivefake.NewSimpleClientset(cd)

	kubeset := fake.NewSimpleClientset(job)

	assert.NotNil(t, monitorDeployStatus(kubeset, hiveset, ClusterName), "err is not nil, when cluster provisioning has a condition")
}

func TestMonitorDeployStatusJobCompletedWithSuccess(t *testing.T) {

	cd := getClusterDeployment()
	cd.Status.Conditions = []hivev1.ClusterDeploymentCondition{
		hivev1.ClusterDeploymentCondition{
			Message: "Provisioned",
			Reason:  "SuccessfulProvision",
		},
	}
	cd.Status.ProvisionRef = &corev1.LocalObjectReference{
		Name: ClusterName + "-12345", // The monitor adds the -provision suffix
	}

	cm := getConfigMap()

	hiveset := hivefake.NewSimpleClientset(cd)
	kubeset := fake.NewSimpleClientset(getProvisionJob(), cm)

	assert.Equal(t, cm.Data[utils.CurrentHiveJob], "", "Should be emtpy")
	assert.Nil(t, monitorDeployStatus(kubeset, hiveset, ClusterName), "err is nil, when cluster provisioning is successful")

	// Put a delay to complete the ClusterDeployment to test the wait loop
	go func() {
		time.Sleep(utils.PauseFiveSeconds)
		cd.Status.WebConsoleURL = "https://my-cluster"

		hiveset.HiveV1().ClusterDeployments(ClusterName).Update(context.TODO(), cd, v1.UpdateOptions{})
		t.Log("ClusterDeployment webConsoleURL applied")
	}()

	cm, err := kubeset.CoreV1().ConfigMaps(ClusterName).Get(context.TODO(), ClusterName, v1.GetOptions{})
	t.Log(err)
	assert.NotEqual(t, cm.Data[utils.CurrentHiveJob], "", "Should be populated")
}

func TestMonitorDeployStatusJobComplete(t *testing.T) {

	cd := getClusterDeployment()
	cd.Status.ProvisionRef = &corev1.LocalObjectReference{
		Name: ClusterName + "-12345", // The monitor adds the -provision suffix
	}

	job := getProvisionJob()
	job.Status.Active = 1
	job.Status.Succeeded = 0

	hiveset := hivefake.NewSimpleClientset(cd)
	kubeset := fake.NewSimpleClientset(job)

	// Put a delay to complete the job to test the wait loop
	go func() {
		time.Sleep(utils.PauseFiveSeconds)
		job.Status.Active = 0
		job.Status.Succeeded = 1
		kubeset.BatchV1().Jobs(ClusterName).Update(context.TODO(), job, v1.UpdateOptions{})
		cd.Status.WebConsoleURL = "https://my-cluster"
		cd.Status.Conditions = []hivev1.ClusterDeploymentCondition{
			hivev1.ClusterDeploymentCondition{
				Message: "Provisioned",
				Reason:  "SuccessfulProvision",
			},
		}
		hiveset.HiveV1().ClusterDeployments(ClusterName).Update(context.TODO(), cd, v1.UpdateOptions{})
	}()

	assert.Nil(t, monitorDeployStatus(kubeset, hiveset, ClusterName), "err is not nil, when cluster provisioning is successful")
}

func TestMonitorDeployStatusJobDelayedComplete(t *testing.T) {

	cd := getClusterDeployment()
	cd.Status.ProvisionRef = &corev1.LocalObjectReference{
		Name: ClusterName + "-12345", // The monitor adds the -provision suffix
	}

	hiveset := hivefake.NewSimpleClientset(cd)
	kubeset := fake.NewSimpleClientset()

	// Put a delay to complete the job to test the wait loop
	go func() {
		time.Sleep(utils.PauseFiveSeconds)
		cd.Status.WebConsoleURL = "https://my-cluster"
		cd.Status.Conditions = []hivev1.ClusterDeploymentCondition{
			hivev1.ClusterDeploymentCondition{
				Message: "Provisioned",
				Reason:  "SuccessfulProvision",
			},
		}
		hiveset.HiveV1().ClusterDeployments(ClusterName).Update(context.TODO(), cd, v1.UpdateOptions{})
		kubeset.BatchV1().Jobs(ClusterName).Create(context.TODO(), getProvisionJob(), v1.CreateOptions{})
	}()

	assert.Nil(t, monitorDeployStatus(kubeset, hiveset, ClusterName), "err is not nil, when cluster provisioning is successful")
}
