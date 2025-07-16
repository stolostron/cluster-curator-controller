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

	clustercuratorv1 "github.com/stolostron/cluster-curator-controller/pkg/api/v1beta1"
	"github.com/stolostron/cluster-curator-controller/pkg/controller/launcher"
	"github.com/stolostron/cluster-curator-controller/pkg/jobs/ansible"
	"github.com/stolostron/cluster-curator-controller/pkg/jobs/hive"
	"github.com/stolostron/cluster-curator-controller/pkg/jobs/hypershift"
	"github.com/stolostron/cluster-curator-controller/pkg/jobs/importer"
	"github.com/stolostron/cluster-curator-controller/pkg/jobs/secrets"
	"github.com/stolostron/cluster-curator-controller/pkg/jobs/utils"
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

	clusterNamespace := clusterName
	// Hypershift clusters name != namespace
	// The code in main sets clusterName = namespace but this doesn't work for Hypershift
	if len(os.Args) == 3 {
		clusterName = os.Args[2]
	}

	// Build a connection to the Hub OCP
	config, err := utils.LoadConfig()
	utils.CheckError(err)

	client, err := utils.GetClient()
	utils.CheckError(err)

	curatorRun(config, client, clusterName, clusterNamespace)
}

func curatorRun(config *rest.Config, client clientv1.Client, clusterName string, clusterNamespace string) {

	var err error
	var cmdErrorMsg = errors.New("Invalid Parameter: \"" + os.Args[1] +
		"\"\nCommand: ./curator [monitor-import|monitor|activate-and-monitor|applycloudprovider-aws|" +
		"applycloudprovider-gcp|applycloudprovider-azure|upgrade-cluster|intermediate-upgrade-cluster|" +
		"final-upgrade-cluster|monitor-upgrade|intermediate-monitor-upgrade|prehook-ansiblejob|posthook-ansiblejob|done]")

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "applycloudprovider-aws", "applycloudprovider-ansible", "monitor-import", "monitor",
			"applycloudprovider-gcp", "applycloudprovider-azure", "activate-and-monitor", "upgrade-cluster",
			"intermediate-upgrade-cluster", "final-upgrade-cluster", "monitor-upgrade", "intermediate-monitor-upgrade",
			"SKIP_ALL_TESTING", "prehook-ansiblejob", "posthook-ansiblejob", "done", "destroy-cluster", "monitor-destroy",
			"detach-nowait", "delete-cluster-namespace":
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
	curator, err := utils.GetClusterCurator(client, clusterName, clusterNamespace)
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
			clusterNamespace,
			CuratorJob,
			v1.ConditionFalse,
			curator.Spec.CuratingJob+" DesiredCuration: "+desiredCuration))

		// Special case
		if jobChoice != launcher.DoneDoneDone {
			utils.CheckError(utils.RecordCurrentStatusCondition(
				client,
				clusterName,
				clusterNamespace,
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
					clusterNamespace,
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
		dynclient, dErr := utils.GetDynset(nil)
		utils.CheckError(dErr)

		clusterType, ctErr := utils.GetClusterType(client, dynclient, clusterName, clusterNamespace, false)
		utils.CheckError(ctErr)

		if clusterType == utils.StandaloneClusterType {
			if err = hive.ActivateDeploy(client, clusterName); err != nil {
				utils.CheckError(utils.RecordFailedCuratorStatusCondition(
					client,
					clusterName,
					clusterNamespace,
					jobChoice,
					v1.ConditionTrue,
					err.Error()))
				klog.Error(err.Error())
				panic(err)
			}
		} else if clusterType == utils.HypershiftClusterType {
			if err = hypershift.ActivateDeploy(dynclient, clusterName, clusterNamespace); err != nil {
				utils.CheckError(utils.RecordFailedCuratorStatusCondition(
					client,
					clusterName,
					clusterNamespace,
					jobChoice,
					v1.ConditionTrue,
					err.Error()))
				klog.Error(err.Error())
				panic(err)
			}
		}
	}

	if jobChoice == "monitor" || jobChoice == "activate-and-monitor" {
		dynclient, dErr := utils.GetDynset(nil)
		utils.CheckError(dErr)

		clusterType, ctErr := utils.GetClusterType(client, dynclient, clusterName, clusterNamespace, false)
		utils.CheckError(ctErr)

		if clusterType == utils.StandaloneClusterType {
			if err := hive.MonitorClusterStatus(config, clusterName, utils.Installing, curator); err != nil {
				utils.CheckError(utils.RecordFailedCuratorStatusCondition(
					client,
					clusterName,
					clusterNamespace,
					jobChoice,
					v1.ConditionTrue,
					err.Error()))
				klog.Error(err.Error())
				panic(err)
			}
		} else if clusterType == utils.HypershiftClusterType {
			if err = hypershift.MonitorClusterStatus(dynclient,
				client,
				clusterName,
				clusterNamespace, utils.Installing, utils.GetMonitorAttempts(utils.Installing, curator)); err != nil {
				utils.CheckError(utils.RecordFailedCuratorStatusCondition(
					client,
					clusterName,
					clusterNamespace,
					jobChoice,
					v1.ConditionTrue,
					err.Error()))
				klog.Error(err.Error())
				panic(err)
			}
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
				clusterNamespace,
				jobChoice,
				v1.ConditionTrue,
				err.Error()))
			klog.Error(err.Error())
			panic(err)
		}
	}

	if jobChoice == "destroy-cluster" {
		dynclient, dErr := utils.GetDynset(nil)
		utils.CheckError(dErr)

		clusterType, ctErr := utils.GetClusterType(client, dynclient, clusterName, clusterNamespace, false)
		utils.CheckError(ctErr)

		if clusterType == utils.StandaloneClusterType {
			if err = hive.DestroyClusterDeployment(client, clusterName); err != nil {
				utils.CheckError(utils.RecordFailedCuratorStatusCondition(
					client,
					clusterName,
					clusterNamespace,
					jobChoice,
					v1.ConditionTrue,
					err.Error()))
				klog.Error(err.Error())
				panic(err)
			}
		} else if clusterType == utils.HypershiftClusterType {
			if err = hypershift.DetachAndMonitor(dynclient, clusterName, curator); err != nil {
				utils.CheckError(utils.RecordFailedCuratorStatusCondition(
					client,
					clusterName,
					clusterNamespace,
					jobChoice,
					v1.ConditionTrue,
					err.Error()))
				klog.Error(err.Error())
				panic(err)
			}

			if err = hypershift.DestroyHostedCluster(dynclient, clusterName, clusterNamespace); err != nil {
				utils.CheckError(utils.RecordFailedCuratorStatusCondition(
					client,
					clusterName,
					clusterNamespace,
					jobChoice,
					v1.ConditionTrue,
					err.Error()))
				klog.Error(err.Error())
				panic(err)
			}
		}
	}

	if jobChoice == "monitor-destroy" {
		dynclient, dErr := utils.GetDynset(nil)
		utils.CheckError(dErr)

		clusterType, ctErr := utils.GetClusterType(client, dynclient, clusterName, clusterNamespace, false)
		utils.CheckError(ctErr)

		if clusterType == utils.StandaloneClusterType {
			if err := hive.MonitorClusterStatus(config, clusterName, utils.Destroying, curator); err != nil {
				utils.CheckError(utils.RecordFailedCuratorStatusCondition(
					client,
					clusterName,
					clusterNamespace,
					jobChoice,
					v1.ConditionTrue,
					err.Error()))
				klog.Error(err.Error())
				panic(err)
			}
		} else if clusterType == utils.HypershiftClusterType {
			if err = hypershift.MonitorClusterStatus(
				dynclient,
				client,
				clusterName,
				clusterNamespace,
				utils.Destroying,
				utils.GetMonitorAttempts(utils.Installing, curator)); err != nil {
				utils.CheckError(utils.RecordFailedCuratorStatusCondition(
					client,
					clusterName,
					clusterNamespace,
					jobChoice,
					v1.ConditionTrue,
					err.Error()))
				klog.Error(err.Error())
				panic(err)
			}
		}
	}

	if jobChoice == "detach-nowait" {
		dynclient, err := utils.GetDynset(nil)
		utils.CheckError(err)

		if err = importer.DetachCluster(dynclient, clusterName); err != nil {
			utils.CheckError(utils.RecordFailedCuratorStatusCondition(
				client,
				clusterName,
				clusterNamespace,
				jobChoice,
				v1.ConditionTrue,
				err.Error()))
			klog.Error(err.Error())
			panic(err)
		}
	}

	if jobChoice == "upgrade-cluster" {
		dynclient, dErr := utils.GetDynset(nil)
		utils.CheckError(dErr)

		clusterType, ctErr := utils.GetClusterType(client, dynclient, clusterName, clusterNamespace, true)
		utils.CheckError(ctErr)

		if clusterType == utils.StandaloneClusterType {
			if err = hive.UpgradeCluster(client, clusterName, curator); err != nil {
				utils.CheckError(utils.RecordFailedCuratorStatusCondition(
					client,
					clusterName,
					clusterNamespace,
					jobChoice,
					v1.ConditionTrue,
					err.Error()))
				klog.Error(err.Error())
				panic(err)
			}
		} else if clusterType == utils.HypershiftClusterType {
			if err = hypershift.UpgradeCluster(client, dynclient, clusterName, curator); err != nil {
				utils.CheckError(utils.RecordFailedCuratorStatusCondition(
					client,
					clusterName,
					clusterNamespace,
					jobChoice,
					v1.ConditionTrue,
					err.Error()))
				klog.Error(err.Error())
				panic(err)
			}
		}
	}

	if jobChoice == "intermediate-upgrade-cluster" || jobChoice == "final-upgrade-cluster" {
		// no need to check cluster type, only hive EUS upgrade supported for now
		isInterVersion := true
		if jobChoice == "final-upgrade-cluster" {
			isInterVersion = false
		}

		if err := hive.EUSUpgradeCluster(client, clusterName, curator, isInterVersion); err != nil {
			utils.CheckError(utils.RecordFailedCuratorStatusCondition(
				client,
				clusterName,
				clusterNamespace,
				jobChoice,
				v1.ConditionTrue,
				err.Error()))
			klog.Error(err.Error())
			panic(err)
		}
	}

	if jobChoice == "intermediate-monitor-upgrade" {
		// no need to check cluster type, only hive EUS upgrade supported for now
		if err = hive.MonitorUpgradeStatus(client, clusterName, curator, true); err != nil {
			utils.CheckError(utils.RecordFailedCuratorStatusCondition(
				client,
				clusterName,
				clusterNamespace,
				jobChoice,
				v1.ConditionTrue,
				err.Error()))
			klog.Error(err.Error())
			panic(err)
		}
	}

	if jobChoice == "monitor-upgrade" {
		dynclient, dErr := utils.GetDynset(nil)
		utils.CheckError(dErr)

		clusterType, ctErr := utils.GetClusterType(client, dynclient, clusterName, clusterNamespace, true)
		utils.CheckError(ctErr)

		if clusterType == utils.StandaloneClusterType {
			if err = hive.MonitorUpgradeStatus(client, clusterName, curator, false); err != nil {
				utils.CheckError(utils.RecordFailedCuratorStatusCondition(
					client,
					clusterName,
					clusterNamespace,
					jobChoice,
					v1.ConditionTrue,
					err.Error()))
				klog.Error(err.Error())
				panic(err)
			}
		} else if clusterType == utils.HypershiftClusterType {
			if err = hypershift.MonitorUpgradeStatus(dynclient, client, clusterName, curator); err != nil {
				utils.CheckError(utils.RecordFailedCuratorStatusCondition(
					client,
					clusterName,
					clusterNamespace,
					jobChoice,
					v1.ConditionTrue,
					err.Error()))
				klog.Error(err.Error())
				panic(err)
			}
		}
	}

	if jobChoice == "delete-cluster-namespace" {

		if err := updateDeleteClusternamespace(client, curator); err != nil {
			utils.CheckError(utils.RecordFailedCuratorStatusCondition(
				client,
				clusterName,
				clusterNamespace,
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
				clusterNamespace,
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
		clusterNamespace,
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
