// Copyright (c) 2020 Red Hat, Inc.
package ansible

import (
	"context"
	"errors"
	"time"

	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/utils"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

func getAnsibleJob() *unstructured.Unstructured {
	ansibleJob := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "tower.ansible.com/v1alpha1",
			"kind":       "AnsibleJob",
			"metadata": map[string]interface{}{
				"name": "",
				"annotations": map[string]string{
					"jobtype": "pre",
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
func RunAnsibleJob(config *rest.Config, namespace string, jobtype string, jobTemplateName string, secretRef string, extraVars map[string]string) (*unstructured.Unstructured, error) {
	klog.V(2).Info("* Run " + jobtype + " AnsibleJob")
	dynclient, err := dynamic.NewForConfig(config)
	utils.CheckError(err)
	ansibleJobName := jobtype + "-job"
	ansibleJobRes := schema.GroupVersionResource{Group: "tower.ansible.com", Version: "v1alpha1", Resource: "ansiblejobs"}
	ansibleJob := getAnsibleJob()
	ansibleJob.Object["metadata"].(map[string]interface{})["name"] = ansibleJobName
	ansibleJob.Object["metadata"].(map[string]interface{})["annotations"].(map[string]string)["jobtype"] = jobtype
	if extraVars != nil {
		ansibleJob.Object["spec"].(map[string]interface{})["extra_vars"] = extraVars
	}
	ansibleJob.Object["spec"].(map[string]interface{})["job_template_name"] = jobTemplateName
	ansibleJob.Object["spec"].(map[string]interface{})["tower_auth_secret"] = secretRef

	klog.V(0).Info("Creating AnsibleJob " + ansibleJobName + " in namespace " + namespace)
	jobResource, err := dynclient.Resource(ansibleJobRes).Namespace(namespace).Create(context.TODO(), ansibleJob, v1.CreateOptions{})
	utils.CheckError(err)
	klog.V(2).Info("Created AnsibleJob ✓")

	klog.V(0).Info("* Monitoring AnsibleJob " + namespace + "/" + jobResource.GetName())
	// Monitor the AnsibeJob resource
	for {
		jobResource, err = dynclient.Resource(ansibleJobRes).Namespace(namespace).Get(context.TODO(), ansibleJobName, v1.GetOptions{})
		if jobResource.Object != nil && jobResource.Object["status"] != nil &&
			jobResource.Object["status"].(map[string]interface{})["ansibleJobResult"] != nil {
			if jobStatus := jobResource.Object["status"].(map[string]interface{})["ansibleJobResult"].(map[string]interface{})["status"]; jobStatus != "" {
				klog.V(2).Info("found result status %v", jobStatus)
				if jobStatus == "successful" {
					klog.V(2).Info("AnsibleJob " + namespace + "/" + ansibleJobName + " finished successfully ✓")
					break
				} else if jobStatus == "error" {
					klog.Warningf("Status: \n\n%v", jobResource.Object["status"])
					return nil, errors.New("AnsibleJob " + namespace + "/" + ansibleJobName + " exited with an error")
				}
			}
		}
		klog.V(2).Info("AnsibleJob " + namespace + "/" + ansibleJobName + " is still running")
		time.Sleep(5 * time.Second)
	}
	return jobResource, nil
}
