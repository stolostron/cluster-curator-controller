// Copyright Contributors to the Open Cluster Management project.
package launcher

import (
	"context"
	"encoding/json"
	"errors"

	clustercuratorv1 "github.com/open-cluster-management/cluster-curator-controller/pkg/api/v1alpha1"
	"github.com/open-cluster-management/cluster-curator-controller/pkg/jobs/utils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const OverrideJob = "overrideJob"
const CurCmd = "./curator"

type Launcher struct {
	client         client.Client
	kubeset        kubernetes.Interface
	imageURI       string
	clusterCurator clustercuratorv1.ClusterCurator
}

func NewLauncher(
	client client.Client,
	kubeset kubernetes.Interface,
	imageURI string,
	clusterCurator clustercuratorv1.ClusterCurator) *Launcher {

	return &Launcher{
		client:         client,
		kubeset:        kubeset,
		imageURI:       imageURI,
		clusterCurator: clusterCurator,
	}
}

const ActivateAndMonitor = "activate-and-monitor"

func getBatchJob(configMapName string, imageURI string) *batchv1.Job {

	var ttlf int32 = 3600
	var resourceSettings = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("0.3m"),
			corev1.ResourceMemory: resource.MustParse("30Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("2m"),
			corev1.ResourceMemory: resource.MustParse("45Mi"),
		},
	}

	newJob := &batchv1.Job{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: "curator-job-",
			Namespace:    configMapName,
			Labels: map[string]string{
				"open-cluster-management": "curator-job",
			},
			Annotations: map[string]string{
				"apply-cloud-provider": "Creating secrets",
				"prehook-ansiblejob":   "Running pre-provisioning AnsibleJob",
				"ActivateAndMonitor":   "Start Provisioning the Cluster and monitor to completion",
				"monitor-import":       "Monitor the managed cluster until it is imported",
				"posthook-ansiblejob":  "Running post-provisioning AnsibleJob",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            new(int32),
			TTLSecondsAfterFinished: &ttlf,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: "cluster-installer",
					RestartPolicy:      corev1.RestartPolicyNever,
					InitContainers: []corev1.Container{
						corev1.Container{
							Name:            "applycloudprovider-ansible",
							Image:           imageURI,
							Command:         append([]string{CurCmd, "applycloudprovider-ansible"}),
							ImagePullPolicy: corev1.PullAlways,
							Env: []corev1.EnvVar{
								corev1.EnvVar{
									Name:  "JOB_CONFIGMAP",
									Value: configMapName,
								},
							},
							Resources: resourceSettings,
						},
						corev1.Container{
							Name:            "prehook-ansiblejob",
							Image:           imageURI,
							Command:         append([]string{CurCmd, "prehook-ansiblejob"}),
							ImagePullPolicy: corev1.PullAlways,
							Env: []corev1.EnvVar{
								corev1.EnvVar{
									Name:  "JOB_TYPE",
									Value: "prehook",
								},
							},
							Resources: resourceSettings,
						},
						corev1.Container{
							Name:            ActivateAndMonitor,
							Image:           imageURI,
							Command:         append([]string{CurCmd, ActivateAndMonitor}),
							ImagePullPolicy: corev1.PullAlways,
							Resources:       resourceSettings,
						},
						corev1.Container{
							Name:            "monitor-import",
							Image:           imageURI,
							Command:         append([]string{CurCmd, "monitor-import"}),
							ImagePullPolicy: corev1.PullAlways,
							Resources:       resourceSettings,
						},
						corev1.Container{
							Name:            "posthook-ansiblejob",
							Image:           imageURI,
							Command:         append([]string{CurCmd, "posthook-ansiblejob"}),
							ImagePullPolicy: corev1.PullAlways,
							Env: []corev1.EnvVar{
								corev1.EnvVar{
									Name:  "JOB_TYPE",
									Value: "posthook",
								},
							},
							Resources: resourceSettings,
						},
					},
					Containers: []corev1.Container{
						corev1.Container{
							Name:    "complete",
							Image:   imageURI,
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
	kubeset := I.kubeset
	clusterName := I.clusterCurator.Namespace
	if I.clusterCurator.Spec.ProviderCredentialPath == "" {
		return errors.New("Missing providerCredentialPath in " + clusterName + "-job ConfigMap")
	}
	newJob := getBatchJob(I.clusterCurator.Name, I.imageURI)

	// Allow us to override the job in the configMap
	klog.V(0).Info("Creating Curator job curator-job in namespace " + clusterName)
	var err error
	if I.clusterCurator.Spec.Install.OverrideJob != nil {
		klog.V(0).Info(" Overriding the Curator job with overrideJob from the " + clusterName + " ClusterCurator resource")
		newJob = &batchv1.Job{}

		err = json.Unmarshal(I.clusterCurator.Spec.Install.OverrideJob.Raw, &newJob)
		if err != nil {
			klog.Warningf("overrideJob:\n---\n%v---", string(I.clusterCurator.Spec.Install.OverrideJob.Raw))
			return err
		}

		klog.V(2).Info(" Basic sanity check for override job")
		if len(newJob.Spec.Template.Spec.InitContainers) == 0 &&
			len(newJob.Spec.Template.Spec.Containers) == 0 {

			klog.Warning(newJob)
			return errors.New("Did not find any InitContainers or Containers defined")
		}
	}
	if err == nil {
		curatorJob, err := kubeset.BatchV1().Jobs(clusterName).Create(context.TODO(), newJob, v1.CreateOptions{})
		if err == nil {
			klog.V(0).Infof(" Created Curator job  ✓ (%v)", curatorJob.Name)
			err = utils.RecordCuratorJobName(I.client, clusterName, curatorJob.Name)
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
