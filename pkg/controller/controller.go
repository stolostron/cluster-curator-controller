// Copyright (c) 2020 Red Hat, Inc.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
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
)

func filterConfigMaps(clusterName string) *amv1.ListOptions {
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
				log.Println("> Investigate Cluster " + mc.Name)
				if cm, err := findJobConfigMap(config, mc); err == nil {
					rbac.ApplyRBAC(config, mc.Name)
					launcher.CreateJob(config, *cm)
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
	jobConfigMaps, err := kubeset.CoreV1().ConfigMaps(mc.Name).List(context.TODO(), *filterConfigMaps(mc.Name))
	utils.CheckError(err)
	for _, cm := range jobConfigMaps.Items {
		log.Println(" Found Configmap " + cm.Name)
		return &cm, nil
	}
	log.Println(" No job ConfigMap found for " + mc.Name + "/" + mc.Name)
	return nil, errors.New("Did not find a ConfigMap")
}

func main() {

	// Build a connection to the ACM Hub OCP
	homePath := os.Getenv("HOME")
	kubeconfig := flag.String("kubeconfig", homePath+"/.kube/config", "")
	flag.Parse()

	var config *rest.Config

	if _, err := os.Stat(homePath + "/.kube/config"); !os.IsNotExist(err) {
		log.Println("Connecting with local kubeconfig")
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	} else {
		log.Println("Connecting using In Cluster Config")
		config, err = rest.InClusterConfig()
	}
	WatchManagedCluster(config)

}
