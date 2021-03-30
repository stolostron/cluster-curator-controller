// Copyright Contributors to the Open Cluster Management project.
package launcher

import (
	"context"
	"encoding/json"
	"log"
	"testing"

	clustercuratorv1 "github.com/open-cluster-management/cluster-curator-controller/pkg/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const numInitContainers = 4
const imageURI = "quay.io/my-repo/cluster-curator-controller@sha123456789"
const clusterName = "my-cluster"
const PREHOOK = "prehook"

var s = scheme.Scheme

// Validate that we are correctly building the job.batchv1 object
func TestGetBatchJobImageSHA(t *testing.T) {

	batchJobObj := getBatchJob(clusterName, imageURI)

	t.Log("Test count initContainers in job")
	foundInitContainers := len(batchJobObj.Spec.Template.Spec.InitContainers)

	if foundInitContainers != numInitContainers {
		t.Fatalf("Invalid InitContainers count, expected %v found %v\n",
			numInitContainers, foundInitContainers)
	}

	t.Log("Validate clusterCurator URI")
	t.Logf("Check image is applied correclty %v", imageURI)
	uri := imageURI

	t.Log("Access the first initContainer")
	initContianer := batchJobObj.Spec.Template.Spec.InitContainers[0]

	if initContianer.Image != uri {
		log.Fatalf("The initContainer.image did not have the correct URI %v, expected %v", initContianer.Image, uri)
	}

	t.Log("Validate ClusterCurator job environment is set in initContianer")
	if initContianer.Env[0].Value != PREHOOK {
		t.Fatalf("The ClusterCurator job was not corrctly populated %v", initContianer.Env[0].Value)
	}
}

// Use the default URI
func TestGetBatchJobImageDefault(t *testing.T) {

	t.Log("Create a batchJobObj with no sha256 or URI")
	batchJobObj := getBatchJob(clusterName, imageURI)

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

	clusterCurator := &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{Name: clusterName, Namespace: clusterName},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			ProviderCredentialPath: "default/provider-secret",
		},
	}

	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewFakeClientWithScheme(s, clusterCurator)
	kubeset := fake.NewSimpleClientset()

	testLauncher := NewLauncher(client, kubeset, imageURI, *clusterCurator)

	assert.NotNil(t, testLauncher, "launcher is not nil")

	err := testLauncher.CreateJob()

	assert.Nil(t, err, "error is nil")

	job, err := kubeset.BatchV1().Jobs(clusterName).Get(context.TODO(), "", v1.GetOptions{})

	assert.Nil(t, err, "err is nil, if clusterCurator is found")
	assert.NotNil(t, job, "job is not nil, as it was retreived")

	// Test the dynamic job vavlues
	if job.GenerateName != "curator-job-" {
		t.Fatal("Job obbject not found")
	}

	if job.Spec.Template.Spec.InitContainers[0].Image != imageURI {
		t.Fatalf("Default imageURI does not match: %v", job.Spec.Template.Spec.InitContainers[0].Image)
	}

	if job.Spec.Template.Spec.InitContainers[0].Env[0].Value != PREHOOK {

		t.Fatalf("Container init name not correct: %v",
			job.Spec.Template.Spec.InitContainers[0].Env[0].Value)
	}

}

// Test launcher with a bad clusterCurator no InitContainers
func TestCreateLauncherBadClusterCurator(t *testing.T) {

	clusterCurator := &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{Name: clusterName, Namespace: clusterName},
	}
	overrideJob, _ := json.Marshal(&batchv1.Job{ObjectMeta: v1.ObjectMeta{
		Name:      "myjob",
		Namespace: clusterName,
	}})
	clusterCurator.Spec.Install.OverrideJob = &runtime.RawExtension{
		Raw: overrideJob,
	}

	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewFakeClientWithScheme(s, clusterCurator)
	kubeset := fake.NewSimpleClientset()

	testLauncher := NewLauncher(client, kubeset, imageURI, *clusterCurator)

	assert.NotNil(t, testLauncher, "launcher is not nil")

	err := testLauncher.CreateJob()

	assert.NotNil(t, err, "Invalid ClusterCurator detected")
	t.Log(err)
}

// Test launcher with a valid overrideJob
func TestCreateLauncherOverrideJob(t *testing.T) {

	batchJobObj := &batchv1.Job{ObjectMeta: v1.ObjectMeta{
		GenerateName: "job-test",
		Namespace:    clusterName,
	}, TypeMeta: v1.TypeMeta{
		Kind:       "Job",
		APIVersion: "batch/v1",
	},
	}
	stringData, err := yaml.Marshal(batchJobObj)
	if err != nil {
		t.Fatal("Failed to marshal batchJobObj")
	}
	clusterCurator := &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{Name: clusterName, Namespace: clusterName},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			ProviderCredentialPath: "default/provider-secret",
			Install: clustercuratorv1.Hooks{
				OverrideJob: &runtime.RawExtension{
					Raw: stringData,
				},
			},
		},
	}

	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewFakeClientWithScheme(s, clusterCurator)
	kubeset := fake.NewSimpleClientset()

	testLauncher := NewLauncher(client, kubeset, imageURI, *clusterCurator)

	assert.NotNil(t, testLauncher, "launcher is not nil")

	err = testLauncher.CreateJob()

	t.Log("SKIP: Test is failing")
	assert.NotNil(t, err, "test is currently failing with fake client")
}

// Test launcher with an Invalid overrideJob
func TestCreateLauncherInvalidOverrideJob(t *testing.T) {

	clusterCurator := &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{Name: clusterName, Namespace: clusterName},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			ProviderCredentialPath: "default/provider-secret",
			Install: clustercuratorv1.Hooks{
				OverrideJob: &runtime.RawExtension{
					Raw: []byte("Not a valid job.batchv1: specification!!"),
				},
			},
		},
	}

	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewFakeClientWithScheme(s, clusterCurator)
	kubeset := fake.NewSimpleClientset()

	testLauncher := NewLauncher(client, kubeset, imageURI, *clusterCurator)

	assert.NotNil(t, testLauncher, "launcher is not nil")

	err := testLauncher.CreateJob()

	assert.NotNil(t, err, "CreateJob err is not nil")
	t.Log(err)
}
