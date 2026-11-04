# Standalone (Hive) Cluster Upgrade Guide

This document describes how to upgrade standalone OpenShift clusters (Hive-provisioned, for example vSphere or cloud IPI) using the Cluster Curator Controller with `upgradeType`.

Use this when you want a **two-stage upgrade**: control plane first, then worker nodes — the OpenShift [control-plane-only update](https://docs.openshift.com/container-platform/latest/updating/updating_a_cluster/updating-cluster-cli.html) pattern.

## Overview

For standalone clusters, worker **MachineConfigPools** stand in for Hypershift NodePools:

| `upgradeType` | Behavior on standalone clusters |
|---------------|----------------------------------|
| `""` (empty, default) | Classic upgrade: update `ClusterVersion`; workers roll with the control plane. No MCP pause/unpause. |
| `ControlPlane` | Pause worker MachineConfigPools, then update `ClusterVersion`. Pools stay paused after the job finishes. |
| `NodePools` | Unpause worker MachineConfigPools; **do not** patch `ClusterVersion`. Monitor waits until pools report updated. |

> **Important:** A paused worker pool blocks future minor upgrades and certificate rotation on those nodes. Always follow a `ControlPlane` curation with a `NodePools` curation to unpause and roll the workers.

`nodePoolNames` selects which MachineConfigPools to pause or unpause. When empty, the default pool is `worker`. The `master` pool must not be listed.

## Prerequisites

- A standalone OpenShift cluster managed by ACM/MCE (Hive `ClusterDeployment` or imported managed cluster)
- The `ClusterCurator` resource name and namespace typically match the managed cluster name/namespace
- Work manager add-on on the managed cluster (needed to patch MachineConfigPools via ManagedClusterAction)

## Major use cases

### Use case 1: Classic full upgrade (default)

Upgrade control plane and workers together in one curation. No `upgradeType` needed.

```yaml
apiVersion: cluster.open-cluster-management.io/v1beta1
kind: ClusterCurator
metadata:
  name: my-cluster
  namespace: my-cluster
  labels:
    open-cluster-management: curator
spec:
  desiredCuration: upgrade
  upgrade:
    desiredUpdate: "4.16.30"
    # upgradeType omitted = default: both control plane and workers
```

**What happens:**

1. Validates the target version against available updates
2. Patches remote `ClusterVersion` with `desiredUpdate`
3. Monitors until the cluster reports the new version
4. Does **not** pause or unpause MachineConfigPools

---

### Use case 2: Two-stage upgrade (control plane, then workers)

This is the primary new flow enabled for standalone clusters.

#### Step A — Upgrade control plane only

```yaml
apiVersion: cluster.open-cluster-management.io/v1beta1
kind: ClusterCurator
metadata:
  name: my-cluster
  namespace: my-cluster
  labels:
    open-cluster-management: curator
spec:
  desiredCuration: upgrade
  upgrade:
    desiredUpdate: "4.16.30"
    upgradeType: ControlPlane
```

**What happens:**

1. Sets `spec.paused: true` on the `worker` MachineConfigPool (or pools listed in `nodePoolNames`)
2. Patches remote `ClusterVersion` to `4.16.30`
3. Monitors until the control plane upgrade completes
4. Leaves worker pools **paused**

Verify after Step A:

```bash
# Control plane version on the hub
oc get managedclusterinfo my-cluster -n my-cluster \
  -o jsonpath='{.status.distributionInfo.ocp.version}{"\n"}'

# On the managed cluster: workers should still be paused
oc get mcp worker -o jsonpath='{.spec.paused}{"\n"}'
```

#### Step B — Roll / unpause workers

After the control plane is at the target version, change `upgradeType` to `NodePools` (same `desiredUpdate`):

```yaml
apiVersion: cluster.open-cluster-management.io/v1beta1
kind: ClusterCurator
metadata:
  name: my-cluster
  namespace: my-cluster
  labels:
    open-cluster-management: curator
spec:
  desiredCuration: upgrade
  upgrade:
    desiredUpdate: "4.16.30"
    upgradeType: NodePools
```

**What happens:**

1. Rejects the request if `desiredUpdate` is **higher** than the current control plane version
2. Does **not** patch `ClusterVersion`
3. Sets `spec.paused: false` on the worker MachineConfigPool(s)
4. Monitors until each MCP pool reports `Updated=True`

The controller detects that `upgradeType` changed and launches a new curator job automatically.

> **Tip:** Keep `desiredUpdate` equal to (or below) the control plane version already applied in Step A. A higher value fails with:
> ```
> NodePools cannot be upgraded to version X.Y.Z which is higher than the control plane version A.B.C. Upgrade the control plane first
> ```

---

### Use case 3: Two-stage upgrade for selected MachineConfigPools

If the cluster has custom pools (for example `infra` or `gpu`) in addition to `worker`, name them in `nodePoolNames`.

#### Step A — Pause specific pools, upgrade control plane

```yaml
apiVersion: cluster.open-cluster-management.io/v1beta1
kind: ClusterCurator
metadata:
  name: my-cluster
  namespace: my-cluster
spec:
  desiredCuration: upgrade
  upgrade:
    desiredUpdate: "4.16.30"
    upgradeType: ControlPlane
    nodePoolNames:
      - worker
      - infra
```

#### Step B — Unpause those pools

```yaml
apiVersion: cluster.open-cluster-management.io/v1beta1
kind: ClusterCurator
metadata:
  name: my-cluster
  namespace: my-cluster
spec:
  desiredCuration: upgrade
  upgrade:
    desiredUpdate: "4.16.30"
    upgradeType: NodePools
    nodePoolNames:
      - worker
      - infra
```

**Rules:**

| `nodePoolNames` | Behavior |
|-----------------|----------|
| Empty / omitted | Defaults to `["worker"]` |
| Explicit list | Only those MachineConfigPools are paused or unpaused |
| Includes `master` | Rejected before any pool is touched |

```text
MachineConfigPool master can not be listed in nodePoolNames, the control plane upgrade depends on it
```

---

### Use case 4: Staggered worker rollout

Upgrade the control plane once, then unpause pools in separate curations.

**1. Control plane (pause all custom worker pools):**

```yaml
spec:
  desiredCuration: upgrade
  upgrade:
    desiredUpdate: "4.16.30"
    upgradeType: ControlPlane
    nodePoolNames:
      - worker
      - infra
      - gpu
```

**2. Roll `worker` first:**

```yaml
spec:
  desiredCuration: upgrade
  upgrade:
    desiredUpdate: "4.16.30"
    upgradeType: NodePools
    nodePoolNames:
      - worker
```

**3. After `worker` is updated, roll `infra`:**

```yaml
spec:
  desiredCuration: upgrade
  upgrade:
    desiredUpdate: "4.16.30"
    upgradeType: NodePools
    nodePoolNames:
      - infra
```

Changing `nodePoolNames` (or `upgradeType`) triggers a new job.

> Pools that were paused in Step 1 and not yet listed in a `NodePools` curation remain paused until you unpause them.

---

### Use case 5: Channel or upstream only (no version)

Channel/upstream-only curations with `ControlPlane` do **not** pause workers (nothing is upgrading yet):

```yaml
apiVersion: cluster.open-cluster-management.io/v1beta1
kind: ClusterCurator
metadata:
  name: my-cluster
  namespace: my-cluster
spec:
  desiredCuration: upgrade
  upgrade:
    channel: "stable-4.16"
    upgradeType: ControlPlane
    # desiredUpdate omitted — workers are left alone
```

Version + channel together with `ControlPlane` still pauses workers when `desiredUpdate` is set:

```yaml
spec:
  desiredCuration: upgrade
  upgrade:
    desiredUpdate: "4.16.30"
    channel: "stable-4.16"
    upgradeType: ControlPlane
```

---

### Use case 6: EUS-to-EUS with control-plane-then-workers

EUS upgrades set both `intermediateUpdate` and `desiredUpdate`. Workers can be held paused across both hops so they reboot once.

#### Step A — EUS control plane (both hops, workers paused)

```yaml
apiVersion: cluster.open-cluster-management.io/v1beta1
kind: ClusterCurator
metadata:
  name: my-cluster
  namespace: my-cluster
spec:
  desiredCuration: upgrade
  upgrade:
    intermediateUpdate: "4.14.40"
    desiredUpdate: "4.16.30"
    channel: "eus-4.16"
    upgradeType: ControlPlane
    monitorTimeout: 240
```

**What happens:**

1. Pauses worker MachineConfigPools
2. Runs intermediate then final EUS control plane upgrades
3. Leaves workers paused

#### Step B — Release workers (after EUS finishes)

`upgradeType: NodePools` is **not** supported while `intermediateUpdate` is still set. Remove `intermediateUpdate` and unpause:

```yaml
apiVersion: cluster.open-cluster-management.io/v1beta1
kind: ClusterCurator
metadata:
  name: my-cluster
  namespace: my-cluster
spec:
  desiredCuration: upgrade
  upgrade:
    desiredUpdate: "4.16.30"
    upgradeType: NodePools
    # intermediateUpdate must be removed
```

If `intermediateUpdate` is still present with `NodePools`, the job fails with:

```text
upgradeType NodePools is not supported for EUS to EUS upgrades, remove intermediateUpdate to release the worker MachineConfigPools
```

---

### Use case 7: Two-stage upgrade with Ansible hooks

Hooks run around each curation job. For a two-stage flow you typically attach hooks to the stage that needs them.

```yaml
apiVersion: cluster.open-cluster-management.io/v1beta1
kind: ClusterCurator
metadata:
  name: my-cluster
  namespace: my-cluster
spec:
  desiredCuration: upgrade
  upgrade:
    desiredUpdate: "4.16.30"
    upgradeType: ControlPlane
    monitorTimeout: 180
    towerAuthSecret: toweraccess
    prehook:
      - name: Pre-Upgrade Health Check
        extra_vars:
          cluster_name: my-cluster
    posthook:
      - name: Notify Control Plane Complete
```

Then switch to workers (hooks optional):

```yaml
spec:
  desiredCuration: upgrade
  upgrade:
    desiredUpdate: "4.16.30"
    upgradeType: NodePools
    towerAuthSecret: toweraccess
    posthook:
      - name: Post-Worker Rollout Validation
```

## Field reference

### `upgradeType` (standalone)

| Value | Pauses MCPs | Updates ClusterVersion | Unpauses MCPs | Monitors |
|-------|-------------|------------------------|---------------|----------|
| `""` (default) | No | Yes | No | ClusterVersion |
| `ControlPlane` | Yes (when `desiredUpdate` set) | Yes | No (stays paused) | ClusterVersion |
| `NodePools` | No | No | Yes | MachineConfigPools |

### `nodePoolNames` (standalone)

| Context | Meaning |
|---------|---------|
| `ControlPlane` | MachineConfigPools to pause before the ClusterVersion update |
| `NodePools` | MachineConfigPools to unpause and wait on |
| Empty | Defaults to `worker` |
| Hosted cluster | Selects Hypershift NodePools (see [hosted-cluster-upgrade.md](hosted-cluster-upgrade.md)) |

### Re-trigger conditions

A new upgrade job is triggered when any of these change:

| Field | Example |
|-------|---------|
| `desiredUpdate` | `4.16.28` → `4.16.30` |
| `channel` / `upstream` | Channel switch |
| `upgradeType` | `ControlPlane` → `NodePools` |
| `nodePoolNames` | `[worker]` → `[infra]` |
| `intermediateUpdate` | EUS hop configuration |

## How it works (standalone)

```
ControlPlane curation
  └─ pause MCP(s)  →  patch ClusterVersion  →  monitor CV  →  MCP(s) remain paused

NodePools curation
  └─ validate desiredUpdate ≤ control plane version
  └─ unpause MCP(s)  →  monitor MCP Updated=True (observedGeneration caught up)
```

Default (empty `upgradeType`) skips the MCP pause/unpause path entirely.

## Viewing status

```bash
oc get clustercurator my-cluster -n my-cluster -o yaml

# Active job
oc get clustercurator my-cluster -n my-cluster -o jsonpath='{.spec.curatorJob}{"\n"}'

# Job logs
oc logs job/<curator-job-name> -n my-cluster -c upgrade-cluster
oc logs job/<curator-job-name> -n my-cluster -c monitor-upgrade
```

On the managed cluster:

```bash
oc get clusterversion version -o yaml
oc get mcp
oc get mcp worker -o yaml
```

## Troubleshooting

| Symptom / error | Likely cause | What to do |
|-----------------|--------------|------------|
| Workers still on old RHCOS after `ControlPlane` job succeeds | Expected — pools are paused | Run a `NodePools` curation |
| `NodePools cannot be upgraded to version X... higher than the control plane` | `desiredUpdate` ahead of CV | Finish `ControlPlane` first, or lower `desiredUpdate` |
| `MachineConfigPool master can not be listed in nodePoolNames` | `master` in the list | Remove `master`; only worker-type pools are allowed |
| `upgradeType NodePools is not supported for EUS...` | `intermediateUpdate` still set | Clear `intermediateUpdate`, keep `desiredUpdate`, set `upgradeType: NodePools` |
| `timed out waiting for MachineConfigPool X to update` | Pool stuck degraded / not updating | Check `oc get mcp X -o yaml` on the managed cluster; raise `monitorTimeout` if the rollout is slow |
| Channel-only `ControlPlane` left workers unpaused | By design when `desiredUpdate` is empty | Expected |

