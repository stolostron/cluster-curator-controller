// Copyright Contributors to the Open Cluster Management project.

package main

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"strings"

	"k8s.io/klog/v2"

	managedclusterclient "github.com/open-cluster-management/api/client/cluster/clientset/versioned"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/ansible"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/hive"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/importer"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/secrets"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/utils"
	"github.com/open-cluster-management/library-go/pkg/config"
	hiveclient "github.com/openshift/hive/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

const LogFlag = "v"
const LogVerbosity = "2"

/* Uses the following environment variables:
 * ./curator applycloudprovider
 *    export CLUSTER_NAME=                  # The name of the cluster
 *    export PROVIDER_CREDENTIAL_PATH=      # The NAMESPACE/SECRET_NAME for the Cloud Provider
 */
func main() {
	var err error
	var clusterName = os.Getenv("CLUSTER_NAME")

	utils.InitKlog()

	if clusterName == "" {
		data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err != nil {
			klog.Warning("Missing the environment variable CLUSTER_NAME")
		}
		utils.CheckError(err)
		clusterName = string(data)
	}

	// Build a connection to the ACM Hub OCP
	config, err := config.LoadConfig("", "", "")
	utils.CheckError(err)

	var cmdErrorMsg = errors.New("Invalid Parameter: \"" + os.Args[1] +
		"\"\nCommand: ./curator [monitor-import|monitor|activate-monitor|applycloudprovider-aws|" +
		"applycloudprovider-gcp|applycloudprovider-azure]")

	if len(os.Args) == 2 {
		switch os.Args[1] {
		case "applycloudprovider-aws", "applycloudprovider-ansible", "monitor-import", "monitor", "ansiblejob",
			"applycloudprovider-gcp", "applycloudprovider-azure":

		case "activate-monitor":

			hiveset, err := hiveclient.NewForConfig(config)
			utils.CheckError(err)

			err = hive.ActivateDeploy(hiveset, clusterName)
			utils.CheckError(err)
		default:
			utils.CheckError(cmdErrorMsg)
		}
		klog.V(2).Info("Mode: " + os.Args[1] + " Cluster")
	} else {
		utils.CheckError(cmdErrorMsg)
	}
	jobChoice := os.Args[1]

	// Create a typed client for kubernetes
	kubeset, err := kubernetes.NewForConfig(config)
	utils.CheckError(err)

	providerCredentialPath := os.Getenv("PROVIDER_CREDENTIAL_PATH")

	var clusterConfigOverride *corev1.ConfigMap
	// Gets the Cluster Configuration overrides
	clusterConfigOverride, err = kubeset.CoreV1().ConfigMaps(clusterName).Get(context.TODO(), clusterName, v1.GetOptions{})
	// Allow an override with the PROVIDER_CREDENTIAL_PATH
	if err == nil {
		klog.V(2).Info("Found clusterConfigOverride \"" + clusterConfigOverride.Data["clusterName"] + "\" ✓")
		if clusterConfigOverride.Data["clusterName"] == "" {
			clusterConfigOverride.Data["clusterName"] = clusterName
		}
		if clusterName != clusterConfigOverride.Data["clusterName"] {
			utils.CheckError(errors.New("Cluster namespace \"" + clusterName +
				"\" does not match the cluster ConfigMap override \"" +
				clusterConfigOverride.Data["clusterName"] + "\""))
		}
		utils.RecordHiveJobContainer(kubeset, clusterConfigOverride, jobChoice)
		providerCredentialPath = clusterConfigOverride.Data["providerCredentialPath"]
	} else {
		if providerCredentialPath == "" || !strings.Contains(jobChoice, "applycloudprovider-") {
			utils.CheckError(err)
		}
		klog.V(0).Info("Using PROVIDER_CREDNETIAL_PATH to find the Cloud Provider secret")
	}

	// Always create the Ansible secret, plus Cloud provider secrets if requested
	var secretData *map[string]string
	if strings.Contains(jobChoice, "applycloudprovider-") {
		secretData = secrets.GetSecretData(kubeset, providerCredentialPath)
		klog.V(2).Info("=> Applying Provider credential \"" + providerCredentialPath + "\" to cluster " + clusterName)
		if jobChoice == "applycloudprovider-aws" {
			err := secrets.CreateAWSSecrets(kubeset, *secretData, clusterName)
			utils.CheckError(err)
		} else if jobChoice == "applycloudprovider-gcp" {
			err := secrets.CreateGCPSecrets(kubeset, *secretData, clusterName)
			utils.CheckError(err)
		} else if jobChoice == "applycloudprovider-azure" {
			err := secrets.CreateAzureSecrets(kubeset, *secretData, clusterName)
			utils.CheckError(err)
		}
		err := secrets.CreateAnsibleSecret(kubeset, *secretData, clusterName)
		utils.CheckError(err)

	}

	if strings.Contains(jobChoice, "monitor") {
		err := hive.MonitorDeployStatus(config, clusterName)
		utils.CheckError(err)
	}
	// Create a client for the manageclusterV1 CustomResourceDefinitions
	if jobChoice == "monitor-import" {
		mcset, err := managedclusterclient.NewForConfig(config)
		utils.CheckError(err)

		utils.CheckError(importer.MonitorImport(mcset, clusterName))
	}

	if jobChoice == "ansiblejob" {
		dynclient, err := dynamic.NewForConfig(config)
		utils.CheckError(err)

		err = ansible.Job(dynclient, clusterConfigOverride)
		utils.CheckError(err)
	}

	klog.V(2).Info("Done!")
}
