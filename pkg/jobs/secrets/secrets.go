// Copyright Contributors to the Open Cluster Management project.
package secrets

import (
	"context"
	"encoding/json"
	"strings"

	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"

	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/utils"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

//  patchStringValue specifies a json patch operation for a string.
type patchStringValue struct {
	Op    string            `json:"op"`
	Path  string            `json:"path"`
	Value map[string]string `json:"value"`
}

const suffix = "-creds"

func GetSecretData(kubeset *kubernetes.Clientset, providerCredentialPath string) *map[string]string {
	secretData := make(map[string]string)
	// Read Cloud Provider Secret and create Hive cluster secrets, Cloud Provider Credential, pull-secret & ssh-private-key
	// Determine kube path for Provider credential
	secretNamespace, secretName, err := utils.PathSplitterFromEnv(providerCredentialPath)
	utils.CheckError(err)

	klog.V(2).Info("=> Retrieving  Provider credential namespace \"" + secretNamespace + "\" secret \"" + secretName + "\"")
	secret, err := kubeset.CoreV1().Secrets(secretNamespace).Get(
		context.TODO(), secretName, v1.GetOptions{})

	utils.CheckError(err)

	err = yaml.Unmarshal(secret.Data["metadata"], &secretData)
	utils.CheckError(err)
	klog.V(0).Info("Found Cloud Provider secret \"" + secret.GetName() + "\" ✓")
	return &secretData
}

func CreateAnsibleSecret(kubeset *kubernetes.Clientset, cpSecretData map[string]string, clusterName string) error {
	// Generate the Ansible Tower credential secret
	klog.V(2).Info("Check if Ansible Tower credentials are present")
	if cpSecretData["ansibleHost"] != "" && cpSecretData["ansibleToken"] != "" {
		stringData := map[string]string{
			"host":  cpSecretData["ansibleHost"],
			"token": cpSecretData["ansibleToken"],
		}
		if err := createPatchSecret(kubeset, stringData, "toweraccess", clusterName, corev1.SecretTypeOpaque); err != nil {
			return nil
		}
	} else {
		klog.Warning("No Ansible Tower credentials found.")
	}
	return nil
}

func CreateAWSSecrets(kubeset *kubernetes.Clientset, cpSecretData map[string]string, clusterName string) error {

	// Generate the AWS Credential secret
	osServicePrincipal := map[string]string{
		"clientId":       cpSecretData["clientId"],
		"clientSecret":   cpSecretData["clientSecret"],
		"tenantId":       cpSecretData["tenantId"],
		"subscriptionId": cpSecretData["subscriptionId"],
	}
	bytes, err := json.Marshal(osServicePrincipal)
	if err = utils.LogError(err); err != nil {
		return err
	}
	stringData := map[string]string{
		"osServicePrincipal.json": string(bytes),
	}
	if err := createPatchSecret(kubeset, stringData, clusterName+suffix, clusterName, corev1.SecretTypeOpaque); err != nil {
		return err
	}
	return createCommonSecrets(kubeset, cpSecretData, clusterName)
}

func CreateGCPSecrets(kubeset *kubernetes.Clientset, cpSecretData map[string]string, clusterName string) error {

	// Generate the AWS Credential secret
	stringData := map[string]string{
		"osServiceAccount.json": cpSecretData["gcServiceAccountKey"],
	}
	if err := createPatchSecret(kubeset, stringData, clusterName+suffix, clusterName, corev1.SecretTypeOpaque); err != nil {
		return err
	}
	return createCommonSecrets(kubeset, cpSecretData, clusterName)
}

func CreateAzureSecrets(kubeset *kubernetes.Clientset, cpSecretData map[string]string, clusterName string) error {

	// Generate the AWS Credential secret
	stringData := map[string]string{
		"aws_access_key_id":     cpSecretData["awsAccessKeyID"],
		"aws_secret_access_key": cpSecretData["awsSecretAccessKeyID"],
	}
	if err := createPatchSecret(kubeset, stringData, clusterName+suffix, clusterName, corev1.SecretTypeOpaque); err != nil {
		return err
	}
	return createCommonSecrets(kubeset, cpSecretData, clusterName)
}

func createCommonSecrets(kubeset *kubernetes.Clientset, cpSecretData map[string]string, clusterName string) error {
	// Generate Pull Secret
	stringData := map[string]string{
		".dockerconfigjson": cpSecretData["pullSecret"],
	}
	if err := createPatchSecret(kubeset, stringData, clusterName+"-pull-secret",
		clusterName, corev1.SecretTypeDockerConfigJson); err != nil {

		return err
	}

	// Generate SSH Private Key
	stringData = map[string]string{
		"ssh-privatekey": cpSecretData["sshPrivatekey"],
	}
	if err := createPatchSecret(kubeset, stringData, clusterName+"-ssh-private-key",
		clusterName, corev1.SecretTypeOpaque); err != nil {

		return err
	}
	return nil
}

func createPatchSecret(
	kubeset *kubernetes.Clientset,
	stringData map[string]string,
	secretName string,
	clusterName string,
	secretType corev1.SecretType) error {

	klog.V(0).Info("Creating secret " + secretName + " in namespace " + clusterName)

	mObj := v1.ObjectMeta{Name: secretName}
	newSecret := &corev1.Secret{StringData: stringData, ObjectMeta: mObj, Type: secretType}
	_, err := kubeset.CoreV1().Secrets(clusterName).Create(context.TODO(), newSecret, v1.CreateOptions{})
	// This is where we patch. To save permissions we use the error instead of a list
	if err != nil && strings.Contains(err.Error(), "already exists") {
		klog.V(2).Info(" X (already exists)")
		patch := []patchStringValue{{
			Op:    "replace",
			Path:  "/stringData",
			Value: stringData,
		}}
		patchInBytes, _ := json.Marshal(patch)
		klog.V(2).Info(" > Patching secret " + secretName + " in namespace " + clusterName)
		_, err = kubeset.CoreV1().Secrets(clusterName).Patch(
			context.TODO(), secretName, types.JSONPatchType, patchInBytes, v1.PatchOptions{})
	}

	if err = utils.LogError(err); err != nil {
		return err
	}
	klog.V(0).Info("Applied Secret ✓")
	return nil
}
