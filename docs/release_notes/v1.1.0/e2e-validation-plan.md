# KubeBlocks 1.1.0 E2E Validation Plan

## Scope

Detailed, code-grounded test case specifications (objectives, steps, exact observables) are maintained in [e2e-test-cases.md](./e2e-test-cases.md); this plan tracks execution against release artifacts.

Validate user-facing v1.1.0 features against the official v1.1.0 image and chart. The final release tag does not exist in this workspace yet; execute this plan after publishing `v1.1.0`. Pre-release smoke runs may use `v1.1.0-beta.6`, but they do not replace final validation because `release-1.1` has commits after that tag.

## Execution Commands

Use GitHub Actions workflows from this repository:

```bash
gh workflow run e2e-kbcli.yml -f VERSION=v1.1.0 -f TEST_TYPE=mysql -f CLOUD_PROVIDER=aks -f CLUSTER_VERSION=1.33 -f INSTANCE_TYPE=amd64
gh workflow run e2e-kbcli.yml -f VERSION=v1.1.0 -f TEST_TYPE=redis-cluster -f CLOUD_PROVIDER=aks -f CLUSTER_VERSION=1.33 -f INSTANCE_TYPE=amd64
gh workflow run e2e-kbcli.yml -f VERSION=v1.1.0 -f TEST_TYPE=mongodb-shard -f CLOUD_PROVIDER=aks -f CLUSTER_VERSION=1.33 -f INSTANCE_TYPE=amd64
gh workflow run e2e-fault.yml -f VERSION=v1.1.0 -f TEST_TYPE=mysql -f CLOUD_PROVIDER=gke -f CLUSTER_VERSION=1.32 -f INSTANCE_TYPE=amd64
```

Use `TEST_TYPE=apecloud-mysql|redis-cluster|mongodb-shard|postgresql` for the selected feature matrix. Run the full release chart matrix from `.github/workflows/release-helm-chart.yml` before publishing GA.

## Feature Matrix

| Feature | Scenario | Expected result | Status |
| --- | --- | --- | --- |
| Multi-cluster support through the Instance API | Install KubeBlocks with multi-cluster kubeconfig/context settings and create a multi-cluster component using the Instance API | A single control plane places instances into the expected data-cluster contexts; local reconciliation remains healthy | Pending final tag |
| Multi-cluster Service references | Create a multi-cluster component that needs Service object references | Service references resolve correctly; reconciliation does not fail with missing Service kind handling | Pending final tag |
| Multi-cluster external managed templates | Use externally managed file templates in multi-cluster mode | External managed templates synchronize correctly and do not create duplicate or stale generated config | Pending final tag |
| Rollout for cluster instances | Create a cluster, apply `Rollout` targeting replacement, create strategy, and canary-style update paths | `Rollout.status.state` succeeds; target instances are updated; stable service remains available | Pending final tag |
| Rollout for sharding | Apply `Rollout` to a sharding cluster | Sharding rollout progresses shard-by-shard and reports detailed rollout status | Pending final tag |
| ComponentNetwork host ports | Create cluster with `spec.componentSpecs[*].network` host port mapping | Pods expose requested host ports; duplicate or invalid host-port assignments are rejected | Pending final tag |
| ComponentNetwork host aliases and DNS | Create cluster with component-level host aliases, DNS policy, and DNS config | Pod spec contains requested network settings; database pods become ready | Pending final tag |
| Dynamic InstanceSet Template adoption | Assign existing pods to named instance templates by ordinal | Selected pods adopt the requested template while preserving pod identity and PVC data | Pending final tag |
| Dynamic InstanceSet Template `serviceVersion` and `compDef` | Tune selected instances with template-level `serviceVersion` or `compDef` | Only selected instances use the heterogeneous template settings; other instances remain unchanged | Pending final tag |
| Sharding lifecycle action | Create a sharding engine cluster with lifecycle hooks and trigger shard add/remove behavior | Sharding lifecycle action runs once per targeted shard event; action status is visible | Pending final tag |
| Scale in specified shard | Scale in one named shard in a sharding cluster | Only the selected shard changes; other shards remain healthy and unchanged | Pending final tag |
| Heterogeneous shards via shard templates | Create a sharding cluster with `shardTemplates` defining a differently-sized shard group | Template shards carry the shard-template label and per-group settings; base shards are unaffected | Pending final tag |
| Volume sharing among instances | Create a cluster with `volumeClaimTemplates[*].persistentVolumeClaimName` | PVCs follow the `<prefix>-<ordinal>` naming and identity is preserved across template adoption | Pending final tag |
| KubeBlocks 1.0.x → 1.1 upgrade | Install latest 1.0.x GA, create clusters, apply `kubeblocks_crds.yaml`, then `helm upgrade` to 1.1.0 | No database pod restarts; new CRDs installed; existing clusters stay Running; 1.1 features usable post-upgrade | Pending final tag |

## Release Blocking Criteria

* All release-blocking workflow runs finish successfully for the final `v1.1.0` artifacts.
* Every feature matrix item has either automated evidence or a documented manual run with command output.
* No failed rollout, sharding, multi-cluster, dynamic-instance-template, host-port, or pod-DNS scenario remains unresolved.
* Selected v1.1 feature scenarios are validated against final chart, CRD, and image artifacts.

## Current Execution Status

Not executed in this session. The official final `v1.1.0` tag/artifacts are not present yet, and the existing latest pre-release tag `v1.1.0-beta.6` does not include the current `release-1.1` head.
