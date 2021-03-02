// Copyright Contributors to the Open Cluster Management project.
package utils

import (
	"context"
	"errors"
	"flag"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/klog/v2"

	hivev1 "github.com/openshift/hive/pkg/apis/hive/v1"
	hiveclient "github.com/openshift/hive/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const PauseTenSeconds = 10 * time.Second
const PauseFiveSeconds = PauseTenSeconds / 2
const CurrentAnsibleJob = "current-ansible-job"

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

// Use to apply overrides for strings
func OverrideStringField(field *string, override string, desc string) {
	if override == "" {
		klog.V(2).Info("Overriding " + desc + " \"" + *field + "\" X (NOT PROVIDED)")
	} else if *field != override {
		*field = override
		klog.V(2).Info("Overriding " + desc + " \"" + *field + "\" ✓")
	} else {
		klog.V(2).Info("Overriding " + desc + " \"" + *field + "\" X (ALREADY EQUAL)")
	}
}

// Use to apply overrides for int64
func OverrideInt64Field(field *int64, override string, desc string) {
	if override == "" {
		klog.V(2).Info("Overriding " + desc + " \"" + strconv.FormatInt(*field, 10) + "\" X (NOT PROVIDED)")
	} else {
		overrideInt, err := strconv.ParseInt(override, 10, 64)
		CheckError(err)
		if *field != overrideInt {
			*field = overrideInt
			klog.V(2).Info("Overriding " + desc + " \"" + strconv.FormatInt(*field, 10) + "\" ✓")
		} else {
			klog.V(2).Info("Overriding " + desc + " \"" + strconv.FormatInt(*field, 10) + "\" X (ALREADY EQUAL)")
		}
	}
}

// Use to apply overrides for int / int32
func OverrideIntField(field *int, override string, desc string) {
	var wideField int64 = int64(*field)
	OverrideInt64Field(&wideField, override, desc)
	*field = int(wideField)
}

func MonitorDeployStatus(config *rest.Config, clusterName string) error {
	hiveset, err := hiveclient.NewForConfig(config)
	if err = LogError(err); err != nil {
		return err
	}
	kubeset, err := kubernetes.NewForConfig(config)
	if err = LogError(err); err != nil {
		return err
	}
	var cluster *hivev1.ClusterDeployment
	klog.V(2).Info("Checking ClusterDeployment status details")
	i := 0
	for i < 30 { // 5min wait
		i++
		// Refresh the clusterDeployment resource
		cluster, err = hiveset.HiveV1().ClusterDeployments(clusterName).Get(context.TODO(), clusterName, v1.GetOptions{})
		if err = LogError(err); err != nil {
			return err
		}
		if len(cluster.Status.Conditions) == 0 && cluster.Status.ProvisionRef != nil && cluster.Status.ProvisionRef.Name != "" {
			klog.V(2).Info("Found ClusterDeployment status details ✓")
			jobName := cluster.Status.ProvisionRef.Name + "-provision"
			jobPath := clusterName + "/" + jobName
			klog.V(2).Info("Checking for provisioning job " + jobPath)
			newJob, err := kubeset.BatchV1().Jobs(clusterName).Get(context.TODO(), jobName, v1.GetOptions{})
			// If the job is missing follow the main loop 5min PauseTen
			if err != nil && strings.Contains(err.Error(), " not found") {
				time.Sleep(PauseTenSeconds) //10s
				continue
			}
			if err = LogError(err); err != nil {
				return err
			}
			klog.V(2).Info("Found job " + jobPath + " ✓ Start monitoring: ")
			elapsedTime := 0
			// Wait while the job is running
			klog.V(0).Info("Wait for the provisioning job in Hive to complete")
			for newJob.Status.Active == 1 {
				if elapsedTime%6 == 0 {
					klog.V(0).Info("Job: " + jobPath + " - " + strconv.Itoa(elapsedTime/6) + "min")
				}
				time.Sleep(PauseTenSeconds) //10s
				elapsedTime++
				newJob, err = kubeset.BatchV1().Jobs(clusterName).Get(context.TODO(), jobName, v1.GetOptions{})
				CheckError(err)
			}
			// If succeeded = 0 then we did not finish
			if newJob.Status.Succeeded == 0 {
				cluster, err = hiveset.HiveV1().ClusterDeployments(clusterName).Get(context.TODO(), clusterName, v1.GetOptions{})
				klog.Warning(cluster.Status.Conditions)
				return errors.New("Provisioning job \"" + jobPath + "\" failed")
			}
			klog.V(0).Info("The provisioning job in Hive completed ✓")
			// Check if we're done
		} else if cluster.Status.WebConsoleURL != "" {
			klog.V(2).Info("Provisioning succeeded ✓")
			break
			// Detect that we've failed
		} else {

			klog.V(0).Info("Attempt: " + strconv.Itoa(i) + "/30, pause 10sec")
			time.Sleep(PauseTenSeconds) //10s

			if len(cluster.Status.Conditions) > 0 &&
				(cluster.Spec.InstallAttemptsLimit == nil || *cluster.Spec.InstallAttemptsLimit != 0) {

				klog.Warning(cluster.Status.Conditions)
				return errors.New("Failure detected")
			} else if i == 19 {
				return errors.New("Timed out waiting for job")
			}
		}
	}
	return nil
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
	recordJobContainer(kubeset, configMap, containerName, "current-hive-job")
}

func recordJobContainer(kubeset kubernetes.Interface, configMap *corev1.ConfigMap, containerName string, cmKey string) {
	configMap.Data[cmKey] = containerName
	_, err := kubeset.CoreV1().ConfigMaps(configMap.Namespace).Update(context.TODO(), configMap, v1.UpdateOptions{})
	if err != nil {
		klog.Warning(err)
	}

}
