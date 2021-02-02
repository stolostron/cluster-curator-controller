// Copyright (c) 2020 Red Hat, Inc.
package main

import (
	"context"
	"errors"
	"flag"
	"io/ioutil"
	"log"
	"os"

	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/ansible"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/utils"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func findAnsibleTemplateNamefromConfigMap(cm *corev1.ConfigMap, jobType string) (string, error) {
	if cm.Data[jobType+"-towertemplatename"] == "" {
		return "", errors.New("Missing prehook-towertemplatename in job ConfigMap " + cm.Name)
	}
	return cm.Data[jobType+"-towertemplatename"], nil
}

func getConfigMap(config *rest.Config, configMapName string, namespace string) *corev1.ConfigMap {
	kubeset, err := kubernetes.NewForConfig(config)
	utils.CheckError(err)
	cm, err := kubeset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), configMapName, v1.GetOptions{})
	utils.CheckError(err)
	return cm
}

/* Command: go run ./pkg/jobs/ansible.go
 */
func main() {
	var kubeconfig *string
	var err error
	var namespace = os.Getenv("CLUSTER_NAME")
	if namespace == "" {
		data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err != nil {
			log.Println("Missing the environment variable CLUSTER_NAME")
		}
		utils.CheckError(err)
		namespace = string(data)
	}

	configMapName := os.Getenv("JOB_CONFIGMAP")
	if configMapName == "" {
		log.Println("No ConfigMap passed, use cluster name")
		configMapName = namespace
	}

	jobType := os.Getenv("JOB_TYPE")
	if jobType != "prehook" && jobType != "posthook" {
		utils.CheckError(errors.New("Missing JOB_TYPE, should prehook or posthook"))
	}

	// Build a connection to the ACM Hub OCP
	homePath := os.Getenv("HOME")
	kubeconfig = flag.String("kubeconfig", homePath+"/.kube/config", "")
	flag.Parse()

	var config *rest.Config

	if _, err = os.Stat(homePath + "/.kube/config"); !os.IsNotExist(err) {
		log.Println("Connecting with local kubeconfig")
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	} else {
		log.Println("Connecting using In Cluster Config")
		config, err = rest.InClusterConfig()
	}
	utils.CheckError(err)

	towerTemplateName, err := findAnsibleTemplateNamefromConfigMap(
		getConfigMap(config, namespace, namespace),
		jobType)
	if err == nil {
		ansible.RunAnsibleJob(config, namespace, jobType, towerTemplateName, "toweraccess", nil)
	} else {
		log.Println(err)
	}

	log.Println("Done!")
}
