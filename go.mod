module github.com/stolostron/cluster-curator-controller

go 1.23.0

toolchain go1.23.7

require (
	github.com/blang/semver/v4 v4.0.0
	github.com/go-logr/logr v1.4.1
	github.com/open-cluster-management/ansiblejob-go-lib v0.0.0-00010101000000-000000000000
	github.com/openshift/api v3.9.1-0.20191111211345-a27ff30ebf09+incompatible
	github.com/openshift/hive/apis v0.0.0-20240503201730-e11a23b88b31
	github.com/stolostron/cluster-lifecycle-api v0.0.0-20220714081119-eae2fe1f05fd
	github.com/stolostron/library-go v0.0.0-20220727113621-f74e0852408a
	github.com/stretchr/testify v1.8.4
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/api v0.29.2
	k8s.io/apimachinery v0.29.2
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.120.1
	open-cluster-management.io/api v0.11.0
	sigs.k8s.io/controller-runtime v0.16.6
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emicklei/go-restful/v3 v3.11.2 // indirect
	github.com/evanphx/json-patch v5.7.0+incompatible // indirect
	github.com/evanphx/json-patch/v5 v5.9.0 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/go-logr/zapr v1.3.0 // indirect
	github.com/go-openapi/jsonpointer v0.20.2 // indirect
	github.com/go-openapi/jsonreference v0.20.4 // indirect
	github.com/go-openapi/swag v0.22.9 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/gnostic-models v0.6.9-0.20230804172637-c7be7c783f49 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/openshift/custom-resource-status v1.1.3-0.20220503160415-f2fdb4999d87 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_golang v1.18.0 // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.46.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/spf13/pflag v1.0.6-0.20210604193023-d5e0c0615ace // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.26.0 // indirect
	golang.org/x/exp v0.0.0-20220722155223-a9213eeb770e // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/oauth2 v0.27.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/term v0.29.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.4.0 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/apiextensions-apiserver v0.29.2 // indirect
	k8s.io/component-base v0.29.2 // indirect
	k8s.io/kube-openapi v0.0.0-20240126223410-2919ad4fcfec // indirect
	k8s.io/utils v0.0.0-20240102154912-e7106e64919e // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

replace (
	github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.4.1 // ensure compatible between controller-runtime and kube-openapi
	github.com/open-cluster-management/ansiblejob-go-lib => github.com/stolostron/ansiblejob-go-lib v0.1.12
	golang.org/x/crypto => golang.org/x/crypto v0.35.0
)

//(hive dependency) from ocp installer
replace (
	github.com/IBM-Cloud/terraform-provider-ibm => github.com/openshift/terraform-provider-ibm v1.26.2-openshift-2
	github.com/kubevirt/terraform-provider-kubevirt => github.com/nirarg/terraform-provider-kubevirt v0.0.0-20201222125919-101cee051ed3
	github.com/metal3-io/baremetal-operator => github.com/openshift/baremetal-operator v0.0.0-20231128154154-6736c9b9c6c8
	github.com/metal3-io/baremetal-operator/apis => github.com/openshift/baremetal-operator/apis v0.0.0-20231128154154-6736c9b9c6c8
	github.com/metal3-io/baremetal-operator/pkg/hardwareutils => github.com/openshift/baremetal-operator/pkg/hardwareutils v0.0.0-20231128154154-6736c9b9c6c8
	github.com/terraform-providers/terraform-provider-aws => github.com/openshift/terraform-provider-aws v1.60.1-0.20211215220004-24df6d73af46
	github.com/terraform-providers/terraform-provider-azurerm => github.com/openshift/terraform-provider-azurerm v1.44.1-0.20210224232508-7509319df0f4
	github.com/terraform-providers/terraform-provider-azurestack => github.com/openshift/terraform-provider-azurestack v0.10.0-openshift
	github.com/terraform-providers/terraform-provider-ignition/v2 => github.com/community-terraform-providers/terraform-provider-ignition/v2 v2.1.0
	k8s.io/cloud-provider-vsphere => github.com/openshift/cloud-provider-vsphere v1.19.1-0.20211222185833-7829863d0558
	sigs.k8s.io/cluster-api-provider-aws => github.com/openshift/cluster-api-provider-aws 005a7656f03f
	sigs.k8s.io/cluster-api-provider-aws/v2 => sigs.k8s.io/cluster-api-provider-aws/v2 v2.0.0-20231024062453-0bf78b04b305
	sigs.k8s.io/cluster-api-provider-azure => github.com/openshift/cluster-api-provider-azure v0.1.0-alpha.3.0.20210626224711-5d94c794092f
	sigs.k8s.io/cluster-api-provider-openstack => github.com/openshift/cluster-api-provider-openstack v0.0.0-20211111204942-611d320170af
)

//(hive dependency) from installer as part of https://github.com/openshift/installer/pull/4350
// Prevent the following modules from upgrading version as result of terraform-provider-kubernetes module
// The following modules need to be locked to compile correctly with terraform-provider-azure and terraform-provider-google
replace (
	github.com/apparentlymart/go-cidr => github.com/apparentlymart/go-cidr v1.0.1
	github.com/go-openapi/errors => github.com/go-openapi/errors v0.19.2
	github.com/go-openapi/spec => github.com/go-openapi/spec v0.19.4
	github.com/go-openapi/validate => github.com/go-openapi/validate v0.19.8
	github.com/hashicorp/go-plugin => github.com/hashicorp/go-plugin v1.2.2
	github.com/ulikunitz/xz => github.com/ulikunitz/xz v0.5.7
	google.golang.org/api => google.golang.org/api v0.25.0
	google.golang.org/genproto => google.golang.org/genproto v0.0.0-20200526211855-cb27e3aa2013
	google.golang.org/grpc => google.golang.org/grpc v1.33.0
)

replace (
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.26.2
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.29.1
	k8s.io/client-go => k8s.io/client-go v0.29.1
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.26.2
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.26.2
	k8s.io/code-generator => k8s.io/code-generator v0.26.2
	k8s.io/component-base => k8s.io/component-base v0.26.2
	k8s.io/cri-api => k8s.io/cri-api v0.26.2
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.26.2
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.26.2
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.26.2
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.26.2
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.26.2
	k8s.io/kubectl => k8s.io/kubectl v0.29.1
	k8s.io/kubelet => k8s.io/kubelet v0.26.2
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.26.2
	k8s.io/metrics => k8s.io/metrics v0.26.2
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.26.2
	kubevirt.io/client-go => kubevirt.io/client-go v0.29.0
)
