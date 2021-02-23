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
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

func Job(config *rest.Config, clusterConfigOverride *corev1.ConfigMap) {
	jobType := os.Getenv("JOB_TYPE")
	if jobType != "prehook" && jobType != "posthook" {
		klog.Fatal("Missing JOB_TYPE environment parameter, use \"prehook\" or \"posthook\"")
	}

	towerTemplateNames, err := FindAnsibleTemplateNamefromConfigMap(
		clusterConfigOverride,
		jobType)
	utils.CheckError(err)
	for _, ttn := range towerTemplateNames {
		klog.V(3).Info("Tower Job name: " + ttn.Name)
		_, err = RunAnsibleJob(config, clusterConfigOverride.Namespace, jobType, ttn, "toweraccess", nil)
		utils.CheckError(err)
	}
}

func getAnsibleJob(jobtype string) *unstructured.Unstructured {
	ansibleJob := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "tower.ansible.com/v1alpha1",
			"kind":       "AnsibleJob",
			"metadata": map[string]interface{}{
				"name": "",
				"annotations": map[string]string{
					"jobtype": jobtype,
				},
			},
			"spec": map[string]interface{}{
				"extra_vars":        map[string]string{},
				"job_template_name": "",
				"tower_auth_secret": "",
			},
		},
	}
	return ansibleJob
}

/* RunAnsibleJob - Run a basic AnsbileJob kind to trigger an Ansible Teamplte Job playbook
 *  config           # kubeconfig
 *  namespace        # The cluster's namespace
 *  jobtype          # "pre" or "post"
 *  jobTemplateName  #Tower Template job to run
 *  secretRef		 # The secret to connect to Tower in the cluster namespace, ie. toweraccess
 */
func RunAnsibleJob(
	config *rest.Config,
	namespace string,
	jobtype string,
	jobTemplate AnsibleJob,
	secretRef string,
	extraVars map[string]string) (*unstructured.Unstructured, error) {

	klog.V(2).Info("* Run " + jobtype + " AnsibleJob")
	dynclient, err := dynamic.NewForConfig(config)
	utils.CheckError(err)
	ansibleJobName := jobtype + "-job"
	ansibleJobRes := schema.GroupVersionResource{Group: "tower.ansible.com", Version: "v1alpha1", Resource: "ansiblejobs"}
	ansibleJob := getAnsibleJob(jobtype)
	ansibleJob.Object["metadata"].(map[string]interface{})["generateName"] = ansibleJobName + "-"
	ansibleJob.Object["metadata"].(map[string]interface{})["annotations"].(map[string]string)["jobtype"] = jobtype
	if extraVars != nil {
		ansibleJob.Object["spec"].(map[string]interface{})["extra_vars"] = extraVars
	}
	klog.V(4).Info((jobTemplate))
	ansibleJob.Object["spec"].(map[string]interface{})["job_template_name"] = jobTemplate.Name
	ansibleJob.Object["spec"].(map[string]interface{})["tower_auth_secret"] = secretRef
	ansibleJob.Object["spec"].(map[string]interface{})["extra_vars"] = jobTemplate.ExtraVars

	klog.V(0).Info("Creating AnsibleJob " + ansibleJobName + " in namespace " + namespace)
	jobResource, err := dynclient.Resource(ansibleJobRes).Namespace(namespace).
		Create(context.TODO(), ansibleJob, v1.CreateOptions{})

	utils.CheckError(err)
	ansibleJobName = jobResource.GetName()
	klog.V(2).Info("Created AnsibleJob ✓")

	klog.V(0).Info("* Monitoring AnsibleJob " + namespace + "/" + jobResource.GetName())

	// Monitor the AnsibeJob resource
	for {
		jobResource, err = dynclient.Resource(ansibleJobRes).Namespace(namespace).
			Get(context.TODO(), ansibleJobName, v1.GetOptions{})

		klog.V(4).Info(jobResource)
		if jobResource.Object != nil && jobResource.Object["status"] != nil {
			jos := jobResource.Object["status"]
			if jos.(map[string]interface{})["ansibleJobResult"] != nil &&
				jos.(map[string]interface{})["ansibleJobResult"].(map[string]interface{})["status"] != nil {

				jobStatus := jos.(map[string]interface{})["ansibleJobResult"].(map[string]interface{})["status"]
				klog.V(2).Infof("Found result status %v", jobStatus)
				if jobStatus == "successful" {
					klog.V(2).Info("AnsibleJob " + namespace + "/" + ansibleJobName + " finished successfully ✓")
					break
				} else if jobStatus == "error" {
					klog.Warningf("Status: \n\n%v", jobResource.Object["status"])
					return nil, errors.New("AnsibleJob " + namespace + "/" + ansibleJobName + " exited with an error")
				}
			}
			if jobResource.Object["status"].(map[string]interface{})["conditions"] != nil {
				for _, condition := range jobResource.Object["status"].(map[string]interface{})["conditions"].([]interface{}) {
					if condition.(map[string]interface{})["reason"] == "Failed" {
						return nil, errors.New(condition.(map[string]interface{})["message"].(string))
					}
				}
			}
		}
		klog.V(2).Info("AnsibleJob " + namespace + "/" + ansibleJobName + " is still running")
		time.Sleep(5 * time.Second)
	}
	return jobResource, nil
}

type AnsibleJob struct {
	Name      string                 `yaml:"name"`
	ExtraVars map[string]interface{} `yaml:"extra_vars,omitempty"`
}

func FindAnsibleTemplateNamefromConfigMap(cm *corev1.ConfigMap, jobType string) ([]AnsibleJob, error) {
	if cm.Data[jobType] == "" {
		return nil, errors.New("Missing " + jobType + "-towertemplatenames in job ConfigMap " + cm.Name)
	}
	ansibleJobs := &[]AnsibleJob{}
	klog.V(4).Info(ansibleJobs)
	klog.V(4).Info(cm.Data[jobType])
	utils.CheckError(yaml.Unmarshal([]byte(cm.Data[jobType]), &ansibleJobs))
	klog.V(4).Info(ansibleJobs)
	if jobType == "prehook" {
		return *ansibleJobs, nil
	}
	return *ansibleJobs, nil
}
