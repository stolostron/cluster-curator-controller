// Copyright Contributors to the Open Cluster Management project.
package launcher

import (
	"context"
	"encoding/json"
	"errors"

	clustercuratorv1 "github.com/stolostron/cluster-curator-controller/pkg/api/v1beta1"
	"github.com/stolostron/cluster-curator-controller/pkg/jobs/utils"
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

const DeleteClusterDeployment = "destroy-cluster"
const MonitorDestroy = "monitor-destroy"
const DeleteClusterNamespace = "delete-cluster-namespace"

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

func getBatchJob(clusterName string, imageURI string, curator clustercuratorv1.ClusterCurator) *batchv1.Job {

	var ttlf int32 = 3600

	desiredCuration := curator.Spec.DesiredCuration
	if curator.Operation != nil && curator.Operation.RetryPosthook != "" {
		desiredCuration = curator.Operation.RetryPosthook
	}

	isPrehook := false
	isPosthook := false

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
		if curator.Spec.Install.Prehook != nil {
			isPrehook = true
		}
		if curator.Spec.Install.Posthook != nil {
			isPosthook = true
		}
		newJob = &batchv1.Job{
			ObjectMeta: v1.ObjectMeta{
				GenerateName: "curator-job-",
				Namespace:    clusterName,
				Labels: map[string]string{
					"open-cluster-management": "curator-job",
				},
				Annotations: map[string]string{
					ActivateAndMonitor: "Start Provisioning the Cluster and monitor to completion",
					MonImport:          "Monitor the managed cluster until it is imported",
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
		if curator.Spec.Upgrade.Prehook != nil {
			isPrehook = true
		}
		if curator.Spec.Upgrade.Posthook != nil {
			isPosthook = true
		}
		if curator.Spec.Upgrade.DesiredUpdate == "" {
			isPrehook = false
			isPosthook = false
		}
		newJob = &batchv1.Job{
			ObjectMeta: v1.ObjectMeta{
				GenerateName: "curator-job-",
				Namespace:    clusterName,
				Labels: map[string]string{
					"open-cluster-management": "curator-job",
				},
				Annotations: map[string]string{
					UpgradeCluster: "Start Upgrading the Cluster and monitor to completion",
					MonUpgrade:     "Monitor upgrade status to completion",
					DoneDoneDone:   "Cluster Curator job has completed",
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
	case "destroy":
		if curator.Spec.Destroy.Prehook != nil {
			isPrehook = true
		}
		if curator.Spec.Destroy.Posthook != nil {
			isPosthook = true
		}
		newJob = &batchv1.Job{
			ObjectMeta: v1.ObjectMeta{
				GenerateName: "curator-job-",
				Namespace:    clusterName,
				Labels: map[string]string{
					"open-cluster-management": "curator-job",
				},
				Annotations: map[string]string{
					DeleteClusterDeployment: "Initiates uninstall of cluster",
					MonitorDestroy:          "Monitor uninstall of cluster",
					DoneDoneDone:            "Cluster Curator job has completed",
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
								Name:            DeleteClusterDeployment,
								Image:           imageURI,
								Command:         append([]string{CurCmd, DeleteClusterDeployment}),
								ImagePullPolicy: corev1.PullIfNotPresent,
								Resources:       resourceSettings,
							},
							corev1.Container{
								Name:            MonitorDestroy,
								Image:           imageURI,
								Command:         append([]string{CurCmd, MonitorDestroy}),
								ImagePullPolicy: corev1.PullIfNotPresent,
								Resources:       resourceSettings,
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
	case "installPosthook", "upgradePosthook":
		if (desiredCuration == "installPosthook" && curator.Spec.Install.Posthook != nil) || (desiredCuration == "upgradePosthook" && curator.Spec.Upgrade.Posthook != nil) {
			isPosthook = true
		}
		newJob = &batchv1.Job{
			ObjectMeta: v1.ObjectMeta{
				GenerateName: "curator-job-",
				Namespace:    clusterName,
				Labels: map[string]string{
					"open-cluster-management": "curator-job",
				},
				Annotations: map[string]string{
					PostAJob:     "Retry a posthook job",
					DoneDoneDone: "Cluster Curator job has completed",
				},
			},
			Spec: batchv1.JobSpec{
				BackoffLimit:            new(int32),
				TTLSecondsAfterFinished: &ttlf,
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						ServiceAccountName: "cluster-installer",
						RestartPolicy:      corev1.RestartPolicyNever,
						InitContainers:     []corev1.Container{},
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
	if isPrehook {
		annotations := newJob.GetAnnotations()
		annotations[PreAJob] = "Running pre-" + desiredCuration + " AnsibleJob"
		initContainers := []corev1.Container{
			corev1.Container{
				Name:            PreAJob,
				Image:           imageURI,
				Command:         append([]string{CurCmd, PreAJob}),
				ImagePullPolicy: corev1.PullIfNotPresent,
				Env: []corev1.EnvVar{
					corev1.EnvVar{
						Name:  "JOB_TYPE",
						Value: "prehook",
					},
				},
				Resources: resourceSettings,
			},
		}

		for _, containers := range newJob.Spec.Template.Spec.InitContainers {
			initContainers = append(initContainers, containers)
		}
		newJob.Spec.Template.Spec.InitContainers = initContainers
	}
	if isPosthook {
		annotations := newJob.GetAnnotations()
		annotations[PostAJob] = "Running post-" + desiredCuration + " AnsibleJob"

		newJob.Spec.Template.Spec.InitContainers = append(newJob.Spec.Template.Spec.InitContainers, corev1.Container{
			Name:            PostAJob,
			Image:           imageURI,
			Command:         append([]string{CurCmd, PostAJob}),
			ImagePullPolicy: corev1.PullIfNotPresent,
			Env: []corev1.EnvVar{
				corev1.EnvVar{
					Name:  "JOB_TYPE",
					Value: "posthook",
				},
			},
			Resources: resourceSettings,
		})
	}
	return newJob
}

func (I *Launcher) CreateJob() error {
	kubeset := I.kubeset
	clusterName := I.clusterCurator.Namespace

	newJob := getBatchJob(I.clusterCurator.Name, I.imageURI, I.clusterCurator)

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
