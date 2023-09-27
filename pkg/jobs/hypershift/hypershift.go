// Copyright Contributors to the Open Cluster Management project.
package hypershift

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	clustercuratorv1 "github.com/stolostron/cluster-curator-controller/pkg/api/v1beta1"
	"github.com/stolostron/cluster-curator-controller/pkg/jobs/utils"
	managedclusterinfov1beta1 "github.com/stolostron/cluster-lifecycle-api/clusterinfo/v1beta1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	clientv1 "sigs.k8s.io/controller-runtime/pkg/client"
)

/*
We must use dynamic types here unfortunately because the Hypershift API requires
an older version of sigs.k8s.io/controller-runtime/pkg/client(v0.13.1) which is
not compatible with the version	that the Hive API requires and results in compile issues.
*/
func ActivateDeploy(dc dynamic.Interface, clusterName string, namespace string) error {
	klog.V(0).Info("* Initiate Hypershift Provisioning")

	// Update HostedCluster
	err := removePausedUntil(dc, clusterName, namespace, utils.HCGVR)
	utils.CheckError(err)

	// Update NodePool
	// Need to account for 0 or multiple NodePools
	nodePools, err := dc.Resource(utils.NPGVR).Namespace(namespace).List(context.TODO(), v1.ListOptions{})
	utils.CheckError(err)

	for _, np := range nodePools.Items {
		spec := np.Object["spec"].(map[string]interface{})
		if spec["clusterName"] != nil {
			npClusterName := spec["clusterName"].(string)
			if npClusterName == clusterName {
				npName := np.Object["metadata"].(map[string]interface{})["name"].(string)
				err = removePausedUntil(dc, npName, namespace, utils.NPGVR)
				utils.CheckError(err)
			}
		}
	}

	return nil
}

func removePausedUntil(
	dc dynamic.Interface,
	clusterName string,
	namespace string,
	resourceType schema.GroupVersionResource) error {
	klog.V(2).Infof("Looking up %v %v namespace %v", resourceType.Resource, clusterName, namespace)

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var resource *unstructured.Unstructured
		var err error

		resource, err = dc.Resource(resourceType).Namespace(namespace).Get(context.TODO(), clusterName, v1.GetOptions{})
		if err != nil {
			return err
		}

		metadata := resource.Object["metadata"].(map[string]interface{})
		if resourceType.Resource == "nodepools" {
			clusterName = metadata["name"].(string)
		}
		klog.V(2).Infof("Found %v %v in namespace %v ✓",
			resourceType.Resource, clusterName, metadata["namespace"].(string))

		spec := resource.Object["spec"].(map[string]interface{})
		if spec["pausedUntil"] != nil && spec["pausedUntil"].(string) != "true" {
			klog.V(2).Info("Handle " + resourceType.Resource + " directly")
			return nil
		}

		// Remove the pause spec
		patch := []utils.PatchStringValue{{
			Op:   "remove",
			Path: "/spec/pausedUntil",
		}}

		patchInBytes, err := json.Marshal(patch)
		if err != nil {
			return err
		}

		klog.V(2).Infof("Patching %v %v in namespace %v ✓",
			resourceType.Resource, clusterName, metadata["namespace"].(string))
		_, err = dc.Resource(resourceType).Namespace(namespace).Patch(
			context.TODO(), clusterName, types.JSONPatchType, patchInBytes, v1.PatchOptions{})
		if err != nil {
			return err
		}
		klog.V(2).Info("Updated " + resourceType.Resource + " ✓")
		return nil
	})

	return err
}

func MonitorClusterStatus(
	dc dynamic.Interface,
	client clientv1.Client,
	clusterName string,
	namespace string,
	jobType string,
	monitorAttempts int) error {
	klog.V(0).Info("Waiting up to " + strconv.Itoa(monitorAttempts*5) + "s for Hypershift Provisioning job")
	jobName := ""
	var hostedCluster *unstructured.Unstructured

	for i := 1; i <= monitorAttempts; i++ {

		// Refresh the hostedCluster resource
		hostedCluster, err := dc.Resource(utils.HCGVR).Namespace(namespace).Get(
			context.TODO(), clusterName, v1.GetOptions{})

		// Destroy path
		if err = utils.LogError(err); err != nil {
			// If the hostedCluster is already gone
			if jobType == utils.Destroying && k8serrors.IsNotFound(err) {
				klog.Warning("No hosted cluster for " + clusterName + " was found")
				return nil
			}
			return err
		}

		// Install path
		if jobType == utils.Installing && isHostedReady(hostedCluster, false) {
			klog.V(2).Info("Provisioning succeeded ✓")

			if jobName != "" {
				utils.CheckError(utils.RecordCurrentStatusCondition(
					client,
					clusterName,
					namespace,
					"hypershift-provisioning-job",
					v1.ConditionTrue,
					jobName))
			}

			return nil
		} else if !isHostedReady(hostedCluster, false) || jobType == utils.Destroying {
			klog.V(2).Info("Found HostedCluster status details ✓")

			if jobType == utils.Destroying {
				jobName = clusterName + "-" + utils.Destroying
			} else {
				// No ProvisionRef in HostedCluster, we use infra-id instead
				metadata := hostedCluster.Object["metadata"].(map[string]interface{})
				labels := metadata["labels"].(map[string]interface{})
				jobName = labels["hypershift.openshift.io/auto-created-for-infra"].(string) + "-provision"
			}

			jobPath := namespace + "/" + jobName

			klog.V(2).Info("Found job " + jobPath + " ✓ Start monitoring: ")
			elapsedTime := 0

			// Wait while the job is running
			klog.V(0).Info("Wait for the " + jobType + "ing job from Hypershift to complete")

			utils.CheckError(utils.RecordCurrentStatusCondition(
				client,
				clusterName,
				namespace,
				"hypershift-"+jobType+"ing-job",
				v1.ConditionFalse,
				jobName))

			for !isHostedReady(hostedCluster, false) {
				if elapsedTime%6 == 0 {
					klog.V(0).Info("Job: " + jobPath + " - " + strconv.Itoa(elapsedTime/6) + "min")
				}
				time.Sleep(utils.PauseTenSeconds) // 10s
				elapsedTime++

				// Reset hostedCluster, so we make sure we're getting clean data (not cached)
				hostedCluster, err = dc.Resource(utils.HCGVR).Namespace(namespace).Get(
					context.TODO(), clusterName, v1.GetOptions{})

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
					namespace,
					"hypershift-"+jobType+"ing-job",
					v1.ConditionTrue,
					jobName))
				return nil
			}

			klog.V(0).Info("The " + jobType + "ing job from Hypershift completed ✓")
		} else {
			klog.V(0).Info("The " + jobType + "ing job from Hypershift failed ✓")
			/*
				  Detect that we've failed but there's no Hypershift equivalent of ProvisionStoppedCondition.
					The problem with trying to detect failure for hypershift is that there's no total failure state
					where the operator will give up trying. Users can always fix an issue ie. WebIdentity error to
					allow the provision to continue.
			*/
		}
	}

	if hostedCluster != nil && hostedCluster.Object["status"] != nil {
		klog.Warning(hostedCluster.Object["status"].(map[string]interface{})["conditions"])
	}
	return errors.New("Timed out waiting for job")
}

/*
Unlike Hive where there's a single status, Hypershift has multiple conditions to determine whether
the cluster provision is successful or ongoing. There's no fail state as the operator will keep trying.
Degraded - False provision succeeded, True provision ongoing
ClusterVersionProgressing - False provision succeeded, True provision ongoing
ClusterVersionAvailable - False provision ongoing, True provision succeeded
Available - False provision ongoing, True provision succeeded
Progressing - False provision succeeded, True provision ongoing (Upgrade only)

All these conditions need to by in provision succeeded state for the cluster to be ready
*/
func isHostedReady(hostedCluster *unstructured.Unstructured, isUpgrade bool) bool {
	if hostedCluster.Object["status"] == nil {
		return false
	}
	status := hostedCluster.Object["status"].(map[string]interface{})
	conditions := status["conditions"].([]interface{})
	var degradedCond *v1.Condition
	var clusterVersionProgressingCond *v1.Condition
	var clusterVersionAvailableCond *v1.Condition
	var availableCond *v1.Condition
	var progressingCond *v1.Condition

	for _, condition := range conditions {
		conditionType := condition.(map[string]interface{})["type"].(string)
		conditionStatus := condition.(map[string]interface{})["status"].(string)

		klog.V(4).Info("Type " + condition.(map[string]interface{})["type"].(string) +
			" Status " + condition.(map[string]interface{})["status"].(string))
		switch conditionType {
		case "Degraded":
			degradedCond = &v1.Condition{
				Type:   conditionType,
				Status: parseConditionStatus(conditionStatus),
			}
		case "ClusterVersionAvailable":
			clusterVersionAvailableCond = &v1.Condition{
				Type:    conditionType,
				Status:  parseConditionStatus(conditionStatus),
				Message: condition.(map[string]interface{})["message"].(string),
			}
		case "ClusterVersionProgressing":
			clusterVersionProgressingCond = &v1.Condition{
				Type:    conditionType,
				Status:  parseConditionStatus(conditionStatus),
				Message: condition.(map[string]interface{})["message"].(string),
			}
		case "Available":
			availableCond = &v1.Condition{
				Type:   conditionType,
				Status: parseConditionStatus(conditionStatus),
			}
		case "Progressing":
			progressingCond = &v1.Condition{
				Type:   conditionType,
				Status: parseConditionStatus(conditionStatus),
			}
		}
	}

	if !isUpgrade {
		if degradedCond == nil ||
			clusterVersionAvailableCond == nil ||
			clusterVersionProgressingCond == nil ||
			availableCond == nil {
			return false
		}
	} else {
		if degradedCond == nil ||
			clusterVersionAvailableCond == nil ||
			clusterVersionProgressingCond == nil ||
			availableCond == nil ||
			progressingCond == nil {
			return false
		}
	}

	klog.V(4).Info("Available " + availableCond.Status)
	if availableCond.Status == v1.ConditionFalse {
		return false
	}
	klog.V(4).Info("degraded " + degradedCond.Status)
	if degradedCond.Status == v1.ConditionTrue {
		return false
	}
	klog.V(4).Info("clusterVersionProgressing " + clusterVersionProgressingCond.Status)
	if clusterVersionProgressingCond.Status == v1.ConditionTrue {
		return false
	}
	klog.V(4).Info("clusterVersionAvailable " + clusterVersionAvailableCond.Status)
	if clusterVersionAvailableCond.Status == v1.ConditionFalse {
		return false
	}
	klog.V(4).Info("clusterVersionProgressingMsg " + clusterVersionProgressingCond.Message)
	if !strings.Contains(clusterVersionProgressingCond.Message, "Cluster version is") {
		return false
	}
	klog.V(4).Info("clusterVersionAvailableMsg " + clusterVersionAvailableCond.Message)
	if !strings.Contains(clusterVersionAvailableCond.Message, "Done applying") {
		return false
	}
	if isUpgrade && progressingCond.Status == v1.ConditionTrue {
		klog.V(4).Info("progressing " + progressingCond.Status)
		return false
	}

	return true
}

func parseConditionStatus(conditionStatus string) v1.ConditionStatus {
	if conditionStatus == "Unknown" {
		return v1.ConditionUnknown
	}
	status, err := strconv.ParseBool(conditionStatus)
	if err != nil {
		return v1.ConditionFalse
	}
	if status {
		return v1.ConditionTrue
	} else {
		return v1.ConditionFalse
	}
}

func UpgradeCluster(
	client clientv1.Client,
	dc dynamic.Interface,
	clusterName string,
	curator *clustercuratorv1.ClusterCurator) error {
	klog.V(0).Info("* Initiate Upgrade")
	klog.V(2).Info("Looking up managedclusterinfo " + clusterName)

	desiredUpdate := curator.Spec.Upgrade.DesiredUpdate

	if err := validateUpgradeVersion(client, clusterName, curator, desiredUpdate); err != nil {
		return err
	}

	// Only support upgrading HC AND NPs to same versions for now
	image := "quay.io/openshift-release-dev/ocp-release:" + desiredUpdate + "-multi"

	// Patch HostedCluster with new image
	err := patchUpgradeVersion(dc, clusterName, curator.Namespace, utils.HCGVR, image)
	utils.CheckError(err)

	// Need to account for 0 or multiple NodePools
	nodePools, err := dc.Resource(utils.NPGVR).Namespace(curator.Namespace).List(context.TODO(), v1.ListOptions{})
	utils.CheckError(err)

	for _, np := range nodePools.Items {
		spec := np.Object["spec"].(map[string]interface{})
		npClusterName := spec["clusterName"].(string)
		if npClusterName == clusterName {
			npName := np.Object["metadata"].(map[string]interface{})["name"].(string)
			err = patchUpgradeVersion(dc, npName, curator.Namespace, utils.NPGVR, image)
			utils.CheckError(err)
		}
	}

	return nil
}

func MonitorUpgradeStatus(
	dc dynamic.Interface,
	client clientv1.Client,
	clusterName string,
	curator *clustercuratorv1.ClusterCurator) error {
	upgradeAttempts := utils.GetRetryTimes(curator.Spec.Upgrade.MonitorTimeout, 120, utils.PauseSixtySeconds)
	klog.V(0).Info("Monitoring up to " + strconv.Itoa(upgradeAttempts) + " attempts for Hypershift Upgrade job")
	elapsedTime := 0

	// Refresh the hostedCluster resource
	hostedCluster, err := dc.Resource(utils.HCGVR).Namespace(curator.Namespace).Get(
		context.TODO(), clusterName, v1.GetOptions{})
	if err != nil {
		return err
	}

	for i := 0; i < upgradeAttempts; i++ {
		if isHostedReady(hostedCluster, true) {
			klog.V(2).Info("Upgrade succeeded ✓")
			utils.CheckError(utils.RecordCurrentStatusCondition(
				client,
				clusterName,
				curator.Namespace,
				"hypershift-upgrade-job",
				v1.ConditionTrue,
				"upgrade-job"))

			return nil
		} else {
			for !isHostedReady(hostedCluster, true) {
				if elapsedTime%6 == 0 {
					klog.V(0).Info("Upgrade Job:  - " + strconv.Itoa(elapsedTime/6) + "min")
				}
				time.Sleep(utils.PauseTenSeconds) // 10s
				elapsedTime++

				// Reset hostedCluster, so we make sure we're getting clean data (not cached)
				hostedCluster, err = dc.Resource(utils.HCGVR).Namespace(curator.Namespace).Get(
					context.TODO(), clusterName, v1.GetOptions{})

				if err != nil && k8serrors.IsNotFound(err) {
					break
				}

				utils.CheckError(err)
			}
		}
	}

	if hostedCluster != nil {
		klog.Warning(hostedCluster.Object["status"].(map[string]interface{})["conditions"])
	}
	return errors.New("Timed out waiting for job")
}

func patchUpgradeVersion(
	dc dynamic.Interface,
	clusterName string,
	namespace string,
	resourceType schema.GroupVersionResource,
	image string) error {
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		// Update the image spec
		patch := []utils.PatchStringValue{{
			Op:    "replace",
			Path:  "/spec/release/image",
			Value: image,
		}}

		patchInBytes, _ := json.Marshal(patch)

		klog.V(2).Infof("Patching %v %v in namespace %v ✓", resourceType.Resource, clusterName, namespace)
		_, err := dc.Resource(resourceType).Namespace(namespace).Patch(
			context.TODO(), clusterName, types.JSONPatchType, patchInBytes, v1.PatchOptions{})
		if err != nil {
			return err
		}
		log.Println("Updated " + resourceType.Resource + " ✓")
		return nil
	})

	return err
}

func validateUpgradeVersion(
	client clientv1.Client,
	clusterName string,
	curator *clustercuratorv1.ClusterCurator,
	desiredUpdate string) error {
	managedClusterInfo := managedclusterinfov1beta1.ManagedClusterInfo{}
	if err := client.Get(context.TODO(), types.NamespacedName{
		Namespace: clusterName,
		Name:      clusterName,
	}, &managedClusterInfo); err != nil {
		return err
	}

	desiredSemver, err := semver.Make(desiredUpdate)
	if err != nil {
		return err
	}
	currentSemver, err := semver.Make(managedClusterInfo.Status.DistributionInfo.OCP.Version)
	if err != nil {
		return err
	}
	if desiredSemver.Equals(currentSemver) {
		return errors.New("Cannot upgrade to the same version")
	}

	return nil
}

func DestroyHostedCluster(dc dynamic.Interface, clusterName string, namespace string) error {
	klog.V(0).Infof("Deleting Hosted Cluster for %v in namespace %v\n", clusterName, namespace)

	_, err := dc.Resource(utils.HCGVR).Namespace(namespace).Get(context.TODO(), clusterName, v1.GetOptions{})

	if err != nil {
		if k8serrors.IsNotFound(err) {
			klog.Warning("Could not retreive hosted cluster " + clusterName + " may have already been deleted")
			return nil
		}
		return err
	}

	err = dc.Resource(utils.HCGVR).Namespace(namespace).Delete(context.TODO(), clusterName, v1.DeleteOptions{})
	if err != nil {
		return err
	}

	return nil
}

func DetachAndMonitor(dc dynamic.Interface, clusterName string, curator *clustercuratorv1.ClusterCurator) error {
	var mcGVR = schema.GroupVersionResource{
		Group:    "cluster.open-cluster-management.io",
		Version:  "v1",
		Resource: "managedclusters",
	}
	klog.V(0).Info("=> Monitoring ManagedCluster detach of " + clusterName)

	hostedCluster, err := dc.Resource(utils.HCGVR).Namespace(curator.Namespace).Get(
		context.TODO(), clusterName, v1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			klog.Warning("Could not retreive hosted cluster " + clusterName + " may have been deleted")
		} else {
			return err
		}
	}

	spec := hostedCluster.Object["spec"].(map[string]interface{})
	if spec["platform"] == nil {
		return errors.New("Not able to find HostedCluster platform type")
	}
	hcType := spec["platform"].(map[string]interface{})["type"].(string)

	if hcType != "KubeVirt" && hcType != "Agent" {
		return errors.New("Destroying HosterCluster type " + hcType + " is not supported. Use the HostedCluster CLI.")
	}

	retryCount := utils.GetRetryTimes(curator.Spec.Destroy.JobMonitorTimeout, 5, utils.PauseTwoSeconds)
	_, err = dc.Resource(mcGVR).Get(context.TODO(), clusterName, v1.GetOptions{})

	if err != nil && k8serrors.IsNotFound(err) {
		klog.Warning("Could not retreive managed cluster " + clusterName + " may have been deleted")
		return nil
	} else if err != nil {
		return err
	}

	klog.V(0).Info("Deleting ManagedCluster " + clusterName)
	// Delete will hang until resource is delete, no need to monitor
	err = dc.Resource(mcGVR).Delete(context.TODO(), clusterName, v1.DeleteOptions{})
	if err != nil {
		return err
	}

	// Monitor managed cluster delete
	for i := 1; i <= retryCount; i++ {
		_, err := dc.Resource(mcGVR).Get(context.TODO(), clusterName, v1.GetOptions{})

		if err != nil && k8serrors.IsNotFound(err) {
			klog.Warning("Could not retreive managed cluster " + clusterName + " may have been deleted")
			return nil
		} else if err != nil {
			return err
		}

		time.Sleep(utils.PauseTenSeconds * 3)
	}
	return errors.New("Time out waiting for hosted cluster to detach")
}
