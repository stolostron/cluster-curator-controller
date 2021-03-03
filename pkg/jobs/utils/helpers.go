// Copyright Contributors to the Open Cluster Management project.
package utils

import (
	"context"
	"errors"
	"flag"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/klog/v2"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

const PauseTenSeconds = 10 * time.Second
const PauseFiveSeconds = PauseTenSeconds / 2
const CurrentAnsibleJob = "current-ansible-job"
const CurrentHiveJob = "current-hive-job"

func InitKlog() {

	klog.InitFlags(nil)
	flag.Parse()

}

// Simple error function
func CheckError(err error) {
	if err != nil {
		klog.Error(err.Error())
		panic(err)
	}
}

func LogError(err error) error {
	if err != nil {
		klog.Warning(err.Error())
		return err
	}
	return nil
}

func LogWarning(err error) {
	if err != nil {
		klog.Warning(err.Error())
	}
}

//Path splitter NAMSPACE/RESOURCE_NAME
func PathSplitterFromEnv(path string) (namespace string, resource string, err error) {
	values := strings.Split(path, "/")
	if len(values) != 2 {
		return "", "", errors.New("Resource name was not provided NAMESPACE/RESOURCE_NAME, found: " + path)
	}
	if values[0] == "" || values[1] == "" {
		return "", "", errors.New("NameSpace was not provided NAMESPACE/RESORUCE_NAME, found: " + path)
	}
	return values[0], values[1], nil
}

func RecordAnsibleJobDyn(dynset dynamic.Interface, configMap *corev1.ConfigMap, containerName string) {
	ansibleJobGVR := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	configMap.Data[CurrentAnsibleJob] = containerName
	unstructConfigMap := &unstructured.Unstructured{}
	unstructConfigMap.Object, _ = runtime.DefaultUnstructuredConverter.ToUnstructured(configMap)
	_, err := dynset.Resource(ansibleJobGVR).Update(context.TODO(), unstructConfigMap, v1.UpdateOptions{})
	if err != nil {
		klog.Warning(err)
	}
}

func RecordAnsibleJob(kubeset kubernetes.Interface, configMap *corev1.ConfigMap, containerName string) {
	recordJobContainer(kubeset, configMap, containerName, CurrentAnsibleJob)
}

func RecordHiveJobContainer(kubeset kubernetes.Interface, configMap *corev1.ConfigMap, containerName string) {
	recordJobContainer(kubeset, configMap, containerName, CurrentHiveJob)
}

func recordJobContainer(kubeset kubernetes.Interface, configMap *corev1.ConfigMap, containerName string, cmKey string) {
	configMap.Data[cmKey] = containerName
	_, err := kubeset.CoreV1().ConfigMaps(configMap.Namespace).Update(context.TODO(), configMap, v1.UpdateOptions{})
	if err != nil {
		klog.Warning(err)
	}

}
