// Copyright Contributors to the Open Cluster Management project.
package launcher

import (
	"context"
	"encoding/json"
	"errors"

	clustercuratorv1 "github.com/open-cluster-management/cluster-curator-controller/pkg/api/v1beta1"
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
const PreAJob = "prehook-ansiblejob"
const PostAJob = "posthook-ansiblejob"
const MonImport = "monitor-import"
const DoneDoneDone = "done"

const ActivateAndMonitor = "activate-and-monitor"
const UpgradeCluster = "upgrade-cluster"
const MonUpgrade = "monitor-upgrade"

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

func getBatchJob(clusterName string, imageURI string, desiredCuration string) *batchv1.Job {

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
	var newJob = &batchv1.Job{}
	switch desiredCuration {
	case "install":
		newJob = &batchv1.Job{
			ObjectMeta: v1.ObjectMeta{
				GenerateName: "curator-job-",
				Namespace:    clusterName,
				Labels: map[string]string{
					"open-cluster-management": "curator-job",
				},
				Annotations: map[string]string{
					PreAJob:            "Running pre-provisioning AnsibleJob",
					ActivateAndMonitor: "Start Provisioning the Cluster and monitor to completion",
					MonImport:          "Monitor the managed cluster until it is imported",
					PostAJob:           "Running post-provisioning AnsibleJob",
					DoneDoneDone:       "Cluster Curator job has completed",
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
								Name:            PreAJob,
								Image:           imageURI,
								Command:         append([]string{CurCmd, PreAJob}),
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
								Name:            MonImport,
								Image:           imageURI,
								Command:         append([]string{CurCmd, MonImport}),
								ImagePullPolicy: corev1.PullAlways,
								Resources:       resourceSettings,
							},
							corev1.Container{
								Name:            PostAJob,
								Image:           imageURI,
								Command:         append([]string{CurCmd, PostAJob}),
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
								Name:    DoneDoneDone,
								Image:   imageURI,
								Command: append([]string{CurCmd, DoneDoneDone}),
							},
						},
					},
				},
			},
		}
	case "upgrade":
		newJob = &batchv1.Job{
			ObjectMeta: v1.ObjectMeta{
				GenerateName: "curator-job-",
				Namespace:    clusterName,
				Labels: map[string]string{
					"open-cluster-management": "curator-job",
				},
				Annotations: map[string]string{
					PreAJob:        "Running pre-provisioning AnsibleJob",
					UpgradeCluster: "Start Upgrading the Cluster and monitor to completion",
					PostAJob:       "Running post-provisioning AnsibleJob",
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
								Name:            PreAJob,
								Image:           imageURI,
								Command:         append([]string{CurCmd, PreAJob}),
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
								Name:            UpgradeCluster,
								Image:           imageURI,
								Command:         append([]string{CurCmd, UpgradeCluster}),
								ImagePullPolicy: corev1.PullAlways,
								Resources:       resourceSettings,
							},
							corev1.Container{
								Name:            MonUpgrade,
								Image:           imageURI,
								Command:         append([]string{CurCmd, MonUpgrade}),
								ImagePullPolicy: corev1.PullAlways,
								Resources:       resourceSettings,
							},
							corev1.Container{
								Name:            PostAJob,
								Image:           imageURI,
								Command:         append([]string{CurCmd, PostAJob}),
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
								Name:    DoneDoneDone,
								Image:   imageURI,
								Command: append([]string{CurCmd, DoneDoneDone}),
							},
						},
					},
				},
			},
		}
	}
	return newJob
}

func (I *Launcher) CreateJob() error {
	kubeset := I.kubeset
	clusterName := I.clusterCurator.Namespace

	newJob := getBatchJob(I.clusterCurator.Name, I.imageURI, I.clusterCurator.Spec.DesiredCuration)

	// Allow us to override the job in the Cluster Curator
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
		// isNewJobCreate := false
		// if I.clusterCurator.Spec.CuratingJob != "" {
		// 	curatorJob, err := kubeset.BatchV1().Jobs(clusterName).Get(context.TODO(), I.clusterCurator.Spec.CuratingJob, v1.GetOptions{})
		// 	if err != nil {
		// 		return err
		// 	}
		// 	if curatorJob.Status.Conditions != nil {
		// 		for _, condition := range curatorJob.Status.Conditions {
		// 			if (condition.Type == "Complete" && condition.Status == "True") || (condition.Type == "Failed" && condition.Status == "True") {
		// 				isNewJobCreate = true
		// 			}
		// 		}
		// 	} else {
		// 		klog.V(0).Infof(" Previous Curator job is still running (%v)", curatorJob.Name)
		// 	}
		// } else {
		// 	isNewJobCreate = true
		// }
		// // Create a new curating job only if previous job Completed or Failed
		// if isNewJobCreate {
		curatorJob, err := kubeset.BatchV1().Jobs(clusterName).Create(context.TODO(), newJob, v1.CreateOptions{})
		if err == nil {
			klog.V(0).Infof(" Created Curator job  ✓ (%v)", curatorJob.Name)
			err = utils.RecordCuratorJobName(I.client, clusterName, curatorJob.Name)
			if err != nil {
				return err
			}
		}
		//}
	}
	if err != nil {
		return err
	}
	return nil
}
