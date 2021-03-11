// Copyright Contributors to the Open Cluster Management project.
package utils

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/types"

	"k8s.io/klog/v2"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

const PauseTwoSeconds = 2 * time.Second
const PauseTenSeconds = PauseTwoSeconds * 5
const PauseFiveSeconds = PauseTenSeconds / 2
const CurrentAnsibleJob = "active-ansible-job"
const CurrentHiveJob = "hive-provisioning-job"
const CurrentCuratorContainer = "curating-with-container"
const DefaultImageURI = "registry.ci.openshift.org/open-cluster-management/cluster-curator-controller:latest"

type PatchStringValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

const LogVerbosity = 2

func InitKlog(logLevel int) {

	klog.InitFlags(nil)
	flag.Set("v", strconv.Itoa(logLevel))
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

func RecordAnsibleJobDyn(dynset dynamic.Interface, clusterName string, containerName string) {
	cmGVR := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}

	patch := []PatchStringValue{{
		Op:    "replace",
		Path:  "/data/" + CurrentAnsibleJob,
		Value: containerName,
	}}

	patchInBytes, _ := json.Marshal(patch)

	_, err := dynset.Resource(cmGVR).Namespace(clusterName).Patch(
		context.TODO(), clusterName, types.JSONPatchType, patchInBytes, v1.PatchOptions{})
	if err != nil {
		klog.Warning(err)
	}
}

func RecordCurrentCuratorContainer(kubeset kubernetes.Interface, clusterName string, containerName string) {
	recordJobContainer(kubeset, clusterName, containerName, CurrentCuratorContainer)
}

func RecordHiveJobContainer(kubeset kubernetes.Interface, clusterName, containerName string) {
	recordJobContainer(kubeset, clusterName, containerName, CurrentHiveJob)
}

func recordJobContainer(kubeset kubernetes.Interface, clusterName string, containerName string, cmKey string) {
	patch := []PatchStringValue{{
		Op:    "replace",
		Path:  "/data/" + cmKey,
		Value: containerName,
	}}

	patchInBytes, _ := json.Marshal(patch)
	_, err := kubeset.CoreV1().ConfigMaps(clusterName).Patch(
		context.TODO(), clusterName, types.JSONPatchType, patchInBytes, v1.PatchOptions{})
	if err != nil {
		klog.Warning(err)
	}

}
