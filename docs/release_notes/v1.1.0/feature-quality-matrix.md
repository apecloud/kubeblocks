# KubeBlocks 1.1 Feature Quality Matrix

This matrix maps KubeBlocks 1.1 features and late-cycle fixes to release-note coverage, documentation, test evidence, and release risk.

Status legend:

* `Covered`: already represented in the current 1.1 release-note drafts.
* `Partial`: mentioned, but needs more detail or validation linkage.
* `Missing`: not yet represented in the release-note drafts.
* `N/A`: not intended for user-facing release notes unless the release owner decides otherwise.

## Summary

| Area | Release Note | Feature Guide | Test Plan | Risk | Release Decision |
| --- | --- | --- | --- | --- | --- |
| Multi-cluster support through Instance API | Covered | Covered | Covered | High | P0 validation required. |
| Rollout API | Covered | Covered | Covered | High | Keep bugfix details out of release notes; P0 validation required. |
| ComponentNetwork API | Covered | Covered | Covered | Medium | P1 validation required. |
| Dynamic InstanceSet template adoption | Covered | Covered | Covered | Medium | P1 validation required. |
| Sharding lifecycle actions | Covered | Covered | Covered | Medium | P1 validation required. |
| Cluster default resources | Covered | Missing | Missing | Medium | Included in the combined resource configuration item; validation required. |
| Addon job resources | Covered | Missing | Missing | Medium | Included in the combined resource configuration item; validation required. |
| Vertical scaling explicit zero requests | N/A | N/A | Missing | High | Keep out of release notes; regression evidence still required. |
| RebuildInstance helper PV identity | N/A | N/A | Missing | High | Keep out of release notes; regression evidence still required. |
| CUE `@iecQuantity` | N/A | N/A | Missing | Low | Keep out of user-facing release notes unless addon-author notes are added later. |
| Ops reason constants and typo backport | N/A | N/A | N/A | Low | Keep out of release notes. |
| Go toolchain and trivy maintenance | N/A | N/A | N/A | Low | Keep out of release notes. |

## Key Feature Matrix

### Multi-Cluster Support Through the Instance API

| Field | Details |
| --- | --- |
| User value | A single control-plane KubeBlocks deployment can manage database instances across multiple Kubernetes data clusters. |
| Main references | `#9697`, `#9546`, `#6951`, `#6809`, `#7083`, `#7133` |
| Release-note status | Covered in `v1.1.0.md` highlights and cluster management section. |
| Feature-guide status | Covered by `multi-cluster.md`. |
| Test-plan status | Covered by `e2e-test-cases.md` E2E-1. |
| Required validation | Create a cluster from the control cluster, verify `Instance` objects, verify pods/PVCs in data clusters, delete a data-cluster pod and confirm repair, disable one data cluster and confirm new instances avoid it. |
| Risk | High. This crosses API, manager setup, data-plane clients, status aggregation, and upgrade behavior. |
| Release action | Keep as a top highlight; record one complete multi-cluster E2E result before GA. |

### Rollout API

| Field | Details |
| --- | --- |
| User value | Users can perform controlled instance updates with in-place, replace, and create/canary strategies, including sharding rollout. |
| Main references | `#9456`, `#9522`, `#10010`, `#10009`, `#9997`, `#10230` |
| Release-note status | Covered. Main capability is covered; post-beta.6 bugfix details are intentionally excluded from user-facing release notes. |
| Feature-guide status | Covered by `rollout-api.md`. |
| Test-plan status | Covered by `e2e-test-cases.md` E2E-2. |
| Required validation | Replace rollout, create/canary rollout, sharding rollout, status transitions, PVC safety, and regression for canary templates from other rollouts. |
| Risk | High. Rollout can create and delete instances and templates; selector mistakes can affect another rollout. |
| Release action | Keep as a top highlight; require P0 regression evidence for canary-template isolation even though the bugfix detail is not listed in release notes. |

### ComponentNetwork API

| Field | Details |
| --- | --- |
| User value | Users can configure component-level host networking, explicit host ports, host aliases, DNS policy, and DNS config from the `Cluster` spec. |
| Main references | `#9892`, `#9415`, `#9986`, `#7302`, `#7381`, `#7493` |
| Release-note status | Covered in highlights and cluster management section. |
| Feature-guide status | Covered by `host-ports-specification.md`. |
| Test-plan status | Covered by `e2e-test-cases.md` E2E-3. |
| Required validation | Host network, explicit host ports, automatic host-port allocation, host aliases, DNS policy/config, and cleanup of allocated ports on component deletion. |
| Risk | Medium. Misconfiguration can create invalid pods or cause port collisions. |
| Release action | Keep as a key feature; verify examples against final Helm values and default host-port ranges. |

### Dynamic InstanceSet Template Adoption

| Field | Details |
| --- | --- |
| User value | Users can move selected existing pods into named instance templates by ordinal while preserving pod/PVC identity. |
| Main references | `#9722`, `#9475`, `#9283`, `#9360`, `#9343`, `#9174`, `#10002` |
| Release-note status | Covered in highlights and workloads section. |
| Feature-guide status | Covered by `dynamic-instance-adoption.md`. |
| Test-plan status | Covered by `e2e-test-cases.md` E2E-4. |
| Required validation | Assign ordinals to named templates, move ordinals back to the default template, preserve pod names/PVCs, stop/start clusters with flat discrete ordinals. |
| Risk | Medium. Identity preservation is critical for stateful workloads. |
| Release action | Keep as a key feature; require evidence that template reassignment does not recreate PVC identity unexpectedly. |

### Sharding Lifecycle Actions

| Field | Details |
| --- | --- |
| User value | Sharded databases can run lifecycle hooks when a sharding is provisioned/terminated or a shard is added/removed. |
| Main references | `#9830`, `#9491`, `#9417`, `#9522`, `#9180`, `#8272`, `#8276` |
| Release-note status | Covered in highlights and sharding section. |
| Feature-guide status | Covered by `sharding-lifecycle-actions.md`. |
| Test-plan status | Covered by `e2e-test-cases.md` E2E-5. |
| Required validation | `postProvision`, `preTerminate`, `shardAdd`, and `shardRemove`; shard-specific environment variables; `targetShardSelector`; failure and retry behavior. |
| Risk | Medium. Hook failures can block sharding lifecycle or leave external metadata inconsistent. |
| Release action | Keep as a key feature; validate at least one realistic addon or test fixture path. |

## Late-Cycle Feature and Fix Matrix

### Cluster Default Resources

| Field | Details |
| --- | --- |
| User value | Operators can configure default resource requests/limits for clusters or components that omit explicit resources. |
| Main references | `#10227`, commit `ce69be208` |
| Release-note status | Covered by the combined Resource Configuration Improvements entry in `v1.1.0.md`. |
| Feature-guide status | Missing. |
| Test-plan status | Missing. Unit coverage exists in `pkg/controller/factory/builder_test.go`; release validation still needs a smoke test. |
| Required validation | Install or configure Helm values with default resources; create a cluster without component resources; verify generated containers receive defaults; verify explicit component resources still win. |
| Risk | Medium. Incorrect defaulting can over-request resources or override user intent. |
| Release action | Keep in the combined resource configuration item; add validation evidence before GA. |

### Addon Job Resources Configuration

| Field | Details |
| --- | --- |
| User value | Addon installation jobs can receive configured resource requests/limits, improving scheduling and resource governance. |
| Main references | `#10223`, commit `829020472` |
| Release-note status | Covered by the combined Resource Configuration Improvements entry in `v1.1.0.md`. |
| Feature-guide status | Missing. |
| Test-plan status | Missing. Unit coverage exists in `controllers/extensions/addon_controller_test.go`; release validation still needs an addon install smoke test. |
| Required validation | Configure addon job resources through Helm values; install at least one addon; verify job pod resources; verify default behavior when values are omitted. |
| Risk | Medium. Bad resource wiring can block addon installation. |
| Release action | Keep in the combined resource configuration item; add validation evidence before GA. |

### Vertical Scaling Explicit Zero Requests

| Field | Details |
| --- | --- |
| User value | Vertical scaling respects explicit zero-value requests instead of treating them as omitted fields. |
| Main references | `#10234`, commit `2050f8527` |
| Release-note status | N/A. Excluded from user-facing release notes per release scope. |
| Feature-guide status | N/A. |
| Test-plan status | Missing. Unit coverage exists in `pkg/operations/vertical_scaling_test.go`; release validation needs a targeted operations regression. |
| Required validation | Apply vertical scaling with explicit zero request values; verify the intended request fields are preserved or cleared according to the API semantics; verify non-zero scaling still works. |
| Risk | High. Resource request handling is user-visible and affects scheduling. |
| Release action | Keep out of release notes; require regression evidence before GA. |

### RebuildInstance Helper PV Identity

| Field | Details |
| --- | --- |
| User value | RebuildInstance recovery preserves helper PV identity when reverting reclaim policy, reducing the risk of storage handoff corruption. |
| Main references | `#10191`, `#10235`, commits `24b6c316e`, `397baf0b2` |
| Release-note status | N/A. Excluded from user-facing release notes per release scope. |
| Feature-guide status | N/A. |
| Test-plan status | Missing. Unit coverage exists in `pkg/operations/rebuild_instance_test.go`; release validation needs a targeted storage regression. |
| Required validation | Run RebuildInstance with PVC/PV handoff; force or simulate rollback path; verify helper PV identity and reclaim policy are restored correctly. |
| Risk | High. Storage identity mistakes can affect data safety. |
| Release action | Keep out of release notes; require regression evidence before GA. |

### CUE `@iecQuantity` Attribute

| Field | Details |
| --- | --- |
| User value | Parameter validation can recognize IEC quantity-style values alongside Kubernetes resource quantity annotations. |
| Main references | `#10220`, commit `a4a0b8877` |
| Release-note status | N/A. Excluded from user-facing release notes per release scope. |
| Feature-guide status | N/A. |
| Test-plan status | Missing. Code change is in `pkg/parameters/validate`. |
| Required validation | Validate a parameter schema using `@iecQuantity`; verify accepted and rejected values. |
| Risk | Low. Mostly developer/addon-author facing. |
| Release action | Keep out of user-facing release notes unless addon-author notes are added later. |

### Ops Reason Constants and Typo Backport

| Field | Details |
| --- | --- |
| User value | More consistent operation reasons and corrected messages. |
| Main references | `#10216`, commit `15f226c5f` |
| Release-note status | N/A. Excluded from user-facing release notes per release scope. |
| Feature-guide status | N/A. |
| Test-plan status | N/A unless reason strings are contractually consumed by users. |
| Required validation | If reason constants are part of user-facing status, verify expected condition/event reasons. |
| Risk | Low. |
| Release action | Keep out of release notes unless downstream tooling depends on the constants. |

### Go Toolchain and Trivy Maintenance

| Field | Details |
| --- | --- |
| User value | Build and image security maintenance. |
| Main references | `#10231`, commit `0df550714` |
| Release-note status | N/A. Excluded from user-facing release notes per release scope. |
| Feature-guide status | N/A. |
| Test-plan status | N/A beyond normal build/image scan verification. |
| Required validation | Build images and run scan workflow with final artifacts. |
| Risk | Low. |
| Release action | Keep out of release notes. |

## Release-Note Updates Needed

Current `v1.1.0.md` release-note handling:

| Section | Entry |
| --- | --- |
| Highlights | Added Resource Configuration Improvements as a single user-facing item. |
| Cluster Management | Added one combined Resource Configuration Improvements entry covering cluster default resources and addon job resources. |
| Excluded by scope | Bugfix details, toolchain updates, trivy maintenance, and low-level validation changes are intentionally not listed. |

## P0 Validation Queue

These should run before any broader P1 manual validation:

1. Multi-cluster create, repair, scale, and disabled-context behavior.
2. Rollout replace, create/canary, sharding rollout, and canary-template isolation.
3. Vertical scaling explicit zero request regression.
4. RebuildInstance PVC/PV handoff and rollback regression.
5. Fresh install with final chart/images.
6. Upgrade from 1.0.x to 1.1.0 and from latest 1.1 beta to final.

## P1 Validation Queue

Run after P0 is stable:

1. ComponentNetwork host network, host ports, host aliases, DNS policy, and DNS config.
2. Dynamic InstanceSet template adoption with identity preservation.
3. Sharding lifecycle action hooks and shard-specific context.
4. Cluster default resources smoke test.
5. Addon job resources smoke test.
6. CUE `@iecQuantity` validation smoke test if included in release notes.

## Evidence Tracking Template

Use this template when recording manual results:

| Field | Value |
| --- | --- |
| Feature | TBD |
| Test case | TBD |
| Commit | TBD |
| Environment | TBD |
| Commands or CI job | TBD |
| Result | Pass/Fail |
| Logs or artifact link | TBD |
| Follow-up issue or PR | TBD |
| Release decision | Ship/Fix/Document known issue |
