// Copyright (c) 2020 Red Hat, Inc.
package main

import (
	"context"
	"errors"
	"flag"
	"io/ioutil"
	"os"
	"strings"

	"k8s.io/klog/v2"

	yaml "github.com/ghodss/yaml"
	managedclusterclient "github.com/open-cluster-management/api/client/cluster/clientset/versioned"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/create"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/importer"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/secrets"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/utils"
	hiveclient "github.com/openshift/hive/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

//Path splitter NAMSPACE/RESOURCE_NAME
func pathSplitterFromEnv(path string) (namespace string, resource string, err error) {
	values := strings.Split(path, "/")
	if values[0] == "/" {
		utils.CheckError(errors.New("NameSpace was not provided NAMESPACE/RESORUCE_NAME, found: " + path))
	}
	if len(values) != 2 {
		utils.CheckError(errors.New("Resource name was not provided NAMESPACE/RESOURCE_NAME, found: " + path))
	}
	return values[0], values[1], nil
}

/* Command: go run ./pkg/jobs/aws.go [create|import|applycloudprovider]
 *
 * Uses the following environment variables:
 * ./awsjob applycloudprovider
 *    export CLUSTER_NAME=                  # The name of the cluster
 *    export PROVIDER_CREDENTIAL_PATH=      # The NAMESPACE/SECRET_NAME for the Cloud Provider
 * ./awsjob create
 *    export CLUSTER_NAME=                  # The name of the cluster
 *    export PROVIDER_CREDENTIAL_PATH=      # The NAMESPACE/SECRET_NAME for the Cloud Provider
 *    export CLUSTER_CONFIG_TEMPLATE_PATH=  # The NAMESPACE/CONFIGMAP_NAME for the cluster template
 * ./awsjob import
 *    export CLUSTER_NAME=                  # The name of the cluster
 *    export CLUSTER_CONFIG_TEMPLATE_PATH=  # The NAMESPACE/CONFIGMAP_NAME for the cluster template
 */
func main() {
	var kubeconfig *string
	var err error

	var clusterName = os.Getenv("CLUSTER_NAME")
	if clusterName == "" {
		data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err != nil {
			klog.Warning("Missing the environment variable CLUSTER_NAME")
		}
		utils.CheckError(err)
		clusterName = string(data)
	}

	// Build a connection to the ACM Hub OCP
	homePath := os.Getenv("HOME")
	kubeconfig = flag.String("kubeconfig", homePath+"/.kube/config", "")
	flag.Parse()

	var config *rest.Config

	if _, err = os.Stat(homePath + "/.kube/config"); !os.IsNotExist(err) {
		klog.V(2).Info("Connecting with local kubeconfig")
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	} else {
		klog.V(2).Info("Connecting using In Cluster Config")
		config, err = rest.InClusterConfig()
	}
	utils.CheckError(err)

	if len(os.Args) == 2 {
		switch os.Args[1] {
		case "applycloudprovider-aws", "applycloudprovider-ansible", "import", "create-aws", "monitor":
		case "activate-monitor":
			hiveset, err := hiveclient.NewForConfig(config)
			utils.CheckError(err)
			create.ActivateDeploy(hiveset, clusterName)
		default:
			utils.CheckError(errors.New("Invalid Parameter: \"" + os.Args[1] + "\"\nCommand: ./curator [create|import|applycloudprovider]"))
		}
		klog.V(2).Info("Mode: " + os.Args[1] + " Cluster")
	} else {
		utils.CheckError(errors.New("Invalid Parameter: \"" + os.Args[1] + "\"\nCommand: ./curator [create|import|applycloudprovider]"))
	}
	jobChoice := os.Args[1]

	// Create a typed client for kubernetes
	kubeset, err := kubernetes.NewForConfig(config)
	utils.CheckError(err)

	providerCredentialPath := os.Getenv("PROVIDER_CREDENTIAL_PATH")

	var clusterConfigTemplate, clusterConfigOverride *corev1.ConfigMap
	// Gets the Cluster Configuration overrides
	clusterConfigOverride, err = kubeset.CoreV1().ConfigMaps(clusterName).Get(context.TODO(), clusterName, v1.GetOptions{})
	// Allow an override with the PROVIDER_CREDENTIAL_PATH
	if err == nil {
		utils.CheckError(err)
		klog.V(2).Info("Found clusterConfigOverride \"" + clusterConfigOverride.Data["clusterName"] + "\" ✓")
		if clusterName != clusterConfigOverride.Data["clusterName"] {
			utils.CheckError(errors.New("Cluster namespace " + clusterName + " does not match the cluster ConfigMap override " + clusterConfigOverride.Data["clusterName"]))
		}
		utils.RecordJobContainer(config, clusterConfigOverride, jobChoice)
		providerCredentialPath = clusterConfigOverride.Data["providerCredentialPath"]
	} else {
		if providerCredentialPath == "" || !strings.Contains(jobChoice, "applycloudprovider-") {
			utils.CheckError(err)
		}
		klog.V(0).Info("Using PROVIDER_CREDNETIAL_PATH to find the Cloud Provider secret")
	}

	// Always create the Ansible secret, plus Cloud provider secrets if requested
	secretData := make(map[string]string)
	if strings.Contains(jobChoice, "applycloudprovider-") {
		// Read Cloud Provider Secret and create Hive cluster secrets, Cloud Provider Credential, pull-secret & ssh-private-key
		// Determine kube path for Provider credential
		secretNamespace, secretName, err := pathSplitterFromEnv(providerCredentialPath)
		utils.CheckError(err)

		klog.V(2).Info("=> Applying Provider credential namespace \"" + secretNamespace + "\" secret \"" + secretName + "\" to cluster " + clusterName)
		secret, err := kubeset.CoreV1().Secrets(secretNamespace).Get(context.TODO(), secretName, v1.GetOptions{})
		utils.CheckError(err)

		err = yaml.Unmarshal(secret.Data["metadata"], &secretData)
		utils.CheckError(err)
		klog.V(0).Info("Found Cloud Provider secret \"" + secret.GetName() + "\" ✓")
		if jobChoice == "applycloudprovider-aws" {
			secrets.CreateAWSSecrets(kubeset, secretData, clusterName)
		} else if jobChoice == "applycloudprovider-gcp" {
			secrets.CreateGCPSecrets(kubeset, secretData, clusterName)
		} else if jobChoice == "applycloudprovider-azure" {
			secrets.CreateAzureSecrets(kubeset, secretData, clusterName)
		}
		secrets.CreateAnsibleSecret(kubeset, secretData, clusterName)
	}

	var cmNameSpace, ClusterCMTemplate string
	// Create cluster resources, ClusterDeployment, MachinePool & install-config secret
	if jobChoice == "import" || strings.Contains(jobChoice, "create-") {

		// Determine kube path for Cluster Template
		cmNameSpace, ClusterCMTemplate, err = pathSplitterFromEnv(clusterConfigOverride.Data["clusterConfigTemplatePath"])
		utils.CheckError(err)

		// Gets the Cluster Configuration Template, defaults!
		clusterConfigTemplate, err = kubeset.CoreV1().ConfigMaps(cmNameSpace).Get(context.TODO(), ClusterCMTemplate, v1.GetOptions{})
		utils.CheckError(err)
		klog.V(0).Info("Found clusterConfigTemplate \"" + cmNameSpace + "/" + ClusterCMTemplate + "\" ✓")
	}

	if jobChoice == "create-aws" {
		// Transfer extra keys from Cloud Provider Secret if not overridden
		if secretData["baseDomain"] != "" && clusterConfigOverride.Data["baseDomain"] == "" {
			clusterConfigOverride.Data["baseDomain"] = secretData["baseDomain"]
			klog.V(2).Info("Using baseDomain from Cloud Provider, \"" + clusterConfigOverride.Data["baseDomain"] + "\"")
		}

		klog.V(0).Info("=> Creating Cluster in namespace \"" + clusterName + "\" using ConfigMap Template \"" + cmNameSpace + "/" + ClusterCMTemplate + "\" and ConfigMap Override \"" + clusterName)
		hiveset, err := hiveclient.NewForConfig(config)
		utils.CheckError(err)

		create.CreateInstallConfig(kubeset, clusterConfigTemplate, clusterConfigOverride, secretData["sshPublickey"])
		create.CreateClusterDeployment(hiveset, clusterConfigTemplate, clusterConfigOverride)
		create.CreateMachinePool(hiveset, clusterConfigTemplate, clusterConfigOverride)
	}
	if strings.Contains(jobChoice, "monitor") {
		utils.MonitorDeployStatus(config, clusterName)
	}
	// Create a client for the manageclusterV1 CustomResourceDefinitions
	if jobChoice == "import" {
		klog.V(0).Info("=> Importing Cluster in namespace \"" + clusterName + "\" using ConfigMap Template \"" + cmNameSpace + "/" + ClusterCMTemplate + "\" and ConfigMap Override \"" + clusterName)
		managedclusterclient, err := managedclusterclient.NewForConfig(config)
		utils.CheckError(err)

		importer.CreateKlusterletAddonConfig(config, clusterConfigTemplate, clusterConfigOverride)
		importer.CreateManagedCluster(managedclusterclient, clusterConfigTemplate, clusterConfigOverride)
	}

	klog.V(2).Info("Done!")
}
