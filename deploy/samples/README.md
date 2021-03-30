# Cloud Provider Secret job
This job is meant to build out the AWS secrets needed from an AWS Cloud Provider secret stored in user namespace.

## Kuberenetes Job
### Requirements
- Cloud Provider secret in a user namespace.
- Cluster Namespace
- OCP Connection with admin rights to create roles & rolebindings
- ACM places clusters in a namespace that is the same as their name. _ie. CLUSTER_NAME=my-cluster, then the NAMESPACE=my-cluster._

### Executing
1. Edit file: `cluster-ns-rolebinding.yaml`
  - Change `CLUSTER_NAMESPACE` to the name of the cluster
  - Apply changes
  ```
  oc -n CLUSTER_NAME create -f deploy/jobs/cluster-ns-rolebinding.yaml
  ```

2. Edit file: `provider-ns-rolebinding`
  - Change `PROVIDER_SECRET_NAME` to the name of the Cloud Provider secret
  - Change `CLUSTER_NAMESPACE` to the name of the cluster
  - Apply changes
  ```
  oc -n CLOUD_PROVIDER_NAMESPACE create -f deploy/jobs/provider-ns-rolebinding.yaml
  ```

3. Edit file: `create-aws-secrets.yaml`
  - Change `CLUSTER_NAME` in `value: CLUSTER_NAME` to the cluster's name
  - Change `NAMESPACE/SECRET_NAME` in `value: NAMESPACE/SECRET_NAME` to the Cloud Provider Namespace slash the Cloud Provider secret name
  - Apply changes
  ```
  oc -n CLUSTER_NAME create -f deploy/jobs/create-aws-secrets.yaml
  ```

4. That is it. You will find three secrets created or patched (if they already existed) in the cluster's namespace.

## CLI
### Requirements
- Tested with go v1.15.2
- Cloud Provider secret in a user namespace
- Cluster Namespace
- Connection to OCP as an Admin.

### Executing
1. Setup the appropriate variables:
```bash
export PROVIDER_CREDENTIAL_PATH=USER_NAMESPACE/CLOUD_PROVIDER_SECRET_NAME
export CLUSTER_NAME=MY_CLUSTER_NAME
```
Where `USER_NAMESPACE` is the namespace where the Cloud Provider secret is found and `CLOUD_PROVIDER_SECRET_NAME` is the secret resource's name.

2. Run the program from the CLI
```bash
go run ./pkg/jobs
```
You will see output as the secrets are created or patched(if they exist)

3. (Alternative) Build a binary
```bash
go build -o set-secrets ./pkg/jobs

# This creates a binary "set-secrets", that you can run.
#  Export the same environment variables
#  Then run:
#  ./set-secrets
```

