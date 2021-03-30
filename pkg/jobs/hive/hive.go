// Copyright Contributors to the Open Cluster Management project.
package hive

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/utils"
	hivev1 "github.com/openshift/hive/pkg/apis/hive/v1"
	hiveclient "github.com/openshift/hive/pkg/client/clientset/versioned"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	clientv1 "sigs.k8s.io/controller-runtime/pkg/client"
)

//  patchStringValue specifies a json patch operation for a string.
type patchStringValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value int32  `json:"value"`
}

const MonitorAttempts = 6

func ActivateDeploy(hiveset hiveclient.Interface, clusterName string) error {
	klog.V(0).Info("* Initiate Provisioning")
	klog.V(2).Info("Looking up cluster " + clusterName)
	cluster, err := hiveset.HiveV1().ClusterDeployments(clusterName).Get(context.TODO(), clusterName, v1.GetOptions{})
	if err != nil {
		return err
	}

	klog.V(2).Info("Found cluster " + cluster.Name + " ✓")
	if cluster.Spec.InstallAttemptsLimit == nil || *cluster.Spec.InstallAttemptsLimit != 0 {
		return errors.New("ClusterDeployment.spec.installAttemptsLimit is not 0")
	}

	// Update the installAttemptsLimit
	intValue := int32(1)
	patch := []patchStringValue{{
		Op:    "replace",
		Path:  "/spec/installAttemptsLimit",
		Value: intValue,
	}}
	patchInBytes, _ := json.Marshal(patch)
	*cluster.Spec.InstallAttemptsLimit = 1
	_, err = hiveset.HiveV1().ClusterDeployments(clusterName).Patch(
		context.TODO(), clusterName, types.JSONPatchType, patchInBytes, v1.PatchOptions{})
	if err != nil {
		return err
	}
	log.Println("Updated ClusterDeployment ✓")
	return nil
}

func MonitorDeployStatus(config *rest.Config, clusterName string) error {
	hiveset, err := hiveclient.NewForConfig(config)
	if err = utils.LogError(err); err != nil {
		return err
	}
	client, err := utils.GetClient()
	if err = utils.LogError(err); err != nil {
		return err
	}
	return monitorDeployStatus(client, hiveset, clusterName)
}

func monitorDeployStatus(client clientv1.Client, hiveset hiveclient.Interface, clusterName string) error {

	klog.V(0).Info("Waiting up to 30s for Hive Provisioning job")
	jobName := ""

	for i := 1; i <= MonitorAttempts; i++ { // 30s wait

		// Refresh the clusterDeployment resource
		cluster, err := hiveset.HiveV1().ClusterDeployments(clusterName).Get(
			context.TODO(), clusterName, v1.GetOptions{})

		if err = utils.LogError(err); err != nil {
			return err
		}

		if cluster.Status.WebConsoleURL != "" {
			klog.V(2).Info("Provisioning succeeded ✓")

			if jobName != "" {
				utils.CheckError(utils.RecordCurrentStatusCondition(client, clusterName, jobName, v1.ConditionTrue, "Hive provisioning job"))
			}

			break
		} else if cluster.Status.ProvisionRef != nil &&
			cluster.Status.ProvisionRef.Name != "" {

			klog.V(2).Info("Found ClusterDeployment status details ✓")
			jobName = cluster.Status.ProvisionRef.Name + "-provision"
			jobPath := clusterName + "/" + jobName

			klog.V(2).Info("Checking for provisioning job " + jobPath)
			newJob := &batchv1.Job{}
			err := client.Get(context.Background(), types.NamespacedName{Namespace: clusterName, Name: jobName}, newJob)

			// If the job is missing follow the main loop 5min Pause
			if err != nil && strings.Contains(err.Error(), " not found") {
				klog.Warningf("Could not retrieve job: %v", err)
				time.Sleep(utils.PauseTenSeconds) //10s
				continue
			}

			if err = utils.LogError(err); err != nil {
				return err
			}

			klog.V(2).Info("Found job " + jobPath + " ✓ Start monitoring: ")
			elapsedTime := 0

			// Wait while the job is running
			klog.V(0).Info("Wait for the provisioning job in Hive to complete")

			utils.CheckError(utils.RecordCurrentStatusCondition(client, clusterName, jobName, v1.ConditionFalse, "Hive provisioning job"))

			for newJob.Status.Active == 1 {
				if elapsedTime%6 == 0 {
					klog.V(0).Info("Job: " + jobPath + " - " + strconv.Itoa(elapsedTime/6) + "min")
				}
				time.Sleep(utils.PauseTenSeconds) //10s
				elapsedTime++

				// Reset the job, so we make sure we're getting clean data (not cached)
				newJob = &batchv1.Job{}
				utils.CheckError(client.Get(
					context.Background(),
					types.NamespacedName{Namespace: clusterName, Name: jobName},
					newJob))
			}

			// If succeeded = 0 then we did not finish
			if newJob.Status.Succeeded == 0 {
				cluster, err = hiveset.HiveV1().ClusterDeployments(clusterName).Get(context.TODO(), clusterName, v1.GetOptions{})
				klog.Warning(cluster.Status.Conditions)
				return errors.New("Provisioning job \"" + jobPath + "\" failed")
			}

			klog.V(0).Info("The provisioning job in Hive completed ✓")
			// Detect that we've failed
		} else {

			klog.V(0).Infof("Attempt: "+strconv.Itoa(i)+"/%v, pause %v", MonitorAttempts, utils.PauseFiveSeconds)
			time.Sleep(utils.PauseFiveSeconds)

			for _, condition := range cluster.Status.Conditions {
				if condition.Status == "True" && (condition.Type == hivev1.ProvisionFailedCondition ||
					condition.Type == hivev1.ClusterImageSetNotFoundCondition) {
					klog.Warning(cluster.Status.Conditions)
					return errors.New("Failure detected")
				}
			}
			if i == MonitorAttempts {
				klog.Warning(cluster.Status.Conditions)
				return errors.New("Timed out waiting for job")
			}
		}
	}
	return nil
}
