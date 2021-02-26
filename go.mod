module github.com/open-cluster-management/cluster-curator-controller

go 1.15

require (
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/open-cluster-management/api v0.0.0-20201210143210-581cab55c797
	github.com/open-cluster-management/library-go v0.0.0-20210208174614-f3ad264f145a
	github.com/openshift/hive v1.0.14
	github.com/stretchr/testify v1.6.1
	gopkg.in/yaml.v2 v2.3.0
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776 // indirect
	k8s.io/api v0.19.0
	k8s.io/apimachinery v0.19.0
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/klog/v2 v2.3.0
	sigs.k8s.io/yaml v1.2.0
)

replace (
	bitbucket.org/ww/goautoneg => github.com/munnerz/goautoneg v0.0.0-20120707110453-a547fc61f48d
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
	github.com/coreos/etcd => go.etcd.io/etcd v3.3.22+incompatible
	github.com/go-logr/logr => github.com/go-logr/logr v0.2.1
	github.com/go-logr/zapr => github.com/go-logr/zapr v0.2.0
	github.com/gorilla/websocket => github.com/gorilla/websocket v1.4.2
	github.com/mattn/go-sqlite3 => github.com/mattn/go-sqlite3 v1.10.0
	github.com/metal3-io/baremetal-operator => github.com/metal3-io/baremetal-operator v0.0.0-20201010064625-48f105067ad6
	github.com/metal3-io/cluster-api-provider-baremetal => github.com/metal3-io/cluster-api-provider-baremetal v0.2.2
	github.com/open-cluster-management/endpoint-operator => github.com/open-cluster-management/endpoint-operator v1.0.0
	github.com/terraform-providers/terraform-provider-aws => github.com/terraform-providers/terraform-provider-aws v1.60.0
	github.com/terraform-providers/terraform-provider-azurerm => github.com/terraform-providers/terraform-provider-azurerm v1.44.0
	github.com/terraform-providers/terraform-provider-ignition/v2 => github.com/community-terraform-providers/terraform-provider-ignition/v2 v2.1.0
	k8s.io/client-go => k8s.io/client-go v0.19.0
	sigs.k8s.io/cluster-api-provider-aws => sigs.k8s.io/cluster-api-provider-aws v0.6.3
	sigs.k8s.io/cluster-api-provider-azure => sigs.k8s.io/cluster-api-provider-azure v0.4.9
	sigs.k8s.io/cluster-api-provider-gcp => sigs.k8s.io/cluster-api-provider-gcp v0.2.0-alpha.2
	sigs.k8s.io/cluster-api-provider-openstack => sigs.k8s.io/cluster-api-provider-openstack v0.3.3
)
