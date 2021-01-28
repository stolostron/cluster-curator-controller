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
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

/*Path splitter NAMSPACE/RESOURCE_NAME
func pathSplitterFromEnv(envVar string) (namespace string, resource string, err error) {
	var path = os.Getenv(envVar)
	if path == "" {
		return "", "", errors.New("Environment variable " + envVar + " missing, format: NAMESPACE/RESOURCE_NAME")
	}
	values := strings.Split(path, "/")
	if values[0] == "/" {
		utils.CheckError(errors.New("NameSpace was not provided NAMESPACE/RESORUCE_NAME, found: " + path))
	}
	if len(values) != 2 {
		utils.CheckError(errors.New("Resource name was not provided NAMESPACE/RESOURCE_NAME, found: " + path))
	}
	return values[0], values[1], nil
}
*/

func findAnsibleTemplateNamefromConfigMap(config *rest.Config, configMapName string, namespace string, jobType string) (string, error) {
	kubeset, err := kubernetes.NewForConfig(config)
	utils.CheckError(err)
	cm, err := kubeset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), configMapName, v1.GetOptions{})
	utils.CheckError(err)
	if cm.Data[jobType+"-towertemplatename"] == "" {
		return "", errors.New("Missing prehook-towertemplatename in job ConfigMap " + configMapName)
	}
	return cm.Data[jobType+"-towertemplatename"], nil
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
		log.Println("No ConfigMap, nothing to do")
		os.Exit(0)
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

	towerTemplateName, err := findAnsibleTemplateNamefromConfigMap(config, configMapName, namespace, jobType)
	if err == nil {
		ansible.RunAnsibleJob(config, namespace, jobType, towerTemplateName, "toweraccess", nil)
	} else {
		log.Println(err)
	}

	log.Println("Done!")
}
