# Cluster-Curator

## Purpose
This project contains jobs and controllers for curating work around Cluster Provisioning. It is designed to extend the capabilities already present within Hive `ClusterDeployment` and Open Cluster Management `ManagedCluster` kinds.

## Architecture
The controller found here, monitors for `ManageCluster` kinds.  When new instances of the resource are created, this controller creates a default Kubernetes Job that runs: `applycloudprovider`, `pre-hook Ansible`, `activate-and-monitor` and `posthook Ansible`.  The Job can be overridden with your own job flow.

![Architecture diagram](docs/ansiblejob-flow.png "Architecture")\
For more details on job flow within our architecture see our [**swimlane chart**](https://swimlanes.io/u/kGNg12_Vw).
## Controller:
- cluster-curator-controller (ccc), watches for `ManagedCluster` kind create resource action.

### Deploy
```bash
oc apply -k deploy/controller
```
This deployment defaults to the namespace `open-cluster-management`. Each time a new `ManagedCluster` resource is created, you will see operations take place in the controller pod's log.

## Jobs

| Job action | Description | Cloud Provider | Override ConfigMap | Template ConfigMap |
| :---------:| :---------: | :------------: | :----------------: | :----------------: |
|applycloudprovider-(aws/gcp/azure/vmware)| Creates AWS/GCP/Azure/VMware related credentials for a cluster deployment | X | X | |
|applycloudprovider-ansible | Creates the Ansible tower secret for a cluster deployment (included in applycloudprovider-aws) | X | X | |
| activate-and-monitor | Sets `ClusterDeployment.spec.installAttempsLimit: 1`, then monitors the deployment of the cluster | | X |  |
| monitor-import | Creates the ManagedCluster and KlusterletAddonConfig for a cluster | | X | X |
| prehook-ansiblejob posthook-ansiblejob | Creates an AnsibleJob resource and monitors it to completion |  | X |  |
| monitor | Watches a `ClusterDeployment` Provisioning Job | | | |


Here is an example of each job described above. You can add and remove instances of the job containers as needed. You can also inject your own containers `./deploy/jobs/create-cluster.yaml`

## Provisioning
### Prerequisites
1. Grant access to the Cloud Provider secret (needed for most containers in the job)
```bash
## Provide your own values for:
# * CP_NAME (my-cloud-provider-secret_)
# * CP_NAMESPACE (default)
# * CLUSTER_NAME (my-cluster)

## CREATE ## (First time)
oc process -f deploy/provider-credentials/rbac-cloudprovider.yaml -p CP_NAME=my-cloud-provider-secret -p CP_NAMESPACE=default -p CLUSTER_NAME=my-cluster | oc apply -f -

## UPDATE ## If the rolebinding already exists
# * CP_NAMESPACE where the Cloud Provider secret is created
# * CP_NAME the name of your cloud provider, note the "-cpv" suffix in the command
# * CLUSTER_NAME the name of our new cluster
kubectl -n CP_NAMESPACE patch roleBinding CP_NAME-cpv --type=json -p='[{"op": "add", "path": "/subjects/-", "value": {"kind": "ServiceAccount","name":"cluster-installer","namespace":"CLUSTER_NAME"} }]'

## DELETE ##
oc process -f deploy/provider-credentials/rbac-cloudprovider.yaml -p CP_NAME=my-cloud-provider-secret -p CP_NAMESPACE=default -p CLUSTER_NAME=my-cluster | oc delete -f -
```
2. Creating a cluster (AWS)
Edit `./deploy/jobs/create-cluster.yaml` if you want to add your own containers (steps) to the
curator job. Your containers are added to the overRide job stanza. If you want to use the default curator-job, remove the `overrideJob` stanza.
```bash
# Generate a new YAML for your cluster
# * CLUSTER_NAME (my-cluster)
# * CLUSTER_IMAGE_SET (img4.6.15-x86-64-appsub, must exist on the ACM hub)
# * BASE_DOMAIN (my-domain.com)
# * PROVIDER_CREDENTIAL_PATH (default/secret-name)
oc process -f deploy/jobs/create-cluster.yaml -p CLUSTER_NAME=my-cluster -p CLUSTER_IMAGE_SET=img4.6.15-x86-64-appsub -p BASE_DOMAIN=my-domain.com -p PROVIDER_CREDENTIAL_PATH=default/secret-name -o yaml --raw=true | sed -e 's/^apiVersion:/---\napiVersion:/g' > my-cluster.yaml

# You can commit this yaml to Git as part of a GitOps flow or apply it directly to an ACM Hub cluster.
oc apply -f my-cluster.yaml
```
This will create a ManagedCluster resource which the cluster-curator-controller will identify and creates a curator-job.  Check the ConfigMap for the keys `curator-job` and `curator-job-container`. These will tell you which step (container) the job is running.  You can view the logs by combining the `curator-job` value and the `curator-job-container` value in the following command
```bash
# ConfigMap
#   data:
#     curator-job: curator-job-MJE7f-m3M39
#     curator-job-container: applycloudprovider-aws

# Run the following command to see the logs
oc logs job.batch/curator-job-MJE7f-m3M39 applycloudprovider-aws

# Add a "-f" to the end if you want to tail the output
```
If there is a failure, the job will show Failure.  Look at the `curator-job-container` value to see which step in the provisioning failed and review the logs above. If the `curator-job-contianer` is `monitor`, there may be an additional `provisioning` job. Check this log for additional information.

The generated YAML can be committed to a Git repository. You can then use an ACM Subscription to apply the YAML (provision) on the ACM Hub.  Repeat steps 1 & 3 to create new clusters.
 