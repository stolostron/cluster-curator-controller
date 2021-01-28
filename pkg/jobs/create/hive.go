// Copyright (c) 2020 Red Hat, Inc.
package create

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"strings"

	yaml "github.com/ghodss/yaml"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/utils"
	hivev1 "github.com/openshift/hive/pkg/apis/hive/v1"
	hiveclient "github.com/openshift/hive/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func ActivateDeploywithConfig(config *rest.Config, clusterName string) {
	hiveset, err := hiveclient.NewForConfig(config)
	utils.CheckError(err)
	ActivateDeploy(hiveset, clusterName)
}

//  patchStringValue specifies a json patch operation for a string.
type patchStringValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value int32  `json:"value"`
}

func ActivateDeploy(hiveset *hiveclient.Clientset, clusterName string) {
	log.Println("* Initiate Provisioning")
	log.Println("Looking up cluster " + clusterName)
	cluster, err := hiveset.HiveV1().ClusterDeployments(clusterName).Get(context.TODO(), clusterName, v1.GetOptions{})
	utils.CheckError(err)
	log.Println("Found cluster " + cluster.Name + " ✓")
	if *cluster.Spec.InstallAttemptsLimit != 0 {
		log.Panic("ClusterDeployment.spec.installAttemptsLimit is not 0")
	}
	// Generate the AWS Credential secret
	intValue := int32(1)
	patch := []patchStringValue{{
		Op:    "replace",
		Path:  "/spec/installAttemptsLimit",
		Value: intValue,
	}}
	patchInBytes, _ := json.Marshal(patch)
	*cluster.Spec.InstallAttemptsLimit = 1
	_, err = hiveset.HiveV1().ClusterDeployments(clusterName).Patch(context.TODO(), clusterName, types.JSONPatchType, patchInBytes, v1.PatchOptions{})
	/*log.Println("Update ClusterDeployment" + clusterName + " to start provisioning")
	_, err = hiveset.HiveV1().ClusterDeployments(clusterName).Update(context.TODO(), cluster, v1.UpdateOptions{})*/
	utils.CheckError(err)
	log.Println("Updated ClusterDeployment ✓")
}

// Create the ClusterDeployment resource from the Template, but apply supported overrides
func CreateClusterDeployment(hiveset *hiveclient.Clientset, configMapTemplate *corev1.ConfigMap, configMapOverride *corev1.ConfigMap) {
	log.Println("* Prepare ClusterDeployment")
	newCluster := &hivev1.ClusterDeployment{}
	//hivev1.ClusterDeployment is defined with json for unmarshaling
	cdJSON, err := yaml.YAMLToJSON([]byte(configMapTemplate.Data["clusterDeployment"]))
	utils.CheckError(err)
	err = json.Unmarshal(cdJSON, &newCluster)
	utils.CheckError(err)

	// Merge overrides
	if configMapOverride != nil {
		clusterName := configMapOverride.Data["clusterName"]
		utils.OverrideStringField(&newCluster.ClusterName, clusterName, "clusterName")
		utils.OverrideStringField(&newCluster.Name, clusterName, "name")
		utils.OverrideStringField(&newCluster.Spec.BaseDomain, configMapOverride.Data["baseDomain"], "baseDomain")
		utils.OverrideStringField(&newCluster.Spec.Provisioning.InstallConfigSecretRef.Name, clusterName+"-install-config", "spec.platform.provisioning.installConfigSecretRef.name")
		utils.OverrideStringField(&newCluster.Spec.Provisioning.SSHPrivateKeySecretRef.Name, clusterName+"-ssh-private-key", "spec.platform.provisioning.sshPrivateKeySecretRef.name")
		utils.OverrideStringField(&newCluster.Spec.Provisioning.ImageSetRef.Name, configMapOverride.Data["imageSetRef"], "spec.provisioning.imageSetRef.name")
		utils.OverrideStringField(&newCluster.Spec.Platform.AWS.CredentialsSecretRef.Name, clusterName+"-creds", "spec.platform.aws.credentialsSecretRef.name")
		utils.OverrideStringField(&newCluster.Spec.Platform.AWS.Region, configMapOverride.Data["region"], "spec.platform.aws.region")
		utils.OverrideStringField(&newCluster.Spec.PullSecretRef.Name, clusterName+"-pull-secret", "spec.pullSecretRef.name")
		if configMapOverride.Data["addLabels"] != "" {
			var labels map[string]string
			err := yaml.Unmarshal([]byte(configMapOverride.Data["addLabels"]), &labels)
			utils.CheckError(err)
			if newCluster.Labels == nil {
				newCluster.Labels = make(map[string]string)
			}
			for key, val := range labels {
				newCluster.Labels[key] = val
			}
		}
	} else {
		log.Println("No overrides provided")
	}

	log.Print("Creating ClusterDeployment " + newCluster.GetName() + " in namespace " + newCluster.GetName())
	_, err = hiveset.HiveV1().ClusterDeployments(newCluster.GetName()).Create(context.TODO(), newCluster, v1.CreateOptions{})
	utils.CheckError(err)
	log.Println("Created ClusterDeployment ✓")
}

func CreateMachinePool(hiveset *hiveclient.Clientset, configMapTemplate *corev1.ConfigMap, configMapOverride *corev1.ConfigMap) {
	log.Println("* Prepare MachinePool")
	newMachinePool := &hivev1.MachinePool{}
	//hivev1.newMachinePool is defined with json for unmarshaling
	mpJSON, err := yaml.YAMLToJSON([]byte(configMapTemplate.Data["machinePool"]))
	utils.CheckError(err)
	err = json.Unmarshal(mpJSON, &newMachinePool)
	utils.CheckError(err)

	// Merge overrides
	if configMapOverride != nil {
		utils.OverrideStringField(&newMachinePool.Name, configMapOverride.Data["clusterName"]+"-worker", "name")
		utils.OverrideStringField(&newMachinePool.Spec.ClusterDeploymentRef.Name, configMapOverride.Data["clusterName"], "spec.clusterDeploymentRef.name")
		utils.OverrideIntField(&newMachinePool.Spec.Platform.AWS.EC2RootVolume.IOPS, configMapOverride.Data["worker-vol-iops"], "spec.platform.aws.rootVolume.iops")
		utils.OverrideIntField(&newMachinePool.Spec.Platform.AWS.EC2RootVolume.Size, configMapOverride.Data["worker-vol-size"], "spec.platform.aws.rootVolume.size")
		utils.OverrideStringField(&newMachinePool.Spec.Platform.AWS.EC2RootVolume.Type, configMapOverride.Data["worker-vol-type"], "spec.platform.aws.rootVolume.type")
		utils.OverrideStringField(&newMachinePool.Spec.Platform.AWS.Type, configMapOverride.Data["worker-ami-type"], "spec.platform.aws.type")
		utils.OverrideInt64Field(newMachinePool.Spec.Replicas, configMapOverride.Data["worker-replicas"], "spec.replicas")

	} else {
		log.Println("No overrides provided")
	}

	log.Print("Creating MachinePool " + newMachinePool.GetName() + " in namespace " + newMachinePool.GetName())
	_, err = hiveset.HiveV1().MachinePools(configMapOverride.Data["clusterName"]).Create(context.TODO(), newMachinePool, v1.CreateOptions{})
	utils.CheckError(err)
	log.Println("Created MachinePool ✓")
}

func applyValueInt(dest map[string]interface{}, path string, source string) {
	applyValue(dest, path, source, true)
}
func applyValueString(dest map[string]interface{}, path string, source string) {
	applyValue(dest, path, source, false)
}

func applyValue(dest map[string]interface{}, path string, source string, useInt bool) {
	if source != "" {
		traverse := dest
		keys := strings.Split(path, ".")
		for i, key := range keys {
			if i == len(keys)-1 {
				if useInt {
					s, err := strconv.Atoi(source)
					utils.CheckError(err)
					traverse[key] = s
				} else {
					traverse[key] = source
				}
				log.Println("Overriding " + path + " \"" + source + "\" ✓")
			} else {
				traverse = traverse[key].(map[string]interface{})
			}
		}
	} else {
		log.Println("Overriding " + path + " \"" + source + "\" X (NOT PROVIDED)")
	}
}

func CreateInstallConfig(kubeset *kubernetes.Clientset, configMapTemplate *corev1.ConfigMap, configMapOverride *corev1.ConfigMap, sshPublickey string) {
	log.Println("* Prepare install-config secret")
	newInstallConfig := &corev1.Secret{}
	installConfig := make(map[string]interface{})

	//Convert the YAML string into Go struct
	err := yaml.Unmarshal([]byte(configMapTemplate.Data["installConfig"]), &installConfig)
	utils.CheckError(err)
	data := configMapOverride.Data
	clusterName := data["clusterName"]
	installConfig["metadata"].(map[string]interface{})["name"] = clusterName
	// Apply overrides
	installConfig["baseDomain"] = data["baseDomain"]
	applyValueString(installConfig, "platform.aws.region", data["region"])
	applyValueInt(installConfig["compute"].([]interface{})[0].(map[string]interface{}), "replicas", data["worker-replicas"])
	applyValueString(installConfig["compute"].([]interface{})[0].(map[string]interface{}), "platform.aws.type", data["worker-ami-type"])
	applyValueInt(installConfig["compute"].([]interface{})[0].(map[string]interface{}), "platform.aws.rootVolume.iops", data["worker-vol-iops"])
	applyValueInt(installConfig["compute"].([]interface{})[0].(map[string]interface{}), "platform.aws.rootVolume.size", data["worker-vol-size"])
	applyValueString(installConfig["compute"].([]interface{})[0].(map[string]interface{}), "platform.aws.rootVolume.type", data["worker-vol-type"])
	installConfig["sshKey"] = sshPublickey
	byteData, err := yaml.Marshal(&installConfig)
	utils.CheckError(err)
	newInstallConfig.StringData = make(map[string]string)
	newInstallConfig.StringData["install-config.yaml"] = string(byteData)

	newInstallConfig.Name = clusterName + "-install-config"
	newInstallConfig.Namespace = clusterName
	log.Print("Creating install-config secret " + clusterName + "-install-config in namespace " + clusterName)
	_, err = kubeset.CoreV1().Secrets(clusterName).Create(context.TODO(), newInstallConfig, v1.CreateOptions{})
	utils.CheckError(err)
	log.Println("Created install-config secret ✓")
}
