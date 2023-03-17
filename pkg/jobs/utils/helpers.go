// Copyright Contributors to the Open Cluster Management project.
package utils

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"github.com/stolostron/library-go/pkg/config"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/klog/v2"

	ajv1 "github.com/open-cluster-management/ansiblejob-go-lib/api/v1alpha1"
	hivev1 "github.com/openshift/hive/apis/hive/v1"
	clustercuratorv1 "github.com/stolostron/cluster-curator-controller/pkg/api/v1beta1"
	managedclusteractionv1beta1 "github.com/stolostron/cluster-lifecycle-api/action/v1beta1"
	managedclusterinfov1beta1 "github.com/stolostron/cluster-lifecycle-api/clusterinfo/v1beta1"
	managedclusterviewv1beta1 "github.com/stolostron/cluster-lifecycle-api/view/v1beta1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
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

const Installing = "provision"
const Destroying = "uninstall"

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

// Path splitter NAMSPACE/RESOURCE_NAME
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
	CheckError(corev1.AddToScheme(curatorScheme))

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

func RecordAnsibleJobStatusUrlCondition(
	client clientv1.Client,
	clusterName string,
	containerName string,
	conditionStatus v1.ConditionStatus,
	url string) error {

	return recordCuratedStatusCondition(
		client,
		clusterName,
		containerName,
		conditionStatus,
		"ansiblejob_url",
		url)
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

func DeleteClusterNamespace(client kubernetes.Interface, clusterName string) error {

	pods, err := client.CoreV1().Pods(clusterName).List(context.Background(), v1.ListOptions{})

	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	for _, pod := range pods.Items {
		if pod.Status.Phase != "" && pod.Status.Phase == "Running" {
			if !strings.Contains(pod.Name, clusterName+"-uninstall") {
				return errors.New("There was a running pod: " + pod.Name + ", in the cluster namespace " + clusterName)
			}
		}
	}

	// Delete the namespace
	return client.CoreV1().Namespaces().Delete(context.Background(), clusterName, v1.DeleteOptions{})
}

// Because Unmarshal for map[string]interface{}, uses map[interface{}]interface{} above the root leaf, runtime client does not support it.
func ConvertMap(m interface{}) interface{} {
	if m == nil {
		klog.Warning("No yaml found")
		return ""
	}

	switch m.(type) {
	case []interface{}:
		var ret interface{}
		ret = []interface{}{}

		for i := 0; i < len(m.([]interface{})); i++ {
			ret = append(ret.([]interface{}), ConvertMap(m.([]interface{})[i]))
		}
		return ret

	case map[interface{}]interface{}:
		ret := map[string]interface{}{}
		for key, value := range m.(map[interface{}]interface{}) {
			klog.V(4).Infof("key: %v", key.(string))
			switch value.(type) {

			case map[interface{}]interface{}:

				ret[key.(string)] = ConvertMap(value.(map[interface{}]interface{}))

			case []interface{}:

				ret[key.(string)] = []interface{}{}

				for i := 0; i < len(value.([]interface{})); i++ {

					ret[key.(string)] = append(ret[key.(string)].([]interface{}), ConvertMap(value.([]interface{})[i].(interface{})))
				}

			default:

				// Drop sensitive keys
				if key.(string) != "username" && key.(string) != "password" {
					ret[key.(string)] = fmt.Sprintf("%v", value)
				}
			}
		}
		return ret
	default:
		return fmt.Sprintf("%v", m)
	}
}

func GetRetryTimes(timeout, defaultTimeout int, interval time.Duration) int {
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	return int(time.Duration(timeout) * time.Minute / interval)
}

func NeedToUpgrade(curator clustercuratorv1.ClusterCurator) (bool, error) {
	jobCondtion := meta.FindStatusCondition(curator.Status.Conditions, "clustercurator-job")
	if jobCondtion == nil {
		// no clustercurator-job conditon, a new curation, run the upgrade
		klog.V(2).Info(fmt.Sprintf("No ClusterCuratorJob for curator %q", curator.Name))
		return true, nil
	}

	if jobCondtion.Status == metav1.ConditionFalse {
		// job is not done, do nothing
		klog.V(2).Info("The ClusterCuratorJob of the curator %q is not done, do nothing", curator.Name)
		return false, nil
	}

	if !strings.Contains(jobCondtion.Message, "upgrade") {
		klog.V(2).Info(fmt.Sprintf("Previous curator %q is not for upgrade, %q)", curator.Name, jobCondtion.Message))
		// last job is not for upgrade, run the upgrade
		return true, nil
	}

	if strings.Contains(jobCondtion.Message, "Failed") {
		klog.V(2).Info(fmt.Sprintf("Previous curator %q is failed, %q", curator.Name, jobCondtion.Message))
		if strings.Contains(jobCondtion.Message, GetCurrentVersionInfo(&curator)) {
			// last job failed and desired version is unchanged, do nothing
			klog.V(2).Info(fmt.Sprintf("last job failed and desired version is unchanged, do not need to upgrade"))
			return false, nil
		}

		// To be compatible with 2.2.0 version
		if strings.Contains(jobCondtion.Message, fmt.Sprintf("Version (%s)", curator.Spec.Upgrade.DesiredUpdate)) {
			klog.V(2).Info(fmt.Sprintf("last job failed and desired version is unchanged, do not need to upgrade"))
			return false, nil
		}

		// last job failed and desired version is changed, upgrade
		return true, nil
	}

	specDesiredUpdate := curator.Spec.Upgrade.DesiredUpdate
	if specDesiredUpdate == "" {
		specDesiredUpdate = "0.0.0"
	}

	desiredVersion, err := semver.Make(specDesiredUpdate)
	if err != nil {
		return false, err
	}

	currentChannel, currentUpstream, currentVersion, err := parseVersionInfo(jobCondtion.Message)
	if err != nil {
		klog.V(2).Info(fmt.Sprintf("Previous curator has a wrong clustercurator-job condition message, %v", err))
		return true, nil
	}

	klog.V(2).Info(fmt.Sprintf("Curator %q channel, current=%v desired=%v", curator.Name, currentChannel, curator.Spec.Upgrade.Channel))
	klog.V(2).Info(fmt.Sprintf("Curator %q upstream, current=%v desired=%v", curator.Name, currentUpstream, curator.Spec.Upgrade.Upstream))
	klog.V(2).Info(fmt.Sprintf("Curator %q version, current=%v desired=%v", curator.Name, currentVersion, desiredVersion))

	if desiredVersion.Compare(currentVersion) == 1 {
		// desired version is greater than current version, need upgrade
		return true, nil
	}

	if curator.Spec.Upgrade.Channel != "" && curator.Spec.Upgrade.Channel != currentChannel {
		return true, nil
	}

	if curator.Spec.Upgrade.Upstream != "" && curator.Spec.Upgrade.Upstream != currentUpstream {
		return true, nil
	}

	// desired version is less than or equal to current version, or channel or upstream is not changed, do nothing
	return false, nil
}

func GetCurrentVersionInfo(curator *clustercuratorv1.ClusterCurator) string {
	return fmt.Sprintf("%s;%s;%s", curator.Spec.Upgrade.DesiredUpdate, curator.Spec.Upgrade.Channel, curator.Spec.Upgrade.Upstream)
}

func parseVersionInfo(msg string) (channel, upstream string, semversion semver.Version, err error) {
	index := strings.Index(msg, "(")
	if index == -1 {
		return channel, upstream, semversion, fmt.Errorf("missing '(' in the message")
	}
	versionInfo := msg[index:]
	lastIndex := strings.Index(versionInfo, ")")
	if lastIndex == -1 {
		return channel, upstream, semversion, fmt.Errorf("missing ')' in the message")
	}

	infos := strings.Split(versionInfo[1:lastIndex], ";")
	if len(infos) != 3 {
		return channel, upstream, semversion, fmt.Errorf("wrong split message")
	}

	version := infos[0]
	channel = infos[1]
	upstream = infos[2]

	// there are only channel or upstream
	if version == "" {
		version = "0.0.0"
	}

	semversion, err = semver.Make(version)
	if err != nil {
		return channel, upstream, semversion, fmt.Errorf("")
	}

	return channel, upstream, semversion, nil
}
