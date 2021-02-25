// Copyright Contributors to the Open Cluster Management project.
package create

import (
	"context"
	"encoding/json"

	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/utils"
	hiveclient "github.com/openshift/hive/pkg/client/clientset/versioned"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

func ActivateDeploywithConfig(config *rest.Config, clusterName string) {
	hiveset, err := hiveclient.NewForConfig(config)
	utils.CheckError(err)
	ActivateDeploy(hiveset, clusterName)
}

//  patchStringValue specifies a json patch operation for a string.
type patchStringValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value int32  `json:"value"`
}

func ActivateDeploy(hiveset *hiveclient.Clientset, clusterName string) {
	klog.V(0).Info("* Initiate Provisioning")
	klog.V(2).Info("Looking up cluster " + clusterName)
	cluster, err := hiveset.HiveV1().ClusterDeployments(clusterName).Get(context.TODO(), clusterName, v1.GetOptions{})
	utils.CheckError(err)
	klog.V(2).Info("Found cluster " + cluster.Name + " ✓")
	if *cluster.Spec.InstallAttemptsLimit != 0 {
		klog.Fatal("ClusterDeployment.spec.installAttemptsLimit is not 0")
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

	utils.CheckError(err)
	klog.V(0).Info("Updated ClusterDeployment ✓")
}
