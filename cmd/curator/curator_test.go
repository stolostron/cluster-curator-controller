// Copyright Contributors to the Open Cluster Management project.

package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	clusterversionv1 "github.com/openshift/api/config/v1"
	hivev1 "github.com/openshift/hive/apis/hive/v1"
	clustercuratorv1 "github.com/stolostron/cluster-curator-controller/pkg/api/v1beta1"
	"github.com/stolostron/cluster-curator-controller/pkg/jobs/utils"
	managedclusterviewv1beta1 "github.com/stolostron/cluster-lifecycle-api/view/v1beta1"
	"github.com/stolostron/library-go/pkg/config"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

const ClusterName = "my-cluster"
const ClusterNamespace = "clusters"
const NodepoolName = "my-cluster-us-east-2"

var s = scheme.Scheme

func getClusterCurator() *clustercuratorv1.ClusterCurator {
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

func getClusterCuratorWithInstallOperation() *clustercuratorv1.ClusterCurator {
	return &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Operation: &clustercuratorv1.Operation{
			RetryPosthook: "installPosthook",
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "install",
		},
	}
}

func getClusterCuratorWithUpgradeOperation() *clustercuratorv1.ClusterCurator {
	return &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Operation: &clustercuratorv1.Operation{
			RetryPosthook: "upgradePosthook",
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "upgrade",
		},
	}
}

func getHypershiftClusterCurator() *clustercuratorv1.ClusterCurator {
	return &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterNamespace,
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "install",
		},
	}
}

func getHostedCluster(hcType string, hcConditions []interface{}) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "hypershift.openshift.io/v1beta1",
			"kind":       "HostedCluster",
			"metadata": map[string]interface{}{
				"name":      ClusterName,
				"namespace": ClusterNamespace,
				"labels": map[string]interface{}{
					"hypershift.openshift.io/auto-created-for-infra": ClusterName + "-xyz",
				},
			},
			"spec": map[string]interface{}{
				"pausedUntil": "true",
				"platform": map[string]interface{}{
					"type": hcType,
				},
				"release": map[string]interface{}{
					"image": "quay.io/openshift-release-dev/ocp-release:4.13.6-multi",
				},
			},
			"status": map[string]interface{}{
				"conditions": hcConditions,
			},
		},
	}
}

func getNodepool(npName string, npNamespace string, npClusterName string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "hypershift.openshift.io/v1beta1",
			"kind":       "NodePool",
			"metadata": map[string]interface{}{
				"name":      npName,
				"namespace": npNamespace,
			},
			"spec": map[string]interface{}{
				"pausedUntil": "true",
				"clusterName": npClusterName,
				"release": map[string]interface{}{
					"image": "quay.io/openshift-release-dev/ocp-release:4.13.6-multi",
				},
			},
		},
	}
}

func getEUSUpgradeClusterCurator() *clustercuratorv1.ClusterCurator {
	return &clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "upgrade",
			Upgrade: clustercuratorv1.UpgradeHooks{
				IntermediateUpdate: "4.13.37",
				DesiredUpdate:      "4.14.16",
				MonitorTimeout:     120,
			},
		},
	}
}

func getEUSClusterVersionManagedClusterView() *managedclusterviewv1beta1.ManagedClusterView {
	clusterversion := &clusterversionv1.ClusterVersion{
		TypeMeta: v1.TypeMeta{
			APIVersion: "config.openshift.io/v1",
			Kind:       "ClusterVersion",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "version",
		},
		Spec: clusterversionv1.ClusterVersionSpec{
			Channel:   "stable-4.5",
			ClusterID: "201ad26c-67d6-416a",
			Upstream:  "https://api.openshift.com/api",
		},
		Status: clusterversionv1.ClusterVersionStatus{
			Conditions: []clusterversionv1.ClusterOperatorStatusCondition{
				{
					LastTransitionTime: v1.NewTime(time.Now()),
					Message:            "Done applying 4.13.37",
					Status:             "True",
					Type:               "Available",
				},
				{
					LastTransitionTime: v1.NewTime(time.Now()),
					Message:            "Working towards 4.13.37: 100% complete",
					Status:             "True",
					Type:               "Progressing",
				},
			},
		},
	}

	b, _ := json.Marshal(clusterversion)
	return &managedclusterviewv1beta1.ManagedClusterView{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Spec: managedclusterviewv1beta1.ViewSpec{
			Scope: managedclusterviewv1beta1.ViewScope{
				Kind:    "ClusterVersion",
				Name:    "version",
				Version: "v1",
				Group:   "config.openshift.io",
			},
		},
		Status: managedclusterviewv1beta1.ViewStatus{
			Conditions: []v1.Condition{
				{
					Message: "Watching resources successfully",
					Reason:  "GetResourceProcessing",
					Status:  "True",
					Type:    "Processing",
				},
			},
			Result: runtime.RawExtension{
				Raw: b,
			},
		},
	}
}
func TestCuratorRunNoParam(t *testing.T) {

	defer func() {
		r := recover()
		t.Log(r.(error).Error())

		if !strings.Contains(r.(error).Error(), "Command: ./curator [") &&
			!strings.Contains(r.(error).Error(), "Invalid Parameter: \"\"") {
			t.Fatal(r)
		}
		t.Log("Detected missing paramter")
	}()

	os.Args[1] = ""

	curatorRun(nil, nil, ClusterName, ClusterName)
}

func TestCuratorRunWrongParam(t *testing.T) {

	defer func() {
		r := recover()
		t.Log(r.(error).Error())

		if !strings.Contains(r.(error).Error(), "Command: ./curator [") &&
			!strings.Contains(r.(error).Error(), "something-wrong") {
			t.Fatal(r)
		}
		t.Log("Detected wrong paramter")
	}()

	os.Args[1] = "something-wrong"

	curatorRun(nil, nil, ClusterName, ClusterName)
}

func TestCuratorRunNoClusterCurator(t *testing.T) {

	defer func() {
		r := recover()
		t.Log(r.(error).Error())

		if !strings.Contains(r.(error).Error(), "clustercurators.cluster.open-cluster-management.io \"my-cluster\"") {
			t.Fatal(r)
		}
		t.Log("Detected missing ClusterCurator resource")
	}()

	s := scheme.Scheme
	s.AddKnownTypes(utils.CCGVR.GroupVersion(), &clustercuratorv1.ClusterCurator{})

	client := clientfake.NewClientBuilder().WithScheme(s).Build()

	os.Args[1] = "SKIP_ALL_TESTING"

	curatorRun(nil, client, ClusterName, ClusterName)
}

func TestCuratorRunClusterCurator(t *testing.T) {

	s := scheme.Scheme
	s.AddKnownTypes(utils.CCGVR.GroupVersion(), &clustercuratorv1.ClusterCurator{})

	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(getClusterCurator()).Build()

	os.Args[1] = "SKIP_ALL_TESTING"

	assert.NotPanics(t, func() { curatorRun(nil, client, ClusterName, ClusterName) }, "no panic when ClusterCurator found and skip test")
}

func TestCuratorRunClusterCuratorInstallUpgradeOperation(t *testing.T) {
	s := scheme.Scheme
	s.AddKnownTypes(utils.CCGVR.GroupVersion(), &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(getClusterCuratorWithInstallOperation()).Build()

	os.Args[1] = "SKIP_ALL_TESTING"

	assert.NotPanics(t, func() { curatorRun(nil, client, ClusterName, ClusterName) }, "no panic when ClusterCurator found and skip test")

	client = clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(getClusterCuratorWithUpgradeOperation()).Build()

	assert.NotPanics(t, func() { curatorRun(nil, client, ClusterName, ClusterName) }, "no panic when ClusterCurator found and skip test")
}

func TestCuratorRunNoProviderCredentialPath(t *testing.T) {

	defer func() {
		r := recover()
		t.Log(r.(error).Error())

		if !strings.Contains(r.(error).Error(), "Missing spec.providerCredentialPath") {
			t.Fatal(r)
		}
		t.Log("Detected missing provierCredentialPath")
	}()

	s := scheme.Scheme
	hivev1.AddToScheme(s)
	s.AddKnownTypes(utils.CCGVR.GroupVersion(), &clustercuratorv1.ClusterCurator{})

	client := clientfake.NewClientBuilder().WithRuntimeObjects(getClusterCurator()).WithScheme(s).Build()

	os.Args[1] = "applycloudprovider-ansible"

	curatorRun(nil, client, ClusterName, ClusterName)
}

func TestCuratorRunProviderCredentialPathEnv(t *testing.T) {

	defer func() {
		r := recover()
		t.Log(r.(error).Error())

		if !strings.Contains(r.(error).Error(), "secrets \"secretname\"") {
			t.Fatal(r)
		}
		t.Log("Detected missing namespace/secretName")
	}()

	os.Setenv("PROVIDER_CREDENTIAL_PATH", "namespace/secretname")
	client := clientfake.NewClientBuilder().WithScheme(s).Build()

	os.Args[1] = "applycloudprovider-ansible"

	curatorRun(nil, client, ClusterName, ClusterName)
}

func TestInvokeMonitor(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected recover, but failed")
		}
	}()

	os.Setenv("PROVIDER_CREDENTIAL_PATH", "namespace/secretname")
	os.Args[1] = "monitor"

	curatorRun(nil, clientfake.NewClientBuilder().Build(), ClusterName, ClusterName)
}

func TestInvokeMonitorImport(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected recover, but failed")
		}
	}()

	os.Setenv("PROVIDER_CREDENTIAL_PATH", "namespace/secretname")
	os.Args[1] = "monitor-import"

	curatorRun(nil, clientfake.NewClientBuilder().Build(), ClusterName, ClusterName)
}

func TestInvokeMonitorDestroy(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected recover, but failed")
		}
	}()

	os.Setenv("PROVIDER_CREDENTIAL_PATH", "namespace/secretname")
	os.Args[1] = "monitor-destroy"

	curatorRun(nil, clientfake.NewClientBuilder().Build(), ClusterName, ClusterName)
}

func TestUpgradFailed(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected recover, but failed")
		}
	}()

	os.Args[1] = "upgrade-cluster"

	s := scheme.Scheme
	s.AddKnownTypes(utils.CCGVR.GroupVersion(), &clustercuratorv1.ClusterCurator{})

	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(
		&clustercuratorv1.ClusterCurator{
			ObjectMeta: v1.ObjectMeta{
				Name:      ClusterName,
				Namespace: ClusterName,
			},
			Spec: clustercuratorv1.ClusterCuratorSpec{
				DesiredCuration: "upgrade",
				Upgrade: clustercuratorv1.UpgradeHooks{
					DesiredUpdate: "4.11.4",
				},
			},
		},
	).Build()

	curatorRun(nil, client, ClusterName, ClusterName)
}

func TestUpgradDone(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected error %v", r)
		}
	}()

	os.Args[1] = "done"

	s := scheme.Scheme
	s.AddKnownTypes(utils.CCGVR.GroupVersion(), &clustercuratorv1.ClusterCurator{})

	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(
		&clustercuratorv1.ClusterCurator{
			ObjectMeta: v1.ObjectMeta{
				Name:      ClusterName,
				Namespace: ClusterName,
			},
			Spec: clustercuratorv1.ClusterCuratorSpec{
				DesiredCuration: "upgrade",
				Upgrade: clustercuratorv1.UpgradeHooks{
					DesiredUpdate: "4.11.4",
				},
			},
		},
	).Build()

	curatorRun(nil, client, ClusterName, ClusterName)
}

func TestHypershiftActivate(t *testing.T) {
	// Test will fail because we can't pass in a fake dynamic client
	// But that's ok, we just need to test the curator code
	defer func() { // recover from not having a hub config object
		if r := recover(); r == nil {
			t.Fatal("expected recover, but failed")
		}
	}()

	os.Args[1] = "activate-and-monitor"
	os.Args[2] = ClusterName

	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(
		getHypershiftClusterCurator(),
		getHostedCluster("AWS", []interface{}{}),
		getNodepool(NodepoolName, ClusterNamespace, ClusterName),
	).Build()

	config, _ := config.LoadConfig("", "", "")

	curatorRun(config, client, ClusterNamespace, ClusterName)
}

func TestHypershiftMonitor(t *testing.T) {
	// Test will fail because we can't pass in a fake dynamic client
	// But that's ok, we just need to test the curator code
	defer func() { // recover from not having a hub config object
		if r := recover(); r == nil {
			t.Fatal("expected recover, but failed")
		}
	}()

	os.Args[1] = "monitor"
	os.Args[2] = ClusterName

	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(
		getHypershiftClusterCurator(),
		getHostedCluster("AWS", []interface{}{}),
		getNodepool(NodepoolName, ClusterNamespace, ClusterName),
	).Build()

	config, _ := config.LoadConfig("", "", "")

	curatorRun(config, client, ClusterNamespace, ClusterName)
}

func TestHypershiftDestroyCluster(t *testing.T) {
	// Test will fail because we can't pass in a fake dynamic client
	// But that's ok, we just need to test the curator code
	defer func() { // recover from not having a hub config object
		if r := recover(); r == nil {
			t.Fatal("expected recover, but failed")
		}
	}()

	os.Args[1] = "destroy-cluster"
	os.Args[2] = ClusterName

	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(
		getHypershiftClusterCurator(),
		getHostedCluster("KubeVirt", []interface{}{}),
		getNodepool(NodepoolName, ClusterNamespace, ClusterName),
	).Build()

	config, _ := config.LoadConfig("", "", "")

	curatorRun(config, client, ClusterNamespace, ClusterName)
}

func TestHypershiftMonitorDestroy(t *testing.T) {
	// Test will fail because we can't pass in a fake dynamic client
	// But that's ok, we just need to test the curator code
	defer func() { // recover from not having a hub config object
		if r := recover(); r == nil {
			t.Fatal("expected recover, but failed")
		}
	}()

	os.Args[1] = "monitor-destroy"
	os.Args[2] = ClusterName

	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(
		getHypershiftClusterCurator(),
		getHostedCluster("KubeVirt", []interface{}{}),
		getNodepool(NodepoolName, ClusterNamespace, ClusterName),
	).Build()

	config, _ := config.LoadConfig("", "", "")

	curatorRun(config, client, ClusterNamespace, ClusterName)
}

func TestHypershiftUpgradeCluster(t *testing.T) {
	// Test will fail because we can't pass in a fake dynamic client
	// But that's ok, we just need to test the curator code
	defer func() { // recover from not having a hub config object
		if r := recover(); r == nil {
			t.Fatal("expected recover, but failed")
		}
	}()

	os.Args[1] = "upgrade-cluster"
	os.Args[2] = ClusterName

	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(
		getHypershiftClusterCurator(),
		getHostedCluster("AWS", []interface{}{}),
		getNodepool(NodepoolName, ClusterNamespace, ClusterName),
	).Build()

	config, _ := config.LoadConfig("", "", "")

	curatorRun(config, client, ClusterNamespace, ClusterName)
}

func TestHypershiftMonitorUpgrade(t *testing.T) {
	// Test will fail because we can't pass in a fake dynamic client
	// But that's ok, we just need to test the curator code
	defer func() { // recover from not having a hub config object
		if r := recover(); r == nil {
			t.Fatal("expected recover, but failed")
		}
	}()

	os.Args[1] = "monitor-upgrade"
	os.Args[2] = ClusterName

	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(
		getHypershiftClusterCurator(),
		getHostedCluster("AWS", []interface{}{}),
		getNodepool(NodepoolName, ClusterNamespace, ClusterName),
	).Build()

	config, _ := config.LoadConfig("", "", "")

	curatorRun(config, client, ClusterNamespace, ClusterName)
}

func TestEUSIntermediateUpgrade(t *testing.T) {
	// Test will fail because we don't have all the objects
	// But that's ok, we just need to test the curator code
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected recover, but failed")
		}
	}()

	os.Args[1] = "intermediate-upgrade-cluster"
	os.Args[2] = ClusterName

	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(
		getEUSUpgradeClusterCurator(),
	).Build()

	config, _ := config.LoadConfig("", "", "")

	curatorRun(config, client, ClusterName, ClusterName)
}

func TestEUSFinalUpgrade(t *testing.T) {
	// Test will fail because we don't have all the objects
	// But that's ok, we just need to test the curator code
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected recover, but failed")
		}
	}()

	os.Args[1] = "final-upgrade-cluster"
	os.Args[2] = ClusterName

	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(
		getEUSUpgradeClusterCurator(),
	).Build()

	config, _ := config.LoadConfig("", "", "")

	curatorRun(config, client, ClusterName, ClusterName)
}

func TestEUSMonitorUpgrade(t *testing.T) {
	// Test will fail because we don't have all the objects
	// But that's ok, we just need to test the curator code
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected recover, but failed")
		}
	}()

	os.Args[1] = "intermediate-monitor-upgrade"
	os.Args[2] = ClusterName

	s.AddKnownTypes(clustercuratorv1.SchemeBuilder.GroupVersion, &clustercuratorv1.ClusterCurator{})
	client := clientfake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(
		getEUSUpgradeClusterCurator(),
		getEUSClusterVersionManagedClusterView(),
	).Build()

	config, _ := config.LoadConfig("", "", "")

	curatorRun(config, client, ClusterName, ClusterName)
}

func TestIntermediateUpdateImmutability(t *testing.T) {
	clustercuratorv1.AddToScheme(scheme.Scheme)
	m := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "deploy", "crd"),
		},
	}

	var err error
	var cfg *rest.Config
	if cfg, err = m.Start(); err != nil {
		log.Fatal(err)
	}

	defer m.Stop()

	var c client.Client

	if c, err = client.New(cfg, client.Options{Scheme: scheme.Scheme}); err != nil {
		log.Fatal(err)
	}

	err = c.Create(context.Background(), &corev1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Name: ClusterName,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	err = c.Create(context.Background(), getEUSUpgradeClusterCurator())
	if err != nil {
		log.Fatal(err)
	}

	curator := clustercuratorv1.ClusterCurator{}
	c.Get(context.TODO(), types.NamespacedName{
		Namespace: ClusterName,
		Name:      ClusterName,
	}, &curator)

	patch := []byte(
		`{
			"spec": {
				"upgrade": {
					"intermediateUpdate": "4.13.38"
				}
			}
		}`)

	err = c.Patch(context.Background(), &curator, client.RawPatch(types.MergePatchType, patch))

	c.Get(context.TODO(), types.NamespacedName{
		Namespace: ClusterName,
		Name:      ClusterName,
	}, &curator)
	assert.True(t, strings.Contains(err.Error(), "The intermediateUpdate cannot be modified"), "The intermediateUpdate cannot be modified error found")
	assert.True(t, curator.Spec.Upgrade.IntermediateUpdate == "4.13.37", "intermediateUpdate is not changed")
}

func TestDesiredUpdateImmutabilityWhenIntermediateUpdateExists(t *testing.T) {
	clustercuratorv1.AddToScheme(scheme.Scheme)
	m := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "deploy", "crd"),
		},
	}

	var err error
	var cfg *rest.Config
	if cfg, err = m.Start(); err != nil {
		log.Fatal(err)
	}

	defer m.Stop()

	var c client.Client

	if c, err = client.New(cfg, client.Options{Scheme: scheme.Scheme}); err != nil {
		log.Fatal(err)
	}

	err = c.Create(context.Background(), &corev1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Name: ClusterName,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	err = c.Create(context.Background(), getEUSUpgradeClusterCurator())
	if err != nil {
		log.Fatal(err)
	}

	curator := clustercuratorv1.ClusterCurator{}
	c.Get(context.TODO(), types.NamespacedName{
		Namespace: ClusterName,
		Name:      ClusterName,
	}, &curator)

	patch := []byte(
		`{
			"spec": {
				"upgrade": {
					"desiredUpdate": "4.14.17"
				}
			}
		}`)

	err = c.Patch(context.Background(), &curator, client.RawPatch(types.MergePatchType, patch))

	assert.True(t, strings.Contains(err.Error(), "The desiredUpdate cannot be modified when intermediateUpdate exists"),
		"desireUpdate immutable when intermediateUpdate exists validation successful")
}

func TestIntermediateUpdateCreation(t *testing.T) {
	clustercuratorv1.AddToScheme(scheme.Scheme)
	m := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "deploy", "crd"),
		},
	}

	var err error
	var cfg *rest.Config
	if cfg, err = m.Start(); err != nil {
		log.Fatal(err)
	}

	defer m.Stop()

	var c client.Client

	if c, err = client.New(cfg, client.Options{Scheme: scheme.Scheme}); err != nil {
		log.Fatal(err)
	}

	err = c.Create(context.Background(), &corev1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Name: ClusterName,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	curator := clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "upgrade",
			Upgrade: clustercuratorv1.UpgradeHooks{
				IntermediateUpdate: "4.13.37",
				MonitorTimeout:     120,
			},
		},
	}

	err = c.Create(context.TODO(), &curator)
	assert.True(t, strings.Contains(err.Error(), "The intermediateUpdate cannot be created if desiredUpdate is missing or empty"),
		"Cannot create curator with only intermediateUpdate validation successful")
}

func TestAddIntermediateUpdateToExistingCurator(t *testing.T) {
	clustercuratorv1.AddToScheme(scheme.Scheme)
	m := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "deploy", "crd"),
		},
	}

	var err error
	var cfg *rest.Config
	if cfg, err = m.Start(); err != nil {
		log.Fatal(err)
	}

	defer m.Stop()

	var c client.Client

	if c, err = client.New(cfg, client.Options{Scheme: scheme.Scheme}); err != nil {
		log.Fatal(err)
	}

	err = c.Create(context.Background(), &corev1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Name: ClusterName,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	curator := clustercuratorv1.ClusterCurator{
		ObjectMeta: v1.ObjectMeta{
			Name:      ClusterName,
			Namespace: ClusterName,
		},
		Spec: clustercuratorv1.ClusterCuratorSpec{
			DesiredCuration: "upgrade",
			Upgrade: clustercuratorv1.UpgradeHooks{
				DesiredUpdate:  "4.14.16",
				MonitorTimeout: 120,
			},
		},
	}

	err = c.Create(context.Background(), &curator)
	if err != nil {
		log.Fatal(err)
	}

	curator = clustercuratorv1.ClusterCurator{}
	c.Get(context.TODO(), types.NamespacedName{
		Namespace: ClusterName,
		Name:      ClusterName,
	}, &curator)

	patch := []byte(
		`{
			"spec": {
				"upgrade": {
					"intermediateUpdate": "4.13.38"
				}
			}
		}`)

	err = c.Patch(context.Background(), &curator, client.RawPatch(types.MergePatchType, patch))

	assert.True(t, strings.Contains(err.Error(), "The intermediateUpdate cannot be added via update if desiredUpdate already exists"),
		"Cannot add intermediateUpdate to existing curator validation successful")
}
