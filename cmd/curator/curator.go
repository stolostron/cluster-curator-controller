// Copyright Contributors to the Open Cluster Management project.

package main

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"strings"

	"k8s.io/klog/v2"

	"github.com/open-cluster-management/cluster-curator-controller/pkg/controller/launcher"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/ansible"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/hive"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/importer"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/secrets"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/utils"
	"github.com/open-cluster-management/library-go/pkg/config"
	hiveclient "github.com/openshift/hive/pkg/client/clientset/versioned"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	clientv1 "sigs.k8s.io/controller-runtime/pkg/client"
)

const CuratorJob = "clustercurator-job"

/* Uses the following environment variables:
 * ./curator applycloudprovider
 *    export CLUSTER_NAME=                  # The name of the cluster
 *    export PROVIDER_CREDENTIAL_PATH=      # The NAMESPACE/SECRET_NAME for the Cloud Provider
 */
func main() {
	var clusterName = os.Getenv("CLUSTER_NAME")

	utils.InitKlog(utils.LogVerbosity)

	if clusterName == "" {
		data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err != nil {
			klog.Warning("Missing the environment variable CLUSTER_NAME")
		}
		utils.CheckError(err)
		clusterName = string(data)
	}

	// Build a connection to the Hub OCP
	config, err := config.LoadConfig("", "", "")
	utils.CheckError(err)

	client, err := utils.GetClient()
	utils.CheckError(err)

	curatorRun(config, &client, clusterName)
	// Gets the Cluster Configuration overrides
	curator, err := utils.GetClusterCurator(client, clusterName)
	utils.CheckError(err)

	curator.Spec.DesiredCuration = ""
	curator.Spec.CuratingJob = ""
	curator.Status.Conditions = nil
	err = client.Update(context.TODO(), curator)
	utils.CheckError(err)
}

func curatorRun(config *rest.Config, client *clientv1.Client, clusterName string) {

	var err error
	var cmdErrorMsg = errors.New("Invalid Parameter: \"" + os.Args[1] +
		"\"\nCommand: ./curator [monitor-import|monitor|activate-and-monitor|applycloudprovider-aws|" +
		"applycloudprovider-gcp|applycloudprovider-azure|upgrade-cluster|monitor-upgrade|done]")

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "applycloudprovider-aws", "applycloudprovider-ansible", "monitor-import", "monitor", "ansiblejob",
			"applycloudprovider-gcp", "applycloudprovider-azure", "activate-and-monitor", "upgrade-cluster", "monitor-upgrade",
			"SKIP_ALL_TESTING", "prehook-ansiblejob", "posthook-ansiblejob", "done":
		default:
			utils.CheckError(cmdErrorMsg)
		}
		klog.V(2).Info("Mode: " + os.Args[1] + " Cluster")
	} else {
		utils.CheckError(cmdErrorMsg)
	}
	jobChoice := os.Args[1]

	providerCredentialPath := os.Getenv("PROVIDER_CREDENTIAL_PATH")

	// Gets the Cluster Configuration overrides
	curator, err := utils.GetClusterCurator(*client, clusterName)

	// Allow an override with the PROVIDER_CREDENTIAL_PATH
	if err == nil {
		klog.V(2).Info("Found clusterCurator resource \"" + curator.Namespace + "\" ✓")

		utils.CheckError(utils.RecordCurrentStatusCondition(
			*client,
			clusterName,
			CuratorJob,
			v1.ConditionFalse,
			curator.Spec.CuratingJob))

		// Special case
		if jobChoice != launcher.DoneDoneDone {
			utils.CheckError(utils.RecordCurrentStatusCondition(
				*client,
				clusterName,
				jobChoice,
				v1.ConditionFalse,
				"Executing init container "+jobChoice))
		}
		providerCredentialPath = curator.Spec.ProviderCredentialPath

		// This makes sure we set the curator-job condition to false when there is a failure
		defer func() {
			if r := recover(); r != nil {
				utils.CheckError(utils.RecordCurrentStatusCondition(
					*client,
					clusterName,
					CuratorJob,
					v1.ConditionTrue,
					curator.Spec.CuratingJob))
				panic(r)
			}
		}()
	} else if err != nil && providerCredentialPath == "" {
		utils.CheckError(err)

	} else {
		klog.V(0).Info("Using PROVIDER_CREDNETIAL_PATH to find the Cloud Provider secret")
	}

	if providerCredentialPath == "" && strings.Contains(jobChoice, "applycloudprovider-") {
		klog.Warningf("providerCredentialPath: " + providerCredentialPath)
		utils.CheckError(errors.New("Missing spec.providerCredentialPath in ClusterCurator: " + clusterName))
	}

	var secretData *map[string]string
	if strings.Contains(jobChoice, "applycloudprovider-") {

		kubeset, err := utils.GetKubeset()
		utils.CheckError(err)

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
		err = secrets.CreateAnsibleSecret(kubeset, *secretData, clusterName)
		utils.CheckError(err)

	}

	if jobChoice == "activate-and-monitor" {
		hiveset, err := hiveclient.NewForConfig(config)
		utils.CheckError(err)

		err = hive.ActivateDeploy(hiveset, clusterName)
		utils.CheckError(err)
	}

	if jobChoice == "monitor" {
		err := hive.MonitorDeployStatus(config, clusterName)
		utils.CheckError(err)
	}
	// Create a client for the manageclusterV1 CustomResourceDefinitions
	if jobChoice == "monitor-import" {
		dynclient, err := utils.GetDynset(nil)
		utils.CheckError(err)

		utils.CheckError(importer.MonitorMCInfoImport(dynclient, clusterName))
	}

	if jobChoice == "upgrade-cluster" {
		// dynclient, err := utils.GetDynset(nil)
		// utils.CheckError(err)

		err = hive.UpgradeCluster(*client, clusterName, curator)
		utils.CheckError(err)
	}

	if jobChoice == "monitor-upgrade" {
		err = hive.MonitorUpgradeStatus(*client, clusterName, curator)
		utils.CheckError(err)
	}

	if jobChoice == "ansiblejob" {

		err = ansible.Job(*client, curator)
		utils.CheckError(err)
	}

	// Override finished init container message with finished curator-job message
	msg := "Completed executing init container"
	condition := v1.ConditionTrue

	if jobChoice == "done" {
		jobChoice = CuratorJob
		msg = curator.Spec.CuratingJob
		condition = v1.ConditionTrue
	}

	// Used to signal end of job as well as end of initi container
	utils.CheckError(utils.RecordCurrentStatusCondition(
		*client,
		clusterName,
		jobChoice,
		condition,
		msg))

	klog.V(2).Info("Done!")
}
