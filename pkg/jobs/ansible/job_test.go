// Copyright Contributors to the Open Cluster Management project.
package ansible

import (
	"context"
	"os"
	"testing"
	"time"

	ajv1 "github.com/open-cluster-management/ansiblejob-go-lib/api/v1alpha1"
	hivev1 "github.com/openshift/hive/apis/hive/v1"
	clustercuratorv1 "github.com/stolostron/cluster-curator-controller/pkg/api/v1beta1"
	"github.com/stolostron/cluster-curator-controller/pkg/jobs/utils"
	managedclusterinfov1beta1 "github.com/stolostron/cluster-lifecycle-api/clusterinfo/v1beta1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"open-cluster-management.io/api/client/cluster/clientset/versioned/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const EnvJobType = "JOB_TYPE"
const ClusterName = "my-cluster"
const AnsibleJobName = "my-ansiblejob-12345"
const SecretRef = "toweraccess"
const AnsibleJobTemplateName = "Ansible Tower Template to run as a job"

var ansibleJob = getAnsibleJob(PREHOOK, AnsibleJobTemplateName, SecretRef, nil, AnsibleJobName, ClusterName)
var s = scheme.Scheme

func getClusterCurator() *clustercuratorv1.ClusterCurator {
	return &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "install",
			Install: clustercuratorv1.Hooks{
				Prehook: []clustercuratorv1.Hook{
					clustercuratorv1.Hook{
						Name: "Service now App Update",
						ExtraVars: &runtime.RawExtension{
							Raw: []byte(`{"variable1": "1","variable2": "2"}`),
						},
					},
				},
				Posthook: []clustercuratorv1.Hook{
					clustercuratorv1.Hook{
						Name: "Service now App Update",
						ExtraVars: &runtime.RawExtension{
							Raw: []byte(`{"variable1": "3","variable2": "4"}`),
						},
					},
				},
			},
		},
	}
}

func getClusterCuratorEmpty() *clustercuratorv1.ClusterCurator {
	return &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "install",
		},
	}
}

func genClusterDeployment() *hivev1.ClusterDeployment {
	return &hivev1.ClusterDeployment{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
	}
}

func genMachinePool() *hivev1.MachinePool {
	return &hivev1.MachinePool{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName + MPSUFFIX,
			Namespace: ClusterName,
		},
	}
}

// The platform section is a mixture of three, this is just for testing pruposes
func genInstallConfigSecret() *corev1.Secret {
	installConfigString := `
apiVersion: v1
metadata:
  name: my-cluster
baseDomain: my.domain.com
controlPlane:
    hyperthreading: Enabled
    name: control-plane
    platform:
        aws:
            rootVolume:
                iops: 100
                size: 100
                type: gp2
            type: m5.xlarge
        replicas: 3
compute:
- hyperthreading: Enabled
  name: worker
  platform:
      aws:
          rootVolume:
              iops: 100
              size: 100
              type: gp2
          type: m5.xlarge
      replicas: 3
networking:
    clusterNetwork:
    - cidr: 10.128.0.0/14
    hostPrefix: 23
    machineCIDR: 10.0.0.0/16
    networkType: OVNKubernetes
    serviceNetwork:
    - 172.30.0.0/16
platform:
    aws:
        region: us-east-1
    openstack:
        externalNetwork: my-network
        lbFloatingIP: 2.2.2.2
        ingressFloatingIP: 2.2.2.3
        cloud: openstack
    vsphere:
        vCenter: "https://my-vcenter/"
        username: DO_NOT_SHOW
        password: SHOULD_NOT_SEE
        datacenter: my-datacenter
        defaultDatastore: my-datastore
        cluster: my-cluster
        apiVIP: 0.0.0.0
        ingressVIP: 1.1.1.1
        network: my-network
    baremetal:
        libvirtURI: qemu+ssh://root@hypervisor.ocp4-edge-bm-h15-0.qe.lab.redhat.com/system
        provisioningNetworkCIDR: 1.1.1.1./20
        provisioningNetworkInterface: enp1s0
        provisioningBridge: provisioning
        externalBridge: baremetal
        apiVIP:
        ingressVIP:
        bootstrapOSImage: >-
          http://registry.ocp4-edge-bm-h15-0.qe.lab.redhat.com:8080/images/rhcos-46.82.202011260640-0-qemu.x86_64.qcow2.gz?sha256=99928ff40c2d8e3aa358d9bd453102e3d1b5e9694fb5d54febc56e275f35da51
        clusterOSImage: >-
          http://registry.ocp4-edge-bm-h15-0.qe.lab.redhat.com:8080/images/rhcos-46.82.202011260640-0-openstack.x86_64.qcow2.gz?sha256=2bd648e09f086973accd8ac1e355ce0fcd7dfcc16bc9708c938801fcf10e219e
        hosts:
          - name: 'bm4'
            namespace: 'default'
            role: management
            bmc:
              address: 'example.com:80'
              disableCertificateVerification: true
              username: # injected by server
              password: # injected by server
            bootMACAddress: 00:90:7F:12:DE:7A
            hardwareProfile: default
          - name: 'bm5'
            namespace: 'default'
            role: management
            bmc:
              address: 'example.com:80'
              disableCertificateVerification: true
              username: # injected by server
              password: # injected by server
            bootMACAddress: 00:90:7F:12:DE:7B
            hardwareProfile: default
          - name: 'bm6'
            namespace: 'default'
            role: management
            bmc:
              address: 'example.com:80'
              disableCertificateVerification: true
              username: # injected by server
              password: # injected by server
            bootMACAddress: 00:90:7F:12:DE:7C
            hardwareProfile: default
pullSecret: \"\" # skip, hive will inject based on it's secrets
sshKey: |-
    secret_value`

	return &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName + "-install-config",
			Namespace: ClusterName,
		},
		Data: map[string][]byte{
			"install-config.yaml": []byte(installConfigString),
		},
	}
}

func buildAnsibleJob(ajs string, k8sJob string) *ajv1.AnsibleJob {
	return &ajv1.AnsibleJob{
		TypeMeta: v1.TypeMeta{
			Kind:       "AnsibleJob",
			APIVersion: "tower.ansible.com/v1alpha1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      AnsibleJobName,
			Namespace: ClusterName,
			Annotations: map[string]string{
				utils.CurrentAnsibleJob: PREHOOK,
			},
		},
		Spec: ajv1.AnsibleJobSpec{
			TowerAuthSecretName: SecretRef,
			JobTemplateName:     AnsibleJobTemplateName,
		},
		Status: ajv1.AnsibleJobStatus{
			AnsibleJobResult: ajv1.AnsibleJobResult{
				Status: ajs,
			},
			K8sJob: ajv1.K8sJob{
				NamespacedName: k8sJob,
			},
			Conditions: []ajv1.Condition{
				ajv1.Condition{
					Reason:  "Failed",
					Message: "The job failed from condition",
				},
			},
		},
	}
}

func TestJobNoEnvVar(t *testing.T) {

	os.Setenv("JOB_TYPE", "")

	t.Log("No JOB_TYPE variable configured")

	assert.NotNil(t, Job(nil, nil), "err not nil, when no os.env JOB_TYPE")
}

func TestJobInvalidDesiredCuration(t *testing.T) {

	cc := getClusterCurator()

	os.Setenv("JOB_TYPE", PREHOOK)

	cc.Spec.DesiredCuration = "INVALID CHOICE"

	assert.NotNil(t, Job(nil, cc), "err not nil, DesiredCuration value is not VALID")
}

func TestJobNoClusterCurator(t *testing.T) {

	// We should never get in this situation, but if it happens then send a panic
	t.Log(PREHOOK)
	os.Setenv(EnvJobType, PREHOOK)
	assert.Panics(t, func() { Job(nil, nil) }, "Panics when no ClusterCurator is present")

	t.Log(POSTHOOK)
	os.Setenv(EnvJobType, POSTHOOK)
	assert.Panics(t, func() { Job(nil, nil) }, "Panics when no ClusterCurator is present")
}

func TestJobNoClusterCuratorData(t *testing.T) {

	// If prehook or posthook is not defined in the ClusterCurator skip
	t.Logf("Test %v", PREHOOK)
	os.Setenv(EnvJobType, PREHOOK)
	assert.Nil(t, Job(nil, getClusterCuratorEmpty()), "err nil, when no Ansible posthooks")

	t.Logf("Test %v", POSTHOOK)
	assert.Nil(t, Job(nil, getClusterCuratorEmpty()), "err nil, when no Ansible prehooks")
}

func TestFindAnsibleTemplateNamefromClusterCurator(t *testing.T) {

	cc := getClusterCurator()

	t.Logf("Test %v", PREHOOK)
	ansibleTemplates, _ := FindAnsibleTemplateNamefromCurator(&cc.Spec.Install, PREHOOK)
	t.Log(ansibleTemplates)
	assert.NotEmpty(t, ansibleTemplates, "Not empty if AnsibleJobs found")

	t.Logf("Test %v", POSTHOOK)
	ansibleTemplates, _ = FindAnsibleTemplateNamefromCurator(&cc.Spec.Install, POSTHOOK)
	t.Log(ansibleTemplates)
	assert.NotEmpty(t, ansibleTemplates, "Not empty if AnsibleJobs found")
}

func TestJob(t *testing.T) {

	cc := getClusterCurator()

	t.Logf("Test %v", PREHOOK)
	os.Setenv(EnvJobType, PREHOOK)

	s.AddKnownTypes(ajv1.SchemeBuilder.GroupVersion, &ajv1.AnsibleJob{})
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(hivev1.SchemeBuilder.GroupVersion, &hivev1.ClusterDeployment{}, &hivev1.MachinePool{})
	s.AddKnownTypes(corev1.SchemeGroupVersion, &corev1.Secret{})
	client := clientfake.NewFakeClientWithScheme(s, getClusterCurator(), genClusterDeployment(), genInstallConfigSecret(), genMachinePool())
	t.Logf("client has been initialized")

	go func() {
		time.Sleep(utils.PauseFiveSeconds)

		curator := &clustercuratorv1.ClusterCurator{}

		_ = client.Get(
			context.Background(),
			types.NamespacedName{Namespace: ClusterName, Name: ClusterName},
			curator)

		jobName := curator.Status.Conditions[0].Message
		t.Logf("clusterCurator job: %v", jobName)

		newJob := buildAnsibleJob("successful", AnsibleJobTemplateName)

		newJob.SetName(jobName)
		newJob.SetNamespace(ClusterName)

		// This is not the best way to simulate the job being completed.
		assert.Nil(t,
			client.Delete(context.Background(), newJob),
			"err is nil, when ansibleJob resource is deleted")
		assert.Nil(t,
			client.Create(context.Background(), newJob),
			"err is nil, when ansibleJob resource is created")
		t.Logf("AnsibleJob %v marked successful", jobName)
	}()
	err := Job(client, cc)

	assert.Nil(t, err,
		"err nil, when Ansible Job created and monitored with AnsibleJobStatus successful")
}

func TestJobPosthook(t *testing.T) {

	cc := getClusterCurator()

	t.Logf("Test %v", POSTHOOK)
	os.Setenv(EnvJobType, POSTHOOK)

	s.AddKnownTypes(ajv1.SchemeBuilder.GroupVersion, &ajv1.AnsibleJob{})
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(hivev1.SchemeBuilder.GroupVersion, &hivev1.ClusterDeployment{}, &hivev1.MachinePool{})
	client := clientfake.NewFakeClientWithScheme(s, getClusterCurator(), genClusterDeployment(), genMachinePool())

	// buildAnsibleJob("successful", AnsibleJobTemplateName),
	go func() {
		time.Sleep(utils.PauseFiveSeconds)

		curator := &clustercuratorv1.ClusterCurator{}

		_ = client.Get(
			context.Background(),
			types.NamespacedName{Namespace: ClusterName, Name: ClusterName},
			curator)

		jobName := curator.Status.Conditions[0].Message
		t.Logf("clusterCurator job: %v", jobName)

		newJob := buildAnsibleJob("successful", AnsibleJobTemplateName)

		newJob.SetName(jobName)
		newJob.SetNamespace(ClusterName)

		// This is not the best way to simulate the job being completed.
		assert.Nil(t,
			client.Delete(context.Background(), newJob),
			"err is nil, when ansibleJob resource is deleted")
		assert.Nil(t,
			client.Create(context.Background(), newJob),
			"err is nil, when ansibleJob resource is created")
		t.Logf("AnsibleJob %v marked successful", jobName)
	}()
	err := Job(client, cc)

	assert.Nil(t, err,
		"err nil, when Ansible Job created and monitored with AnsibleJobStatus successful")
}
func TestMonitorAnsibleJobAnsibleJobStatusSuccessfulPreHook(t *testing.T) {

	cc := getClusterCurator()
	aj := buildAnsibleJob("successful", AnsibleJobTemplateName)

	t.Logf("Test %v", PREHOOK)
	os.Setenv(EnvJobType, PREHOOK)

	s.AddKnownTypes(ajv1.SchemeBuilder.GroupVersion, &ajv1.AnsibleJob{})
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewFakeClientWithScheme(s, aj, cc)

	mapAJ, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(&aj)
	unstructAJ := &unstructured.Unstructured{Object: mapAJ}

	assert.Nil(t, MonitorAnsibleJob(
		client,
		unstructAJ,
		cc), "err nil, when successful")
}

func TestMonitorAnsibleJobAnsibleJobStatusError(t *testing.T) {

	cc := getClusterCurator()
	aj := buildAnsibleJob("error", "")

	t.Logf("Test %v", PREHOOK)
	os.Setenv(EnvJobType, PREHOOK)

	s.AddKnownTypes(ajv1.SchemeBuilder.GroupVersion, &ajv1.AnsibleJob{})
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewFakeClientWithScheme(s, aj, cc)

	mapAJ, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(&aj)
	unstructAJ := &unstructured.Unstructured{Object: mapAJ}

	assert.NotNil(t, MonitorAnsibleJob(client, unstructAJ, cc), "err nil, when successful")
}

func TestMonitorAnsibleJobK8sJob(t *testing.T) {

	cc := getClusterCurator()
	aj := buildAnsibleJob("na", ClusterName+"/"+AnsibleJobName)

	t.Logf("Test %v", POSTHOOK)
	os.Setenv(EnvJobType, POSTHOOK)

	s.AddKnownTypes(ajv1.SchemeBuilder.GroupVersion, &ajv1.AnsibleJob{})
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewFakeClientWithScheme(s, aj, cc)

	mapAJ, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(&aj)
	unstructAJ := &unstructured.Unstructured{Object: mapAJ}

	assert.NotNil(t, MonitorAnsibleJob(client, unstructAJ, cc), "err not nil, when condition.reason = Failed")

	// Todo: Come back and figure out why ClusterCurator is not returning from dynamic fake.
	curator := &clustercuratorv1.ClusterCurator{}
	assert.Nil(
		t,
		client.Get(context.Background(), types.NamespacedName{Namespace: ClusterName, Name: ClusterName}, curator),
		"err is nil, clusterCurator is retrieved")

	assert.Equal(
		t,
		AnsibleJobName, curator.Status.Conditions[0].Message,
		"ClusterCurator Ansible Job object name correct")
}

func TestRunAnsibleJob(t *testing.T) {

	cc := getClusterCurator()

	t.Logf("Test %v", POSTHOOK)
	os.Setenv(EnvJobType, POSTHOOK)

	s.AddKnownTypes(ajv1.SchemeBuilder.GroupVersion, &ajv1.AnsibleJob{})
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(hivev1.SchemeBuilder.GroupVersion, &hivev1.ClusterDeployment{}, &hivev1.MachinePool{})
	s.AddKnownTypes(corev1.SchemeGroupVersion, &corev1.Secret{})
	client := clientfake.NewFakeClientWithScheme(s, cc, genClusterDeployment(), genMachinePool(), genInstallConfigSecret())

	aJob, err := RunAnsibleJob(client, cc, POSTHOOK, cc.Spec.Install.Posthook[0], "toweraccess")
	assert.Nil(t, err, "err is nil when job is started")
	t.Logf("Fake ansibleJob launched with name: %v", aJob.GetName())
}

func TestAnsibleJobExtraVars(t *testing.T) {

	cc := getClusterCurator()

	t.Logf("Test %v", POSTHOOK)
	os.Setenv(EnvJobType, POSTHOOK)

	s.AddKnownTypes(ajv1.SchemeBuilder.GroupVersion, &ajv1.AnsibleJob{})
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(hivev1.SchemeBuilder.GroupVersion, &hivev1.ClusterDeployment{}, &hivev1.MachinePool{})
	s.AddKnownTypes(corev1.SchemeGroupVersion, &corev1.Secret{})
	client := clientfake.NewFakeClientWithScheme(s, cc, genClusterDeployment(), genMachinePool(), genInstallConfigSecret())

	aJob, err := RunAnsibleJob(client, cc, POSTHOOK, cc.Spec.Install.Posthook[0], "toweraccess")
	assert.Nil(t, err, "err is nil when job is started")
	t.Logf("Fake ansibleJob launched with name: %v", aJob.GetName())

	extraVars := aJob.Object["spec"].(map[string]interface{})["extra_vars"].(map[string]interface{})
	assert.NotNil(t, extraVars, "not nil, extra_vars")
	t.Logf("extraVars: %v", extraVars)

	// Test all the expected keys
	assert.NotNil(t, extraVars["cluster_deployment"], "nil when cluster_deployment missing")
	assert.NotNil(t, extraVars["install_config"], "nil when install_config missing")

	// Look at the platform values
	platform := extraVars["install_config"].(map[string]interface{})["platform"].(map[string]interface{})

	assert.NotNil(t, platform["openstack"], "nil if vsphere is missing")
	assert.NotNil(t, platform["vsphere"], "nil if openstack is missing")
	assert.NotNil(t, platform["aws"], "nil if aws is missing")

	// openstack
	openstack := platform["openstack"].(map[string]interface{})
	assert.NotNil(t, openstack["externalNetwork"], "nil if missing externalNetwork")
	assert.NotNil(t, openstack["lbFloatingIP"], "nil if missing lbFloatingIP")
	assert.NotNil(t, openstack["ingressFloatingIP"], "nil if missing ingressFloatingIP")
	assert.NotNil(t, openstack["cloud"], "nil if missing cloud")

	// vsphere
	vsphere := platform["vsphere"].(map[string]interface{})
	assert.NotNil(t, vsphere["vCenter"], "nil if missing vCenter")
	assert.NotNil(t, vsphere["datacenter"], "nil if missing datacenter")
	assert.NotNil(t, vsphere["defaultDatastore"], "nil if missing defaultDatastore")
	assert.NotNil(t, vsphere["cluster"], "nil if missing cluster")
	assert.NotNil(t, vsphere["apiVIP"], "nil if missing apiVIP")
	assert.NotNil(t, vsphere["ingressVIP"], "nil if missing ingressVIP")
	assert.NotNil(t, vsphere["network"], "nil if missing network")
	assert.Nil(t, vsphere["password"], "should be nil as we don't want to include this")
	assert.Nil(t, vsphere["username"], "should be nil as we don't want to include this")

	// basemetal
	baremetal := platform["baremetal"].(map[string]interface{})
	assert.Nil(t, baremetal["hosts"].([]interface{})[0].(map[string]interface{})["bmc"].(map[string]interface{})["username"], "should be nil as we don't want to include this")
	assert.Nil(t, baremetal["hosts"].([]interface{})[0].(map[string]interface{})["bmc"].(map[string]interface{})["password"], "should be nil as we don't want to include this")
	assert.NotNil(t, baremetal["hosts"].([]interface{})[0].(map[string]interface{})["bmc"].(map[string]interface{})["address"], "should not be nil as we don't want to include this")
	assert.Nil(t, baremetal["hosts"].([]interface{})[1].(map[string]interface{})["bmc"].(map[string]interface{})["username"], "should be nil as we don't want to include this")
	assert.Nil(t, baremetal["hosts"].([]interface{})[1].(map[string]interface{})["bmc"].(map[string]interface{})["password"], "should be nil as we don't want to include this")
	assert.NotNil(t, baremetal["hosts"].([]interface{})[1].(map[string]interface{})["bmc"].(map[string]interface{})["address"], "should not be nil as we don't want to include this")
	assert.Nil(t, baremetal["hosts"].([]interface{})[2].(map[string]interface{})["bmc"].(map[string]interface{})["username"], "should be nil as we don't want to include this")
	assert.Nil(t, baremetal["hosts"].([]interface{})[2].(map[string]interface{})["bmc"].(map[string]interface{})["password"], "should be nil as we don't want to include this")
	assert.NotNil(t, baremetal["hosts"].([]interface{})[2].(map[string]interface{})["bmc"].(map[string]interface{})["address"], "should not be nil as we don't want to include this")

}

/*
func TestMonitorAnsibleRetryForLoop(t *testing.T) {

	cc := getClusterCurator()

	t.Logf("Test %v", POSTHOOK)
	os.Setenv(EnvJobType, POSTHOOK)

	s.AddKnownTypes(ajv1.SchemeBuilder.GroupVersion, &ajv1.AnsibleJob{})

	aj := buildAnsibleJob("na", AnsibleJobName)
	aj.Status.Conditions[0].Reason = "unknown"

	s.AddKnownTypes(ajv1.SchemeBuilder.GroupVersion, &ajv1.AnsibleJob{}, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewFakeClientWithScheme(s, aj, cc)

	mapAJ, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(&aj)
	unstructAJ := &unstructured.Unstructured{Object: mapAJ}

	assert.Panics(t, func() { MonitorAnsibleJob(client, unstructAJ, cc) }, "Panics when For loop times out")
}*/

/*
func TestMonitorAnsibleRetryForLoopJobResourceObjectNil(t *testing.T) {

	s.AddKnownTypes(ajv1.SchemeBuilder.GroupVersion, &ajv1.AnsibleJob{})
	aj := &ajv1.AnsibleJob{
		TypeMeta: v1.TypeMeta{
			Kind:       "AnsibleJob",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      AnsibleJobName,
			Namespace: ClusterName,
		},
		Status: ajv1.AnsibleJobStatus{
			Conditions: []ajv1.Condition{},
		},
	}
	dynclient := dynfake.NewSimpleDynamicClient(s, aj)

	assert.Panics(t, func() { MonitorAnsibleJob(dynclient, ansibleJob, nil) }, "Panics when For loop times out, no condition status")
}*/

func getUpgradeClusterCurator() *clustercuratorv1.ClusterCurator {
	return &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "upgrade",
			Upgrade: clustercuratorv1.UpgradeHooks{
				Prehook: []clustercuratorv1.Hook{
					clustercuratorv1.Hook{
						Name: "Service now App Update",
						ExtraVars: &runtime.RawExtension{
							Raw: []byte(`{"variable1": "1","variable2": "2"}`),
						},
					},
				},
				Posthook: []clustercuratorv1.Hook{
					clustercuratorv1.Hook{
						Name: "Service now App Update",
						ExtraVars: &runtime.RawExtension{
							Raw: []byte(`{"variable1": "3","variable2": "4"}`),
						},
					},
				},
			},
		},
	}
}

func genManagedClusterInfo() *managedclusterinfov1beta1.ManagedClusterInfo {
	return &managedclusterinfov1beta1.ManagedClusterInfo{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Spec: managedclusterinfov1beta1.ClusterInfoSpec{
			LoggingCA:      nil,
			MasterEndpoint: "https://managedclusterapiserver:6443",
		},
		Status: managedclusterinfov1beta1.ClusterInfoStatus{
			Conditions:  nil,
			Version:     "1.19.1",
			KubeVendor:  "Openshift",
			CloudVendor: "AWS",
			ClusterID:   "abc",
			DistributionInfo: managedclusterinfov1beta1.DistributionInfo{
				Type: "OCP",
				OCP: managedclusterinfov1beta1.OCPDistributionInfo{
					Version:                 "OCP4.1",
					AvailableUpdates:        nil,
					DesiredVersion:          "",
					UpgradeFailed:           false,
					Channel:                 "stable",
					Desired:                 managedclusterinfov1beta1.OCPVersionRelease{},
					VersionAvailableUpdates: nil,
					VersionHistory:          nil,
					ManagedClusterClientConfig: managedclusterinfov1beta1.ClientConfig{
						URL:      "https://managedclusterapiserver:6443",
						CABundle: []byte("abc"),
					},
				},
			},
			ConsoleURL:      "https://console.managedclusterapiserver:6443",
			NodeList:        nil,
			LoggingEndpoint: corev1.EndpointAddress{},
			LoggingPort:     corev1.EndpointPort{},
		},
	}
}

func TestUpgradeAnsibleJobExtraVars(t *testing.T) {
	tests := []struct {
		name                   string
		curation               string
		clusterCurator         *clustercuratorv1.ClusterCurator
		managedClusterInfo     *managedclusterinfov1beta1.ManagedClusterInfo
		expectedClusterInfoVar bool
	}{
		{
			name:                   "upgrade has clusterInfo",
			curation:               "upgrade",
			clusterCurator:         getUpgradeClusterCurator(),
			managedClusterInfo:     genManagedClusterInfo(),
			expectedClusterInfoVar: true,
		},
		{
			name:                   "upgrade has no clusterInfo",
			curation:               "upgrade",
			clusterCurator:         getUpgradeClusterCurator(),
			managedClusterInfo:     nil,
			expectedClusterInfoVar: false,
		},
		{
			name:                   "install has clusterInfo",
			curation:               "install",
			clusterCurator:         getClusterCurator(),
			managedClusterInfo:     genManagedClusterInfo(),
			expectedClusterInfoVar: false,
		},
	}

	os.Setenv(EnvJobType, POSTHOOK)

	s.AddKnownTypes(ajv1.SchemeBuilder.GroupVersion, &ajv1.AnsibleJob{})
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(hivev1.SchemeBuilder.GroupVersion, &hivev1.ClusterDeployment{}, &hivev1.MachinePool{})
	s.AddKnownTypes(corev1.SchemeGroupVersion, &corev1.Secret{})
	s.AddKnownTypes(managedclusterinfov1beta1.SchemeGroupVersion, &managedclusterinfov1beta1.ManagedClusterInfo{})
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var fakeClient client.WithWatch
			if test.managedClusterInfo != nil {
				fakeClient = clientfake.NewFakeClientWithScheme(s, test.clusterCurator, genClusterDeployment(), genMachinePool(), genInstallConfigSecret(), test.managedClusterInfo)
			} else {
				fakeClient = clientfake.NewFakeClientWithScheme(s, test.clusterCurator, genClusterDeployment(), genMachinePool(), genInstallConfigSecret())
			}

			var hook clustercuratorv1.Hook
			switch test.curation {
			case "install":
				hook = test.clusterCurator.Spec.Install.Posthook[0]
			case "upgrade":
				hook = test.clusterCurator.Spec.Upgrade.Posthook[0]
			}
			aJob, err := RunAnsibleJob(fakeClient, test.clusterCurator, POSTHOOK, hook, "toweraccess")
			assert.Nil(t, err, "err is nil when job is started")

			extraVars := aJob.Object["spec"].(map[string]interface{})["extra_vars"].(map[string]interface{})
			assert.NotNil(t, extraVars, "not nil, extra_vars")
			t.Logf("extraVars: %v", extraVars)

			_, existed := extraVars["cluster_info"]
			assert.Equal(t, existed, test.expectedClusterInfoVar)

			if test.expectedClusterInfoVar {
				// clusterinfo
				assert.NotNil(t, extraVars["cluster_info"], "nil when cluster_info missing")
				assert.NotNil(t, extraVars["cluster_info"].(map[string]interface{})["distributionInfo"], "should not be nil as we want to include this")
				assert.Nil(t, extraVars["cluster_info"].(map[string]interface{})["distributionInfo"].(map[string]interface{})["managedClusterClientConfig"], "should be nil as we don't want to include this")
				assert.NotNil(t, extraVars["cluster_info"].(map[string]interface{})["clusterName"], "should not be nil as we want to include this")
				assert.NotNil(t, extraVars["cluster_info"].(map[string]interface{})["apiServer"], "should not be nil as we want to include this")
				assert.NotNil(t, extraVars["cluster_info"].(map[string]interface{})["kubeVendor"], "should not be nil as we want to include this")
				assert.NotNil(t, extraVars["cluster_info"].(map[string]interface{})["cloudVendor"], "should not be nil as we want to include this")
				assert.NotNil(t, extraVars["cluster_info"].(map[string]interface{})["clusterID"], "should not be nil as we want to include this")
			}

		})
	}
}

func TestInventory(t *testing.T) {
	tests := []struct {
		name                 string
		inventory            string
		expectedInventory    bool
		expectedInventoryVar string
	}{
		{
			name:                 "configure inventory",
			inventory:            ClusterName,
			expectedInventory:    true,
			expectedInventoryVar: ClusterName,
		},
		{
			name:              "configure no inventory",
			inventory:         "",
			expectedInventory: false,
		},
	}

	os.Setenv(EnvJobType, POSTHOOK)

	s.AddKnownTypes(ajv1.SchemeBuilder.GroupVersion, &ajv1.AnsibleJob{})
	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	s.AddKnownTypes(hivev1.SchemeBuilder.GroupVersion, &hivev1.ClusterDeployment{}, &hivev1.MachinePool{})
	s.AddKnownTypes(corev1.SchemeGroupVersion, &corev1.Secret{})

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cc := getClusterCurator()
			if test.inventory != "" {
				cc.Spec.Inventory = test.inventory
			}

			client := clientfake.NewFakeClientWithScheme(s, cc, genClusterDeployment(), genMachinePool(), genInstallConfigSecret())
			aJob, err := RunAnsibleJob(client, cc, POSTHOOK, cc.Spec.Install.Posthook[0], "toweraccess")
			assert.Nil(t, err, "err is nil when job is started")

			extraVars := aJob.Object["spec"].(map[string]interface{})["extra_vars"].(map[string]interface{})
			assert.NotNil(t, extraVars, "not nil, extra_vars")

			_, existed := extraVars["inventory"]
			assert.Equal(t, existed, test.expectedInventory)
			if test.expectedInventory {
				assert.Equal(t, extraVars["inventory"], test.expectedInventoryVar)
			}
		})
	}
}
