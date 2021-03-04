// Copyright Contributors to the Open Cluster Management project.
package utils

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
)

func TestCheckErrorNil(t *testing.T) {

	InitKlog()
	assert.NotPanics(t, func() { CheckError(nil) }, "No panic, when err is not present")
}

func TestCheckErrorNotNil(t *testing.T) {

	assert.Panics(t, func() { CheckError(errors.New("TeST")) }, "Panics when a err is received")
}

func TestLogErrorNil(t *testing.T) {

	assert.Nil(t, LogError(nil), "err nil, when no err message")
}

func TestLogErrorNotNil(t *testing.T) {

	assert.NotNil(t, LogError(errors.New("TeST")), "err nil, when no err message")
}

// TODO, replace all instances of klog.Warning that include an IF, this saves us 2x lines of code
func TestLogWarning(t *testing.T) {

	assert.NotPanics(t, func() { LogWarning(nil) }, "No panic, when logging warnings")
	assert.NotPanics(t, func() { LogWarning(errors.New("TeST")) }, "No panic, when logging warnings")
}

func TestPathSplitterFromEnv(t *testing.T) {

	_, _, err := PathSplitterFromEnv("")
	assert.NotNil(t, err, "err not nil, when empty path")

	_, _, err = PathSplitterFromEnv("value")
	assert.NotNil(t, err, "err not nil, when only one value")

	_, _, err = PathSplitterFromEnv("value/")
	assert.NotNil(t, err, "err not nil, when only one value with split present")

	_, _, err = PathSplitterFromEnv("/value")
	assert.NotNil(t, err, "err not nil, when only one value with split present")

	namespace, secretName, err := PathSplitterFromEnv("ns1/s1")

	assert.Nil(t, err, "err nil, when path is split successfully")
	assert.Equal(t, namespace, "ns1", "namespace should be ns1")
	assert.Equal(t, secretName, "s1", "secret name should be s1")

}

const ClusterName = "my-cluster"
const PREHOOK = "prehook"
const jobName = "my-jobname-12345"

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
			PREHOOK: "    - name: Service now App Update\n" +
				"      extra_vars:\n" +
				"        variable1: \"1\"\n" +
				"        variable2: \"2\"\n",
		},
	}
}

func TestRecordAnsibleJobDyn(t *testing.T) {

	cm := getConfigMap()
	s := scheme.Scheme
	s.AddKnownTypes(corev1.SchemeGroupVersion, &corev1.ConfigMap{})

	dynclient := dynfake.NewSimpleDynamicClient(s, cm)

	assert.NotPanics(t, func() { RecordAnsibleJobDyn(dynclient, cm, jobName) }, "no panics, when update successful")
	assert.Equal(t, jobName, cm.Data[CurrentAnsibleJob])
}

func TestRecordAnsibleJobDynWarning(t *testing.T) {

	cm := getConfigMap()

	dynclient := dynfake.NewSimpleDynamicClient(runtime.NewScheme())

	assert.NotPanics(t, func() { RecordAnsibleJobDyn(dynclient, cm, jobName) }, "no panics, when update successful")
}

func TestRecordAnsibleJob(t *testing.T) {

	cm := getConfigMap()

	kubeset := fake.NewSimpleClientset(cm)

	assert.NotEqual(t, jobName, cm.Data[CurrentAnsibleJob])
	assert.NotPanics(t, func() { RecordAnsibleJob(kubeset, cm, jobName) }, "no panics, when update successful")
	assert.Equal(t, jobName, cm.Data[CurrentAnsibleJob])
}

func TestRecordAnsibleJobWarning(t *testing.T) {

	cm := getConfigMap()

	kubeset := fake.NewSimpleClientset()

	assert.NotPanics(t, func() { RecordAnsibleJob(kubeset, cm, jobName) }, "no panics, when update successful")
}

func TestRecordHiveJobContainer(t *testing.T) {

	cm := getConfigMap()

	kubeset := fake.NewSimpleClientset(cm)

	assert.NotEqual(t, jobName, cm.Data[CurrentHiveJob])
	assert.NotPanics(t, func() { RecordHiveJobContainer(kubeset, cm, jobName) }, "no panics, when update successful")
	assert.Equal(t, jobName, cm.Data[CurrentHiveJob])
}

func TestRecordHiveJobContainerwarning(t *testing.T) {

	cm := getConfigMap()

	kubeset := fake.NewSimpleClientset()

	assert.NotPanics(t, func() { RecordHiveJobContainer(kubeset, cm, jobName) }, "no panics, when update successful")
}
