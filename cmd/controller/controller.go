// Copyright Contributors to the Open Cluster Management project.

package main

import (
	"context"
	"errors"
	"os"
	"strconv"
	"time"

	managedclusterclient "github.com/open-cluster-management/api/client/cluster/clientset/versioned"
	mcv1 "github.com/open-cluster-management/api/cluster/v1"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/controller/launcher"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/rbac"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/utils"
	"github.com/open-cluster-management/library-go/pkg/config"
	"github.com/operator-framework/operator-sdk/pkg/leader"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func filterConfigMaps() *metav1.ListOptions {
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set(metav1.LabelSelector{MatchLabels: map[string]string{
			"open-cluster-management": "curator",
		}}.MatchLabels).String(),
	}
	return &listOptions
}

func WatchManagedCluster(config *rest.Config) {
	managedclusterclient, err := managedclusterclient.NewForConfig(config)
	utils.CheckError(err)
	kubeset, err := kubernetes.NewForConfig(config)
	utils.CheckError(err)

	imageUri := os.Getenv("IMAGE_URI")
	if imageUri == "" {
		imageUri = "registry.ci.openshift.org/open-cluster-management/cluster-curator-controller:latest"
		klog.Warning("IMAGE_URI=" + imageUri + ", becauese environment variable was not set")
	}
	watchlist := cache.NewListWatchFromClient(
		managedclusterclient.ClusterV1().RESTClient(),
		"managedclusters",
		v1.NamespaceAll,
		fields.Everything(),
	)
	_, controller := cache.NewInformer(
		watchlist,
		&mcv1.ManagedCluster{},
		0, //Duration
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				mc := obj.(*mcv1.ManagedCluster)
				klog.V(2).Info("> Investigate Cluster " + mc.Name)

				for i := 5; i < 60; i = i * 2 { //40s wait
					if cm, err := findJobConfigMap(kubeset, mc); err == nil {
						if cm.Data["curator-job"] == "" {
							err := rbac.ApplyRBAC(kubeset, mc.Name)
							if err := utils.LogError(err); err != nil {
								break
							}
							jobLaunch := launcher.NewLauncher(kubeset, imageUri, *cm)
							if err := utils.LogError(jobLaunch.CreateJob()); err != nil {
								break
							}
						} else {
							klog.Warning(" Curator job has already run")
						}
						break
					} else {
						// If the managedCluster has status.capacity, it has been imported
						if mc.Status.Capacity != nil {
							break
						}
						klog.V(2).Info("ConfigMap not found in namespace " + mc.Name + ", try again in " + strconv.Itoa(i) + "s")
						time.Sleep(time.Duration(i) * time.Second) //10s
					}
				}
			},
			UpdateFunc: func(oldObj interface{}, newObj interface{}) {
				// Not implemented
			},
			DeleteFunc: func(obj interface{}) {
				// Not implemented
			},
		},
	)

	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(stop)
	for {
		time.Sleep(time.Second)
	}
}

func findJobConfigMap(kubeset *kubernetes.Clientset, mc *mcv1.ManagedCluster) (*v1.ConfigMap, error) {
	// Filtered search in the namespace
	jobConfigMaps, err := kubeset.CoreV1().ConfigMaps(mc.Name).List(context.TODO(), *filterConfigMaps())
	if err != nil {
		return nil, err
	}
	for _, cm := range jobConfigMaps.Items {
		klog.V(2).Info(" Found Configmap " + cm.Name)
		return &cm, nil
	}
	klog.Warning(" No job ConfigMap found for " + mc.Name + "/" + mc.Name)
	return nil, errors.New("Did not find a ConfigMap")
}

func main() {

	utils.InitKlog()

	ctx := context.TODO()
	// Become the leader before proceeding
	err := leader.Become(ctx, "cluster-curator-controller-lock")
	if err != nil {
		klog.Error(err, "Did not obtain leader cluster-curator-controller-lock")
		os.Exit(1)
	}

	config, err := config.LoadConfig("", "", "")
	utils.CheckError(err)

	WatchManagedCluster(config)

}
