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

	clustercuratorv1 "github.com/open-cluster-management/cluster-curator-controller/pkg/api/v1beta1"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/utils"
	managedclusteractionv1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/action/v1beta1"
	managedclusterinfov1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/internal.open-cluster-management.io/v1beta1"
	managedclusterviewv1beta1 "github.com/open-cluster-management/multicloud-operators-foundation/pkg/apis/view/v1beta1"
	hivev1 "github.com/openshift/hive/apis/hive/v1"
	hiveclient "github.com/openshift/hive/pkg/client/clientset/versioned"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

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
const UpgradeAttempts = 60
const MCVUpgradeLabel = "cluster-curator-upgrade"

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
				utils.CheckError(utils.RecordCurrentStatusCondition(
					client,
					clusterName,
					"hive-provisioning-job",
					v1.ConditionTrue,
					jobName))
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

			utils.CheckError(utils.RecordCurrentStatusCondition(
				client,
				clusterName,
				"hive-provisioning-job",
				v1.ConditionFalse,
				jobName))

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

func UpgradeCluster(client clientv1.Client, clusterName string, curator *clustercuratorv1.ClusterCurator) error {
	klog.V(0).Info("* Initiate Upgrade")
	klog.V(2).Info("Looking up managedclusterinfo " + clusterName)

	desiredUpdate := curator.Spec.Upgrade.DesiredUpdate

	if err := validateUpgradeVersion(client, clusterName, curator); err != nil {
		return err
	}

	klog.V(2).Info("Check if managedclusterview exists " + clusterName)

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
				return getErr
			}
			klog.Warning(err)
			continue
		}
		labels := resultmcview.ObjectMeta.GetLabels()
		if len(labels) == 0 {
			return getErr
		}
		if _, ok := labels[MCVUpgradeLabel]; ok {
			if resultmcview.Status.Result.Raw != nil {
				break
			}
		} else {
			return getErr
		}
	}

	resultClusterVersion := resultmcview.Status.Result

	clusterVersion := map[string]interface{}{}

	if resultClusterVersion.Raw != nil {
		err := json.Unmarshal(resultClusterVersion.Raw, &clusterVersion)
		utils.CheckError(err)
	} else {
		return getErr
	}

	if cvAvailableUpdates, ok := clusterVersion["status"].(map[string]interface{})["availableUpdates"].([]interface{}); ok {
		for _, version := range cvAvailableUpdates {
			if version.(map[string]interface{})["version"] == desiredUpdate {
				clusterVersion["spec"].(map[string]interface{})["desiredUpdate"] = version
				break
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
		return err
	}

	mcaStatus := managedclusteractionv1beta1.ManagedClusterAction{}
	for i := 1; i <= 5; i++ {
		time.Sleep(utils.PauseFiveSeconds)
		if err := client.Get(context.TODO(), types.NamespacedName{
			Namespace: clusterName,
			Name:      clusterName,
		}, &mcaStatus); err != nil {
			if i == 5 {
				return err
			}
			klog.Warning(err)
			continue
		}
		if mcaStatus.Status.Conditions != nil {
			break
		}
	}

	if mcaStatus.Status.Conditions != nil {
		for _, condition := range mcaStatus.Status.Conditions {
			if condition.Status == v1.ConditionFalse && condition.Reason == managedclusteractionv1beta1.ReasonUpdateResourceFailed {
				klog.Warning("ManagedClusterAction failed to update remote clusterversion", mcaStatus.Status.Conditions)
				return errors.New("Remote clusterversion update failed")
			}
			if condition.Status == v1.ConditionTrue && condition.Type == managedclusteractionv1beta1.ConditionActionCompleted {
				klog.V(2).Info("Remote clusterversion updated successfully " + clusterName)
			}
		}
	} else {
		return errors.New("Remote clusterversion update failed")
	}

	if err := client.Delete(context.TODO(), &mcaStatus); err != nil {
		return err
	}

	return nil
}

func MonitorUpgradeStatus(client clientv1.Client, clusterName string, curator *clustercuratorv1.ClusterCurator) error {
	desiredUpdate := curator.Spec.Upgrade.DesiredUpdate
	channel := curator.Spec.Upgrade.Channel
	upstream := curator.Spec.Upgrade.Upstream
	resultmcview := managedclusterviewv1beta1.ManagedClusterView{}

	var timeoutErr error
	isChannelUpstreamUpdate := false
	for i := 0; i < UpgradeAttempts; i++ {

		getErr := errors.New("Failed to get managedclusterview")
		if err := client.Get(context.TODO(), types.NamespacedName{
			Namespace: clusterName,
			Name:      clusterName,
		}, &resultmcview); err != nil {
			return err
		}

		labels := resultmcview.ObjectMeta.GetLabels()
		if len(labels) == 0 {
			return getErr
		}
		if _, ok := labels[MCVUpgradeLabel]; !ok {
			return getErr
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
							i = UpgradeAttempts
							break
						}
					} else if i == (UpgradeAttempts - 1) {
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

	isValidVersion := false
	if desiredUpdate != "" && managedClusterInfo.Status.DistributionInfo.OCP.AvailableUpdates != nil {
		for _, version := range managedClusterInfo.Status.DistributionInfo.OCP.AvailableUpdates {
			if version == desiredUpdate {
				isValidVersion = true
			}
		}
	}
	if desiredUpdate != "" && !isValidVersion {
		return errors.New("Provided version is not valid")
	}

	isValidChannel := false

	if channel != "" && managedClusterInfo.Status.DistributionInfo.OCP.AvailableUpdates != nil {
		if managedClusterInfo.Status.DistributionInfo.OCP.Desired.Channels != nil {
			for _, c := range managedClusterInfo.Status.DistributionInfo.OCP.Desired.Channels {
				if c == channel {
					isValidChannel = true
					break
				}
			}
		}
	}
	if channel != "" && !isValidChannel {
		return errors.New("Provided channel is not valid")
	}

	return nil
}
