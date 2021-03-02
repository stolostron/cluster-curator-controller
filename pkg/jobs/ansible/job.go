// Copyright Contributors to the Open Cluster Management project.
package ansible

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/utils"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
)

const PREHOOK = "prehook"
const POSTHOOK = "posthook"

var ansibleJobGVR = schema.GroupVersionResource{
	Group: "tower.ansible.com", Version: "v1alpha1", Resource: "ansiblejobs"}

func Job(dynclient dynamic.Interface, clusterConfigOverride *corev1.ConfigMap) error {
	jobType := os.Getenv("JOB_TYPE")
	if jobType != PREHOOK && jobType != POSTHOOK {
		return errors.New("Missing JOB_TYPE environment parameter, use \"prehook\" or \"posthook\"")
	}

	towerTemplateNames, err := FindAnsibleTemplateNamefromConfigMap(
		clusterConfigOverride,
		jobType)

	if err != nil {
		return err
	}

	for _, ttn := range towerTemplateNames {
		klog.V(3).Info("Tower Job name: " + ttn.Name)
		jobResource, err := RunAnsibleJob(dynclient, clusterConfigOverride, jobType, ttn, "toweraccess")
		if err != nil {
			return err
		}
		klog.V(0).Infof("Monitor AnsibleJob: %v", jobResource.GetName())
		if jobResource.GetName() == "" {
			return errors.New("Name was not generated")
		}

		err = MonitorAnsibleJob(dynclient, jobResource, clusterConfigOverride)
		if err != nil {
			return err
		}
	}
	return err
}

func getAnsibleJob(jobtype string,
	ansibleJobTemplate string,
	secretRef string,
	extraVars map[string]interface{},
	ansibleJobName string,
	clusterName string) *unstructured.Unstructured {

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
				"extra_vars":        extraVars,
			},
		},
	}
	return ansibleJob
}

/* RunAnsibleJob - Run a basic AnsbileJob kind to trigger an Ansible Teamplte Job playbook
 *  config           # kubeconfig
 *  namespace        # The cluster's namespace
 *  jobtype          # "pre" or "post"
 *  jobTemplateName  # Tower Template job to run
 *  secretRef		 # The secret to connect to Tower in the cluster namespace, ie. toweraccess
 */
func RunAnsibleJob(
	dynclient dynamic.Interface,
	clusterConfigOverride *corev1.ConfigMap,
	jobtype string,
	jobTemplate AnsibleJob,
	secretRef string) (*unstructured.Unstructured, error) {

	klog.V(2).Info("* Run " + jobtype + " AnsibleJob")

	namespace := clusterConfigOverride.Namespace
	klog.V(4).Info((jobTemplate))

	ansibleJob := getAnsibleJob(jobtype, jobTemplate.Name, secretRef, jobTemplate.ExtraVars, "", clusterConfigOverride.Namespace)

	klog.V(0).Info("Creating AnsibleJob " + ansibleJob.GetName() + " in namespace " + namespace)
	jobResource, err := dynclient.Resource(ansibleJobGVR).Namespace(namespace).
		Create(context.TODO(), ansibleJob, v1.CreateOptions{})

	if err != nil {
		return nil, err
	}

	klog.V(2).Info("Created AnsibleJob ✓")

	return jobResource, nil
}

func MonitorAnsibleJob(
	dynclient dynamic.Interface,
	jobResource *unstructured.Unstructured,
	clusterConfigOverride *corev1.ConfigMap) error {

	namespace := jobResource.GetNamespace()
	ansibleJobName := jobResource.GetName()
	klog.V(0).Info("* Monitoring AnsibleJob " + namespace + "/" + jobResource.GetName())

	// Monitor the AnsibeJob resource
	for {
		jobResource, err := dynclient.Resource(ansibleJobGVR).Namespace(namespace).
			Get(context.TODO(), ansibleJobName, v1.GetOptions{})

		if err != nil {
			return err
		}

		klog.V(4).Info(jobResource)

		// Track initialization of status
		if jobResource.Object == nil || jobResource.Object["status"] == nil ||
			jobResource.Object["status"].(map[string]interface{})["conditions"] == nil {

			klog.V(2).Infof("AnsibleJob %v/%v is initializing", namespace, ansibleJobName)
			time.Sleep(utils.PauseFiveSeconds)
			continue
		}

		jos := jobResource.Object["status"]
		if jos.(map[string]interface{})["ansibleJobResult"] != nil {

			jobStatus := jos.(map[string]interface{})["ansibleJobResult"].(map[string]interface{})["status"]
			klog.V(2).Infof("Found result status %v", jobStatus)

			if jobStatus == "successful" {

				klog.V(2).Infof("AnsibleJob %v/%v finished successfully ✓", namespace, ansibleJobName)
				break

			} else if jobStatus == "error" {

				klog.Warningf("Status: \n\n%v", jobResource.Object["status"])
				return errors.New("AnsibleJob " + namespace + "/" + ansibleJobName + " exited with an error")
			}
		}

		// Store the job name for the UI to use
		if jos.(map[string]interface{})["k8sJob"] != nil {
			utils.RecordAnsibleJobDyn(
				dynclient,
				clusterConfigOverride,
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

func FindAnsibleTemplateNamefromConfigMap(cm *corev1.ConfigMap, jobType string) ([]AnsibleJob, error) {
	if cm == nil {
		return nil, errors.New("No ConfigMap provided")
	}
	if cm.Data[jobType] == "" {
		return nil, errors.New("Missing " + jobType + " in job ConfigMap " + cm.Name)
	}
	ansibleJobs := &[]AnsibleJob{}
	klog.V(4).Info(cm.Data[jobType])
	utils.CheckError(yaml.Unmarshal([]byte(cm.Data[jobType]), &ansibleJobs))
	klog.V(4).Info(ansibleJobs)
	return *ansibleJobs, nil
}
