// Copyright Contributors to the Open Cluster Management project.
package ansible

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	ajv1 "github.com/open-cluster-management/ansiblejob-go-lib/api/v1alpha1"
	"github.com/open-cluster-management/api/client/cluster/clientset/versioned/scheme"
	clustercuratorv1 "github.com/open-cluster-management/cluster-curator-controller/pkg/api/v1alpha1"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/utils"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const EnvJobType = "JOB_TYPE"
const ClusterName = "my-cluster"
const AnsibleJobName = "my-ansiblejob-12345"
const SecretRef = "toweraccess"
const AnsibleJobTemplateName = "Ansible Tower Template to run as a job"

var ansibleJob = getAnsibleJob(PREHOOK, AnsibleJobTemplateName, SecretRef, nil, AnsibleJobName, ClusterName)
var s = scheme.Scheme

func getClusterCurator() *clustercuratorv1.ClusterCurator {
	return &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "install",
			Install: clustercuratorv1.Hooks{
				Prehook: []clustercuratorv1.Hook{
					clustercuratorv1.Hook{
						Name: "Service now App Update",
						ExtraVars: &runtime.RawExtension{
							Raw: []byte(`{"variable1": "1","variable2": "2"}`),
						},
					},
				},
				Posthook: []clustercuratorv1.Hook{
					clustercuratorv1.Hook{
						Name: "Service now App Update",
						ExtraVars: &runtime.RawExtension{
							Raw: []byte(`{"variable1": "3","variable2": "4"}`),
						},
					},
				},
			},
		},
	}
}

func getClusterCuratorEmpty() *clustercuratorv1.ClusterCurator {
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

func buildAnsibleJob(ajs string, k8sJob string) *ajv1.AnsibleJob {
	return &ajv1.AnsibleJob{
		TypeMeta: v1.TypeMeta{
			Kind:       "AnsibleJob",
			APIVersion: "tower.ansible.com/v1alpha1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      AnsibleJobName,
			Namespace: ClusterName,
			Annotations: map[string]string{
				utils.CurrentAnsibleJob: PREHOOK,
			},
		},
		Spec: ajv1.AnsibleJobSpec{
			TowerAuthSecretName: SecretRef,
			JobTemplateName:     AnsibleJobTemplateName,
		},
		Status: ajv1.AnsibleJobStatus{
			AnsibleJobResult: ajv1.AnsibleJobResult{
				Status: ajs,
			},
			K8sJob: ajv1.K8sJob{
				NamespacedName: k8sJob,
			},
			Conditions: []ajv1.Condition{
				ajv1.Condition{
					Reason:  "Failed",
					Message: "The job failed from condition",
				},
			},
		},
	}
}

func TestJobNoEnvVar(t *testing.T) {

	os.Setenv("JOB_TYPE", "")

	t.Log("No JOB_TYPE variable configured")

	assert.NotNil(t, Job(nil, nil), "err not nil, when no os.env JOB_TYPE")
}

func TestJobInvalidDesiredCuration(t *testing.T) {

	cc := getClusterCurator()

	os.Setenv("JOB_TYPE", PREHOOK)

	cc.Spec.DesiredCuration = "INVALID CHOICE"

	assert.NotNil(t, Job(nil, cc), "err not nil, DesiredCuration value is not VALID")
}

func TestJobNoClusterCurator(t *testing.T) {

	// We should never get in this situation, but if it happens then send a panic
	t.Log(PREHOOK)
	os.Setenv(EnvJobType, PREHOOK)
	assert.Panics(t, func() { Job(nil, nil) }, "Panics when no ClusterCurator is present")

	t.Log(POSTHOOK)
	os.Setenv(EnvJobType, POSTHOOK)
	assert.Panics(t, func() { Job(nil, nil) }, "Panics when no ClusterCurator is present")
}

func TestJobNoClusterCuratorData(t *testing.T) {

	// If prehook or posthook is not defined in the ClusterCurator skip
	t.Logf("Test %v", PREHOOK)
	os.Setenv(EnvJobType, PREHOOK)
	assert.Nil(t, Job(nil, getClusterCuratorEmpty()), "err nil, when no Ansible posthooks")

	t.Logf("Test %v", POSTHOOK)
	assert.Nil(t, Job(nil, getClusterCuratorEmpty()), "err nil, when no Ansible prehooks")
}

func TestFindAnsibleTemplateNamefromClusterCurator(t *testing.T) {

	cc := getClusterCurator()

	t.Logf("Test %v", PREHOOK)
	ansibleTemplates, _ := FindAnsibleTemplateNamefromCurator(&cc.Spec.Install, PREHOOK)
	t.Log(ansibleTemplates)
	assert.NotEmpty(t, ansibleTemplates, "Not empty if AnsibleJobs found")

	t.Logf("Test %v", POSTHOOK)
	ansibleTemplates, _ = FindAnsibleTemplateNamefromCurator(&cc.Spec.Install, POSTHOOK)
	t.Log(ansibleTemplates)
	assert.NotEmpty(t, ansibleTemplates, "Not empty if AnsibleJobs found")
}

func TestJob(t *testing.T) {

	cc := getClusterCurator()

	t.Logf("Test %v", PREHOOK)
	os.Setenv(EnvJobType, PREHOOK)

	s.AddKnownTypes(ajv1.SchemeBuilder.GroupVersion, &ajv1.AnsibleJob{})
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewFakeClientWithScheme(s, getClusterCurator())

	// buildAnsibleJob("successful", AnsibleJobTemplateName),
	go func() {
		time.Sleep(utils.PauseFiveSeconds)

		curator := &clustercuratorv1.ClusterCurator{}

		_ = client.Get(
			context.Background(),
			types.NamespacedName{Namespace: ClusterName, Name: ClusterName},
			curator)

		namespaceName := curator.Status.Conditions[0].Type
		t.Logf("clusterCurator job: %v", namespaceName)
		s := strings.Split(namespaceName, "/")
		newJob := buildAnsibleJob("successful", AnsibleJobTemplateName)

		newJob.SetName(s[1])
		newJob.SetNamespace(s[0])

		// This is not the best way to simulate the job being completed.
		assert.Nil(t,
			client.Delete(context.Background(), newJob),
			"err is nil, when ansibleJob resource is deleted")
		assert.Nil(t,
			client.Create(context.Background(), newJob),
			"err is nil, when ansibleJob resource is created")
		t.Logf("AnsibleJob %v marked successful", s[1])
	}()
	err := Job(client, cc)

	assert.Nil(t, err,
		"err nil, when Ansible Job created and monitored with AnsibleJobStatus successful")
}

func TestJobPosthook(t *testing.T) {

	cc := getClusterCurator()

	t.Logf("Test %v", POSTHOOK)
	os.Setenv(EnvJobType, POSTHOOK)

	s.AddKnownTypes(ajv1.SchemeBuilder.GroupVersion, &ajv1.AnsibleJob{})
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewFakeClientWithScheme(s, getClusterCurator())

	// buildAnsibleJob("successful", AnsibleJobTemplateName),
	go func() {
		time.Sleep(utils.PauseFiveSeconds)

		curator := &clustercuratorv1.ClusterCurator{}

		_ = client.Get(
			context.Background(),
			types.NamespacedName{Namespace: ClusterName, Name: ClusterName},
			curator)

		namespaceName := curator.Status.Conditions[0].Type
		t.Logf("clusterCurator job: %v", namespaceName)
		s := strings.Split(namespaceName, "/")
		newJob := buildAnsibleJob("successful", AnsibleJobTemplateName)

		newJob.SetName(s[1])
		newJob.SetNamespace(s[0])

		// This is not the best way to simulate the job being completed.
		assert.Nil(t,
			client.Delete(context.Background(), newJob),
			"err is nil, when ansibleJob resource is deleted")
		assert.Nil(t,
			client.Create(context.Background(), newJob),
			"err is nil, when ansibleJob resource is created")
		t.Logf("AnsibleJob %v marked successful", s[1])
	}()
	err := Job(client, cc)

	assert.Nil(t, err,
		"err nil, when Ansible Job created and monitored with AnsibleJobStatus successful")
}
func TestMonitorAnsibleJobAnsibleJobStatusSuccessfulPreHook(t *testing.T) {

	cc := getClusterCurator()
	aj := buildAnsibleJob("successful", AnsibleJobTemplateName)

	t.Logf("Test %v", PREHOOK)
	os.Setenv(EnvJobType, PREHOOK)

	s.AddKnownTypes(ajv1.SchemeBuilder.GroupVersion, &ajv1.AnsibleJob{})
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewFakeClientWithScheme(s, aj, cc)

	mapAJ, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(&aj)
	unstructAJ := &unstructured.Unstructured{Object: mapAJ}

	assert.Nil(t, MonitorAnsibleJob(
		client,
		unstructAJ,
		cc), "err nil, when successful")
}

func TestMonitorAnsibleJobAnsibleJobStatusError(t *testing.T) {

	cc := getClusterCurator()
	aj := buildAnsibleJob("error", "")

	t.Logf("Test %v", PREHOOK)
	os.Setenv(EnvJobType, PREHOOK)

	s.AddKnownTypes(ajv1.SchemeBuilder.GroupVersion, &ajv1.AnsibleJob{})
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewFakeClientWithScheme(s, aj, cc)

	mapAJ, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(&aj)
	unstructAJ := &unstructured.Unstructured{Object: mapAJ}

	assert.NotNil(t, MonitorAnsibleJob(client, unstructAJ, cc), "err nil, when successful")
}

func TestMonitorAnsibleJobK8sJob(t *testing.T) {

	cc := getClusterCurator()
	aj := buildAnsibleJob("na", ClusterName+"/"+AnsibleJobName)

	t.Logf("Test %v", POSTHOOK)
	os.Setenv(EnvJobType, POSTHOOK)

	s.AddKnownTypes(ajv1.SchemeBuilder.GroupVersion, &ajv1.AnsibleJob{})
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewFakeClientWithScheme(s, aj, cc)

	mapAJ, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(&aj)
	unstructAJ := &unstructured.Unstructured{Object: mapAJ}

	assert.NotNil(t, MonitorAnsibleJob(client, unstructAJ, cc), "err not nil, when condition.reason = Failed")

	// Todo: Come back and figure out why configMap is not returning from dynamic fake.
	curator := &clustercuratorv1.ClusterCurator{}
	assert.Nil(
		t,
		client.Get(context.Background(), types.NamespacedName{Namespace: ClusterName, Name: ClusterName}, curator),
		"err is nil, clusterCurator is retrieved")

	assert.Equal(
		t,
		ClusterName+"/"+AnsibleJobName, curator.Status.Conditions[0].Type,
		"ConfigMap Ansible Job object name correct")
}

func TestRunAnsibleJob(t *testing.T) {

	cc := getClusterCurator()

	t.Logf("Test %v", POSTHOOK)
	os.Setenv(EnvJobType, POSTHOOK)

	s.AddKnownTypes(ajv1.SchemeBuilder.GroupVersion, &ajv1.AnsibleJob{})
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewFakeClientWithScheme(s, cc)

	aJob, err := RunAnsibleJob(client, cc, POSTHOOK, cc.Spec.Install.Posthook[0], "toweraccess")
	assert.Nil(t, err, "err is nil when job is started")
	t.Logf("Fake ansibleJob launched with name: %v", aJob.GetName())
}

/*
func TestMonitorAnsibleRetryForLoop(t *testing.T) {

	cc := getClusterCurator()

	t.Logf("Test %v", POSTHOOK)
	os.Setenv(EnvJobType, POSTHOOK)

	s.AddKnownTypes(ajv1.SchemeBuilder.GroupVersion, &ajv1.AnsibleJob{})

	aj := buildAnsibleJob("na", AnsibleJobName)
	aj.Status.Conditions[0].Reason = "unknown"

	s.AddKnownTypes(ajv1.SchemeBuilder.GroupVersion, &ajv1.AnsibleJob{}, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewFakeClientWithScheme(s, aj, cc)

	mapAJ, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(&aj)
	unstructAJ := &unstructured.Unstructured{Object: mapAJ}

	assert.Panics(t, func() { MonitorAnsibleJob(client, unstructAJ, cc) }, "Panics when For loop times out")
}*/

/*
func TestMonitorAnsibleRetryForLoopJobResourceObjectNil(t *testing.T) {

	s.AddKnownTypes(ajv1.SchemeBuilder.GroupVersion, &ajv1.AnsibleJob{})
	aj := &ajv1.AnsibleJob{
		TypeMeta: v1.TypeMeta{
			Kind:       "AnsibleJob",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      AnsibleJobName,
			Namespace: ClusterName,
		},
		Status: ajv1.AnsibleJobStatus{
			Conditions: []ajv1.Condition{},
		},
	}
	dynclient := dynfake.NewSimpleDynamicClient(s, aj)

	assert.Panics(t, func() { MonitorAnsibleJob(dynclient, ansibleJob, nil) }, "Panics when For loop times out, no condition status")
}*/
