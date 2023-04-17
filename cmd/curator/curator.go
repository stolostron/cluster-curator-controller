// Copyright Contributors to the Open Cluster Management project.

package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"k8s.io/klog/v2"

	hiveclient "github.com/openshift/hive/pkg/client/clientset/versioned"
	clustercuratorv1 "github.com/stolostron/cluster-curator-controller/pkg/api/v1beta1"
	"github.com/stolostron/cluster-curator-controller/pkg/controller/launcher"
	"github.com/stolostron/cluster-curator-controller/pkg/jobs/ansible"
	"github.com/stolostron/cluster-curator-controller/pkg/jobs/hive"
	"github.com/stolostron/cluster-curator-controller/pkg/jobs/importer"
	"github.com/stolostron/cluster-curator-controller/pkg/jobs/secrets"
	"github.com/stolostron/cluster-curator-controller/pkg/jobs/utils"
	"github.com/stolostron/library-go/pkg/config"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

	curatorRun(config, client, clusterName)
}

func curatorRun(config *rest.Config, client clientv1.Client, clusterName string) {

	var err error
	var cmdErrorMsg = errors.New("Invalid Parameter: \"" + os.Args[1] +
		"\"\nCommand: ./curator [monitor-import|monitor|activate-and-monitor|applycloudprovider-aws|" +
		"applycloudprovider-gcp|applycloudprovider-azure|upgrade-cluster|monitor-upgrade|" +
		"prehook-ansiblejob|posthook-ansiblejob|done]")

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "applycloudprovider-aws", "applycloudprovider-ansible", "monitor-import", "monitor",
			"applycloudprovider-gcp", "applycloudprovider-azure", "activate-and-monitor", "upgrade-cluster", "monitor-upgrade",
			"SKIP_ALL_TESTING", "prehook-ansiblejob", "posthook-ansiblejob", "done", "destroy-cluster",
			"monitor-destroy", "detach-nowait", "delete-cluster-namespace":
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
	curator, err := utils.GetClusterCurator(client, clusterName)
	var desiredCuration string
	if curator != nil {
		desiredCuration = curator.Spec.DesiredCuration
		if curator.Operation != nil && curator.Operation.RetryPosthook != "" {
			if curator.Operation.RetryPosthook == "installPosthook" {
				desiredCuration = "install"
			}
			if curator.Operation.RetryPosthook == "upgradePosthook" {
				desiredCuration = "upgrade"
			}
		}
	}

	// Allow an override with the PROVIDER_CREDENTIAL_PATH
	if err == nil {
		klog.V(2).Info("Found clusterCurator resource \"" + curator.Namespace + "\" ✓")

		utils.CheckError(utils.RecordCurrentStatusCondition(
			client,
			clusterName,
			CuratorJob,
			v1.ConditionFalse,
			curator.Spec.CuratingJob+" DesiredCuration: "+desiredCuration))

		// Special case
		if jobChoice != launcher.DoneDoneDone {
			utils.CheckError(utils.RecordCurrentStatusCondition(
				client,
				clusterName,
				jobChoice,
				v1.ConditionFalse,
				"Executing init container "+jobChoice))
		}
		providerCredentialPath = curator.Spec.ProviderCredentialPath

		// This makes sure we set the curator-job condition to false when there is a failure
		defer func() {
			if r := recover(); r != nil {
				message := curator.Spec.CuratingJob + " DesiredCuration: " + desiredCuration
				if desiredCuration == "upgrade" {
					message = message + " Version (" + utils.GetCurrentVersionInfo(curator) + ")"
				}
				message = message + " Failed - " + fmt.Sprintf("%v", r)
				utils.CheckError(utils.RecordFailedCuratorStatusCondition(
					client,
					clusterName,
					CuratorJob,
					v1.ConditionTrue,
					message))
				// Remove curatingJob and desiredCuration from curator resource for failed job
				updateFailingClusterCurator(client, curator)
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

		if err = hive.ActivateDeploy(hiveset, clusterName); err != nil {
			utils.CheckError(utils.RecordFailedCuratorStatusCondition(
				client,
				clusterName,
				jobChoice,
				v1.ConditionTrue,
				err.Error()))
			klog.Error(err.Error())
			panic(err)
		}
	}

	if jobChoice == "monitor" || jobChoice == "activate-and-monitor" {
		if err := hive.MonitorClusterStatus(config, clusterName, utils.Installing, curator); err != nil {
			utils.CheckError(utils.RecordFailedCuratorStatusCondition(
				client,
				clusterName,
				jobChoice,
				v1.ConditionTrue,
				err.Error()))
			klog.Error(err.Error())
			panic(err)
		}
	}

	// Create a client for the manageclusterV1 CustomResourceDefinitions
	if jobChoice == "monitor-import" {
		dynclient, err := utils.GetDynset(nil)
		utils.CheckError(err)

		if err = importer.MonitorMCInfoImport(dynclient, clusterName, curator); err != nil {
			utils.CheckError(utils.RecordFailedCuratorStatusCondition(
				client,
				clusterName,
				jobChoice,
				v1.ConditionTrue,
				err.Error()))
			klog.Error(err.Error())
			panic(err)
		}
	}

	if jobChoice == "destroy-cluster" {
		hiveset, err := hiveclient.NewForConfig(config)
		utils.CheckError(err)

		if err = hive.DestroyClusterDeployment(hiveset, clusterName); err != nil {
			utils.CheckError(utils.RecordFailedCuratorStatusCondition(
				client,
				clusterName,
				jobChoice,
				v1.ConditionTrue,
				err.Error()))
			klog.Error(err.Error())
			panic(err)
		}
	}

	if jobChoice == "monitor-destroy" {
		if err := hive.MonitorClusterStatus(config, clusterName, utils.Destroying, curator); err != nil {
			utils.CheckError(utils.RecordFailedCuratorStatusCondition(
				client,
				clusterName,
				jobChoice,
				v1.ConditionTrue,
				err.Error()))
			klog.Error(err.Error())
			panic(err)
		}
	}

	if jobChoice == "detach-nowait" {
		dynclient, err := utils.GetDynset(nil)
		utils.CheckError(err)

		if err = importer.DetachCluster(dynclient, clusterName); err != nil {
			utils.CheckError(utils.RecordFailedCuratorStatusCondition(
				client,
				clusterName,
				jobChoice,
				v1.ConditionTrue,
				err.Error()))
			klog.Error(err.Error())
			panic(err)
		}
	}

	if jobChoice == "upgrade-cluster" {
		if err = hive.UpgradeCluster(client, clusterName, curator); err != nil {
			utils.CheckError(utils.RecordFailedCuratorStatusCondition(
				client,
				clusterName,
				jobChoice,
				v1.ConditionTrue,
				err.Error()))
			klog.Error(err.Error())
			panic(err)
		}
	}

	if jobChoice == "monitor-upgrade" {
		if err = hive.MonitorUpgradeStatus(client, clusterName, curator); err != nil {
			utils.CheckError(utils.RecordFailedCuratorStatusCondition(
				client,
				clusterName,
				jobChoice,
				v1.ConditionTrue,
				err.Error()))
			klog.Error(err.Error())
			panic(err)
		}
	}

	if jobChoice == "delete-cluster-namespace" {

		if err := updateDeleteClusternamespace(client, curator); err != nil {
			utils.CheckError(utils.RecordFailedCuratorStatusCondition(
				client,
				clusterName,
				jobChoice,
				v1.ConditionTrue,
				err.Error()))
			klog.Error(err.Error())
			panic(err)
		}
	}

	if jobChoice == "prehook-ansiblejob" || jobChoice == "posthook-ansiblejob" {
		if err = ansible.Job(client, curator); err != nil {
			utils.CheckError(utils.RecordFailedCuratorStatusCondition(
				client,
				clusterName,
				jobChoice,
				v1.ConditionTrue,
				err.Error()))
			klog.Error(err.Error())
			panic(err)
		}
	}

	// Override finished init container message with finished curator-job message
	msg := "Completed executing init container"
	condition := v1.ConditionTrue

	if jobChoice == "done" {
		jobChoice = CuratorJob
		msg = curator.Spec.CuratingJob + " DesiredCuration: " + desiredCuration
		condition = v1.ConditionTrue

		if desiredCuration == "upgrade" {
			msg = msg + " Version (" + utils.GetCurrentVersionInfo(curator) + ")"
		}

		// Remove DesireCuration, CuratingJob, Status from curator resource
		updateDoneClusterCurator(client, curator, clusterName)
	}

	// Used to signal end of job as well as end of init container
	utils.CheckError(utils.RecordCurrentStatusCondition(
		client,
		clusterName,
		jobChoice,
		condition,
		msg))

	klog.V(2).Info("Done!")
}

func updateDoneClusterCurator(client clientv1.Client, curator *clustercuratorv1.ClusterCurator, clusterName string) {
	if curator.Spec.DesiredCuration == "upgrade" {
		patch := []byte(`{"spec":{"curatorJob": null},"status": null, "operation": null}`)
		err := client.Patch(context.Background(), curator, clientv1.RawPatch(types.MergePatchType, patch))
		utils.CheckError(err)
		return
	}

	patch := []byte(`{"spec":{"curatorJob": null, "desiredCuration": null},"status": null, "operation": null}`)
	err := client.Patch(context.Background(), curator, clientv1.RawPatch(types.MergePatchType, patch))
	utils.CheckError(err)
}

func updateFailingClusterCurator(client clientv1.Client, curator *clustercuratorv1.ClusterCurator) {
	if curator.Spec.DesiredCuration == "upgrade" {
		patch := []byte(`{"spec":{"curatorJob": null}, "operation": null}`)
		err := client.Patch(context.Background(), curator, clientv1.RawPatch(types.MergePatchType, patch))
		utils.CheckError(err)
		return
	}

	patch := []byte(`{"spec":{"curatorJob": null, "desiredCuration": null}, "operation": null}`)
	err := client.Patch(context.Background(), curator, clientv1.RawPatch(types.MergePatchType, patch))
	utils.CheckError(err)
}

func updateDeleteClusternamespace(client clientv1.Client, curator *clustercuratorv1.ClusterCurator) error {
	patch := []byte(`{"spec":{"curatorJob": null, "desiredCuration": "delete-cluster-namespace"}}`)
	return client.Patch(context.Background(), curator, clientv1.RawPatch(types.MergePatchType, patch))
}
