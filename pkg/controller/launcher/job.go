package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

const OverrideJob = "overrideJob"

type JobSettings struct {
	ImageTag      string
	ConfigMapName string
}

func getJobSetings(configMapName string) *JobSettings {
	js := &JobSettings{
		ImageTag:      os.Getenv("IMAGE_TAG"),
		ConfigMapName: configMapName,
	}
	if js.ImageTag == "" {
		js.ImageTag = ":latest"
	} else {
		js.ImageTag = "@" + js.ImageTag
	}
	return js
}

func getBatchJob(js *JobSettings) *batchv1.Job {
	newJob := &batchv1.Job{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: "curator-job-",
			Labels: map[string]string{
				"open-cluster-management": "curator-job",
			},
			Annotations: map[string]string{
				"apply-cloud-provider": "Creating secrets",
				"prehook-ansiblejob":   "Running pre-provisioning Ansible Job",
				"activate-monitor":     "Start Provisioning Cluster and monitor to completion",
				"posthook-ansiblejob":  "Running post-provisioning Ansible Job",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: new(int32),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: "cluster-installer",
					RestartPolicy:      corev1.RestartPolicyNever,
					InitContainers: []corev1.Container{
						corev1.Container{
							Name:            "applycloudprovider-ansible",
							Image:           "quay.io/jpacker/clustercurator-job:" + js.ImageTag,
							Command:         []string{"./curator", "applycloudprovider-ansible"},
							ImagePullPolicy: corev1.PullAlways,
							Env: []corev1.EnvVar{
								corev1.EnvVar{
									Name:  "JOB_CONFIGMAP",
									Value: js.ConfigMapName,
								},
							},
						},
						corev1.Container{
							Name:            "prehook-ansiblejob",
							Image:           "quay.io/jpacker/clustercurator-job" + js.ImageTag,
							Command:         []string{"./curator", "ansiblejob"},
							ImagePullPolicy: corev1.PullAlways,
							Env: []corev1.EnvVar{
								corev1.EnvVar{
									Name:  "JOB_TYPE",
									Value: "prehook",
								},
							},
						},
						corev1.Container{
							Name:            "monitor-provisioning",
							Image:           "quay.io/jpacker/clustercurator-job" + js.ImageTag,
							Command:         []string{"./curator", "activate-monitor"},
							ImagePullPolicy: corev1.PullAlways,
						},
						corev1.Container{
							Name:            "posthook-ansiblejob",
							Image:           "quay.io/jpacker/clustercurator-job" + js.ImageTag,
							Command:         []string{"./curator", "ansiblejob"},
							ImagePullPolicy: corev1.PullAlways,
							Env: []corev1.EnvVar{
								corev1.EnvVar{
									Name:  "JOB_TYPE",
									Value: "posthook",
								},
							},
						},
					},
					Containers: []corev1.Container{
						corev1.Container{
							Name:    "complete",
							Image:   "quay.io/jpacker/clustercurator-job" + js.ImageTag,
							Command: []string{"echo", "Done!"},
						},
					},
				},
			},
		},
	}
	return newJob
}

func CreateJob(config *rest.Config, jobConfigMap corev1.ConfigMap) error {
	kubeset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	clusterName := jobConfigMap.Namespace
	if jobConfigMap.Data["providerCredentialPath"] == "" {
		return errors.New("Missing providerCredentialPath in " + clusterName + "-job ConfigMap")
	}
	newJob := getBatchJob(getJobSetings(jobConfigMap.Name))

	// Allow us to override the job in the configMap
	klog.V(0).Info("Creating Curator job curator-job in namespace " + clusterName)
	if jobConfigMap.Data[OverrideJob] != "" {
		klog.V(0).Info(" Overriding the Curator job with overrideJob from the " + clusterName + "-job ConfigMap")
		newJob = &batchv1.Job{}
		//hivev1.ClusterDeployment is defined with json for unmarshaling
		jobJSON, err := yaml.YAMLToJSON([]byte(jobConfigMap.Data[OverrideJob]))
		if err == nil {
			err = json.Unmarshal(jobJSON, &newJob)
		}
	}
	if err == nil {
		curatorJob, err := kubeset.BatchV1().Jobs(clusterName).Create(context.TODO(), newJob, v1.CreateOptions{})
		if err == nil {
			klog.V(0).Info(" Created Curator job  ✓")
			jobConfigMap.Data["curator-job"] = curatorJob.Name
			_, err = kubeset.CoreV1().ConfigMaps(clusterName).Update(context.TODO(), &jobConfigMap, v1.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	}
	if err != nil {
		return err
	}
	return nil
}
