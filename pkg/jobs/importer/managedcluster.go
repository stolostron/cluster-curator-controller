// Copyright Contributors to the Open Cluster Management project.

package importer

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	yaml "github.com/ghodss/yaml"
	managedclusterclient "github.com/open-cluster-management/api/client/cluster/clientset/versioned"
	managedclusterv1 "github.com/open-cluster-management/api/cluster/v1"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/utils"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	syaml "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

func Task(config *rest.Config,
	clusterName string,
	clusterConfigTemplate *corev1.ConfigMap,
	clusterConfigOverride *corev1.ConfigMap) {

	klog.V(0).Info("=> Importing Cluster in namespace \"" + clusterName +
		"\" using ConfigMap Template \"" + clusterConfigTemplate.Name + "/" +
		clusterConfigTemplate.Namespace + "\" and ConfigMap Override \"" + clusterName)
	managedclusterclient, err := managedclusterclient.NewForConfig(config)
	utils.CheckError(err)

	dynclient, err := dynamic.NewForConfig(config)
	utils.CheckError(err)
	CreateKlusterletAddonConfig(dynclient, clusterConfigTemplate, clusterConfigOverride)
	CreateManagedCluster(managedclusterclient, clusterConfigTemplate, clusterConfigOverride)
}

func CreateManagedCluster(
	managedclusterset *managedclusterclient.Clientset,
	configMapTemplate *corev1.ConfigMap,
	configMapOverride *corev1.ConfigMap) {

	newCluster := &managedclusterv1.ManagedCluster{}
	//agentset.KlusterletAddonConfig is defined with json for unmarshaling
	cdJSON, err := yaml.YAMLToJSON([]byte(configMapTemplate.Data["managedCluster"]))
	utils.CheckError(err)
	err = json.Unmarshal(cdJSON, &newCluster)
	utils.CheckError(err)

	// Merge overrides
	if configMapOverride != nil {
		utils.OverrideStringField(&newCluster.Name, configMapOverride.Data["clusterName"], "name")
	} else {
		log.Println("No overrides provided")
	}

	log.Print("Creating ManagedCluster " + newCluster.GetName() + " in namespace " + newCluster.GetName())
	_, err = managedclusterset.ClusterV1().ManagedClusters().Create(context.TODO(), newCluster, v1.CreateOptions{})
	utils.CheckError(err)
	log.Println("Created ManagedCluster ✓")
}

func CreateKlusterletAddonConfig(
	dynclient dynamic.Interface,
	configMapTemplate *corev1.ConfigMap,
	configMapOverride *corev1.ConfigMap) {

	kacobj := &unstructured.Unstructured{}
	decoded := syaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	_, gvk, err := decoded.Decode([]byte(configMapTemplate.Data["klusterletAddonConfig"]), nil, kacobj)

	kacRes := schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: strings.ToLower(gvk.Kind) + "s"}

	clusterName := configMapOverride.Data["clusterName"]
	// Merge overrides
	if configMapOverride != nil {
		kacobj.Object["metadata"].(map[string]interface{})["name"] = clusterName
	} else {
		log.Println("No overrides provided")
	}

	log.Print("Creating KlusterletAddonConfig " + clusterName + " in namespace " + clusterName)
	_, err = dynclient.Resource(kacRes).Namespace(clusterName).Create(context.TODO(), kacobj, v1.CreateOptions{})
	utils.CheckError(err)
	log.Println("Created KlusterletAddonConfig ✓")
}
