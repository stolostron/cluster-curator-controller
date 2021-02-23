// Copyright Contributors to the Open Cluster Management project.
package launcher

import (
	"context"
	"encoding/json"
	"errors"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

const OverrideJob = "overrideJob"
const CurCmd = "./curator"

type Launcher struct {
	client       kubernetes.Interface
	imageTag     string
	imageUri     string
	jobConfigMap corev1.ConfigMap
}

func NewLauncher(client kubernetes.Interface, imageTag string, imageUri string, jobConfigMap corev1.ConfigMap) *Launcher {
	return &Launcher{
		client:       client,
		imageTag:     imageTag,
		imageUri:     imageUri,
		jobConfigMap: jobConfigMap,
	}
}

func getBatchJob(imageTag string, configMapName string, imageUri string) *batchv1.Job {

	var flags = []string{"--v", "2"}

	if imageTag == "" {
		imageTag = ":latest"
	} else {
		imageTag = "@" + imageTag
	}

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
							Image:           imageUri + imageTag,
							Command:         append([]string{CurCmd, "applycloudprovider-ansible"}, flags...),
							ImagePullPolicy: corev1.PullAlways,
							Env: []corev1.EnvVar{
								corev1.EnvVar{
									Name:  "JOB_CONFIGMAP",
									Value: configMapName,
								},
							},
						},
						corev1.Container{
							Name:            "prehook-ansiblejob",
							Image:           imageUri + imageTag,
							Command:         append([]string{CurCmd, "ansiblejob"}, flags...),
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
							Image:           imageUri + imageTag,
							Command:         append([]string{CurCmd, "activate-monitor"}, flags...),
							ImagePullPolicy: corev1.PullAlways,
						},
						corev1.Container{
							Name:            "posthook-ansiblejob",
							Image:           imageUri + imageTag,
							Command:         append([]string{CurCmd, "ansiblejob"}, flags...),
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
							Image:   imageUri + imageTag,
							Command: []string{"echo", "Done!"},
						},
					},
				},
			},
		},
	}
	return newJob
}

func (I *Launcher) CreateJob() error {
	kubeset := I.client
	clusterName := I.jobConfigMap.Namespace
	if I.jobConfigMap.Data["providerCredentialPath"] == "" {
		return errors.New("Missing providerCredentialPath in " + clusterName + "-job ConfigMap")
	}
	newJob := getBatchJob(I.imageTag, I.imageUri, I.jobConfigMap.Name)

	// Allow us to override the job in the configMap
	klog.V(0).Info("Creating Curator job curator-job in namespace " + clusterName)
	var err error
	if I.jobConfigMap.Data[OverrideJob] != "" {
		klog.V(0).Info(" Overriding the Curator job with overrideJob from the " + clusterName + "-job ConfigMap")
		newJob = &batchv1.Job{}
		//hivev1.ClusterDeployment is defined with json for unmarshaling
		jobJSON, err := yaml.YAMLToJSON([]byte(I.jobConfigMap.Data[OverrideJob]))
		if err == nil {
			err = json.Unmarshal(jobJSON, &newJob)
		}
		klog.V(2).Info(" Basic sanity check from the Unmarshal")
		if len(newJob.Spec.Template.Spec.InitContainers) == 0 &&
			len(newJob.Spec.Template.Spec.Containers) == 0 {

			klog.Warning(newJob)
			return errors.New("Did not find any InitContainers or Containers defined")
		}
	}
	if err == nil {
		curatorJob, err := kubeset.BatchV1().Jobs(clusterName).Create(context.TODO(), newJob, v1.CreateOptions{})
		if err == nil {
			klog.V(0).Info(" Created Curator job  ✓")
			I.jobConfigMap.Data["curator-job"] = curatorJob.Name
			_, err = kubeset.CoreV1().ConfigMaps(clusterName).Update(context.TODO(), &I.jobConfigMap, v1.UpdateOptions{})
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
