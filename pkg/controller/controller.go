// Copyright (c) 2020 Red Hat, Inc.
package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"strconv"
	"time"

	managedclusterclient "github.com/open-cluster-management/api/client/cluster/clientset/versioned"
	mcv1 "github.com/open-cluster-management/api/cluster/v1"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/controller/launcher"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/rbac"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/utils"
	v1 "k8s.io/api/core/v1"
	amv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

func filterConfigMaps() *amv1.ListOptions {
	listOptions := amv1.ListOptions{
		LabelSelector: labels.Set(amv1.LabelSelector{MatchLabels: map[string]string{
			"open-cluster-management": "curator",
		}}.MatchLabels).String(),
	}
	return &listOptions
}

func WatchManagedCluster(config *rest.Config) {
	managedclusterclient, err := managedclusterclient.NewForConfig(config)
	utils.CheckError(err)

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
					if cm, err := findJobConfigMap(config, mc); err == nil {
						if cm.Data["curator-job"] == "" {
							err := rbac.ApplyRBAC(config, mc.Name)
							utils.LogError(err)
							err = launcher.CreateJob(config, *cm)
							utils.LogError(err)
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
						utils.LogError(err)
						time.Sleep(time.Duration(i) * time.Second) //10s
					}
				}
			},
			UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			},
			DeleteFunc: func(obj interface{}) {
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

func findJobConfigMap(config *rest.Config, mc *mcv1.ManagedCluster) (*v1.ConfigMap, error) {
	kubeset, err := kubernetes.NewForConfig(config)
	utils.CheckError(err)
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

	klog.InitFlags(nil)
	flag.Set("v", "2")
	// Build a connection to the ACM Hub OCP
	homePath := os.Getenv("HOME")
	kubeconfig := flag.String("kubeconfig", homePath+"/.kube/config", "")
	flag.Parse()

	var config *rest.Config

	if _, err := os.Stat(homePath + "/.kube/config"); !os.IsNotExist(err) {
		klog.V(2).Info("Connecting with local kubeconfig")
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	} else {
		klog.V(2).Info("Connecting using In Cluster Config")
		config, err = rest.InClusterConfig()
	}
	WatchManagedCluster(config)

}
