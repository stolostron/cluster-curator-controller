# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Cluster-Curator-Controller is a Kubernetes controller that orchestrates cluster provisioning workflows for Hive `ClusterDeployment`, Open Cluster Management `ManagedCluster`, and Hypershift `HostedCluster`/`NodePool` resources. It creates Kubernetes Jobs that execute pre-hook Ansible, activation/monitoring, and post-hook Ansible tasks.

## Development Commands

### Building and Compilation
```bash
# Compile binaries (creates manager and curator in ./build/_output)
make compile-curator

# Build and push container image
export VERSION=1.0
export REPO_URL=quay.io/MY_REPOSITORY
make push-curator

# Full build
make build
```

### Code Generation
```bash
# Generate deep copy scaffolding
make generate

# Regenerate CRDs
make manifests
```

### Testing and Quality
```bash
# Run unit tests
make unit-tests

# Run linting
make lint

# Check copyright headers
make copyright-check

# Pre-commit validation
make compile
```

### Local Development
```bash
# Run controller locally (ensure kubectl context is set)
./build/_output/cluster-curator-controller

# Run scale tests
make scale-up-test
make scale-down-test
```

### Deployment
```bash
# Deploy to OpenShift cluster with OCM
oc apply -k deploy/controller
```

## Architecture

### Core Components

**Controller (`cmd/manager/main.go`)**
- Main controller manager that watches `ClusterCurator` resources
- Uses controller-runtime framework with leader election
- Configurable metrics endpoint (default :8080)

**Curator Job (`cmd/curator/curator.go`)**
- Binary executed inside Kubernetes Jobs created by the controller
- Handles the actual cluster provisioning workflow

**ClusterCurator Controller (`controllers/clustercurator_controller.go`)**
- Reconciles `ClusterCurator` resources
- Auto-detects cluster type (Hive vs Hypershift)
- Applies RBAC and launches curation jobs
- Handles namespace creation for Hypershift clusters

### Job Workflow (`pkg/jobs/`)

The controller creates Jobs with init containers that run sequentially:

1. **Apply Cloud Provider Secrets** (`pkg/jobs/secrets/`)
2. **Pre-hook Ansible Jobs** (`pkg/jobs/ansible/`)
3. **Activate and Monitor** (`pkg/jobs/hive/` or `pkg/jobs/hypershift/`)
4. **Post-hook Ansible Jobs** (`pkg/jobs/ansible/`)

### Key Packages

- `pkg/api/v1beta1/` - ClusterCurator CRD definitions
- `pkg/controller/launcher/` - Job creation and management
- `pkg/jobs/rbac/` - RBAC utilities for cluster namespaces
- `pkg/jobs/utils/` - Common utilities and logging
- `pkg/jobs/importer/` - ManagedCluster import functionality

### Cluster Type Detection

The controller automatically distinguishes between:
- **Hive clusters**: Uses `ClusterDeployment` with `hive.openshift.io/reconcile-pause` annotation
- **Hypershift clusters**: Uses `HostedCluster`/`NodePool` with `spec.pausedUntil: 'true'`

## Configuration

### Environment Variables
- `IMAGE_URI` - Container image for curator jobs (defaults to hardcoded value if not set)

### CRD Structure
The `ClusterCurator` CRD supports:
- `desiredCuration`: install, scale, upgrade, destroy, delete-cluster-namespace
- `install/scale/upgrade/destroy`: Hook configurations with pre/post Ansible jobs
- `providerCredentialPath`: Cloud provider credentials (format: namespace/secretName)

### RBAC Requirements
- Controller needs cluster-wide permissions for ClusterCurator resources
- Curator jobs need namespace-specific permissions applied automatically
- Hypershift clusters require additional cross-namespace RBAC

## Testing Strategy

- Unit tests focus on controller logic and job creation
- Scale tests validate controller performance under load
- Integration tests require live OpenShift cluster with OCM
- Job containers can be tested by examining logs: `oc logs job/curator-job-<suffix> -c <container-name>`

## Monitoring and Debugging

### Job Status Tracking
Monitor via `ClusterCurator` status conditions - each init container creates a condition with `status: "False"` when complete.

### Log Access
```bash
# Controller logs
oc logs -n open-cluster-management deployment/cluster-curator-controller

# Job logs (replace with actual job name and container)
oc logs job/curator-job-<suffix> -c prehook-ansiblejob
oc logs job/curator-job-<suffix> -c activate-and-monitor
oc logs job/curator-job-<suffix> -c posthook-ansiblejob
```

### Common Issues
- Missing ConfigMaps cause "No jobs to run" messages in controller logs
- RBAC failures show as Init:Error status in job pods
- Check `ClusterCurator.spec.curatorJob` field for active job name