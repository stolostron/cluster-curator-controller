# Hosted Cluster Upgrade Guide

This document describes how to upgrade hosted clusters using the Cluster Curator Controller.

## Overview

The Cluster Curator Controller supports upgrading hosted clusters by:
1. Updating the cluster version (release image) for both control plane and NodePools
2. Updating only the control plane (HostedCluster)
3. Updating only the NodePools (worker nodes)
4. Updating the cluster channel
5. Updating both version and channel together

When an upgrade is triggered, the controller patches the `HostedCluster` resource and/or associated `NodePool` resources based on the `upgradeType` configuration.

## Prerequisites

- A hosted cluster managed by Open Cluster Management
- The `ClusterCurator` resource name must match the `HostedCluster` name
- The `ClusterCurator` resource must be in the same namespace as the `HostedCluster`

## Upgrade Scenarios

### 1. Version Upgrade (Control Plane and NodePools)

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

#### Upgrading Control Plane and Specific NodePools

To upgrade the control plane and only specific NodePools (not all), use the `nodePoolNames` field:

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
    nodePoolNames:
      - my-nodepool-1
      - my-nodepool-2
```

This will:
- Patch the `HostedCluster` resource's `spec.release.image`
- Patch **only** `my-nodepool-1` and `my-nodepool-2` (not other NodePools)
- Monitor both the control plane and the specified NodePools until completion

This is useful when you have multiple NodePools and want to do a rolling upgrade, upgrading the control plane along with a subset of NodePools first.

### 2. Control Plane Only Upgrade

To upgrade only the HostedCluster control plane without upgrading the NodePools:

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
    upgradeType: ControlPlane
```

This will:
- Patch **only** the `HostedCluster` resource's `spec.release.image`
- **Not** patch any `NodePool` resources
- Monitor the `HostedCluster` status conditions until upgrade completes

This is useful when you want to upgrade the control plane first and then upgrade the NodePools separately.

### 3. NodePools Only Upgrade

To upgrade only the NodePools without changing the HostedCluster control plane:

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
    upgradeType: NodePools
```

This will:
- Patch **only** the `NodePool` resources' `spec.release.image`
- **Not** patch the `HostedCluster` resource's release image
- **Not** update the channel (even if specified)
- Monitor the `NodePool` status until all NodePools are ready at the desired version

#### Upgrading Specific NodePools

By default, all NodePools associated with the HostedCluster are upgraded. To upgrade only specific NodePools, use the `nodePoolNames` field:

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
    upgradeType: NodePools
    nodePoolNames:
      - my-nodepool-1
      - my-nodepool-2
```

This will:
- Upgrade **only** the specified NodePools (`my-nodepool-1` and `my-nodepool-2`)
- Skip any other NodePools associated with the HostedCluster
- Monitor only the specified NodePools until they are ready at the desired version

This is useful when:
- You want to do a rolling upgrade of NodePools one at a time
- You have multiple NodePools with different hardware profiles and want to upgrade them at different times
- You need to test the upgrade on a subset of NodePools before upgrading the rest

> **Tip:** Only include NodePools that need to be upgraded. If a NodePool is already at the target version, you don't need to include it in `nodePoolNames`. Including already-upgraded NodePools will still work (the patch is a no-op), but it's more efficient to only specify those that need upgrading.

This is useful when the control plane has already been upgraded and you want to upgrade the worker nodes.

> **Important:** The NodePools version cannot be higher than the HostedCluster control plane version. If you attempt to upgrade NodePools to a version higher than the control plane, you will receive an error:
> ```
> NodePools cannot be upgraded to version X.Y.Z which is higher than HostedCluster version A.B.C. Upgrade the control plane first
> ```

> **Note:** When `upgradeType: NodePools` is set, any `channel` value specified is ignored since channels only apply to the HostedCluster control plane.

### 4. Channel-Only Update

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

### 5. Version and Channel Update

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

### UpgradeType Field

The `upgradeType` field controls which components are upgraded:

| `upgradeType` Value | HostedCluster Upgraded | NodePools Upgraded | Channel Updated |
|---------------------|------------------------|--------------------| ----------------|
| `""` (empty/default) | Yes | Yes | Yes |
| `ControlPlane` | Yes | No | Yes |
| `NodePools` | No | Yes | No |

### NodePoolNames Field

The `nodePoolNames` field allows you to specify which NodePools to upgrade:

| `nodePoolNames` Value | Behavior |
|-----------------------|----------|
| Empty or not specified | All NodePools associated with the HostedCluster are upgraded |
| List of names | Only the specified NodePools are upgraded |

This field is:
- **Effective** when `upgradeType` is empty (default) or `NodePools`
- **Ignored** when `upgradeType` is `ControlPlane`

When `upgradeType` is empty (default) and `nodePoolNames` is specified, the control plane AND the specified NodePools are upgraded.

### Re-triggering Upgrades

The controller automatically detects when upgrade parameters change and triggers a new upgrade job. A new upgrade will be triggered when any of the following fields change:

| Field | Description |
|-------|-------------|
| `desiredUpdate` | Target version changes |
| `channel` | Cluster channel changes |
| `upstream` | Update server changes |
| `upgradeType` | Upgrade scope changes (e.g., from default to `NodePools`) |
| `nodePoolNames` | List of NodePools to upgrade changes |

**Example: Upgrading different NodePools sequentially**

1. First, upgrade the control plane and one NodePool:
```yaml
spec:
  desiredCuration: upgrade
  upgrade:
    desiredUpdate: "4.14.5"
    nodePoolNames:
      - nodepool-1
```

2. After the first upgrade completes, upgrade another NodePool by changing `nodePoolNames`:
```yaml
spec:
  desiredCuration: upgrade
  upgrade:
    desiredUpdate: "4.14.5"
    nodePoolNames:
      - nodepool-2
```

The controller detects that `nodePoolNames` changed and automatically triggers a new upgrade job for `nodepool-2`.

> **Best Practice:** Only specify NodePools that need to be upgraded. If you include a NodePool that is already at the target version (e.g., `nodePoolNames: [nodepool-1, nodepool-2]` when `nodepool-1` is already upgraded), the controller will:
> - Patch both NodePools (no-op for `nodepool-1` since it's already at the target version)
> - Monitor both NodePools (immediate success for `nodepool-1` since it's already ready)
>
> While this works correctly, it's more efficient to only specify NodePools that actually need upgrading.

**Example: Changing upgrade scope**

1. First, upgrade only the control plane:
```yaml
spec:
  desiredCuration: upgrade
  upgrade:
    desiredUpdate: "4.14.5"
    upgradeType: ControlPlane
```

2. After the control plane upgrade completes, upgrade the NodePools:
```yaml
spec:
  desiredCuration: upgrade
  upgrade:
    desiredUpdate: "4.14.5"
    upgradeType: NodePools
```

The controller detects that `upgradeType` changed and triggers a new upgrade job for the NodePools.

### HostedCluster Resource Changes

When upgrading (with `upgradeType` empty or `ControlPlane`), the controller patches these fields on the `HostedCluster`:

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

For version upgrades (with `upgradeType` empty or `NodePools`), all `NodePool` resources belonging to the cluster are patched:

```yaml
spec:
  release:
    image: quay.io/openshift-release-dev/ocp-release:4.14.5-multi
```

> **Note:** Channel is only set on the `HostedCluster` resource, not on `NodePool` resources.

### Monitoring

The controller monitors the upgrade differently based on the `upgradeType`:

**For default upgrades (both control plane and NodePools):**
- Monitors `HostedCluster` status conditions until ready
- Additionally verifies all `NodePool` resources have:
  - `status.version` matching the desired version
  - `Ready` condition is `True`
  - `UpdatingVersion` condition is `False`

**For control plane only upgrades (`upgradeType: ControlPlane`):**
- Monitors only the `HostedCluster` status conditions:
  - `Degraded: False`
  - `Available: True`
  - `ClusterVersionProgressing: False`
  - `ClusterVersionAvailable: True`
  - `Progressing: False`
  - `ClusterVersionProgressing.message` contains "Cluster version is"
  - `ClusterVersionAvailable.message` contains "Done applying"

**For NodePools only upgrades (`upgradeType: NodePools`):**
- Monitors only the `NodePool` resources until all have:
  - `status.version` matching the desired version
  - `spec.release.image` matching the expected image
  - `Ready` condition is `True`
  - `UpdatingVersion` condition is `False`

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

5. **"NodePools cannot be upgraded to version X.Y.Z which is higher than HostedCluster version A.B.C"**
   - When using `upgradeType: NodePools`, the target version cannot exceed the HostedCluster control plane version
   - Upgrade the control plane first using `upgradeType: ControlPlane`, then upgrade the NodePools

### Verify HostedCluster State

```bash
# Check current version and channel
oc get hostedcluster my-hosted-cluster -n clusters -o jsonpath='{.spec.release.image}'
oc get hostedcluster my-hosted-cluster -n clusters -o jsonpath='{.spec.channel}'

# Check status conditions
oc get hostedcluster my-hosted-cluster -n clusters -o jsonpath='{.status.conditions}' | jq
```