// Copyright Contributors to the Open Cluster Management project.
package hive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	hivev1 "github.com/openshift/hive/apis/hive/v1"
	clustercuratorv1 "github.com/stolostron/cluster-curator-controller/pkg/api/v1beta1"
	"github.com/stolostron/cluster-curator-controller/pkg/jobs/utils"
	managedclusteractionv1beta1 "github.com/stolostron/cluster-lifecycle-api/action/v1beta1"
	managedclusterinfov1beta1 "github.com/stolostron/cluster-lifecycle-api/clusterinfo/v1beta1"
	managedclusterviewv1beta1 "github.com/stolostron/cluster-lifecycle-api/view/v1beta1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/blang/semver/v4"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	clientv1 "sigs.k8s.io/controller-runtime/pkg/client"
)

const MCVUpgradeLabel = "cluster-curator-upgrade"
const ForceUpgradeAnnotation = "cluster.open-cluster-management.io/upgrade-allow-not-recommended-versions"
const UpgradeClusterversionBackoffLimit = "cluster.open-cluster-management.io/upgrade-clusterversion-backoff-limit"
const currentNVersion = "4.16.0" // Need to update every ACM release to new N version
const HiveReconcilePauseAnnotation = "hive.openshift.io/reconcile-pause"

var getErr = errors.New("Failed to get remote clusterversion")

func ActivateDeploy(hiveset clientv1.Client, clusterName string) error {
	klog.V(0).Info("* Initiate Provisioning")
	klog.V(2).Info("Looking up cluster " + clusterName)

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		cluster := &hivev1.ClusterDeployment{}
		err := hiveset.Get(context.TODO(), types.NamespacedName{
			Name:      clusterName,
			Namespace: clusterName,
		}, cluster)
		if err != nil {
			return err
		}

		klog.V(2).Info("Found cluster " + cluster.Name + " ✓")
		annotations := cluster.GetAnnotations()
		if annotations[HiveReconcilePauseAnnotation] != "true" {
			log.Println("Handle ClusterDeployment directly")
			return nil
		}

		// Update the pause annotation
		delete(annotations, HiveReconcilePauseAnnotation)
		cluster.SetAnnotations(annotations)
		return hiveset.Update(context.TODO(), cluster)
	})
	return err
}

func MonitorClusterStatus(
	config *rest.Config, clusterName string, jobType string, curator *clustercuratorv1.ClusterCurator) error {
	client, err := utils.GetClient()
	if err = utils.LogError(err); err != nil {
		return err
	}

	return monitorClusterStatus(client, clusterName, jobType, utils.GetMonitorAttempts(jobType, curator))
}

func DestroyClusterDeployment(hiveset clientv1.Client, clusterName string) error {
	klog.V(0).Infof("Deleting Cluster Deployment for %v\n", clusterName)

	cluster := &hivev1.ClusterDeployment{}
	err := hiveset.Get(context.TODO(), types.NamespacedName{
		Name:      clusterName,
		Namespace: clusterName,
	}, cluster)

	if err != nil && k8serrors.IsNotFound(err) {
		klog.Warning("Could not retreive cluster " + clusterName + " may have already been deleted")
		return nil
	} else if err != nil {
		return err
	}

	err = hiveset.Delete(context.TODO(), &hivev1.ClusterDeployment{
		ObjectMeta: v1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
		},
	})

	if err != nil {
		return err
	}

	return nil
}

func monitorClusterStatus(client clientv1.Client, clusterName string, jobType string, monitorAttempts int) error {
	klog.V(0).Info("Waiting up to " + strconv.Itoa(monitorAttempts*5) + "s for Hive Provisioning job")
	jobName := ""
	var cluster *hivev1.ClusterDeployment

	for i := 1; i <= monitorAttempts; i++ {

		// Refresh the clusterDeployment resource
		cluster := &hivev1.ClusterDeployment{}
		err := client.Get(context.TODO(), types.NamespacedName{
			Name:      clusterName,
			Namespace: clusterName,
		}, cluster)

		if err = utils.LogError(err); err != nil {

			// If the cluster deployment is already gone
			if jobType == utils.Destroying && k8serrors.IsNotFound(err) {
				klog.Warning("No cluster deployment for " + clusterName + " was found")
				return nil
			}
			return err
		}

		if jobType == utils.Installing && cluster.Status.WebConsoleURL != "" {
			klog.V(2).Info("Provisioning succeeded ✓")

			if jobName != "" {
				utils.CheckError(utils.RecordCurrentStatusCondition(
					client,
					clusterName,
					clusterName,
					"hive-provisioning-job",
					v1.ConditionTrue,
					jobName))
			}

			return nil

		} else if (cluster.Status.ProvisionRef != nil &&
			cluster.Status.ProvisionRef.Name != "") || jobType == utils.Destroying {

			klog.V(2).Info("Found ClusterDeployment status details ✓")

			if jobType == utils.Destroying {
				jobName = clusterName + "-" + utils.Destroying
			} else {
				jobName = cluster.Status.ProvisionRef.Name + "-provision"
			}

			jobPath := clusterName + "/" + jobName

			klog.V(2).Info("Checking for " + jobType + "ing job " + jobPath)
			newJob := &batchv1.Job{}
			err := client.Get(context.Background(), types.NamespacedName{Namespace: clusterName, Name: jobName}, newJob)

			// If the job is missing, follow the main loop
			if err != nil && k8serrors.IsNotFound(err) {
				klog.Warningf("Could not retrieve job: %v", err)
				time.Sleep(utils.PauseFiveSeconds) // 10s
				continue
			}

			if err = utils.LogError(err); err != nil {
				return err
			}

			klog.V(2).Info("Found job " + jobPath + " ✓ Start monitoring: ")
			elapsedTime := 0

			// Wait while the job is running
			klog.V(0).Info("Wait for the " + jobType + "ing job from Hive to complete")

			utils.CheckError(utils.RecordCurrentStatusCondition(
				client,
				clusterName,
				clusterName,
				"hive-"+jobType+"ing-job",
				v1.ConditionFalse,
				jobName))

			for newJob.Status.Active == 1 {
				if elapsedTime%6 == 0 {
					klog.V(0).Info("Job: " + jobPath + " - " + strconv.Itoa(elapsedTime/6) + "min")
				}
				time.Sleep(utils.PauseTenSeconds) // 10s
				elapsedTime++

				// Reset the job, so we make sure we're getting clean data (not cached)
				newJob = &batchv1.Job{}
				err = client.Get(
					context.Background(),
					types.NamespacedName{Namespace: clusterName, Name: jobName},
					newJob)

				if jobType == utils.Destroying && err != nil && k8serrors.IsNotFound(err) {
					break
				}

				utils.CheckError(err)
			}

			// When Destroying, by this point the job finished
			if jobType == utils.Destroying {
				klog.V(0).Info("Uninstall job complete")
				utils.CheckError(utils.RecordCurrentStatusCondition(
					client,
					clusterName,
					clusterName,
					"hive-"+jobType+"ing-job",
					v1.ConditionTrue,
					jobName))
				return nil
			}

			// If succeeded = 0 then we did not finish
			if newJob.Status.Succeeded == 0 {
				cluster = &hivev1.ClusterDeployment{}
				_ = client.Get(context.TODO(), types.NamespacedName{
					Name:      clusterName,
					Namespace: clusterName,
				}, cluster)

				klog.Warning(cluster.Status.Conditions)
				return errors.New(jobType + "ing job \"" + jobPath + "\" failed")
			}

			klog.V(0).Info("The " + jobType + "ing job from Hive completed ✓")

			// Detect that we've failed
		} else {

			klog.V(0).Infof("Attempt: "+strconv.Itoa(i)+"/%v, pause %v", monitorAttempts, utils.PauseFiveSeconds)
			time.Sleep(utils.PauseFiveSeconds)

			for _, condition := range cluster.Status.Conditions {
				// the cluster provisioning will be treated as failed if the ClusterDeployment has any of
				// the following conditions:
				// 1) ProvisionStoppedCondition is True, it indicates that at least one provision attempt was
				//    made, but there will be no further retries;
				// 2) RequirementsMetCondition is False , it indicates that some pre-provision requirement has not
				//    been met;
				//
				// Check ProvisionStoppedCondition instead of ProvisionFailedCondition because ProvisionFailedCondition
				// is transient. If it's True, hive might still be trying; and if the provisioning subsequently succeed
				// it can be set back to False. Whereas once ProvisionStoppedCondition is True, it means hive has given
				// up completely and won't try anymore.
				if (condition.Status == "True" && condition.Type == hivev1.ProvisionStoppedCondition) ||
					(condition.Type == hivev1.RequirementsMetCondition && condition.Status == "False") {
					klog.Warning(cluster.Status.Conditions)
					return errors.New("Failure detected")
				}
			}
		}
	}
	if cluster != nil {
		klog.Warning(cluster.Status.Conditions)
	}
	return errors.New("Timed out waiting for job")
}

func UpgradeCluster(client clientv1.Client, clusterName string, curator *clustercuratorv1.ClusterCurator) error {
	klog.V(0).Info("* Initiate Upgrade")
	klog.V(2).Info("Looking up managedclusterinfo " + clusterName)

	var retries = 1
	curatorAnnotations := curator.GetAnnotations()

	if curatorAnnotations != nil && curatorAnnotations[UpgradeClusterversionBackoffLimit] != "" {
		backoffLimit, err := strconv.Atoi(curatorAnnotations[UpgradeClusterversionBackoffLimit])
		if err == nil {
			if backoffLimit > 0 && backoffLimit <= 100 {
				retries = backoffLimit
			} else if backoffLimit > 100 {
				retries = 100
			}
		}
	}

	klog.V(0).Info("Retries set to: " + strconv.Itoa(retries))

	desiredUpdate := curator.Spec.Upgrade.DesiredUpdate

	if err := validateUpgradeVersion(client, clusterName, curator); err != nil {
		return err
	}

	klog.V(2).Info("Check if managedclusterview exists " + clusterName)

	successful := false
	for i := 1; i <= retries && !successful; i++ {
		klog.V(2).Info("Update clusterversion attempt " + strconv.Itoa(i))

		mcaStatus, err := retreiveAndUpdateClusterVersion(client, clusterName, curator, desiredUpdate)
		if err != nil {
			return err
		}

		if mcaStatus.Status.Conditions != nil {
			for _, condition := range mcaStatus.Status.Conditions {
				if condition.Status == v1.ConditionFalse && condition.Reason == managedclusteractionv1beta1.ReasonUpdateResourceFailed {
					klog.Warning("ManagedClusterAction failed to update remote clusterversion", mcaStatus.Status.Conditions)
					if i == retries {
						klog.Warning("Max attempts reached updating clusterversion")
						return errors.New("Remote clusterversion update failed")
					} else {
						klog.V(2).Info("Retrying clusterversion update")
					}
				}
				if condition.Status == v1.ConditionTrue && condition.Type == managedclusteractionv1beta1.ConditionActionCompleted {
					klog.V(2).Info("Remote clusterversion updated successfully " + clusterName)
					successful = true
					break
				}
			}
		} else if i == retries {
			return errors.New("Remote clusterversion update failed")
		}

		if err := client.Delete(context.TODO(), &mcaStatus); err != nil {
			return err
		}
	}

	return nil
}

func EUSUpgradeCluster(client clientv1.Client, clusterName string, curator *clustercuratorv1.ClusterCurator, isInterVersion bool) error {
	updateVersion := curator.Spec.Upgrade.IntermediateUpdate
	if !isInterVersion {
		updateVersion = curator.Spec.Upgrade.DesiredUpdate
	}

	if isInterVersion {
		klog.V(0).Info("* Initiate EUS to EUS Intermediate Upgrade to " + updateVersion)
	} else {
		klog.V(0).Info("* Initiate EUS to EUS Final Upgrade to " + updateVersion)
	}

	if err := validateEUSUpgradeVersion(client, clusterName, curator, isInterVersion); err != nil {
		return err
	}

	klog.V(2).Info("Check if managedclusterview exists " + clusterName)

	// For OCP/Kubenetes versions that removed APIs, need to acknowledge this
	// before upgrading or else upgrade will be blocked.
	ocpConfigMCV := &managedclusterviewv1beta1.ManagedClusterView{
		ObjectMeta: v1.ObjectMeta{
			Name:      clusterName + "admack",
			Namespace: clusterName,
			Labels: map[string]string{
				MCVUpgradeLabel: clusterName,
			},
		},
		Spec: managedclusterviewv1beta1.ViewSpec{
			Scope: managedclusterviewv1beta1.ViewScope{
				Kind:      "ConfigMap",
				Name:      "admin-acks",
				Namespace: "openshift-config",
				Version:   "v1",
			},
		},
	}

	ocpConfigView := managedclusterviewv1beta1.ManagedClusterView{}
	if err := client.Get(context.TODO(), types.NamespacedName{
		Name:      clusterName + "admack",
		Namespace: clusterName,
	}, &ocpConfigView); err != nil {
		// check if mcv exists before creating

		klog.V(2).Info("Create managedclusterview " + clusterName + "admack")
		if err := client.Create(context.TODO(), ocpConfigMCV); err != nil {
			return err
		}
	}

	ocpConfigGetErr := errors.New("Failed to get remote admin-ack configmap")
	resultOCPConfigMCV := managedclusterviewv1beta1.ManagedClusterView{}
	if err := waitForMCV(client, clusterName+"admack", clusterName, &resultOCPConfigMCV, ocpConfigGetErr); err != nil {
		return err
	}

	resultConfigMap := resultOCPConfigMCV.Status.Result
	configMap := map[string]interface{}{}

	if resultConfigMap.Raw != nil {
		err := json.Unmarshal(resultConfigMap.Raw, &configMap)
		utils.CheckError(err)
	} else {
		return ocpConfigGetErr
	}

	// Get clusterversion from managed cluster to initiate upgrade
	managedclusterview := &managedclusterviewv1beta1.ManagedClusterView{
		ObjectMeta: v1.ObjectMeta{
			Name:      clusterName,
			Namespace: clusterName,
			Labels: map[string]string{
				MCVUpgradeLabel: clusterName,
			},
		},
		Spec: managedclusterviewv1beta1.ViewSpec{
			Scope: managedclusterviewv1beta1.ViewScope{
				Group:     "config.openshift.io",
				Kind:      "ClusterVersion",
				Name:      "version",
				Namespace: "",
				Version:   "v1",
			},
		},
	}

	mcview := managedclusterviewv1beta1.ManagedClusterView{}
	if err := client.Get(context.TODO(), types.NamespacedName{
		Namespace: clusterName,
		Name:      clusterName,
	}, &mcview); err != nil {

		klog.V(2).Info("Create managedclusterview " + clusterName)
		if err := client.Create(context.TODO(), managedclusterview); err != nil {
			return err
		}
	}

	resultmcview := managedclusterviewv1beta1.ManagedClusterView{}
	if err := waitForMCV(client, clusterName, clusterName, &resultmcview, getErr); err != nil {
		return err
	}

	resultClusterVersion := resultmcview.Status.Result
	clusterVersion := map[string]interface{}{}

	if resultClusterVersion.Raw != nil {
		err := json.Unmarshal(resultClusterVersion.Raw, &clusterVersion)
		utils.CheckError(err)
	} else {
		return getErr
	}

	currentOCPVer := clusterVersion["status"].(map[string]interface{})["desired"].(map[string]interface{})["version"].(string)
	desiredSemVer, err := semver.Make(currentOCPVer)
	if err != nil {
		return err
	}

	ackString := "ack-" + strconv.Itoa(int(desiredSemVer.Major)) + "." + strconv.Itoa(int(desiredSemVer.Minor)) +
		"-kube-1." + strconv.Itoa(int(desiredSemVer.Minor+14)) + "-api-removals-in-" +
		strconv.Itoa(int(desiredSemVer.Major)) + "." + strconv.Itoa(int(desiredSemVer.Minor+1))

	cmData := configMap["data"]
	if cmData != nil {
		// "kube-1.{{ '%02d' | format((ocp_minor_version.split('.')[1] | int) + 14"
		cmData.(map[string]interface{})[ackString] = "true"
	} else {
		configMap["data"] = map[string]interface{}{
			ackString: "true",
		}
	}

	// update managed cluster with acknowledge config to allow upgrade
	var updateConfigMap runtime.RawExtension
	if configMap != nil {
		b, err := json.Marshal(configMap)
		utils.CheckError(err)
		updateConfigMap.Raw = b
	}
	klog.V(2).Info("Create managedclusteraction to update configmap " + clusterName + "admack")
	ocpConfigMCA := &managedclusteractionv1beta1.ManagedClusterAction{
		ObjectMeta: v1.ObjectMeta{
			Name:      clusterName + "admack",
			Namespace: clusterName,
		},
		Spec: managedclusteractionv1beta1.ActionSpec{
			ActionType: managedclusteractionv1beta1.UpdateActionType,
			KubeWork: &managedclusteractionv1beta1.KubeWorkSpec{
				Resource:       "configmaps",
				Name:           "admin-acks",
				Namespace:      "openshift-config",
				ObjectTemplate: updateConfigMap,
			},
		},
	}
	if err := client.Create(context.TODO(), ocpConfigMCA); err != nil {
		return err
	}

	// wait for managedclusteraction results
	ocpConfigMCAStatus := managedclusteractionv1beta1.ManagedClusterAction{}
	for i := 1; i <= 5; i++ {
		time.Sleep(utils.PauseFiveSeconds)
		if err := client.Get(context.TODO(), types.NamespacedName{
			Name:      clusterName + "admack",
			Namespace: clusterName,
		}, &ocpConfigMCAStatus); err != nil {
			if i == 5 {
				return err
			}
			klog.Warning(err)
			continue
		}
		if ocpConfigMCAStatus.Status.Conditions != nil {
			break
		}
	}

	if ocpConfigMCAStatus.Status.Conditions != nil {
		for _, condition := range ocpConfigMCAStatus.Status.Conditions {
			if condition.Status == v1.ConditionFalse && condition.Reason == managedclusteractionv1beta1.ReasonUpdateResourceFailed {
				klog.Warning("ManagedClusterAction failed to update remote clusterversion", ocpConfigMCAStatus.Status.Conditions)
				return errors.New("Remote confimap update failed")
			}
			if condition.Status == v1.ConditionTrue && condition.Type == managedclusteractionv1beta1.ConditionActionCompleted {
				klog.V(2).Info("Remote configmap updated successfully " + clusterName + "admack")
			}
		}
	} else {
		return errors.New("Remote configmap update failed")
	}

	if err := client.Delete(context.TODO(), &ocpConfigMCAStatus); err != nil {
		return err
	}

	// After updating the OCP ack configmap it takes some time for clusterversion to pick up the change
	klog.V(0).Info("Pause 60 seconds for clusterversion to update")
	time.Sleep(utils.PauseSixtySeconds)

	var retries = 1
	curatorAnnotations := curator.GetAnnotations()

	if curatorAnnotations != nil && curatorAnnotations[UpgradeClusterversionBackoffLimit] != "" {
		backoffLimit, err := strconv.Atoi(curatorAnnotations[UpgradeClusterversionBackoffLimit])
		if err == nil {
			if backoffLimit > 0 && backoffLimit <= 100 {
				retries = backoffLimit
			} else if backoffLimit > 100 {
				retries = 100
			}
		}
	}

	klog.V(0).Info("Retries set to: " + strconv.Itoa(retries))

	successful := false
	for i := 1; i <= retries && !successful; i++ {
		klog.V(2).Info("Update clusterversion attempt " + strconv.Itoa(i))

		mcaStatus, err := eusRetreiveAndUpdateClusterVersion(client,
			clusterName, updateVersion, managedclusterview, isInterVersion)
		if err != nil {
			return err
		}

		if mcaStatus.Status.Conditions != nil {
			for _, condition := range mcaStatus.Status.Conditions {
				if condition.Status == v1.ConditionFalse && condition.Reason == managedclusteractionv1beta1.ReasonUpdateResourceFailed {
					klog.Warning("ManagedClusterAction failed to update remote clusterversion", mcaStatus.Status.Conditions)
					if i == retries {
						klog.Warning("Max attempts reached updating clusterversion")
						return errors.New("Remote clusterversion update failed")
					} else {
						klog.V(2).Info("Retrying clusterversion update")
					}
				}
				if condition.Status == v1.ConditionTrue && condition.Type == managedclusteractionv1beta1.ConditionActionCompleted {
					klog.V(2).Info("Remote clusterversion updated successfully " + clusterName)
					successful = true
					break
				}
			}
		} else if i == retries {
			return errors.New("Remote clusterversion update failed")
		}

		if err := client.Delete(context.TODO(), &mcaStatus); err != nil {
			return err
		}
	}

	return nil
}

func MonitorUpgradeStatus(client clientv1.Client, clusterName string, curator *clustercuratorv1.ClusterCurator, isInterUpdate bool) error {
	desiredUpdate := curator.Spec.Upgrade.DesiredUpdate
	if isInterUpdate {
		desiredUpdate = curator.Spec.Upgrade.IntermediateUpdate
	}
	channel := curator.Spec.Upgrade.Channel
	upstream := curator.Spec.Upgrade.Upstream
	resultmcview := managedclusterviewv1beta1.ManagedClusterView{}

	upgradeAttempts := utils.GetRetryTimes(curator.Spec.Upgrade.MonitorTimeout, 120, utils.PauseSixtySeconds)

	var getErr, timeoutErr error
	isChannelUpstreamUpdate := false
	for i := 0; i < upgradeAttempts; i++ {

		if getErr = client.Get(context.TODO(), types.NamespacedName{
			Namespace: clusterName,
			Name:      clusterName,
		}, &resultmcview); getErr != nil {
			// sleep and keep on retrying
			time.Sleep(utils.PauseSixtySeconds)
			continue
		}

		labels := resultmcview.ObjectMeta.GetLabels()
		if len(labels) == 0 {
			return errors.New("Failed to get managedclusterview")
		}
		if _, ok := labels[MCVUpgradeLabel]; !ok {
			return errors.New("Failed to get managedclusterview")
		}
		resultClusterVersion := resultmcview.Status.Result

		clusterVersion := map[string]interface{}{}

		if resultClusterVersion.Raw != nil {
			err := json.Unmarshal(resultClusterVersion.Raw, &clusterVersion)
			utils.CheckError(err)
		}
		if desiredUpdate != "" {
			if cvConditions, ok := clusterVersion["status"].(map[string]interface{})["conditions"].([]interface{}); ok {
				for _, condition := range cvConditions {
					if condition.(map[string]interface{})["type"] == "Available" && condition.(map[string]interface{})["status"] == "True" {
						if strings.Contains(condition.(map[string]interface{})["message"].(string), desiredUpdate) {
							klog.V(2).Info("Upgrade succeeded ✓")
							i = upgradeAttempts
							break
						}
					} else if i == (upgradeAttempts - 1) {
						klog.Warning(cvConditions)
						klog.V(2).Info("Timed out waiting for monitor upgrade job")
						timeoutErr = errors.New("Timed out waiting for monitor upgrade job")
						break
					} else if condition.(map[string]interface{})["type"] == "Progressing" && condition.(map[string]interface{})["status"] == "True" {
						klog.V(2).Info(" Upgrade status " + condition.(map[string]interface{})["message"].(string))
						// update curator status to show upgrade %
						strMessage := "Upgrade status - " + condition.(map[string]interface{})["message"].(string)
						utils.CheckError(utils.RecordCurrentStatusCondition(
							client,
							clusterName,
							clusterName,
							"monitor-upgrade",
							v1.ConditionFalse,
							strMessage))
					}
				}
			}
		}
		if desiredUpdate == "" && channel != "" {
			if clusterVersion["spec"].(map[string]interface{})["channel"] == channel {
				klog.V(2).Info("Updated channel successfully ✓")
				isChannelUpstreamUpdate = true
			}
		}
		if desiredUpdate == "" && upstream != "" {
			if clusterVersion["spec"].(map[string]interface{})["upstream"] == upstream {
				klog.V(2).Info("Updated upstream successfully ✓")
				isChannelUpstreamUpdate = true
			}
		}
		if isChannelUpstreamUpdate {
			break
		}
		time.Sleep(utils.PauseSixtySeconds)
	}

	if err := client.Delete(context.TODO(), &resultmcview); err != nil {
		return err
	}

	if getErr != nil {
		return getErr
	}

	return timeoutErr
}

func validateUpgradeVersion(client clientv1.Client, clusterName string, curator *clustercuratorv1.ClusterCurator) error {

	desiredUpdate := curator.Spec.Upgrade.DesiredUpdate
	channel := curator.Spec.Upgrade.Channel
	upstream := curator.Spec.Upgrade.Upstream

	managedClusterInfo := managedclusterinfov1beta1.ManagedClusterInfo{}
	if err := client.Get(context.TODO(), types.NamespacedName{
		Namespace: clusterName,
		Name:      clusterName,
	}, &managedClusterInfo); err != nil {
		return err
	}

	klog.V(2).Info("kubevendor: ", managedClusterInfo.Status.KubeVendor)

	if managedClusterInfo.Status.KubeVendor != "OpenShift" {
		return errors.New("Can not upgrade non openshift cluster")
	}

	if desiredUpdate == "" && channel == "" && upstream == "" {
		return errors.New("Provide valid upgrade version or channel or upstream")
	}

	curatorAnnotations := curator.GetAnnotations()

	isValidVersion := false

	if curatorAnnotations != nil && curatorAnnotations[ForceUpgradeAnnotation] == "true" {
		klog.V(2).Info("Force upgrade option used, version validation disabled")
		isValidVersion = true
	} else {
		if desiredUpdate != "" && managedClusterInfo.Status.DistributionInfo.OCP.AvailableUpdates != nil {
			for _, version := range managedClusterInfo.Status.DistributionInfo.OCP.AvailableUpdates {
				if version == desiredUpdate {
					isValidVersion = true
				}
			}
		}
	}
	if desiredUpdate != "" && !isValidVersion {
		return errors.New("Provided version is not valid")
	}

	isValidChannel := false

	if channel != "" && managedClusterInfo.Status.DistributionInfo.OCP.Desired.Channels != nil {
		for _, c := range managedClusterInfo.Status.DistributionInfo.OCP.Desired.Channels {
			if c == channel {
				isValidChannel = true
				break
			}
		}
	}
	if channel != "" && !isValidChannel {
		return errors.New("Provided channel is not valid")
	}

	return nil
}

func waitForMCV(client clientv1.Client, clusterName string, clusterNamespace string, mcv *managedclusterviewv1beta1.ManagedClusterView, errMsg error) error {
	for i := 1; i <= 5; i++ {
		time.Sleep(utils.PauseFiveSeconds)
		if err := client.Get(context.TODO(), types.NamespacedName{
			Name:      clusterName,
			Namespace: clusterNamespace,
		}, mcv); err != nil {
			if i == 5 {
				klog.Warning(err)
				return errMsg
			}
			klog.Warning(err)
			continue
		}
		labels := mcv.ObjectMeta.GetLabels()
		if len(labels) == 0 {
			return errMsg
		}
		if _, ok := labels[MCVUpgradeLabel]; ok {
			if mcv.Status.Result.Raw != nil {
				break
			}
		} else {
			return errMsg
		}
	}

	return nil
}

func validateEUSUpgradeVersion(client clientv1.Client, clusterName string, curator *clustercuratorv1.ClusterCurator, isInterVersion bool) error {
	if curator.Spec.Upgrade.DesiredUpdate == "" {
		return errors.New(fmt.Sprintf("DesiredUpdate is required to run EUS to EUS upgrade for Curator %q", curator.Name))
	}
	desiredVersion, err := semver.Make(curator.Spec.Upgrade.DesiredUpdate)
	if err != nil {
		return err
	}

	intermediateVersion, err := semver.Make(curator.Spec.Upgrade.IntermediateUpdate)
	if err != nil {
		return err
	}

	nVersion, err := semver.Make(currentNVersion)
	if err != nil {
		return err
	}

	managedClusterInfo := managedclusterinfov1beta1.ManagedClusterInfo{}
	if err := client.Get(context.TODO(), types.NamespacedName{
		Namespace: clusterName,
		Name:      clusterName,
	}, &managedClusterInfo); err != nil {
		return err
	}

	klog.V(2).Info("kubevendor: ", managedClusterInfo.Status.KubeVendor)

	if managedClusterInfo.Status.KubeVendor != "OpenShift" {
		return errors.New("can not upgrade non openshift cluster")
	}

	currentVersion, err := semver.Make(managedClusterInfo.Status.DistributionInfo.OCP.Version)
	if err != nil {
		return err
	}

	if isInterVersion && (intermediateVersion.Compare(currentVersion) == 0 || intermediateVersion.Compare(currentVersion) == -1) {
		return errors.New(fmt.Sprintf("IntermediateUpdate %v must be greater than current version %v to run EUS to EUS upgrade for Curator %q",
			intermediateVersion, currentVersion, curator.Name))
	}

	// desiredVersion == targeted final EUS version
	if desiredVersion.Compare(intermediateVersion) == 0 || desiredVersion.Compare(intermediateVersion) == -1 {
		return errors.New(fmt.Sprintf("DesiredUpdate %v must be greater than IntermediateUpdate %v to run EUS to EUS upgrade for Curator %q",
			desiredVersion, intermediateVersion, curator.Name))
	}

	if intermediateVersion.Major != currentVersion.Major || desiredVersion.Major != currentVersion.Major {
		return errors.New(fmt.Sprintf("Major version EUS to EUS upgrade in not supported for Curator %q", curator.Name))
	}

	if isInterVersion && (intermediateVersion.Minor != (currentVersion.Minor+1) || desiredVersion.Minor != (currentVersion.Minor+2)) {
		return errors.New(fmt.Sprintf("Minor version EUS to EUS upgrade must be continuous for Curator %q", curator.Name))
	}

	if desiredVersion.Minor > (nVersion.Minor + 1) {
		return errors.New(fmt.Sprintf("Minor version EUS to EUS upgrade cannot go above N+1 for Curator %q", curator.Name))
	}

	return nil
}

func retreiveAndUpdateClusterVersion(
	client clientv1.Client,
	clusterName string,
	curator *clustercuratorv1.ClusterCurator,
	desiredUpdate string) (managedclusteractionv1beta1.ManagedClusterAction, error) {

	mcaStatus := managedclusteractionv1beta1.ManagedClusterAction{}
	managedclusterview := &managedclusterviewv1beta1.ManagedClusterView{
		ObjectMeta: v1.ObjectMeta{
			Name:      clusterName,
			Namespace: clusterName,
			Labels: map[string]string{
				MCVUpgradeLabel: clusterName,
			},
		},
		Spec: managedclusterviewv1beta1.ViewSpec{
			Scope: managedclusterviewv1beta1.ViewScope{
				Group:     "config.openshift.io",
				Kind:      "ClusterVersion",
				Name:      "version",
				Namespace: "",
				Version:   "v1",
			},
		},
	}

	mcview := managedclusterviewv1beta1.ManagedClusterView{}
	if err := client.Get(context.TODO(), types.NamespacedName{
		Namespace: clusterName,
		Name:      clusterName,
	}, &mcview); err != nil {

		klog.V(2).Info("Create managedclusterview " + clusterName)
		if err := client.Create(context.TODO(), managedclusterview); err != nil {
			return mcaStatus, err
		}
	}

	getErr := errors.New("Failed to get remote clusterversion")
	resultmcview := managedclusterviewv1beta1.ManagedClusterView{}
	for i := 1; i <= 5; i++ {
		time.Sleep(utils.PauseFiveSeconds)
		if err := client.Get(context.TODO(), types.NamespacedName{
			Namespace: clusterName,
			Name:      clusterName,
		}, &resultmcview); err != nil {
			if i == 5 {
				klog.Warning(err)
				return mcaStatus, getErr
			}
			klog.Warning(err)
			continue
		}
		labels := resultmcview.ObjectMeta.GetLabels()
		if len(labels) == 0 {
			return mcaStatus, getErr
		}
		if _, ok := labels[MCVUpgradeLabel]; ok {
			if resultmcview.Status.Result.Raw != nil {
				break
			}
		} else {
			return mcaStatus, getErr
		}
	}

	resultClusterVersion := resultmcview.Status.Result

	clusterVersion := map[string]interface{}{}

	if resultClusterVersion.Raw != nil {
		err := json.Unmarshal(resultClusterVersion.Raw, &clusterVersion)
		utils.CheckError(err)
	} else {
		return mcaStatus, getErr
	}

	curatorAnnotations := curator.GetAnnotations()

	if curatorAnnotations != nil && curatorAnnotations[ForceUpgradeAnnotation] == "true" {
		// Usually when the desired version is in availableUpates we use the image hash from there
		// but for non-recommended images we don't have the hash so we have to use the image with
		// a version tag instead
		cvDesiredUpdate := clusterVersion["spec"].(map[string]interface{})["desiredUpdate"]
		if cvDesiredUpdate != nil {
			cvDesiredUpdate.(map[string]interface{})["version"] = desiredUpdate
			cvDesiredUpdate.(map[string]interface{})["force"] = true
			cvDesiredUpdate.(map[string]interface{})["image"] =
				"quay.io/openshift-release-dev/ocp-release:" + desiredUpdate + "-multi"
		} else {
			// For when desiredUpdate does not exist
			clusterVersion["spec"].(map[string]interface{})["desiredUpdate"] = map[string]interface{}{
				"version": desiredUpdate,
				"force":   true,
				"image":   "quay.io/openshift-release-dev/ocp-release:" + desiredUpdate + "-multi",
			}
		}
	} else {
		if cvAvailableUpdates, ok := clusterVersion["status"].(map[string]interface{})["availableUpdates"].([]interface{}); ok {
			for _, version := range cvAvailableUpdates {
				if version.(map[string]interface{})["version"] == desiredUpdate {
					clusterVersion["spec"].(map[string]interface{})["desiredUpdate"] = version
					break
				}
			}
		}
	}

	if curator.Spec.Upgrade.Channel != "" {
		clusterVersion["spec"].(map[string]interface{})["channel"] = curator.Spec.Upgrade.Channel
	}

	if curator.Spec.Upgrade.Upstream != "" {
		clusterVersion["spec"].(map[string]interface{})["upstream"] = curator.Spec.Upgrade.Upstream
	}

	var updateClusterVersion runtime.RawExtension
	if clusterVersion != nil {
		b, err := json.Marshal(clusterVersion)
		utils.CheckError(err)
		updateClusterVersion.Raw = b
	}
	klog.V(2).Info("Create managedclusteraction to update clusterversion " + clusterName)
	managedclusteraction := &managedclusteractionv1beta1.ManagedClusterAction{
		ObjectMeta: v1.ObjectMeta{
			Name:      clusterName,
			Namespace: clusterName,
		},
		Spec: managedclusteractionv1beta1.ActionSpec{
			ActionType: managedclusteractionv1beta1.UpdateActionType,
			KubeWork: &managedclusteractionv1beta1.KubeWorkSpec{
				Resource:       "clusterversions",
				Name:           "version",
				Namespace:      "",
				ObjectTemplate: updateClusterVersion,
			},
		},
	}
	if err := client.Create(context.TODO(), managedclusteraction); err != nil {
		return mcaStatus, err
	}

	for i := 1; i <= 5; i++ {
		time.Sleep(utils.PauseFiveSeconds)
		if err := client.Get(context.TODO(), types.NamespacedName{
			Namespace: clusterName,
			Name:      clusterName,
		}, &mcaStatus); err != nil {
			if i == 5 {
				return mcaStatus, err
			}
			klog.Warning(err)
			continue
		}
		if mcaStatus.Status.Conditions != nil {
			break
		}
	}

	return mcaStatus, nil
}

func eusRetreiveAndUpdateClusterVersion(
	client clientv1.Client,
	clusterName string,
	updateVersion string,
	managedclusterview *managedclusterviewv1beta1.ManagedClusterView,
	isInterVersion bool) (managedclusteractionv1beta1.ManagedClusterAction, error) {

	// Get latest clusterversion or else the object might be stale
	mcview := managedclusterviewv1beta1.ManagedClusterView{}
	mcaStatus := managedclusteractionv1beta1.ManagedClusterAction{}
	resultmcview := managedclusterviewv1beta1.ManagedClusterView{}
	clusterVersion := map[string]interface{}{}

	if err := client.Get(context.TODO(), types.NamespacedName{
		Namespace: clusterName,
		Name:      clusterName,
	}, &mcview); err != nil {

		klog.V(2).Info("Create managedclusterview " + clusterName)
		if err := client.Create(context.TODO(), managedclusterview); err != nil {
			return mcaStatus, err
		}
	}

	if err := waitForMCV(client, clusterName, clusterName, &resultmcview, getErr); err != nil {
		return mcaStatus, err
	}
	resultClusterVersion := resultmcview.Status.Result

	if resultClusterVersion.Raw != nil {
		err := json.Unmarshal(resultClusterVersion.Raw, &clusterVersion)
		utils.CheckError(err)
	} else {
		return mcaStatus, getErr
	}

	cvDesiredUpdate := clusterVersion["spec"].(map[string]interface{})["desiredUpdate"]

	// Always use 'force' option since we cannot get the image hash for EUS to EUS upgrade
	// Using the version image tag to upgrade will be blocked
	if cvDesiredUpdate != nil {
		cvDesiredUpdate.(map[string]interface{})["version"] = updateVersion
		cvDesiredUpdate.(map[string]interface{})["force"] = true
		cvDesiredUpdate.(map[string]interface{})["image"] =
			"quay.io/openshift-release-dev/ocp-release:" + updateVersion + "-multi"
	} else {
		// For when desiredUpdate does not exist
		clusterVersion["spec"].(map[string]interface{})["desiredUpdate"] = map[string]interface{}{
			"version": updateVersion,
			"force":   true,
			"image":   "quay.io/openshift-release-dev/ocp-release:" + updateVersion + "-multi",
		}
	}

	// set channel when upgrading to final EUS version
	if !isInterVersion {
		finalSemVer, err := semver.Make(updateVersion)
		if err != nil {
			return mcaStatus, err
		}
		clusterVersion["spec"].(map[string]interface{})["channel"] = "stable-" + strconv.Itoa(int(finalSemVer.Major)) +
			"." + strconv.Itoa(int(finalSemVer.Minor))
	}

	var updateClusterVersion runtime.RawExtension
	if clusterVersion != nil {
		b, err := json.Marshal(clusterVersion)
		utils.CheckError(err)
		updateClusterVersion.Raw = b
	}
	klog.V(2).Info("Create managedclusteraction to update clusterversion " + clusterName)
	managedclusteraction := &managedclusteractionv1beta1.ManagedClusterAction{
		ObjectMeta: v1.ObjectMeta{
			Name:      clusterName,
			Namespace: clusterName,
		},
		Spec: managedclusteractionv1beta1.ActionSpec{
			ActionType: managedclusteractionv1beta1.UpdateActionType,
			KubeWork: &managedclusteractionv1beta1.KubeWorkSpec{
				Resource:       "clusterversions",
				Name:           "version",
				Namespace:      "",
				ObjectTemplate: updateClusterVersion,
			},
		},
	}
	if err := client.Create(context.TODO(), managedclusteraction); err != nil {
		return mcaStatus, err
	}

	for i := 1; i <= 5; i++ {
		time.Sleep(utils.PauseFiveSeconds)
		if err := client.Get(context.TODO(), types.NamespacedName{
			Namespace: clusterName,
			Name:      clusterName,
		}, &mcaStatus); err != nil {
			if i == 5 {
				return mcaStatus, err
			}
			klog.Warning(err)
			continue
		}
		if mcaStatus.Status.Conditions != nil {
			break
		}
	}

	return mcaStatus, nil
}
