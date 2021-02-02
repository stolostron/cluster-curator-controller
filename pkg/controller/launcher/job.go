package launcher

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/utils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"
)

func getBatchJob(configMapName string) *batchv1.Job {
	imageTag := os.Getenv("IMAGE_TAG")
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
				"monitor-provisioning": "Provisioning Cluster",
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
							Image:           "quay.io/jpacker/clustercurator-job:" + imageTag,
							Command:         []string{"./curator", "applycloudprovider-ansible"},
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
							Image:           "quay.io/jpacker/clustercurator-job" + imageTag,
							Command:         []string{"./ansiblejob"},
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
							Image:           "quay.io/jpacker/clustercurator-job" + imageTag,
							Command:         []string{"./curator", "monitor"},
							ImagePullPolicy: corev1.PullAlways,
						},
						corev1.Container{
							Name:            "posthook-ansiblejob",
							Image:           "quay.io/jpacker/clustercurator-job" + imageTag,
							Command:         []string{"./ansiblejob"},
							ImagePullPolicy: corev1.PullAlways,
							Env: []corev1.EnvVar{
								corev1.EnvVar{
									Name:  "JOB_TYPE",
									Value: "prehook",
								},
							},
						},
					},
					Containers: []corev1.Container{
						corev1.Container{
							Name:    "complete",
							Image:   "quay.io/jpacker/clustercurator-job" + imageTag,
							Command: []string{"echo", "Done!"},
						},
					},
				},
			},
		},
	}
	return newJob
}

func CreateJob(config *rest.Config, jobConfigMap corev1.ConfigMap) {
	kubeset, err := kubernetes.NewForConfig(config)
	utils.CheckError(err)
	clusterName := jobConfigMap.Namespace
	if jobConfigMap.Data["providerCredentialPath"] == "" {
		log.Println("Missing providerCredentialPath in " + clusterName + "-job ConfigMap")
		return
	}
	newJob := getBatchJob(jobConfigMap.Name)
	// Allow us to override the job in the configMap
	log.Print("Creating Curator job curator-job in namespace " + clusterName)
	if jobConfigMap.Data["overrideJob"] != "" {
		log.Print(" Overriding the Curator job with overrideJob from the " + clusterName + "-job ConfigMap")
		newJob = &batchv1.Job{}
		//hivev1.ClusterDeployment is defined with json for unmarshaling
		jobJSON, err := yaml.YAMLToJSON([]byte(jobConfigMap.Data["overrideJob"]))
		if err == nil {
			err = json.Unmarshal(jobJSON, &newJob)
		}
	}
	if err == nil {
		curatorJob, err := kubeset.BatchV1().Jobs(clusterName).Create(context.TODO(), newJob, v1.CreateOptions{})
		if err == nil {
			log.Print(" Created Curator job  ✓")
			jobConfigMap.Data["curator-job"] = curatorJob.Name
			_, err = kubeset.CoreV1().ConfigMaps(clusterName).Update(context.TODO(), &jobConfigMap, v1.UpdateOptions{})
			if err != nil {
				log.Println(err)
			}
		}
	}
	if err != nil {
		log.Println(err)
	}
}
