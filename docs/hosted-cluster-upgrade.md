# Hosted Cluster Upgrade Guide

This document describes how to upgrade hosted clusters using the Cluster Curator Controller.

## Overview

The Cluster Curator Controller supports upgrading hosted clusters by:
1. Updating the cluster version (release image)
2. Updating the cluster channel
3. Updating both version and channel together

When an upgrade is triggered, the controller patches the `HostedCluster` resource and all associated `NodePool` resources with the new configuration.

## Prerequisites

- A hosted cluster managed by Open Cluster Management
- The `ClusterCurator` resource name must match the `HostedCluster` name
- The `ClusterCurator` resource must be in the same namespace as the `HostedCluster`

## Upgrade Scenarios

### 1. Version Upgrade

To upgrade to a specific OpenShift version, specify the `desiredUpdate` field:

```yaml
apiVersion: cluster.open-cluster-management.io/v1beta1
kind: ClusterCurator
metadata:
  name: my-hosted-cluster
  namespace: clusters
spec:
  desiredCuration: upgrade
  upgrade:
    desiredUpdate: "4.14.5"
```

This will:
- Patch the `HostedCluster` resource's `spec.release.image` to `quay.io/openshift-release-dev/ocp-release:4.14.5-multi`
- Patch all `NodePool` resources associated with the cluster with the same image
- Monitor the upgrade until completion

### 2. Channel-Only Update

To update only the cluster channel without changing the version:

```yaml
apiVersion: cluster.open-cluster-management.io/v1beta1
kind: ClusterCurator
metadata:
  name: my-hosted-cluster
  namespace: clusters
spec:
  desiredCuration: upgrade
  upgrade:
    channel: "fast-4.14"
```

This will:
- Validate that the channel is in the list of available channels (`status.version.desired.channels`) if available
- Patch the `HostedCluster` resource's `spec.channel` to `fast-4.14`
- Monitor until the channel is updated
- **No version change occurs**

> **Note on Channel Validation:**
> - If available channels exist in `status.version.desired.channels`, the provided channel must be in that list
> - If available channels are **not found** (e.g., the cluster's channel has not been set yet), a warning is logged and the channel update proceeds without validation
> - You can view available channels with:
>   ```bash
>   oc get hostedcluster my-hosted-cluster -n clusters -o jsonpath='{.status.version.desired.channels}'
>   ```

Common channel values:
- `stable-4.x` - Stable release channel
- `fast-4.x` - Fast release channel  
- `eus-4.x` - Extended Update Support channel
- `candidate-4.x` - Candidate release channel

### 3. Version and Channel Update

To update both the version and channel simultaneously:

```yaml
apiVersion: cluster.open-cluster-management.io/v1beta1
kind: ClusterCurator
metadata:
  name: my-hosted-cluster
  namespace: clusters
spec:
  desiredCuration: upgrade
  upgrade:
    desiredUpdate: "4.14.5"
    channel: "fast-4.14"
```

This will:
- Patch the `HostedCluster` resource's `spec.channel` to `fast-4.14`
- Patch the `HostedCluster` resource's `spec.release.image` to the new version
- Patch all associated `NodePool` resources with the new image
- Monitor until the upgrade completes

## Advanced Configuration

### Monitor Timeout

By default, the upgrade monitor waits up to 120 minutes for the upgrade to complete. You can customize this:

```yaml
apiVersion: cluster.open-cluster-management.io/v1beta1
kind: ClusterCurator
metadata:
  name: my-hosted-cluster
  namespace: clusters
spec:
  desiredCuration: upgrade
  upgrade:
    desiredUpdate: "4.14.5"
    monitorTimeout: 180  # Wait up to 180 minutes
```

### Pre and Post Hooks

You can run Ansible jobs before and after the upgrade:

```yaml
apiVersion: cluster.open-cluster-management.io/v1beta1
kind: ClusterCurator
metadata:
  name: my-hosted-cluster
  namespace: clusters
spec:
  desiredCuration: upgrade
  upgrade:
    desiredUpdate: "4.14.5"
    towerAuthSecret: my-ansible-secret
    prehook:
      - name: Pre-Upgrade Health Check
        extra_vars:
          cluster_name: my-hosted-cluster
    posthook:
      - name: Post-Upgrade Validation
```

## How It Works

### HostedCluster Resource Changes

When upgrading, the controller patches these fields on the `HostedCluster`:

**For version upgrade:**
```yaml
spec:
  release:
    image: quay.io/openshift-release-dev/ocp-release:4.14.5-multi
```

**For channel update:**
```yaml
spec:
  channel: fast-4.14
```

### NodePool Resource Changes

For version upgrades, all `NodePool` resources belonging to the cluster are patched:

```yaml
spec:
  release:
    image: quay.io/openshift-release-dev/ocp-release:4.14.5-multi
```

> **Note:** Channel is only set on the `HostedCluster` resource, not on `NodePool` resources.

### Monitoring

The controller monitors the upgrade by checking the `HostedCluster` status conditions:

**For version upgrades, all these conditions must be met:**
- `Degraded: False`
- `Available: True`
- `ClusterVersionProgressing: False`
- `ClusterVersionAvailable: True`
- `Progressing: False`
- `ClusterVersionProgressing.message` contains "Cluster version is"
- `ClusterVersionAvailable.message` contains "Done applying"

**For channel-only updates:**
- The controller verifies that `spec.channel` matches the desired channel

## Viewing Upgrade Status

Check the `ClusterCurator` status for upgrade progress:

```bash
oc get clustercurator my-hosted-cluster -n clusters -o yaml
```

Example status during upgrade:
```yaml
status:
  conditions:
  - lastTransitionTime: "2024-01-15T10:30:00Z"
    message: "Upgrade status - Working towards 4.14.5: 45% complete"
    reason: Job_has_finished
    status: "False"
    type: monitor-upgrade
```

Example status after successful upgrade:
```yaml
status:
  conditions:
  - lastTransitionTime: "2024-01-15T11:00:00Z"
    message: upgrade-job
    reason: Job_has_finished
    status: "True"
    type: hypershift-upgrade-job
```

## Troubleshooting

### View Curator Job Logs

```bash
# Get the curator job name
oc get clustercurator my-hosted-cluster -n clusters -o jsonpath='{.spec.curatorJob}'

# View upgrade logs
oc logs job/<curator-job-name> -n clusters -c upgrade-cluster

# View monitor logs
oc logs job/<curator-job-name> -n clusters -c monitor-upgrade
```

### Common Issues

1. **"Provide valid upgrade version or channel"**
   - At least one of `desiredUpdate` or `channel` must be specified

2. **"Cannot upgrade to the same version"**
   - The `desiredUpdate` version matches the current cluster version

3. **"Provided channel 'X' is not valid. Available channels: ..."**
   - The specified channel is not in the list of available channels
   - This error only occurs when the HostedCluster has available channels populated
   - Check `status.version.desired.channels` in the HostedCluster for valid options:
     ```bash
     oc get hostedcluster my-hosted-cluster -n clusters -o jsonpath='{.status.version.desired.channels}' | jq
     ```
   - If available channels are not populated yet, the channel will be set with a warning (no validation)

4. **"Timed out waiting for job"**
   - The upgrade did not complete within the `monitorTimeout` period
   - Check the `HostedCluster` status conditions for errors

### Verify HostedCluster State

```bash
# Check current version and channel
oc get hostedcluster my-hosted-cluster -n clusters -o jsonpath='{.spec.release.image}'
oc get hostedcluster my-hosted-cluster -n clusters -o jsonpath='{.spec.channel}'

# Check status conditions
oc get hostedcluster my-hosted-cluster -n clusters -o jsonpath='{.status.conditions}' | jq
```