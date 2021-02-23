// Copyright Contributors to the Open Cluster Management project.
package launcher

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const numInitContainers = 4
const imageTag = "sha256:123456789"
const imageURI = "quay.io/my-repo/cluster-curator-controller"
const configMapName = "my-cluster"
const clusterName = configMapName

// Validate that we are correctly building the job.batchv1 object
func TestGetBatchJob(t *testing.T) {

	t.Log("Starting TestGetBatchJob\nCreate a batchJobObj with sha256")
	batchJobObj := getBatchJob(imageTag, configMapName, imageURI)
	t.Logf("%v", batchJobObj)

	t.Log("Test count initContainers in job")
	foundInitContainers := len(batchJobObj.Spec.Template.Spec.InitContainers)

	if foundInitContainers != 4 {
		t.Fatalf("Invalid InitContainers count, expected %v found %v\n",
			numInitContainers, foundInitContainers)
	}

	t.Log("Validate configmap URI")
	t.Logf("Check image is applied correclty %v@%v", imageURI, imageTag)
	uri := imageURI + "@" + imageTag

	t.Log("Access the first initContainer")
	initContianer := batchJobObj.Spec.Template.Spec.InitContainers[0]

	if initContianer.Image != uri {
		log.Fatalf("The initContainer.image did not have the correct URI %v, expected %v", initContianer.Image, uri)
	}

	t.Log("Validate configMapName is placed in initContianer")
	if initContianer.Env[0].Value != configMapName {
		t.Fatalf("The configMapName was not corrctly populated %v", initContianer.Env[0].Value)
	}

	t.Log("Create a batchJobObj with no sha256 or URI")
	batchJobObj = getBatchJob("", configMapName, imageURI)
	t.Logf("%v", batchJobObj)

	t.Logf("Check image is applied correclty %v@%v", imageURI, imageTag)
	uri = imageURI + ":latest"

	t.Log("Access the first initContainer")
	initContianer = batchJobObj.Spec.Template.Spec.InitContainers[0]

	if initContianer.Image != uri {
		log.Fatalf("The initContainer.image did not have the correct URI %v, expected %v", initContianer.Image, uri)
	}

	t.Log("Finished TestGetBatchJob")
}

// Test the launcher to create a job.batchv1 object
func TestCreateLauncher(t *testing.T) {
	jobConfigMap := &corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{Name: configMapName, Namespace: configMapName},
		Data:       map[string]string{"providerCredentialPath": "default/provider-secret"},
	}

	kubeset := fake.NewSimpleClientset(jobConfigMap)

	testLauncher := NewLauncher(kubeset, imageTag, imageURI, *jobConfigMap)

	assert.NotNil(t, testLauncher, "launcher is not nil")

	err := testLauncher.CreateJob()

	assert.Nil(t, err, "error is nil")

}
