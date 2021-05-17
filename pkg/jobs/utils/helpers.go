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

	"github.com/open-cluster-management/library-go/pkg/config"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/klog/v2"

	ajv1 "github.com/open-cluster-management/ansiblejob-go-lib/api/v1alpha1"
	clustercuratorv1 "github.com/open-cluster-management/cluster-curator-controller/pkg/api/v1beta1"
	managedclusteractionv1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/action/v1beta1"
	managedclusterinfov1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/internal.open-cluster-management.io/v1beta1"
	managedclusterviewv1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/view/v1beta1"
	hivev1 "github.com/openshift/hive/apis/hive/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientv1 "sigs.k8s.io/controller-runtime/pkg/client"
)

const PauseTwoSeconds = 2 * time.Second
const PauseTenSeconds = PauseTwoSeconds * 5
const PauseFiveSeconds = PauseTenSeconds / 2
const PauseSixtySeconds = 60 * time.Second
const CurrentAnsibleJob = "active-ansible-job"
const CurrentHiveJob = "hive-provisioning-job"
const CurrentCuratorContainer = "curating-with-container"
const CurrentCuratorJob = "curatorJob"
const DefaultImageURI = "registry.ci.openshift.org/open-cluster-management/cluster-curator-controller:latest"

const JobHasFinished = "Job_has_finished"
const JobFailed = "Job_failed"

type PatchStringValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

const LogVerbosity = 2

func InitKlog(logLevel int) {

	klog.InitFlags(nil)

	err := flag.Set("v", strconv.Itoa(logLevel))
	CheckError(err)

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

var CCGVR = schema.GroupVersionResource{
	Group:    "cluster.open-cluster-management.io",
	Version:  "v1beta1",
	Resource: "clustercurators"}

func RecordCuratorJob(clusterName, containerName string) error {
	dynset, err := GetDynset(nil)
	CheckError(err)

	return patchDyn(dynset, clusterName, containerName, CurrentCuratorJob)
}

func RecordCuratorJobName(client clientv1.Client, clusterName string, curatorJobName string) error {
	cc, err := GetClusterCurator(client, clusterName)
	if err != nil {
		return err
	}

	cc.Spec.CuratingJob = curatorJobName
	cc.Status = clustercuratorv1.ClusterCuratorStatus{}

	return client.Update(context.Background(), cc)
}

func patchDyn(dynset dynamic.Interface, clusterName string, containerName string, specKey string) error {

	patch := []PatchStringValue{{
		Op:    "replace",
		Path:  "/spec/" + specKey,
		Value: containerName,
	}}

	patchInBytes, _ := json.Marshal(patch)

	_, err := dynset.Resource(CCGVR).Namespace(clusterName).Patch(
		context.TODO(), clusterName, types.JSONPatchType, patchInBytes, v1.PatchOptions{})
	if err != nil {
		return err
	}
	return nil
}

func GetDynset(dynset dynamic.Interface) (dynamic.Interface, error) {

	config, err := config.LoadConfig("", "", "")
	if err != nil {
		return nil, err
	}

	return dynamic.NewForConfig(config)
}

func GetClient() (clientv1.Client, error) {

	config, err := config.LoadConfig("", "", "")
	if err != nil {
		return nil, err
	}

	curatorScheme := runtime.NewScheme()
	CheckError(clustercuratorv1.AddToScheme(curatorScheme))
	CheckError(batchv1.AddToScheme(curatorScheme))
	CheckError(ajv1.AddToScheme(curatorScheme))
	CheckError(hivev1.AddToScheme(curatorScheme))
	CheckError(managedclusteractionv1beta1.AddToScheme(curatorScheme))
	CheckError(managedclusterviewv1beta1.AddToScheme(curatorScheme))
	CheckError(managedclusterinfov1beta1.AddToScheme(curatorScheme))

	return clientv1.New(config, clientv1.Options{Scheme: curatorScheme})
}

func GetKubeset() (kubernetes.Interface, error) {

	config, err := config.LoadConfig("", "", "")
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}

func RecordCurrentStatusCondition(
	client clientv1.Client,
	clusterName string,
	containerName string,
	conditionStatus v1.ConditionStatus,
	message string) error {

	return recordCuratedStatusCondition(
		client,
		clusterName,
		containerName,
		conditionStatus,
		JobHasFinished,
		message)
}

func recordCuratedStatusCondition(
	client clientv1.Client,
	clusterName string,
	containerName string,
	conditionStatus v1.ConditionStatus,
	reason string,
	message string) error {

	curator, err := GetClusterCurator(client, clusterName)
	if err != nil {
		return err
	}

	var newCondition = metav1.Condition{
		Type:    containerName,
		Status:  conditionStatus,
		Reason:  reason,
		Message: message,
	}

	meta.SetStatusCondition(&curator.Status.Conditions, newCondition)

	if err := client.Update(context.TODO(), curator); err != nil {
		return err
	}
	klog.V(4).Infof("newCondition: %v", newCondition)
	return nil
}

func RecordFailedCuratorStatusCondition(
	client clientv1.Client,
	clusterName string,
	containerName string,
	conditionStatus v1.ConditionStatus,
	message string) error {

	return recordCuratedStatusCondition(
		client,
		clusterName,
		containerName,
		conditionStatus,
		JobFailed,
		message)
}

func GetClusterCurator(client clientv1.Client, clusterName string) (*clustercuratorv1.ClusterCurator, error) {

	curator := &clustercuratorv1.ClusterCurator{}

	if err := client.Get(context.TODO(), clientv1.ObjectKey{Namespace: clusterName, Name: clusterName},
		curator); err != nil {
		return nil, err
	}
	klog.V(4).Infof("ClusterCurator: %v", curator)

	return curator, nil
}
