// Copyright Contributors to the Open Cluster Management project.
package ansible

import (
	"os"
	"testing"

	ajv1 "github.com/open-cluster-management/ansiblejob-go-lib/api/v1alpha1"
	"github.com/open-cluster-management/api/client/cluster/clientset/versioned/scheme"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/utils"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

const EnvJobType = "JOB_TYPE"
const ConfigMapName = "my-configmap"
const ClusterName = "my-cluster"
const AnsibleJobName = "my-ansiblejob-12345"
const SecretRef = "toweraccess"
const AnsibleJobTemplateName = "Ansible Tower Template to run as a job"

var ansibleJob = getAnsibleJob(PREHOOK, AnsibleJobTemplateName, SecretRef, nil, AnsibleJobName, ClusterName)
var s = scheme.Scheme

func getConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: v1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      ConfigMapName,
			Namespace: ClusterName,
			Labels: map[string]string{
				"open-cluster-management": "curator",
			},
		},
		Data: map[string]string{},
	}
}

func buildAnsibleJob(ajs string, k8sJob string) *ajv1.AnsibleJob {
	return &ajv1.AnsibleJob{
		TypeMeta: v1.TypeMeta{
			Kind:       "AnsibleJob",
			APIVersion: "v1alpha1",
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

const jobYaml = "    - name: Service now App Update\n" +
	"      extra_vars:\n" +
	"        variable1: 1\n" +
	"        variable2: 2\n"

func TestJobNoEnvVar(t *testing.T) {

	t.Log("No JOB_TYPE variable configured")
	assert.NotNil(t, Job(nil, nil), "err not nil, when no os.env JOB_TYPE")
}

func TestJobNoConfigMap(t *testing.T) {

	t.Log(PREHOOK)
	os.Setenv(EnvJobType, PREHOOK)
	assert.NotNil(t, Job(nil, nil), "err not nil, when no configMap")

	t.Log(POSTHOOK)
	os.Setenv(EnvJobType, PREHOOK)
	assert.NotNil(t, Job(nil, nil), "err not nil, when no configMap")
}

func TestJobNoConfigMapData(t *testing.T) {

	t.Logf("Test %v", PREHOOK)
	os.Setenv(EnvJobType, PREHOOK)
	assert.NotNil(t, Job(nil, getConfigMap()), "err not nil, when no configMap")

	t.Logf("Test %v", POSTHOOK)
	assert.NotNil(t, Job(nil, getConfigMap()), "err not nil, when no configMap")
}

func TestFindAnsibleTemplateNamefromConfigMap(t *testing.T) {

	cm := getConfigMap()
	cm.Data[PREHOOK] = jobYaml
	cm.Data[POSTHOOK] = jobYaml

	t.Logf("Test %v", PREHOOK)
	ansibleTemplates, _ := FindAnsibleTemplateNamefromConfigMap(cm, PREHOOK)
	t.Log(ansibleTemplates)
	assert.NotEmpty(t, ansibleTemplates, "Not empty if AnsibleJobs found")

	t.Logf("Test %v", POSTHOOK)
	ansibleTemplates, _ = FindAnsibleTemplateNamefromConfigMap(cm, POSTHOOK)
	t.Log(ansibleTemplates)
	assert.NotEmpty(t, ansibleTemplates, "Not empty if AnsibleJobs found")
}

func TestJob(t *testing.T) {

	cm := getConfigMap()
	cm.Data[PREHOOK] = jobYaml
	cm.Data[POSTHOOK] = jobYaml

	t.Logf("Test %v", PREHOOK)
	os.Setenv(EnvJobType, PREHOOK)

	s := scheme.Scheme
	s.AddKnownTypes(ajv1.SchemeBuilder.GroupVersion, &ajv1.AnsibleJob{})
	dynclient := dynfake.NewSimpleDynamicClient(s, buildAnsibleJob("successful", AnsibleJobTemplateName))
	dynclient.Fake.PrependReactor("create", "ansiblejobs", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, buildAnsibleJob("successful", AnsibleJobTemplateName), nil
	})
	err := Job(dynclient, cm)

	assert.Nil(t, err,
		"err nil, when Ansible Job created and monitored with AnsibleJobStatus successful")
}

func TestMonitorAnsibleJobAnsibleJobStatusSuccessful(t *testing.T) {

	cm := getConfigMap()
	cm.Data[PREHOOK] = jobYaml
	cm.Data[POSTHOOK] = jobYaml

	t.Logf("Test %v", PREHOOK)
	os.Setenv(EnvJobType, PREHOOK)

	s.AddKnownTypes(ajv1.SchemeBuilder.GroupVersion, &ajv1.AnsibleJob{})
	dynclient := dynfake.NewSimpleDynamicClient(s, buildAnsibleJob("successful", ""))

	assert.Nil(t, MonitorAnsibleJob(dynclient, ansibleJob, cm), "err nil, when successful")
}

func TestMonitorAnsibleJobAnsibleJobStatusError(t *testing.T) {

	cm := getConfigMap()
	cm.Data[PREHOOK] = jobYaml
	cm.Data[POSTHOOK] = jobYaml

	t.Logf("Test %v", PREHOOK)
	os.Setenv(EnvJobType, PREHOOK)

	s.AddKnownTypes(ajv1.SchemeBuilder.GroupVersion, &ajv1.AnsibleJob{})
	dynclient := dynfake.NewSimpleDynamicClient(s, buildAnsibleJob("error", ""))

	assert.NotNil(t, MonitorAnsibleJob(dynclient, ansibleJob, cm), "err nil, when successful")
}

func TestMonitorAnsibleJobK8sJob(t *testing.T) {

	cm := getConfigMap()
	cm.Data[PREHOOK] = jobYaml
	cm.Data[POSTHOOK] = jobYaml

	t.Logf("Test %v", POSTHOOK)
	os.Setenv(EnvJobType, POSTHOOK)

	s.AddKnownTypes(ajv1.SchemeBuilder.GroupVersion, &ajv1.AnsibleJob{})
	dynclient := dynfake.NewSimpleDynamicClient(s, buildAnsibleJob("na", ClusterName+"/"+AnsibleJobName))

	assert.NotNil(t, MonitorAnsibleJob(dynclient, ansibleJob, cm), "err not nil, when condition.reason = Failed")

	t.Logf("Current %v = %v", utils.CurrentAnsibleJob, cm.Data[utils.CurrentAnsibleJob])

	assert.Equal(t, ClusterName+"/"+AnsibleJobName, cm.Data[utils.CurrentAnsibleJob], "ConfigMap Ansible Job object name correct")
}

/*func TestMonitorAnsibleRetryForLoop(t *testing.T) {

	cm := getConfigMap()
	cm.Data[PREHOOK] = jobYaml
	cm.Data[POSTHOOK] = jobYaml

	t.Logf("Test %v", POSTHOOK)
	os.Setenv(EnvJobType, POSTHOOK)

	s.AddKnownTypes(ajv1.SchemeBuilder.GroupVersion, &ajv1.AnsibleJob{})

	aj := buildAnsibleJob("na", AnsibleJobName)
	aj.Status.Conditions[0].Reason = "unknown"

	dynclient := dynfake.NewSimpleDynamicClient(s, aj)

	assert.Panics(t, func() { MonitorAnsibleJob(dynclient, ansibleJob, cm) }, "Panics when For loop times out")
}

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
