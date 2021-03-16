// Copyright Contributors to the Open Cluster Management project.

package importer

import (
	"context"
	"errors"
	"time"

	managedclusterclient "github.com/open-cluster-management/api/client/cluster/clientset/versioned"
	managedclusterv1 "github.com/open-cluster-management/api/cluster/v1"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/utils"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
)

func MonitorImport(mcset managedclusterclient.Interface, clusterName string) error {

	klog.V(0).Info("=> Monitoring ManagedCluster import of \"" + clusterName +
		"\" using Override Template \"" + clusterName + "\"")
	managedCluster, err := mcset.ClusterV1().ManagedClusters().Get(context.TODO(), clusterName, v1.GetOptions{})
	if err != nil {
		return err
	}

	/* Two levels of status.conditions:
	 * managedClusterAvailable
	 * ManagedClusterJoined
	 *
	 * Order is important. We expect the default for a few tries, then ManagedCluster joined
	 * and finally exit when available
	 */
	for {
		if managedCluster.Status.Conditions != nil {
			for _, condition := range managedCluster.Status.Conditions {
				switch condition.Type {

				case managedclusterv1.ManagedClusterConditionHubDenied:
					return errors.New("ManagedCluster join denied")

				case managedclusterv1.ManagedClusterConditionAvailable:
					klog.V(0).Info("ManagedCluster available")
					return nil

				case managedclusterv1.ManagedClusterConditionJoined:
					klog.V(2).Info("ManagedCluster joined but not avaialble")

				default:
					klog.V(2).Infof("Waiting for ManagedCluster to join %v", condition.Message)
				}
			}
		}
		time.Sleep(utils.PauseTenSeconds)
	}
}

const retryCount = 150

func MonitorMCInfoImport(mcset dynamic.Interface, clusterName string) error {

	var mciGVR = schema.GroupVersionResource{
		Group: "internal.open-cluster-management.io", Version: "v1beta1", Resource: "managedclusterinfos"}
	klog.V(0).Info("=> Monitoring ManagedClusterInfos import of \"" + clusterName +
		"\" using Override Template \"" + clusterName + "\"")

	/* Two levels of status.conditions:
	 * managedClusterAvailable
	 * ManagedClusterJoined
	 *
	 * Order is important. We expect the default for a few tries, then ManagedCluster joined
	 * and finally exit when available
	 */
	for i := 1; i <= retryCount; i++ {
		managedCluster, err := mcset.Resource(mciGVR).Namespace(clusterName).Get(context.TODO(), clusterName, v1.GetOptions{})
		if err != nil {
			return err
		}
		if managedCluster.Object["status"].(map[string]interface{})["conditions"] != nil {
			for _, condition := range managedCluster.Object["status"].(map[string]interface{})["conditions"].([]interface{}) {
				switch condition.(map[string]interface{})["type"] {

				case managedclusterv1.ManagedClusterConditionHubDenied:
					return errors.New("ManagedCluster join denied")

				case managedclusterv1.ManagedClusterConditionAvailable:
					klog.V(2).Info("ManagedCluster available")
					return nil

				case managedclusterv1.ManagedClusterConditionJoined:
					klog.V(2).Infof("ManagedCluster joined but not avaialble (%v/%v)", i, retryCount)

				default:
					klog.V(2).Infof("Waiting for ManagedCluster to join %v (%v/%v)",
						condition.(map[string]interface{})["message"], i, retryCount)
				}
			}
		} else {
			klog.V(2).Infof("Waiting for %v ManagedCluster to report conditions (%v/%v)", clusterName, i, retryCount)
		}
		time.Sleep(utils.PauseTwoSeconds)
	}
	return errors.New("Time out waiting for cluster to import")
}
