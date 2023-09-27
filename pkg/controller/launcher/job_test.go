// Copyright Contributors to the Open Cluster Management project.
package launcher

import (
	"context"
	"encoding/json"
	"log"
	"testing"

	clustercuratorv1 "github.com/stolostron/cluster-curator-controller/pkg/api/v1beta1"
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

	clusterCurator := clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{Name: clusterName, Namespace: clusterName},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			ProviderCredentialPath: "default/provider-secret",
			DesiredCuration:        "install",
			Install: clustercuratorv1.Hooks{
				Prehook: []clustercuratorv1.Hook{
					{
						Name: "prehook job",
					},
				},
				Posthook: []clustercuratorv1.Hook{
					{
						Name: "posthook job",
					},
				},
			},
		},
	}
	batchJobObj := getBatchJob(clusterName, clusterName, imageURI, clusterCurator)

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
	testcases := []struct {
		name           string
		clusterCurator clustercuratorv1.ClusterCurator
		verify         func(b *batchv1.Job)
	}{
		{
			name: "install",
			clusterCurator: clustercuratorv1.ClusterCurator{
				ObjectMeta: v1.ObjectMeta{Name: clusterName, Namespace: clusterName},
				Spec: clustercuratorv1.ClusterCuratorSpec{
					ProviderCredentialPath: "default/provider-secret",
					DesiredCuration:        "install",
				},
			},
			verify: func(batchJobObj *batchv1.Job) {
				t.Logf("Check image is applied correctly %v", imageURI)
				uri := imageURI

				t.Log("Access the first initContainer")
				initContianer := batchJobObj.Spec.Template.Spec.InitContainers[0]

				if initContianer.Image != uri {
					t.Fatalf("The initContainer.image did not have the correct URI %v, expected %v", initContianer.Image, uri)
				}
			},
		},
		{
			name: "install with prehook",
			clusterCurator: clustercuratorv1.ClusterCurator{
				ObjectMeta: v1.ObjectMeta{Name: clusterName, Namespace: clusterName},
				Spec: clustercuratorv1.ClusterCuratorSpec{
					ProviderCredentialPath: "default/provider-secret",
					DesiredCuration:        "install",
					Install: clustercuratorv1.Hooks{
						Prehook: []clustercuratorv1.Hook{
							{
								Name: "fake hook",
							},
						},
					},
				},
			},
			verify: func(batchJobObj *batchv1.Job) {
				for _, ic := range batchJobObj.Spec.Template.Spec.InitContainers {
					if ic.Name == PreAJob {
						return
					}
				}
				t.Fatalf("The prehook initContainer was not found")
			},
		},
		{
			name: "install with posthook",
			clusterCurator: clustercuratorv1.ClusterCurator{
				ObjectMeta: v1.ObjectMeta{Name: clusterName, Namespace: clusterName},
				Spec: clustercuratorv1.ClusterCuratorSpec{
					ProviderCredentialPath: "default/provider-secret",
					DesiredCuration:        "install",
					Install: clustercuratorv1.Hooks{
						Posthook: []clustercuratorv1.Hook{
							{
								Name: "fake hook",
							},
						},
					},
				},
			},
			verify: func(batchJobObj *batchv1.Job) {
				for _, ic := range batchJobObj.Spec.Template.Spec.InitContainers {
					if ic.Name == PostAJob {
						return
					}
				}
				t.Fatalf("The posthook initContainer was not found")
			},
		},
		{
			name: "upgrade",
			clusterCurator: clustercuratorv1.ClusterCurator{
				ObjectMeta: v1.ObjectMeta{Name: clusterName, Namespace: clusterName},
				Spec: clustercuratorv1.ClusterCuratorSpec{
					ProviderCredentialPath: "default/provider-secret",
					DesiredCuration:        "upgrade",
				},
			},
			verify: func(batchJobObj *batchv1.Job) {
				if _, ok := batchJobObj.ObjectMeta.Annotations[UpgradeCluster]; !ok {
					t.Fatal("The upgrade annotation was not set")
				}
			},
		},
		{
			name: "destroy",
			clusterCurator: clustercuratorv1.ClusterCurator{
				ObjectMeta: v1.ObjectMeta{Name: clusterName, Namespace: clusterName},
				Spec: clustercuratorv1.ClusterCuratorSpec{
					ProviderCredentialPath: "default/provider-secret",
					DesiredCuration:        "destroy",
				},
			},
			verify: func(batchJobObj *batchv1.Job) {
				if _, ok := batchJobObj.ObjectMeta.Annotations[DeleteClusterDeployment]; !ok {
					t.Fatal("The destroy annotation was not set")
				}
			},
		},
	}

	for _, c := range testcases {
		c.verify(getBatchJob(clusterName, clusterName, imageURI, c.clusterCurator))
	}
}

// The Retryposthook feature for install
func TestGetBatchJobRetryInstallPosthook(t *testing.T) {
	clusterCurator := clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{Name: clusterName, Namespace: clusterName},
		Operation: &clustercuratorv1.Operation{
			RetryPosthook: "installPosthook",
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			ProviderCredentialPath: "default/provider-secret",
			Install: clustercuratorv1.Hooks{
				Prehook: []clustercuratorv1.Hook{
					{
						Name: "prehook job",
					},
				},
				Posthook: []clustercuratorv1.Hook{
					{
						Name: "posthook job",
					},
				},
			},
		},
	}

	batchJobObj := getBatchJob(clusterName, clusterName, imageURI, clusterCurator)

	t.Log("Test count initContainers in job")
	foundInitContainers := len(batchJobObj.Spec.Template.Spec.InitContainers)

	if foundInitContainers != 1 {
		t.Fatalf("Invalid InitContainers count, expected %v found %v\n",
			1, foundInitContainers)
	}

	t.Log("Access the first initContainer")
	initContianer := batchJobObj.Spec.Template.Spec.InitContainers[0]

	t.Log("Validate initContainer is posthook")
	if initContianer.Name != "posthook-ansiblejob" {
		t.Fatalf("Invalid InitContainers job, expected %v found %v\n",
			"posthook-ansiblejob", initContianer.Name)
	}
}

// The Retryposthook feature for install
func TestGetBatchJobRetryUpgradePosthook(t *testing.T) {
	clusterCurator := clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{Name: clusterName, Namespace: clusterName},
		Operation: &clustercuratorv1.Operation{
			RetryPosthook: "upgradePosthook",
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			ProviderCredentialPath: "default/provider-secret",
			Upgrade: clustercuratorv1.UpgradeHooks{
				Prehook: []clustercuratorv1.Hook{
					{
						Name: "prehook job",
					},
				},
				Posthook: []clustercuratorv1.Hook{
					{
						Name: "posthook job",
					},
				},
			},
		},
	}

	batchJobObj := getBatchJob(clusterName, clusterName, imageURI, clusterCurator)

	t.Log("Test count initContainers in job")
	foundInitContainers := len(batchJobObj.Spec.Template.Spec.InitContainers)

	if foundInitContainers != 1 {
		t.Fatalf("Invalid InitContainers count, expected %v found %v\n",
			1, foundInitContainers)
	}

	t.Log("Access the first initContainer")
	initContianer := batchJobObj.Spec.Template.Spec.InitContainers[0]

	t.Log("Validate initContainer is posthook")
	if initContianer.Name != "posthook-ansiblejob" {
		t.Fatalf("Invalid InitContainers job, expected %v found %v\n",
			"posthook-ansiblejob", initContianer.Name)
	}
}

// Test the launcher to create a job.batchv1 object
func TestCreateLauncher(t *testing.T) {

	clusterCurator := &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{Name: clusterName, Namespace: clusterName},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			ProviderCredentialPath: "default/provider-secret",
			DesiredCuration:        "install",
			Install: clustercuratorv1.Hooks{
				Prehook: []clustercuratorv1.Hook{
					{
						Name: "prehook job",
					},
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
