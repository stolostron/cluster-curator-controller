// Copyright Contributors to the Open Cluster Management project.
package launcher

import (
	"context"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const numInitContainers = 4
const imageURI = "quay.io/my-repo/cluster-curator-controller@sha123456789"
const configMapName = "my-cluster"
const clusterName = configMapName

// Validate that we are correctly building the job.batchv1 object
func TestGetBatchJobImageSHA(t *testing.T) {

	batchJobObj := getBatchJob(configMapName, imageURI)

	t.Log("Test count initContainers in job")
	foundInitContainers := len(batchJobObj.Spec.Template.Spec.InitContainers)

	if foundInitContainers != 5 {
		t.Fatalf("Invalid InitContainers count, expected %v found %v\n",
			numInitContainers, foundInitContainers)
	}

	t.Log("Validate configmap URI")
	t.Logf("Check image is applied correclty %v", imageURI)
	uri := imageURI

	t.Log("Access the first initContainer")
	initContianer := batchJobObj.Spec.Template.Spec.InitContainers[0]

	if initContianer.Image != uri {
		log.Fatalf("The initContainer.image did not have the correct URI %v, expected %v", initContianer.Image, uri)
	}

	t.Log("Validate configMapName is placed in initContianer")
	if initContianer.Env[0].Value != configMapName {
		t.Fatalf("The configMapName was not corrctly populated %v", initContianer.Env[0].Value)
	}
}

// Use the default URI
func TestGetBatchJobImageDefault(t *testing.T) {

	t.Log("Create a batchJobObj with no sha256 or URI")
	batchJobObj := getBatchJob(configMapName, imageURI)

	t.Logf("Check image is applied correclty %v", imageURI)
	uri := imageURI

	t.Log("Access the first initContainer")
	initContianer := batchJobObj.Spec.Template.Spec.InitContainers[0]

	if initContianer.Image != uri {
		t.Fatalf("The initContainer.image did not have the correct URI %v, expected %v", initContianer.Image, uri)
	}
}

// Test the launcher to create a job.batchv1 object
func TestCreateLauncher(t *testing.T) {

	jobConfigMap := &corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{Name: configMapName, Namespace: configMapName},
		Data:       map[string]string{"providerCredentialPath": "default/provider-secret"},
	}

	kubeset := fake.NewSimpleClientset(jobConfigMap)

	testLauncher := NewLauncher(kubeset, imageURI, *jobConfigMap)

	assert.NotNil(t, testLauncher, "launcher is not nil")

	err := testLauncher.CreateJob()

	assert.Nil(t, err, "error is nil")

	job, err := kubeset.BatchV1().Jobs(configMapName).Get(context.TODO(), "", v1.GetOptions{})

	assert.Nil(t, err, "err is nil, if ConfigMap found")

	// Test the dynamic job vavlues
	if job.GenerateName != "curator-job-" {
		t.Fatal("Job obbject not found")
	}

	if job.Spec.Template.Spec.InitContainers[0].Image != imageURI {
		t.Fatalf("Default imageURI does not match: %v", job.Spec.Template.Spec.InitContainers[0].Image)
	}

	if job.Spec.Template.Spec.InitContainers[0].Env[0].Value != configMapName {

		t.Fatalf("Container init configMap name does not correct: %v",
			job.Spec.Template.Spec.InitContainers[0].Env[0].Value)
	}

}

// Test launcher with a bad configMap path
func TestCreateLauncherBadConfigMap(t *testing.T) {

	jobConfigMap := &corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{Name: configMapName, Namespace: configMapName},
		Data:       map[string]string{"providerCredentialPathInvalid": "default/provider-secret"},
	}

	kubeset := fake.NewSimpleClientset(jobConfigMap)

	testLauncher := NewLauncher(kubeset, imageURI, *jobConfigMap)

	assert.NotNil(t, testLauncher, "launcher is not nil")

	err := testLauncher.CreateJob()

	assert.NotNil(t, err, "Invalid jobConfigMap detected")
}

// Test launcher with a valid overrideJob
func TestCreateLauncherOverrideJob(t *testing.T) {

	batchJobObj := getBatchJob(configMapName, imageURI)
	stringData, err := yaml.Marshal(batchJobObj)
	if err != nil {
		t.Fatal("Failed to marshal batchJobObj")
	}

	jobConfigMap := &corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{Name: configMapName, Namespace: configMapName},
		Data: map[string]string{
			"providerCredentialPath": "default/provider-secret",
			OverrideJob:              string(stringData),
		},
	}

	kubeset := fake.NewSimpleClientset(jobConfigMap)

	testLauncher := NewLauncher(kubeset, imageURI, *jobConfigMap)

	assert.NotNil(t, testLauncher, "launcher is not nil")

	err = testLauncher.CreateJob()

	assert.Nil(t, err, "Job create is nil")
}

// Test launcher with an Invalid overrideJob
func TestCreateLauncherInvalidOverrideJob(t *testing.T) {

	jobConfigMap := &corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{Name: configMapName, Namespace: configMapName},
		Data: map[string]string{
			"providerCredentialPath": "default/provider-secret",
			OverrideJob:              "Not a valid job.batchv1: specification!!",
		},
	}

	kubeset := fake.NewSimpleClientset(jobConfigMap)

	testLauncher := NewLauncher(kubeset, imageURI, *jobConfigMap)

	assert.NotNil(t, testLauncher, "launcher is not nil")

	err := testLauncher.CreateJob()

	assert.NotNil(t, err, "CreateJob err is not nil")
}
