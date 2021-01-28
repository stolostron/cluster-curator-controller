// Copyright (c) 2020 Red Hat, Inc.
package secrets

import (
	"context"
	"encoding/json"
	"log"
	"strings"

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

func CreateAnsibleSecret(kubeset *kubernetes.Clientset, cpSecretData map[string]string, clusterName string) {
	// Generate the Ansible Tower credential secret
	log.Println("Check if Ansible Tower credentials are present")
	if cpSecretData["ansibleHost"] != "" && cpSecretData["ansibleToken"] != "" {
		stringData := map[string]string{
			"host":  cpSecretData["ansibleHost"],
			"token": cpSecretData["ansibleToken"],
		}
		createPatchSecret(kubeset, stringData, "toweraccess", clusterName, corev1.SecretTypeOpaque)
	} else {
		log.Println("No Ansible Tower credentials found.")
	}

}

func CreateAWSSecrets(kubeset *kubernetes.Clientset, cpSecretData map[string]string, clusterName string) {

	// Generate the AWS Credential secret
	stringData := map[string]string{
		"aws_access_key_id":     cpSecretData["awsAccessKeyID"],
		"aws_secret_access_key": cpSecretData["awsSecretAccessKeyID"],
	}
	createPatchSecret(kubeset, stringData, clusterName+"-creds", clusterName, corev1.SecretTypeOpaque)

	// Generate Pull Secret
	stringData = map[string]string{
		".dockerconfigjson": cpSecretData["pullSecret"],
	}
	createPatchSecret(kubeset, stringData, clusterName+"-pull-secret", clusterName, corev1.SecretTypeDockerConfigJson)

	// Generate SSH Private Key
	stringData = map[string]string{
		"ssh-privatekey": cpSecretData["sshPrivatekey"],
	}
	createPatchSecret(kubeset, stringData, clusterName+"-ssh-private-key", clusterName, corev1.SecretTypeOpaque)

}

func createPatchSecret(kubeset *kubernetes.Clientset, stringData map[string]string, secretName string, clusterName string, secretType corev1.SecretType) {

	log.Print("Creating secret " + secretName + " in namespace " + clusterName)

	mObj := v1.ObjectMeta{Name: secretName}
	newSecret := &corev1.Secret{StringData: stringData, ObjectMeta: mObj, Type: secretType}
	_, err := kubeset.CoreV1().Secrets(clusterName).Create(context.TODO(), newSecret, v1.CreateOptions{})
	// This is where we patch. To save permissions we use the error instead of a list
	if err != nil && strings.Contains(err.Error(), "already exists") {
		log.Println(" X (already exists)")
		patch := []patchStringValue{{
			Op:    "replace",
			Path:  "/stringData",
			Value: stringData,
		}}
		patchInBytes, _ := json.Marshal(patch)
		log.Print(" > Patching secret " + secretName + " in namespace " + clusterName)
		_, err = kubeset.CoreV1().Secrets(clusterName).Patch(context.TODO(), secretName, types.JSONPatchType, patchInBytes, v1.PatchOptions{})
	}

	utils.CheckError(err)
	log.Println("Applied Secret ✓")
}
