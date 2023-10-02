// Copyright Contributors to the Open Cluster Management project.
package utils

import (
	"context"
	"errors"
	"testing"

	clustercuratorv1 "github.com/stolostron/cluster-curator-controller/pkg/api/v1beta1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	dynfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCheckErrorNil(t *testing.T) {

	InitKlog(4)
	assert.NotPanics(t, func() { CheckError(nil) }, "No panic, when err is not present")
}

func TestCheckErrorNotNil(t *testing.T) {

	assert.Panics(t, func() { CheckError(errors.New("TeST")) }, "Panics when a err is received")
}

func TestLogErrorNil(t *testing.T) {

	assert.Nil(t, LogError(nil), "err nil, when no err message")
}

func TestLogErrorNotNil(t *testing.T) {

	assert.NotNil(t, LogError(errors.New("TeST")), "err nil, when no err message")
}

// TODO, replace all instances of klog.Warning that include an IF, this saves us 2x lines of code
func TestLogWarning(t *testing.T) {

	assert.NotPanics(t, func() { LogWarning(nil) }, "No panic, when logging warnings")
	assert.NotPanics(t, func() { LogWarning(errors.New("TeST")) }, "No panic, when logging warnings")
}

func TestPathSplitterFromEnv(t *testing.T) {

	_, _, err := PathSplitterFromEnv("")
	assert.NotNil(t, err, "err not nil, when empty path")

	_, _, err = PathSplitterFromEnv("value")
	assert.NotNil(t, err, "err not nil, when only one value")

	_, _, err = PathSplitterFromEnv("value/")
	assert.NotNil(t, err, "err not nil, when only one value with split present")

	_, _, err = PathSplitterFromEnv("/value")
	assert.NotNil(t, err, "err not nil, when only one value with split present")

	namespace, secretName, err := PathSplitterFromEnv("ns1/s1")

	assert.Nil(t, err, "err nil, when path is split successfully")
	assert.Equal(t, namespace, "ns1", "namespace should be ns1")
	assert.Equal(t, secretName, "s1", "secret name should be s1")

}

const ClusterName = "my-cluster"
const PREHOOK = "prehook"
const jobName = "my-jobname-12345"

func getClusterCurator() *clustercuratorv1.ClusterCurator {
	return &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{},
	}
}

func TestRecordCurrentCuratorJob(t *testing.T) {

	cc := getClusterCurator()

	s := scheme.Scheme
	s.AddKnownTypes(CCGVR.GroupVersion(), &clustercuratorv1.ClusterCurator{})

	dynclient := dynfake.NewSimpleDynamicClient(s, cc)

	assert.NotEqual(t, jobName, cc.Spec.CuratingJob, "Nost equal because curating job name not written yet")
	assert.NotPanics(t, func() {
		patchDyn(dynclient, ClusterName, jobName, CurrentCuratorJob)
	}, "no panics, when update successful")

	ccMap, err := dynclient.Resource(CCGVR).Namespace(ClusterName).Get(context.TODO(), ClusterName, v1.GetOptions{})
	assert.Nil(t, err, "err is nil, when my-cluster clusterCurator resource is retrieved")

	assert.Equal(t,
		jobName, ccMap.Object["spec"].(map[string]interface{})[CurrentCuratorJob],
		"Equal when curator job recorded")
}

func TestRecordCurrentCuratorJobError(t *testing.T) {

	s := scheme.Scheme

	dynclient := dynfake.NewSimpleDynamicClient(s)

	assert.NotPanics(t, func() {

		err := patchDyn(dynclient, ClusterName, jobName, CurrentCuratorJob)
		assert.NotNil(t, err, "err is not nil, when patch fails")

	}, "no panics, when update successful")

}

func TestRecordCuratorJob(t *testing.T) {

	err := RecordCuratorJob(ClusterName, jobName)
	assert.NotNil(t, err, "err is not nil, when failure occurs")
	t.Logf("err:\n%v", err)
}

func TestGetDynset(t *testing.T) {
	_, err := GetDynset(nil)
	assert.Nil(t, err, "err is nil, when dynset is initialized")
}

func TestGetClient(t *testing.T) {
	_, err := GetClient()
	assert.Nil(t, err, "err is nil, when client is initialized")
}

func TestGetKubeset(t *testing.T) {
	_, err := GetKubeset()
	assert.Nil(t, err, "err is nil, when kubset is initialized")
}

func TestRecordCuratedStatusCondition(t *testing.T) {

	cc := getClusterCurator()

	s := scheme.Scheme
	s.AddKnownTypes(CCGVR.GroupVersion(), &clustercuratorv1.ClusterCurator{})

	client := clientfake.NewFakeClientWithScheme(s, cc)

	assert.Nil(t,
		recordCuratedStatusCondition(
			client,
			ClusterName,
			ClusterName,
			CurrentAnsibleJob,
			v1.ConditionTrue,
			JobHasFinished,
			"Almost finished"),
		"err is nil, when conditon successfully set")

	ccNew := &clustercuratorv1.ClusterCurator{}
	assert.Nil(t,
		client.Get(context.Background(), types.NamespacedName{Namespace: ClusterName, Name: ClusterName}, ccNew),
		"err is nil, when ClusterCurator resource is retreived")
	t.Log(ccNew)
	assert.Equal(t,
		ccNew.Status.Conditions[0].Type, CurrentAnsibleJob,
		"equal when status condition correctly recorded")

}

func TestRecordCurrentStatusConditionNoResource(t *testing.T) {

	s := scheme.Scheme
	s.AddKnownTypes(CCGVR.GroupVersion(), &clustercuratorv1.ClusterCurator{})

	client := clientfake.NewFakeClientWithScheme(s)

	err := RecordCurrentStatusCondition(
		client,
		ClusterName,
		ClusterName,
		CurrentAnsibleJob,
		v1.ConditionTrue,
		"Almost finished")
	assert.NotNil(t, err, "err is not nil, when conditon can not be written")
	t.Logf("err: %v", err)
}

func TestGetClusterCurator(t *testing.T) {

	cc := getClusterCurator()

	s := scheme.Scheme
	s.AddKnownTypes(CCGVR.GroupVersion(), &clustercuratorv1.ClusterCurator{})

	client := clientfake.NewFakeClientWithScheme(s, cc)

	_, err := GetClusterCurator(client, ClusterName, ClusterName)
	assert.Nil(t, err, "err is nil, when ClusterCurator resource is retrieved")
}

func TestGetClusterCuratorNoResource(t *testing.T) {

	s := scheme.Scheme
	s.AddKnownTypes(CCGVR.GroupVersion(), &clustercuratorv1.ClusterCurator{})

	client := clientfake.NewFakeClientWithScheme(s)

	cc, err := GetClusterCurator(client, ClusterName, ClusterName)
	assert.Nil(t, cc, "cc is nil, when ClusterCurator resource is not found")
	assert.NotNil(t, err, "err is not nil, when ClusterCurator is not found")
	t.Logf("err: %v", err)
}

func TestRecordCuratorJobName(t *testing.T) {

	cc := getClusterCurator()

	s := scheme.Scheme
	s.AddKnownTypes(CCGVR.GroupVersion(), &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewFakeClientWithScheme(s, cc)

	err := RecordCuratorJobName(client, ClusterName, ClusterName, "my-job-ABCDE")

	assert.Nil(t, err, "err nil, when Job name written to ClusterCurator.Spec.curatorJob")

}

func TestRecordCuratorJobNameInvalidCurator(t *testing.T) {

	s := scheme.Scheme
	s.AddKnownTypes(CCGVR.GroupVersion(), &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewFakeClientWithScheme(s)

	err := RecordCuratorJobName(client, ClusterName, ClusterName, "my-job-ABCDE")

	assert.NotNil(t, err, "err nil, when Job name written to ClusterCurator.Spec.curatorJob")
	t.Logf("Detected Errror: %v", err.Error())

}

func getClusterNamespace() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Name: ClusterName,
		},
	}
}

func TestDeleteNamespaceNoPods(t *testing.T) {

	kubeset := fake.NewSimpleClientset(getClusterNamespace())

	assert.Nil(t, DeleteClusterNamespace(kubeset, ClusterName))

	_, err := kubeset.CoreV1().Namespaces().Get(context.Background(), ClusterName, v1.GetOptions{})
	assert.Contains(t, err.Error(), " not found")

}

func TestDeleteNamespaceNoNamespace(t *testing.T) {

	kubeset := fake.NewSimpleClientset()

	assert.Contains(t, DeleteClusterNamespace(kubeset, ClusterName).Error(), " not found")
}

func getPod(podName string, podPhase corev1.PodPhase) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:      podName,
			Namespace: ClusterName,
		},
		Status: corev1.PodStatus{
			Phase: podPhase,
		},
	}
}

func TestDeleteNamespacePodsRunning(t *testing.T) {

	kubeset := fake.NewSimpleClientset(
		getClusterNamespace(),
		getPod("pod1", corev1.PodRunning),
		getPod(ClusterName+"-uninstall", corev1.PodRunning),
		getPod("pod2", corev1.PodSucceeded))

	assert.NotNil(t, DeleteClusterNamespace(kubeset, ClusterName), "not nil, when namespace can not be deleted")

	_, err := kubeset.CoreV1().Namespaces().Get(context.Background(), ClusterName, v1.GetOptions{})
	assert.Nil(t, err, "nil when namespace was found")
}

func TestDeleteNamespacePodsSuceeded(t *testing.T) {

	kubeset := fake.NewSimpleClientset(
		getClusterNamespace(),
		getPod("pod1", corev1.PodSucceeded),
		getPod(ClusterName+"-uninstall", corev1.PodRunning),
		getPod("pod2", corev1.PodSucceeded))

	assert.Nil(t, DeleteClusterNamespace(kubeset, ClusterName), "nil, when namespace can be deleted")

	_, err := kubeset.CoreV1().Namespaces().Get(context.Background(), ClusterName, v1.GetOptions{})
	assert.Contains(t, err.Error(), " not found")
}

func TestGetRetryTimes(t *testing.T) {
	retryTimes := GetRetryTimes(5, 5, PauseTwoSeconds)
	assert.Equal(t, 150, retryTimes)

	retryTimes = GetRetryTimes(0, 5, PauseTwoSeconds)
	assert.Equal(t, 150, retryTimes)

	retryTimes = GetRetryTimes(120, 120, PauseSixtySeconds)
	assert.Equal(t, 120, retryTimes)
}

func TestParseVersionInfo(t *testing.T) {
	cases := []struct {
		name             string
		msg              string
		expectedVersion  string
		expectedChannel  string
		expectedUpstream string
		expectedErr      bool
	}{
		{
			name:            "done msg",
			msg:             "curator-job-xxxx DesiredCuration: upgrade Version (4.11.4;;)",
			expectedVersion: "4.11.4",
			expectedErr:     false,
		},
		{
			name:             "done msg - all",
			msg:              "curator-job-xxxx DesiredCuration: upgrade Version (4.11.4;stable-4.10;upstream)",
			expectedVersion:  "4.11.4",
			expectedChannel:  "stable-4.10",
			expectedUpstream: "upstream",
			expectedErr:      false,
		},
		{
			name:        "broken msg",
			msg:         "curator-job-xxxx DesiredCuration: upgrade Version )",
			expectedErr: true,
		},
		{
			name:        "broken msg",
			msg:         "curator-job-xxxx DesiredCuration: upgrade Version (",
			expectedErr: true,
		},
		{
			name:        "broken msg",
			msg:         "curator-job-xxxx DesiredCuration: upgrade Version ()",
			expectedErr: true,
		},
	}

	for _, c := range cases {
		actualChannel, actualUpstream, actualVersion, err := parseVersionInfo(c.msg)
		if err != nil && !c.expectedErr {
			t.Errorf("unexpected error %v", err)
		}

		if c.expectedErr {
			continue
		}

		if actualChannel != c.expectedChannel {
			t.Errorf("expected %s, but %s", c.expectedChannel, actualChannel)
		}

		if actualUpstream != c.expectedUpstream {
			t.Errorf("expected %s, but %s", c.expectedUpstream, actualUpstream)
		}

		if actualVersion.String() != c.expectedVersion {
			t.Errorf("expected %s, but %s", c.expectedVersion, actualVersion)
		}
	}
}

func TestNeedToUpgrade(t *testing.T) {
	cases := []struct {
		name            string
		curator         clustercuratorv1.ClusterCurator
		expectedUpgrade bool
		expectedErr     bool
	}{
		{
			name:            "job is not started",
			curator:         clustercuratorv1.ClusterCurator{},
			expectedUpgrade: true,
			expectedErr:     false,
		},
		{
			name: "job is running",
			curator: clustercuratorv1.ClusterCurator{
				Status: clustercuratorv1.ClusterCuratorStatus{
					Conditions: []v1.Condition{
						{
							Status: v1.ConditionFalse,
							Type:   "clustercurator-job",
						},
					},
				},
			},
			expectedUpgrade: false,
			expectedErr:     false,
		},
		{
			name: "non-upgrade job",
			curator: clustercuratorv1.ClusterCurator{
				Status: clustercuratorv1.ClusterCuratorStatus{
					Conditions: []v1.Condition{
						{
							Message: "curator-job-xxxx DesiredCuration: install",
							Status:  v1.ConditionTrue,
							Type:    "clustercurator-job",
						},
					},
				},
			},
			expectedUpgrade: true,
			expectedErr:     false,
		},
		{
			name: "failed job - desired version is unchanged",
			curator: clustercuratorv1.ClusterCurator{
				Spec: clustercuratorv1.ClusterCuratorSpec{
					Upgrade: clustercuratorv1.UpgradeHooks{
						DesiredUpdate: "4.11.4",
					},
				},
				Status: clustercuratorv1.ClusterCuratorStatus{
					Conditions: []v1.Condition{
						{
							Message: "curator-job-xxxx DesiredCuration: upgrade Version (4.11.4;;) Failed - error",
							Status:  v1.ConditionTrue,
							Type:    "clustercurator-job",
						},
					},
				},
			},
			expectedUpgrade: false,
			expectedErr:     false,
		},
		{
			name: "failed job - desired version is unchanged in old version",
			curator: clustercuratorv1.ClusterCurator{
				Spec: clustercuratorv1.ClusterCuratorSpec{
					Upgrade: clustercuratorv1.UpgradeHooks{
						DesiredUpdate: "4.11.4",
					},
				},
				Status: clustercuratorv1.ClusterCuratorStatus{
					Conditions: []v1.Condition{
						{
							Message: "curator-job-xxxx DesiredCuration: upgrade Version (4.11.4) Failed - error",
							Status:  v1.ConditionTrue,
							Type:    "clustercurator-job",
						},
					},
				},
			},
			expectedUpgrade: false,
			expectedErr:     false,
		},
		{
			name: "failed job - desired version is changed",
			curator: clustercuratorv1.ClusterCurator{
				Spec: clustercuratorv1.ClusterCuratorSpec{
					Upgrade: clustercuratorv1.UpgradeHooks{
						DesiredUpdate: "4.11.4",
					},
				},
				Status: clustercuratorv1.ClusterCuratorStatus{
					Conditions: []v1.Condition{
						{
							Message: "curator-job-xxxx DesiredCuration: upgrade Version (4.11.3;;) Failed - error",
							Status:  v1.ConditionTrue,
							Type:    "clustercurator-job",
						},
					},
				},
			},
			expectedUpgrade: true,
			expectedErr:     false,
		},
		{
			name: "invalid desired version",
			curator: clustercuratorv1.ClusterCurator{
				Spec: clustercuratorv1.ClusterCuratorSpec{
					Upgrade: clustercuratorv1.UpgradeHooks{
						DesiredUpdate: "invalid",
					},
				},
				Status: clustercuratorv1.ClusterCuratorStatus{
					Conditions: []v1.Condition{
						{
							Message: "curator-job-xxxx DesiredCuration: upgrade Version (4.11.3)",
							Status:  v1.ConditionTrue,
							Type:    "clustercurator-job",
						},
					},
				},
			},
			expectedUpgrade: false,
			expectedErr:     true,
		},
		{
			name: "invalid version in msg",
			curator: clustercuratorv1.ClusterCurator{
				Spec: clustercuratorv1.ClusterCuratorSpec{
					Upgrade: clustercuratorv1.UpgradeHooks{
						DesiredUpdate: "4.11.4",
					},
				},
				Status: clustercuratorv1.ClusterCuratorStatus{
					Conditions: []v1.Condition{
						{
							Message: "curator-job-xxxx DesiredCuration: upgrade Version (invalid)",
							Status:  v1.ConditionTrue,
							Type:    "clustercurator-job",
						},
					},
				},
			},
			expectedUpgrade: true,
			expectedErr:     false,
		},
		{
			name: "desired version is not changed",
			curator: clustercuratorv1.ClusterCurator{
				Spec: clustercuratorv1.ClusterCuratorSpec{
					Upgrade: clustercuratorv1.UpgradeHooks{
						DesiredUpdate: "4.11.4",
					},
				},
				Status: clustercuratorv1.ClusterCuratorStatus{
					Conditions: []v1.Condition{
						{
							Message: "curator-job-xxxx DesiredCuration: upgrade Version (4.11.4;;)",
							Status:  v1.ConditionTrue,
							Type:    "clustercurator-job",
						},
					},
				},
			},
			expectedUpgrade: false,
			expectedErr:     false,
		},
		{
			name: "desired version is changed",
			curator: clustercuratorv1.ClusterCurator{
				Spec: clustercuratorv1.ClusterCuratorSpec{
					Upgrade: clustercuratorv1.UpgradeHooks{
						DesiredUpdate: "4.11.5",
					},
				},
				Status: clustercuratorv1.ClusterCuratorStatus{
					Conditions: []v1.Condition{
						{
							Message: "curator-job-xxxx DesiredCuration: upgrade Version (4.11.4;;)",
							Status:  v1.ConditionTrue,
							Type:    "clustercurator-job",
						},
					},
				},
			},
			expectedUpgrade: true,
			expectedErr:     false,
		},
		{
			name: "desired channel is changed",
			curator: clustercuratorv1.ClusterCurator{
				Spec: clustercuratorv1.ClusterCuratorSpec{
					Upgrade: clustercuratorv1.UpgradeHooks{
						Channel: "stable-4.11",
					},
				},
				Status: clustercuratorv1.ClusterCuratorStatus{
					Conditions: []v1.Condition{
						{
							Message: "curator-job-xxxx DesiredCuration: upgrade Version (;stable-4.10;)",
							Status:  v1.ConditionTrue,
							Type:    "clustercurator-job",
						},
					},
				},
			},
			expectedUpgrade: true,
			expectedErr:     false,
		},
		{
			name: "desired upsteam is changed",
			curator: clustercuratorv1.ClusterCurator{
				Spec: clustercuratorv1.ClusterCuratorSpec{
					Upgrade: clustercuratorv1.UpgradeHooks{
						Upstream: "server2",
					},
				},
				Status: clustercuratorv1.ClusterCuratorStatus{
					Conditions: []v1.Condition{
						{
							Message: "curator-job-xxxx DesiredCuration: upgrade Version (;stable-4.10;server1)",
							Status:  v1.ConditionTrue,
							Type:    "clustercurator-job",
						},
					},
				},
			},
			expectedUpgrade: true,
			expectedErr:     false,
		},
		{
			name: "a broken msg",
			curator: clustercuratorv1.ClusterCurator{
				Spec: clustercuratorv1.ClusterCuratorSpec{
					Upgrade: clustercuratorv1.UpgradeHooks{
						DesiredUpdate: "4.11.5",
					},
				},
				Status: clustercuratorv1.ClusterCuratorStatus{
					Conditions: []v1.Condition{
						{
							Message: "curator-job-xxxx DesiredCuration: upgrade Version )",
							Status:  v1.ConditionTrue,
							Type:    "clustercurator-job",
						},
					},
				},
			},
			expectedUpgrade: true,
			expectedErr:     false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual, err := NeedToUpgrade(c.curator)
			if err != nil && !c.expectedErr {
				t.Errorf("unexpected error %v", err)
			}
			if actual != c.expectedUpgrade {
				t.Errorf("expected %v, but %v", c.expectedUpgrade, actual)
			}
		})
	}
}

func TestGetMonitorAttempts(t *testing.T) {
	attempts := GetMonitorAttempts("", &clustercuratorv1.ClusterCurator{})
	assert.Equal(t, 150, attempts)

	attempts = GetMonitorAttempts("provision", &clustercuratorv1.ClusterCurator{
		Spec: clustercuratorv1.ClusterCuratorSpec{
			Install: clustercuratorv1.Hooks{
				JobMonitorTimeout: 6,
			},
		},
	})
	assert.Equal(t, 180, attempts)

	attempts = GetMonitorAttempts("uninstall", &clustercuratorv1.ClusterCurator{
		Spec: clustercuratorv1.ClusterCuratorSpec{
			Destroy: clustercuratorv1.Hooks{
				JobMonitorTimeout: 15,
			},
		},
	})
	assert.Equal(t, 450, attempts)
}
