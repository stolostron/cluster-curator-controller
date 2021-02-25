// Copyright Contributors to the Open Cluster Management project.
package secrets

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// We want to be able to dynamically build a Cloud Provider
const cpName = "my-cloudprovider"
const cpNamespace = "default"
const cpPath = cpNamespace + "/" + cpName
const AwsKeyValue = "fakekey9876543210asdfghjkl"
const AwsKeySecretValue = "fakesecretmnbvcxzlkjhgfdsapooiuyt"
const HostURL = "https://tower.my-domain.com"

func getCPMap() map[string]string {
	return map[string]string{
		"pullSecret": "{\"auths\":{\"cloud.openshift.com\":{\"auth\":\"abc1234\",\"email\":\"test@redhat.com\"}," +
			"\"quay.io\":{\"auth\":\"abc1234\",\"email\":\"test@redhat.com\"},\"registry.connect.redhat.com\"" +
			":{\"auth\":\"abc1234\",\"email\":\"test@redhat.com\"}}}",

		"sshPublickey": "ssh-rsa abc4321  your_email@example.com",

		"SshPrivatekey": "-----BEGIN RSA PRIVATE KEY-----\n" +
			"my certificate\n" +
			"-----END RSA PRIVATE KEY-----",
		"baseDomain": "my-domain.com",
	}
}

func initKubesetWithCP(providerStanza string) *fake.Clientset {
	return fake.NewSimpleClientset(&corev1.Secret{
		TypeMeta: v1.TypeMeta{
			Kind:       "secret",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      cpName,
			Namespace: cpNamespace,
		},
		StringData: map[string]string{
			"metadata": providerStanza,
		},
	})
}

// Test pulling in the Cloud Provider secret
func TestGetSecretData(t *testing.T) {

	t.Log("Create Cloud Provider secret")
	cpMap := getCPMap()
	cpMap["awsAccessKeyID"] = AwsKeyValue
	cpMap["awsSecretAccessKeyID"] = AwsKeyValue
	myMap, _ := yaml.Marshal(cpMap)
	kubeset := initKubesetWithCP(string(myMap))

	t.Log("Read Cloud Provider secret")
	awsSecret := GetSecretData(kubeset, "default/my-cloudprovider")
	assert.NotNil(t, awsSecret, "Cloud Provider secret not nil")

	t.Log("Test Panic on invalid namespace/secret path")
	assert.Panics(t, func() { GetSecretData(kubeset, cpPath+"1") }, "Panic on invalid secret path")

	t.Log("Test Panic on empty namespace/secret path")
	assert.Panics(t, func() { GetSecretData(kubeset, "") }, "Panic on empty secret path")
}

// Create a Secret from a Cloud Provider secret
func TestCreateAnsibleSecret(t *testing.T) {

	t.Log("Create Cloud Provider secret") // Cloud Provider credentials not actually used
	cpMap := getCPMap()
	cpMap["awsAccessKeyID"] = AwsKeyValue
	cpMap["awsSecretAccessKeyID"] = AwsKeyValue
	cpMap["ansibleHost"] = HostURL
	cpMap["ansibleToken"] = AwsKeySecretValue // Reuse existing value
	kubeset := fake.NewSimpleClientset()

	assert.Nil(t, CreateAnsibleSecret(kubeset, cpMap, cpNamespace), "Continue when error nil")

	t.Log("Check that the Ansible Secret was created")

	ansibleSecret, err := kubeset.CoreV1().Secrets(cpNamespace).Get(context.TODO(), AnsibleSecretName, v1.GetOptions{})
	assert.Nil(t, err, "err not nil, for GET Ansible secret")

	if string(ansibleSecret.StringData["host"]) != HostURL ||
		string(ansibleSecret.StringData["token"]) != AwsKeySecretValue {

		t.Fatalf("Ansible secret is not well formed.\n%v", ansibleSecret)
	}

	t.Log("Test that we patch the secret")

	cpMap["ansibleToken"] = AwsKeyValue
	assert.Nil(t, CreateAnsibleSecret(kubeset, cpMap, cpNamespace), "err should be nil when Ansible secret patched")

	ansibleSecret, err = kubeset.CoreV1().Secrets(cpNamespace).Get(context.TODO(), AnsibleSecretName, v1.GetOptions{})
	assert.Nil(t, err, "err not nil, for GET Ansible secret")
	assert.NotNil(t, ansibleSecret, "ansibleSecret not nil when patching")

	if string(ansibleSecret.StringData["host"]) != HostURL ||
		string(ansibleSecret.StringData["token"]) != AwsKeyValue {

		t.Fatalf("Ansible secret is not well formed.\n%v", ansibleSecret)
	}
}

func TestMissingAnsibleCredentials(t *testing.T) {

	t.Log("Create Cloud Provider secret") // Cloud Provider credentials not actually used
	cpMap := getCPMap()
	cpMap["awsAccessKeyID"] = AwsKeyValue
	cpMap["awsSecretAccessKeyID"] = AwsKeyValue

	// Reset the fake kubeset
	kubeset := fake.NewSimpleClientset()
	assert.Nil(t, CreateAnsibleSecret(kubeset, cpMap, cpNamespace), "err nil when Ansible secret nothing to create")

	ansibleSecret, err := kubeset.CoreV1().Secrets(cpNamespace).Get(context.TODO(), AnsibleSecretName, v1.GetOptions{})
	assert.NotNil(t, err, "err is not nil, for GET Ansible secret when none present")
	assert.Nil(t, ansibleSecret, "ansibleSecret is nil, when no Ansible parameters")
}

// These functions are just for development, but we will test that they create secrets
func TestCreateAzureSecret(t *testing.T) {

	cpMap := getCPMap()
	cpMap["clientId"] = AwsKeyValue
	cpMap["clientSecret"] = AwsKeySecretValue
	cpMap["tenantId"] = AwsKeySecretValue
	cpMap["subscriptionId"] = AwsKeySecretValue

	t.Log("Create Cloud Provider Map")
	kubeset := fake.NewSimpleClientset(
		&corev1.Namespace{
			TypeMeta: v1.TypeMeta{
				Kind:       "namespace",
				APIVersion: "v1",
			},
			ObjectMeta: v1.ObjectMeta{
				Name: cpName,
			},
		})

	assert.Nil(t, CreateAzureSecrets(kubeset, cpMap, cpName), "err is nil, when Azure secrets created")

	// Check all 4 secrets are found
	t.Log("Verify exists Azure credential secret")
	secret, _ := kubeset.CoreV1().Secrets(cpName).Get(context.TODO(), cpName+suffixCreds, v1.GetOptions{})
	assert.NotNil(t, secret, "Credential secret is not nil, as it was created for Azure")

	t.Log("Verify exists Azure Pull secret")
	secret, _ = kubeset.CoreV1().Secrets(cpName).Get(context.TODO(), cpName+suffixPull, v1.GetOptions{})
	assert.NotNil(t, secret, "Pull secret is not nil, as it was created for Azure")

	t.Log("Verify exists Azure ssh-private-key secret")
	secret, _ = kubeset.CoreV1().Secrets(cpName).Get(context.TODO(), cpName+suffixPull, v1.GetOptions{})
	assert.NotNil(t, secret, "ssh-private-key secret is not nil, as it was created for Azure")
}

func TestCreateGCPSecret(t *testing.T) {

	cpMap := getCPMap()
	cpMap["gcServiceAccountKey"] = AwsKeyValue

	t.Log("Create Cloud Provider Map")
	kubeset := fake.NewSimpleClientset(
		&corev1.Namespace{
			TypeMeta: v1.TypeMeta{
				Kind:       "namespace",
				APIVersion: "v1",
			},
			ObjectMeta: v1.ObjectMeta{
				Name: cpName,
			},
		})

	assert.Nil(t, CreateGCPSecrets(kubeset, cpMap, cpName), "err is nil, when GCP secrets created")

	// Check all 4 secrets are found
	t.Log("Verify exists GCP credential secret")
	secret, _ := kubeset.CoreV1().Secrets(cpName).Get(context.TODO(), cpName+suffixCreds, v1.GetOptions{})
	assert.NotNil(t, secret, "Credential secret is not nil, as it was created for GCP")

	t.Log("Verify exists GCP Pull secret")
	secret, _ = kubeset.CoreV1().Secrets(cpName).Get(context.TODO(), cpName+suffixPull, v1.GetOptions{})
	assert.NotNil(t, secret, "Pull secret is not nil, as it was created for GCP")

	t.Log("Verify exists GCP ssh-private-key secret")
	secret, _ = kubeset.CoreV1().Secrets(cpName).Get(context.TODO(), cpName+suffixPull, v1.GetOptions{})
	assert.NotNil(t, secret, "ssh-private-key secret is not nil, as it was created for Azure")
}

func TestCreateAWSSecret(t *testing.T) {

	cpMap := getCPMap()
	cpMap["awsAccessKeyID"] = AwsKeyValue
	cpMap["awsSecretAccessKeyID"] = AwsKeySecretValue

	t.Log("Create Cloud Provider Map")
	kubeset := fake.NewSimpleClientset(
		&corev1.Namespace{
			TypeMeta: v1.TypeMeta{
				Kind:       "namespace",
				APIVersion: "v1",
			},
			ObjectMeta: v1.ObjectMeta{
				Name: cpName,
			},
		})

	assert.Nil(t, CreateAWSSecrets(kubeset, cpMap, cpName), "err is nil, when AWS secrets created")

	// Check all 4 secrets are found
	t.Log("Verify exists AWS credential secret")
	secret, _ := kubeset.CoreV1().Secrets(cpName).Get(context.TODO(), cpName+suffixCreds, v1.GetOptions{})
	assert.NotNil(t, secret, "Credential secret is not nil, as it was created for AWS")

	t.Log("Verify exists AWS Pull secret")
	secret, _ = kubeset.CoreV1().Secrets(cpName).Get(context.TODO(), cpName+suffixPull, v1.GetOptions{})
	assert.NotNil(t, secret, "Pull secret is not nil, as it was created for AWS")

	t.Log("Verify exists AWS ssh-private-key secret")
	secret, _ = kubeset.CoreV1().Secrets(cpName).Get(context.TODO(), cpName+suffixPull, v1.GetOptions{})
	assert.NotNil(t, secret, "ssh-private-key secret is not nil, as it was created for Azure")
}
