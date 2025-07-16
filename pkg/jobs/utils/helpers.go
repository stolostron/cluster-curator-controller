// Copyright Contributors to the Open Cluster Management project.
package utils

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/klog/v2"

	hivev1 "github.com/openshift/hive/apis/hive/v1"
	ajv1 "github.com/stolostron/cluster-curator-controller/pkg/api/v1alpha1"
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
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
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

const StandaloneClusterType = "standalone"
const HypershiftClusterType = "hypershift"

var HCGVR = schema.GroupVersionResource{
	Group:    "hypershift.openshift.io",
	Version:  "v1beta1",
	Resource: "hostedclusters",
}

var NPGVR = schema.GroupVersionResource{
	Group:    "hypershift.openshift.io",
	Version:  "v1beta1",
	Resource: "nodepools",
}

var CCGVR = schema.GroupVersionResource{
	Group:    "cluster.open-cluster-management.io",
	Version:  "v1beta1",
	Resource: "clustercurators",
}

var CDGVR = schema.GroupVersionResource{
	Group:    "hive.openshift.io",
	Version:  "v1",
	Resource: "hive.openshift.io",
}

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

func RecordCuratorJob(clusterName, containerName string) error {
	dynset, err := GetDynset(nil)
	CheckError(err)

	return patchDyn(dynset, clusterName, containerName, CurrentCuratorJob)
}

func RecordCuratorJobName(
	client clientv1.Client,
	clusterName string,
	clusterNamespace string,
	curatorJobName string) error {
	cc, err := GetClusterCurator(client, clusterName, clusterNamespace)
	if err != nil {
		return err
	}

	cc.Spec.CuratingJob = curatorJobName

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

	config, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	return dynamic.NewForConfig(config)
}

func GetClient() (clientv1.Client, error) {

	config, err := LoadConfig()
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

	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}

func RecordCurrentStatusCondition(
	client clientv1.Client,
	clusterName string,
	clusterNamespace string,
	containerName string,
	conditionStatus v1.ConditionStatus,
	message string) error {

	return recordCuratedStatusCondition(
		client,
		clusterName,
		clusterNamespace,
		containerName,
		conditionStatus,
		JobHasFinished,
		message)
}

func RecordAnsibleJobStatusUrlCondition(
	client clientv1.Client,
	clusterName string,
	clusterNamespace string,
	containerName string,
	conditionStatus v1.ConditionStatus,
	url string) error {

	return recordCuratedStatusCondition(
		client,
		clusterName,
		clusterNamespace,
		containerName,
		conditionStatus,
		"ansiblejob_url",
		url)
}

func recordCuratedStatusCondition(
	client clientv1.Client,
	clusterName string,
	clusterNamespace string,
	containerName string,
	conditionStatus v1.ConditionStatus,
	reason string,
	message string) error {

	curator, err := GetClusterCurator(client, clusterName, clusterNamespace)
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
	clusterNamespace string,
	containerName string,
	conditionStatus v1.ConditionStatus,
	message string) error {

	return recordCuratedStatusCondition(
		client,
		clusterName,
		clusterNamespace,
		containerName,
		conditionStatus,
		JobFailed,
		message)
}

func GetClusterCurator(
	client clientv1.Client,
	clusterName string,
	clusterNamespace string) (*clustercuratorv1.ClusterCurator, error) {

	curator := &clustercuratorv1.ClusterCurator{}

	if err := client.Get(context.TODO(), clientv1.ObjectKey{Namespace: clusterNamespace, Name: clusterName},
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
		klog.V(2).Info(fmt.Sprintf("The ClusterCuratorJob of the curator %q is not done, do nothing", curator.Name))
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

func GetClusterType(
	hiveset clientv1.Client,
	dc dynamic.Interface,
	clusterName string,
	clusterNamespace string,
	isUpgrade bool) (string, error) {
	// shortcut for Hypershift cluster
	if clusterName != clusterNamespace {
		return HypershiftClusterType, nil
	}

	// if clusterName and clusterNamespace are equal we need more info
	cluster := &hivev1.ClusterDeployment{}
	err := hiveset.Get(context.TODO(), types.NamespacedName{
		Name:      clusterName,
		Namespace: clusterName,
	}, cluster)
	if err == nil && cluster != nil {
		return StandaloneClusterType, nil
	} else if !k8serrors.IsNotFound(err) {
		return "", err
	}

	hostedCluster, hcErr := dc.Resource(HCGVR).Namespace(clusterNamespace).Get(
		context.TODO(), clusterName, v1.GetOptions{})
	if hcErr == nil && hostedCluster != nil {
		return HypershiftClusterType, nil
	} else if !strings.Contains(hcErr.Error(), "not found") &&
		!strings.Contains(hcErr.Error(), "could not find") &&
		!strings.Contains(hcErr.Error(), "is forbidden") {
		return "", hcErr
	}

	if err != nil && hcErr != nil {
		if isUpgrade {
			// default to standalone type for upgrading imported clusters
			// as we cannot get managedclusters at the cluster scope
			klog.V(0).Info("No ClusterDeployment or HostedCluster found. Since this is upgrade, will treat it as an imported cluster")
			return StandaloneClusterType, nil
		}
		return "", errors.New("Failed to determine the cluster type, cannot find ClusterDeployment or HostedCluster")
	}

	return StandaloneClusterType, hcErr
}

func GetMonitorAttempts(jobType string, curator *clustercuratorv1.ClusterCurator) int {
	monitorAttempts := GetRetryTimes(0, 5, PauseTwoSeconds)
	switch jobType {
	case Installing:
		monitorAttempts = GetRetryTimes(curator.Spec.Install.JobMonitorTimeout, 5, PauseTwoSeconds)
	case Destroying:
		monitorAttempts = GetRetryTimes(curator.Spec.Destroy.JobMonitorTimeout, 5, PauseTwoSeconds)
	}

	return monitorAttempts
}

// Copied from https://github.com/stolostron/library-go which is no longer supported
// but this is a useful func for local testing as we don't need to build the image
// LoadConfig loads the kubeconfig and returns a *rest.Config
// url: The url of the server
// kubeconfig: The path of the kubeconfig, if empty the KUBECONFIG environment variable will be used.
// context: The context to use to search the *rest.Config in the kubeconfig file,
// if empty the current-context will be used.
// Search in the following order:
// provided kubeconfig path, KUBECONFIG environment variable, in the cluster, in the user home directory.
// If the context is not provided and the url is not provided, it returns a *rest.Config the for the current-context.
// If the context is not provided but the url provided, it returns a *rest.Config for the server identified by the url.
// If the context is provided, it returns a *rest.Config for the provided context.
func LoadConfig() (*rest.Config, error) {
	kubeconfig := os.Getenv("DEV_ONLY_KUBECONFIG")
	if kubeconfig != "" {
		return configFromFile(kubeconfig)
	}
	return rest.InClusterConfig()
}

func configFromFile(kubeconfig string) (*rest.Config, error) {
	config, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		return nil, err
	}
	return clientcmd.NewDefaultClientConfig(
		*config,
		&clientcmd.ConfigOverrides{}).ClientConfig()
}
