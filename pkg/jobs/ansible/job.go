// Copyright Contributors to the Open Cluster Management project.
package ansible

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	clustercuratorv1 "github.com/open-cluster-management/cluster-curator-controller/pkg/api/v1beta1"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/utils"
	hivev1 "github.com/openshift/hive/apis/hive/v1"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const PREHOOK = "prehook"
const POSTHOOK = "posthook"
const MPSUFFIX = "-worker"
const ICSUFFIX = "-install-config"

var ansibleJobGVR = schema.GroupVersionResource{
	Group: "tower.ansible.com", Version: "v1alpha1", Resource: "ansiblejobs"}

func Job(client client.Client, curator *clustercuratorv1.ClusterCurator) error {
	jobType := os.Getenv("JOB_TYPE")
	if jobType != PREHOOK && jobType != POSTHOOK {
		return errors.New("Missing JOB_TYPE environment parameter, use \"prehook\" or \"posthook\"")
	}

	//var hooks clustercuratorv1.Hooks
	var prehook []clustercuratorv1.Hook
	var posthook []clustercuratorv1.Hook
	var towerauthsecret string
	switch curator.Spec.DesiredCuration {
	case "install":
		prehook = curator.Spec.Install.Prehook
		posthook = curator.Spec.Install.Posthook
		towerauthsecret = curator.Spec.Install.TowerAuthSecret
	case "upgrade":
		prehook = curator.Spec.Upgrade.Prehook
		posthook = curator.Spec.Upgrade.Posthook
		towerauthsecret = curator.Spec.Upgrade.TowerAuthSecret
	case "destroy":
		prehook = curator.Spec.Destroy.Prehook
		posthook = curator.Spec.Destroy.Posthook
		towerauthsecret = curator.Spec.Destroy.TowerAuthSecret
		/*	case "scale":
				hooks = curator.Spec.Scale
			case "upgrade":
				hooks = curator.Spec.Upgrade
		*/
	default:
		return errors.New("The Spec.DesiredCuration value is not supported: " + curator.Spec.DesiredCuration)
	}

	// Extract the prehooks or posthooks
	hooksToRun := prehook
	if jobType == POSTHOOK {
		hooksToRun = posthook
	}

	// Move on when clusterCurator is missing or job hook is missing
	if len(hooksToRun) == 0 {
		klog.V(0).Infof("No ansibleJob detected for %v", jobType)
		return nil
	}

	for _, ttn := range hooksToRun {
		klog.V(3).Info("Tower Job name: " + ttn.Name)
		jobResource, err := RunAnsibleJob(client, curator, jobType, ttn, towerauthsecret)
		if err != nil {
			return err
		}

		klog.V(0).Infof("Monitor AnsibleJob: %v", jobResource.GetName())
		if jobResource.GetName() == "" {
			return errors.New("Name was not generated")
		}
		klog.V(4).Infof("AnsibleJob: %v", jobResource)
		err = MonitorAnsibleJob(client, jobResource, curator)
		if err != nil {
			return err
		}
	}

	return nil
}

func getAnsibleJob(jobtype string,
	ansibleJobTemplate string,
	secretRef string,
	extraVars *runtime.RawExtension,
	ansibleJobName string,
	clusterName string) *unstructured.Unstructured {

	/*mapExtraVars := map[string]interface{}{}
	if extraVars != nil {

		klog.V(4).Infof("Converting extra_vars to map: %v",extraVars)
		marshalledExtraVars, err := json.Marshal(extraVars)
		utils.CheckError(err)

		err = json.Unmarshal(marshalledExtraVars, &mapExtraVars)
		utils.CheckError(err)
	}*/

	ansibleJob := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "tower.ansible.com/v1alpha1",
			"kind":       "AnsibleJob",
			"metadata": map[string]interface{}{
				"generateName": jobtype + "job-",
				"name":         ansibleJobName,
				"namespace":    clusterName,
				"annotations": map[string]interface{}{
					"jobtype": jobtype,
				},
			},
			"spec": map[string]interface{}{
				"job_template_name": ansibleJobTemplate,
				"tower_auth_secret": secretRef,
			},
		},
	}

	// This is to translate the runtime.RawExtension to a map[string]interface{}
	mapExtraVars := map[string]interface{}{}

	if extraVars != nil {

		err := json.Unmarshal(extraVars.Raw, &mapExtraVars)
		utils.CheckError(err)
	}

	ansibleJob.Object["spec"].(map[string]interface{})["extra_vars"] = mapExtraVars

	return ansibleJob
}

// Retreive the cluster deployment for use in the extra_vars
func getClusterDeployment(client client.Client, clusterName string) (map[string]interface{}, error) {
	cd := hivev1.ClusterDeployment{}

	if err := client.Get(context.Background(), types.NamespacedName{
		Namespace: clusterName,
		Name:      clusterName,
	}, &cd); err != nil {
		return nil, err
	}

	return runtime.DefaultUnstructuredConverter.ToUnstructured(&cd)
}

// Retreive the Machine Pool for use in the extra_vars
func getMachinePool(client client.Client, clusterName string) (map[string]interface{}, error) {
	mp := hivev1.MachinePool{}

	if err := client.Get(context.Background(), types.NamespacedName{
		Namespace: clusterName,
		Name:      clusterName + MPSUFFIX,
	}, &mp); err != nil {
		return nil, err
	}

	return runtime.DefaultUnstructuredConverter.ToUnstructured(&mp)
}

// Extract the control, compute and networking keys from the install config. This skips sensitive values
func getInstallConfig(client client.Client, clusterName string) (map[string]interface{}, error) {
	ic := corev1.Secret{}

	if err := client.Get(context.Background(), types.NamespacedName{
		Namespace: clusterName,
		Name:      clusterName + ICSUFFIX,
	}, &ic); err != nil {
		return nil, err
	}

	unmarshalled := map[string]interface{}{}
	err := yaml.Unmarshal(ic.Data["install-config.yaml"], &unmarshalled)
	if err != nil {
		return nil, err
	}

	// Instead of deleting keys from unmarshal, copy only the keys we want
	// Use ConvertMap as unmarshal uses map[interface{]}]interface{} instead
	// of map[string]interface{}
	subset := map[string]interface{}{}
	subset["networking"] = utils.ConvertMap(unmarshalled["networking"])
	subset["compute"] = utils.ConvertMap(unmarshalled["compute"])
	subset["controlPlane"] = utils.ConvertMap(unmarshalled["controlPlane"])
	subset["platform"] = utils.ConvertMap(unmarshalled["platform"]).(map[string]interface{})

	klog.V(4).Infof("install-config: %v", subset)

	return subset, nil
}

// Not currently used, represents an OPT-IN approach
func parsePlatform(m interface{}) interface{} {

	newMap := utils.ConvertMap(m).(map[string]interface{})
	ret := map[string]interface{}{}

	for platformType, _ := range newMap {

		klog.V(4).Infof("platformType: %v", platformType)

		if platformType == "vsphere" {
			ret[platformType] = map[string]interface{}{}

			for key, value := range newMap[platformType].(map[string]interface{}) {

				// Makes it easy to read and skip additional keys
				switch key {

				// vmware
				case "vCenter", "datacenter", "defaultDatastore", "cluster", "apiVIP",
					"ingressVIP", "network":

					klog.V(4).Infof("key: value %v: %v", key, value)
					ret[platformType].(map[string]interface{})[key] = value
				}
			}
		} else if platformType == "baremetal" {
			ret[platformType] = map[string]interface{}{}

			for key, value := range newMap[platformType].(map[string]interface{}) {

				// Makes it easy to read and skip additional keys
				switch key {

				// vmware
				case "libvirtURI", "provisioningNetworkCIDR", "provisioningNetworkInterface", "provisioningBridge", "externalBridge",
					"apiVIP", "ingressVIP", "bootstrapOSImage", "clusterOSImage":

					klog.V(4).Infof("key: value %v: %v", key, value)
					ret[platformType].(map[string]interface{})[key] = value
				}
			}
		} else {
			ret[platformType] = newMap[platformType]
		}
	}
	return ret
}

/* RunAnsibleJob - Run a basic AnsbileJob kind to trigger an Ansible Teamplte Job playbook
 *  config           # kubeconfig
 *  namespace        # The cluster's namespace
 *  jobtype          # "pre" or "post"
 *  jobTemplateName  # Tower Template job to run
 *  secretRef		 # The secret to connect to Tower in the cluster namespace, ie. toweraccess
 */
func RunAnsibleJob(
	client client.Client,
	curator *clustercuratorv1.ClusterCurator,
	jobtype string,
	hookToRun clustercuratorv1.Hook,
	secretRef string) (*unstructured.Unstructured, error) {

	klog.V(2).Info("* Run " + jobtype + " AnsibleJob")

	namespace := curator.Namespace
	klog.V(4).Infof("hookToRun: %v", hookToRun)

	ansibleJob := getAnsibleJob(
		jobtype,
		hookToRun.Name,
		secretRef,
		hookToRun.ExtraVars,
		"",
		namespace)

	cd, err := getClusterDeployment(client, namespace)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			klog.Warning("Did not find clusterDeployment")
		} else {
			return nil, err
		}
	} else {
		ansibleJob.Object["spec"].(map[string]interface{})["extra_vars"].(map[string]interface{})["cluster_deployment"] = cd["spec"]
	}

	mp, err := getInstallConfig(client, namespace)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			klog.Warning("Did not find install-config")
		} else {
			return nil, err
		}
	} else {
		ansibleJob.Object["spec"].(map[string]interface{})["extra_vars"].(map[string]interface{})["install_config"] = mp
	}

	klog.V(0).Info("Creating AnsibleJob " + ansibleJob.GetName() + " in namespace " + namespace)
	klog.V(4).Infof("ansibleJob: %v", ansibleJob)
	err = client.Create(context.Background(), ansibleJob)

	if err != nil {
		return nil, err
	}

	klog.V(2).Info("Created AnsibleJob ✓")

	return ansibleJob, nil
}

func MonitorAnsibleJob(
	client client.Client,
	jobResource *unstructured.Unstructured,
	curator *clustercuratorv1.ClusterCurator) error {

	namespace := jobResource.GetNamespace()
	ansibleJobName := jobResource.GetName()
	klog.V(0).Info("* Monitoring AnsibleJob " + namespace + "/" + jobResource.GetName())

	utils.CheckError(utils.RecordCurrentStatusCondition(
		client,
		curator.Namespace,
		"current-ansiblejob",
		v1.ConditionFalse,
		jobResource.GetName()))

	// Monitor the AnsibeJob resource
	foundUrlOnce := false
	for {

		err := client.Get(context.Background(), types.NamespacedName{
			Namespace: namespace,
			Name:      ansibleJobName,
		}, jobResource)

		if err != nil {
			return err
		}

		klog.V(4).Infof("ansibleJob: %v", jobResource)

		// Track initialization of status
		if jobResource.Object == nil || jobResource.Object["status"] == nil ||
			jobResource.Object["status"].(map[string]interface{})["conditions"] == nil {

			klog.V(2).Infof("AnsibleJob %v/%v is initializing", namespace, ansibleJobName)
			time.Sleep(utils.PauseFiveSeconds)
			continue
		}

		jos := jobResource.Object["status"]
		if jos.(map[string]interface{})["ansibleJobResult"] != nil {

			jobStatusUrl := jos.(map[string]interface{})["ansibleJobResult"].(map[string]interface{})["url"]
			klog.V(2).Infof("Found result url %v", jobStatusUrl)

			if !foundUrlOnce && jobStatusUrl != nil {
				utils.CheckError(utils.RecordAnsibleJobStatusUrlCondition(
					client,
					curator.Namespace,
					jobResource.GetName(),
					v1.ConditionTrue,
					jobStatusUrl.(string)))
				foundUrlOnce = true
			}

			jobStatus := jos.(map[string]interface{})["ansibleJobResult"].(map[string]interface{})["status"]
			klog.V(2).Infof("Found result status %v", jobStatus)

			if jobStatus == "successful" {

				klog.V(2).Infof("AnsibleJob %v/%v finished successfully ✓", namespace, ansibleJobName)

				utils.CheckError(utils.RecordCurrentStatusCondition(
					client,
					curator.Namespace,
					"current-ansiblejob",
					v1.ConditionTrue,
					jobResource.GetName()))

				break
			} else if jobStatus == "error" {

				return errors.New("AnsibleJob " + namespace + "/" + ansibleJobName + " exited with an error")
			}
		}

		// This is where you would be able to store the actual KubernetesJob name
		if jos.(map[string]interface{})["k8sJob"] != nil {
			klog.V(2).Infof("Ansible Kube Job: %v",
				jos.(map[string]interface{})["k8sJob"].(map[string]interface{})["namespacedName"].(string))
		}

		for _, condition := range jobResource.Object["status"].(map[string]interface{})["conditions"].([]interface{}) {

			if condition.(map[string]interface{})["reason"] == "Failed" {
				return errors.New(condition.(map[string]interface{})["message"].(string))
			}
		}
		klog.V(2).Infof("AnsibleJob %v/%v is still running", namespace, ansibleJobName)
		time.Sleep(utils.PauseFiveSeconds)
	}
	return nil
}

type AnsibleJob struct {
	Name      string                 `yaml:"name"`
	ExtraVars map[string]interface{} `yaml:"extra_vars,omitempty"`
}

func FindAnsibleTemplateNamefromCurator(
	hooks *clustercuratorv1.Hooks,
	jobType string) ([]clustercuratorv1.Hook, error) {

	if hooks == nil {
		utils.CheckError(errors.New("No Ansible job hooks found"))
	}
	hooksToRun := hooks.Prehook
	if jobType == POSTHOOK {
		hooksToRun = hooks.Posthook
	}

	if len(hooksToRun) == 0 {
		return nil, errors.New("Missing " + jobType + " in curator kind ")
	}
	return hooksToRun, nil
}
