

# Provisioning a Cluster
1. `oc new-project my-cluster`
2. Edit Cloud Provider RoleBinding, and add the default service account and namespace to the subjects section
```yaml
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
    name: cpv-NAME                     ## CHANGE: Name of Cloud Provider Secret
    namespace: default                 ## CHANGE: Namespace of Cloud Provider Secret
subjects:
- kind: ServiceAccount
  name: default
  namespace: cluster2
- kind: ServiceAccount
  name: default
  namespace: my-cluster                 ## CHANGE: Namespace of the new cluster
roleRef:
  kind: Role
  name: cpw-NAME
  apiGroup: rbac.authorization.k8s.io
```
3. Create the Pre/Post job configmap:
```yaml
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-cluster
  namespace: my-cluster
  labels:
    open-cluster-management: curator      ### THIS IS REQUIRED, USED TO IDENTIFY THE CONFIGMAP
data:
  providerCredentialPath: default/cloudProviderName
```
4. Create the ClusterDeployment, ManagedCluster, MachinePool and KlusterletAddonConfig resources


# Provisioning Flow
Once the resources above are created, the cluster-curator-controller will apply the appropriate RBAC to the cluster namespace and initiate a curation job.
1. The job has the same name as the cluster, ie. `my-cluster`
2. Check the job status
```bash
oc -n my-cluster get jobs
```
3. Initially there will be just the curator job. If you have included a Prehook AnsibleJob, it will be created
```bash
oc -n my-cluster get ansibleJobs  #There will be just one job that should reach status "COMPLETED"

# The ansibleJob will result in a second batchv1/job
oc -n my-cluster get jobs  
```
4. There should now be a completed batchv1/job that has completed to run the Anisble Tower Job template
5. The curator job will continue on to activate the ClusterDeployment by removing the annotation `hive.openshift.io/reconcile-pause`, it was zero up to this point
6. The curator job will then monitor the Cluster
7. If the cluster deploys successfully, then a Posthook AnsibleJob will run if it is defined.

## Job stages
* The pod for the curator job will go through 4 stages that can be seen in the `STATUS`
1. `Init: 1/4` Secrets have been applied to the Cluster namespace successfully. (`Init: 0/4` means running)
2. `Init: 2/4` Prehook AnsibleJob has completed successfully. (`Init: 1/4` means running )
3. `Init: 3/4` Successful activation of the ClusterDeployment and monitoring of provisioning. (`Init: 2/4` means running)
4. `Init: 3/4` Posthook AnsibleJob has completed successfully. (`Init: 3/4` means running)
```bash
> oc -n my-cluster get pods

NAME              READY   STATUS       RESTARTS   AGE
my-cluster-yd3j2  0/1     Init: 3/4    0          7m11s   # Stage 3 has completed, stage 4 is running
prehook-job-cltd4 0/1     Completed    0          6m55s
```

## Log commands
How to get the logs from different stages
```bash
# Change to the cluster's project
oc project my-cluster

oc logs my-cluster-yd3j2 apply-cloud-provider
oc logs my-cluster-yd3j2 prehook-ansiblejob
oc logs my-cluster-yd3j2 monitor-provisioning
oc logs my-cluster-yd3j2 posthook-ansiblejob
```



* If the ConfigMap is missing, `oc logs cluster-curator-controller` will show
```bash
2021/01/26 14:28:24 Investigate Cluster my-cluster
2021/01/26 14:28:24 No jobs to run for my-cluster/my-cluster
``` 
Any problem with the AnsibleJob, activating the ClusterDeployment or monitoring will be shown by an incomplete job.
``` bash
oc -n my-cluster get jobs
```
  * Next check the pod, you  will see `READY` as `0/1` and `STATUS` as `Init:Error`
```bash
oc -n my-cluster get pods
```
  * Now get the logs
```bash
oc -n my-cluster logs my-cluster-<suffix> apply-cloud-provider

# You should see this message:
2021/01/26 19:32:45 ansiblejobs.tower.ansible.com is forbidden: User "system:serviceaccount:my-cluster:default" cannot create resource "ansiblejobs" in API group "tower.ansible.com" in the namespace "my-cluster"
panic: ansiblejobs.tower.ansible.com is forbidden: User "system:serviceaccount:my-cluster:default" cannot create resource "ansiblejobs" in API group "tower.ansible.com" in the namespace "my-cluster"