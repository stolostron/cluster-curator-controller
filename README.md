# Cluster-Curator

## Purpose
This project contains jobs and controllers for curating work around Cluster Provisioning. It is designed to extend the capabilities already present with Hive ClusterDeployment and Open-Cluster-Management ManagedCluster kinds.

## Architecture
The control found here, monitors for ManageCluster kinds.  When new instances of the resource are created, this control creates and monitors pre-job(s), post-job(s) and the deployment of clusters.
PRE_JOB -> CLUSTER_PROVISION -> POST_JOB

### Controller:
- cluster-curator-controller (ccc), watches for `ManagedCluster` kind create resource action

### Jobs (AWS)
- `applycloudprovider` takes an accessible Cloud Provider secret and creates the required Hive secrets for cluster provisioning
- `create` provisions clusters, creates the ClusterDeployment, MachinePool, ManagedCluster, KlusterletAddonConfig and install-config-secret objects
- `monitor` determines when the Hive Cluster Provisioning is complete
- `import` creates just the ManagedCluster and KlusterletAddonsConfig objects
- ansibleJobs will apply an ansibleJob, monitor it to completion, then activate a cluster deployment and monitor it to completion

### Running jobs
* Connect to an ACM hub cluster

`Hive jobs`
```bash
go run ./pkg/aws.go <job>  # Hive related jobs
```
`Ansible jobs`
```
go run ./pkg/ansible.go # Runs an AnsibleJob and then activates the ClusterDeployment and monitrs it
```



