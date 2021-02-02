# Cluster-Curator

## Purpose
This project contains jobs and controllers for curating work around Cluster Provisioning. It is designed to extend the capabilities already present with Hive `ClusterDeployment` and Open-Cluster-Management `ManagedCluster` kinds.

## Architecture
The control found here, monitors for `ManageCluster` kinds.  When new instances of the resource are created, this controller creates a default Kubernetes Job. applycloudprovider, pre-hook Ansible, activate-monitor and posthook Ansible.  The Job can be overridden with your own job flow.

## Controller:
- cluster-curator-controller (ccc), watches for `ManagedCluster` kind create resource action.

### Deploy
```bash
oc apply -k deploy/controller
```
This deployment defaults to the namespace `open-cluster-management`. Each time a new `ManagedCluster` resource is created, you will see operations take place in the control pod's log.

## Job containers

| Job action | Description | Cloud Provider | Override ConfigMap | Template ConfigMap |
| :---------:| :---------: | :------------: | :----------------: | :----------------: |
|applycloudprovider-aws | Creates AWS related crednetials for a cluster deployment | X | X | |
|applycloudprovider-ansible | Creates the Ansible tower secret for a cluster deployment (included in applycloudprovider-aws) | X | X | |
| create-was | Creates the ClusterDeployment, ManachinePool and install-config secret for a cluster deployment. |  | X | X |
| activate-monitor | Sets `ClusterDeployment.spec.installAttempsLimit: 1`, then monitors the deployment of the cluster | | X |  |
| import | Creates the ManagedCluster and KlusterletAddonConfig for a cluster | | X | X |
| prehook-ansiblejob posthook-ansiblejob | Creates an AnsibleJob resource and monitors it to completion |  | X |  |

Here is an example of each job described here. You can add and remove instances of the job containers as needed. You can also inject your own containers `./deploy/jobs/create-cluster.yaml`

## Doing a deploy
### Prerequisits
1. Grant access to the Cloud Provider secret (needed for most containers in the job)
```bash
# Provide your own values for:
* CP_NAME (my-cloud-provider-secret_)
* CP_NAMESPACE (default)
* CLUSTER_NAME (my-cluster)

## CREATE ##
oc process -f deploy/jobs/provider-ns-rolebinding.yaml -p CP_NAME=my-cloud-provider-secret -p CP_NAMESPACE=default -p CLUSTER_NAME=my-cluster | oc apply -f -

## DELETE ##
oc process -f deploy/jobs/provider-ns-rolebinding.yaml -p CP_NAME=my-cloud-provider-secret -p CP_NAMESPACE=default -p CLUSTER_NAME=my-cluster | oc delete -f -
```
2. Create a Cluster configMap template. Will be read-only to all service accounts in the cluster by default.
```bash
# Provide your own values for:
# * TEMPLATE_NAME (aws-template)
# * TEMPLATE_NAMESPACE (default)

## CREATE ##
oc process -f deploy/cluster-templates-configmaps/aws-clusterdeployment.yaml -p TEMPLATE_NAME=aws-template TEMPLATE_NAMESPACE=default | oc apply -f -

## DELETE ##
oc process -f deploy/cluster-templates-configmaps/aws-clusterdeployment.yaml -p TEMPLATE_NAME=aws-template TEMPLATE_NAMESPACE=default | oc delete -f -
```
3. Creating a cluster
```bash
# Generate a new YAML for your cluster
# * CLUSTER_NAME (my-cluster)
# * CLUSTER_IMAGE_SET (img4.6.15-x86-64-appsub)
# * BASE_DOMAIN (my-domain.com)
oc process -f deploy/jobs/create-cluster.yaml -p CLUSTER_NAME=my-cluster -p CLUSTER_IMAGE_SET=img4.6.15-x86-64-appsub -p BASE_DOMAIN=my-domain.com > my-cluster.yaml

oc apply -f my-cluster.yaml
```
This will create a ManagedCluster resource which the cluster-curator-controller will identify and creates a curator-job.  Check the ConfigMap for `curator-job` and `curator-job-container`. This will tell you which step (container) the job is running.  You can view the logs by combining the `curator-job` value and the `curator-job-container` value in the following command
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

The generated YAML can be committed to a Git repository. You can then use an ACM Subscription to apply the YAML (provision) on the ACM Hub.
 