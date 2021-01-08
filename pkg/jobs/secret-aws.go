// Copyright (c) 2020 Red Hat, Inc.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	"os"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

//  patchStringValue specifies a json patch operation for a string.
type patchStringValue struct {
	Op    string            `json:"op"`
	Path  string            `json:"path"`
	Value map[string]string `json:"value"`
}

// Simple error function
func checkError(err error) {
	if err != nil {
		panic(err.Error())
	}
}

func createAWSSecrets(kubeset *kubernetes.Clientset, cpSecretData map[string]string, clusterName string) {

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

	fmt.Print("Creating secret " + secretName + " in namespace " + clusterName)

	mObj := v1.ObjectMeta{Name: secretName}
	newSecret := &corev1.Secret{StringData: stringData, ObjectMeta: mObj, Type: secretType}
	_, err := kubeset.CoreV1().Secrets(clusterName).Create(context.TODO(), newSecret, v1.CreateOptions{})
	// This is where we patch. To save permissions we use the error instead of a list
	if err != nil && strings.Contains(err.Error(), "already exists") {
		fmt.Println(" X (already exists)")
		patch := []patchStringValue{{
			Op:    "replace",
			Path:  "/stringData",
			Value: stringData,
		}}
		patchInBytes, _ := json.Marshal(patch)
		fmt.Print(" > Patching secret " + secretName + " in namespace " + clusterName)
		_, err = kubeset.CoreV1().Secrets(clusterName).Patch(context.TODO(), secretName, types.JSONPatchType, patchInBytes, v1.PatchOptions{})
	}

	checkError(err)
	fmt.Println(" ✓")
}

func main() {
	var kubeconfig *string

	// Determine kube path for Provider credential
	var pcPath = os.Getenv("PROVIDER_CREDENTIAL_PATH")
	if pcPath == "" {
		panic("Environment variable PROVIDER_CREDENTIAL_PATH missing")
	}
	// Determine new Cluster name
	var clusterName = os.Getenv("CLUSTER_NAME")
	if clusterName == "" {
		panic("Environment variable CLUSTER_NAME is missing")
	}

	values := strings.Split(pcPath, "/")
	if values[0] == "/" {
		panic("NameSpace was not provided NAMESPACE/SECRET_NAME, found: " + pcPath)
	}
	if len(values) != 2 {
		panic("Environment variable PROVIDER_CREDENTIAL_PATH is not in the format NAMESPACE/SECRET_NAME, found: " + pcPath)
	}
	nameSpace := values[0]
	secretName := values[1]
	fmt.Println("Using Provider credential namespace \"" + nameSpace + "\" secret \"" + secretName + "\"")

	// Build a connection to the ACM Hub OCP
	homePath := os.Getenv("HOME")
	kubeconfig = flag.String("kubeconfig", homePath+"/.kube/config", "")
	flag.Parse()

	var config *rest.Config
	var err error
	if _, err := os.Stat(homePath + "/.kube/config"); !os.IsNotExist(err) {
		fmt.Println("Connecting with local kubeconfig")
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	} else {
		fmt.Println("Connecting using In Cluster Config")
		config, err = rest.InClusterConfig()
	}
	checkError(err)

	// Create a client for kubernetes to access events
	// kubeset, err := ...
	kubeset, err := kubernetes.NewForConfig(config)
	checkError(err)
	secret, err := kubeset.CoreV1().Secrets(nameSpace).Get(context.TODO(), secretName, v1.GetOptions{})
	checkError(err)

	secretData := make(map[string]string)
	err = yaml.Unmarshal(secret.Data["metadata"], &secretData)
	checkError(err)
	fmt.Println("Found Cloud Provider secret \"" + secret.GetName() + "\" ✓")

	createAWSSecrets(kubeset, secretData, clusterName)
	fmt.Println("Done!")
}
