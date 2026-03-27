# H-Scale To ITS Implementation Plan

## Goal

Move horizontal scale execution from the component controller path into the InstanceSet path, while preserving the observable behavior of `main` until the new path is proven equivalent.

This means:

- `Component` remains the user-facing intent entrypoint.
- `InstanceSet` becomes the execution owner for scale-out and scale-in.
- Existing behavior on `main` must not regress during migration.
- Existing status and observability contracts must remain compatible unless an explicit change is planned and documented.

## Non-Goals

- Redesign all lifecycle APIs at once.
- Change user-facing semantics during the migration.
- Drop existing status fields just because the internal implementation changes.

## Migration Principles

1. Migrate state ownership before action ownership.
2. Migrate scale-out before scale-in.
3. Preserve current observable behavior first; optimize later.
4. Keep `Component` as the orchestration boundary until ITS paths are fully proven.
5. Treat status compatibility as part of the contract, not as an implementation detail.
6. Any newly introduced lifecycle gate must have a complete execution path behind it.
7. Classic `InstanceSet` remains pod-centric and must not depend on `Instance` as its execution substrate.
8. `InstanceSet2` and `Instance` may inform API/state design, but they are not the rollout path for the default workload.

## Test Principles

1. Baseline test case design must be driven by functional behavior, not by the current implementation structure.
2. Test assertions should verify externally observable semantics:
   - object behavior
   - lifecycle outcomes
   - status contract
   - controller-visible failure handling
3. Test implementation is allowed to be implementation-aware and may be rewritten as the migration evolves.
4. If implementation changes but functional behavior does not, update the test implementation rather than the functional assertion.
5. Only change a baseline functional assertion when the intended product behavior changes and that change is explicitly agreed.
6. Avoid freezing incidental details that are not part of the functional contract.

## Success Criteria

- Horizontal scale execution is owned by `InstanceSet`.
- `Component` status remains correct and compatible.
- `InstanceSet.status.instanceStatus` remains complete and stable.
- Data load, member join, member leave, and switchover paths are fully wired and test-covered.
- Scale-in and deletion paths are idempotent and do not deadlock on partial failures.
- Old component-side h-scale execution logic can be removed without behavior regressions.

## Workstreams

### WS1. Behavioral Baseline

Purpose: lock down what `main` does today before migrating ownership.

Tasks:

- Add or refresh end-to-end tests for:
  - scale to zero with PVC retention policies
  - reconfigure and restart interactions during/after h-scale
  - component phase transitions during scale operations
  - `InstanceSet.status.instanceStatus` field completeness
- Record which lifecycle behaviors are already observable on `main`, and which are only available after new workload API/state is introduced:
  - scale-out without member join
  - scale-out with member join
  - scale-out with data dump and data load
  - scale-in without member leave
  - scale-in with member leave
- Explicitly cover status fields:
  - `Role`
  - `Configs`
  - `InVolumeExpansion`
- Add `Provisioned`, `DataLoaded`, and `MemberJoined` baseline coverage only after those fields become part of the agreed workload contract.
- Replace existing `PIt` coverage for target scenarios with active tests where feasible.

Exit criteria:

- We have a documented behavior baseline relative to `main`.
- Migration acceptance can be judged by test results rather than code inspection.

Status: `TODO`

### WS2. Common Lifecycle Domain Layer

Purpose: separate lifecycle semantics from controller placement without changing the classic ITS workload model.

Tasks:

- Keep lifecycle helpers reusable, but do not introduce `Instance` as a dependency of classic `InstanceSet`.
- Isolate reusable state transition helpers for:
  - provisioned
  - data loaded
  - member joined
  - member left
- Make lifecycle helper contracts explicit:
  - what inputs they need
  - what status they mutate
  - what errors are blocking vs non-blocking

Exit criteria:

- Lifecycle semantics can be reused from component, ITS, and instance paths without forking behavior.

Status: `IN_PROGRESS`

### WS3. State Downshift Into ITS

Purpose: make `InstanceSet` the source of truth for h-scale-related runtime state before moving all action execution.

Tasks:

- Define the canonical meaning of:
  - `Provisioned`
  - `DataLoaded`
  - `MemberJoined`
  - `InVolumeExpansion`
- Keep `InstanceSet.status.instanceStatus` backward-compatible.
- Preserve `Configs` population in classic ITS status reconciliation.
- Source lifecycle state from pod-centric ITS reconciliation paths rather than from `Instance` objects.
- Ensure readiness calculations that use new fields do not break existing flows.
- Add reconciliation invariants:
  - missing pod should not silently corrupt status
  - status rebuild should be idempotent

Exit criteria:

- ITS owns lifecycle state representation without regressing current status contract.

Status: `IN_PROGRESS`

### WS4. Scale-Out Execution Downshift

Purpose: move scale-out execution into ITS first.

Tasks:

- Make ITS create new replicas and advance their lifecycle state.
- Ensure data replication execution is complete, not just gated by status fields.
- Wire data dump/data load completion back into the controller-owned state machine.
- Ensure `MemberJoin` only runs after required data load is complete.
- Keep component-side behavior as a compatibility shell until ITS execution is proven stable.

Required checks:

- No path leaves replicas stuck forever in `DataLoaded=false`.
- No path marks replicas joined before prerequisites are complete.

Exit criteria:

- Scale-out behavior is ITS-owned and equivalent to `main`.

Status: `IN_PROGRESS`

### WS5. Scale-In And Deletion Downshift

Purpose: move shrink and delete safely after scale-out is stable.

Tasks:

- Define scale-in lifecycle semantics explicitly:
  - when switchover is required
  - when member leave is required
  - what happens if the pod is already gone
  - what is best-effort vs blocking
- Make delete/scale-in idempotent across retries.
- Ensure PVC retention behavior remains unchanged.
- Ensure object deletion does not deadlock when Pod state is partially lost.

Required checks:

- `Instance` deletion must not block forever just because the Pod disappeared first.
- `InstanceSet` scale-in must not wedge status or leak members on retry loops.

Exit criteria:

- Scale-in and deletion are safe under retry, stale status, and partial cluster failures.

Status: `TODO`

### WS6. Component Compatibility Shell

Purpose: keep component behavior stable while shrinking its execution role.

Tasks:

- Keep component as the intent translation layer.
- Make component status consume ITS state rather than duplicating execution logic.
- Remove dependency on old component-side replica status structures.
- Keep task event handling and env/config wiring compatible while ownership moves downward.

Exit criteria:

- Component exposes the same behavior externally while no longer owning h-scale execution.

Status: `IN_PROGRESS`

### WS7. ITS2 Convergence

Purpose: avoid accidental semantic divergence while keeping rollout ownership on classic ITS.

Tasks:

- Keep assistant object handling and headless service semantics aligned where useful.
- Reuse compatible lifecycle/state naming where it reduces cognitive overhead.
- Avoid expanding ITS2-only behavior as part of the h-scale migration, since classic ITS is the only rollout target.

Exit criteria:

- ITS and ITS2 do not diverge in core lifecycle terminology without an explicit decision, but classic ITS remains the implementation priority.

Status: `IN_PROGRESS`

### WS8. Final Cutover And Cleanup

Purpose: remove old execution paths only after the new ones are proven.

Tasks:

- Remove obsolete component-side h-scale execution logic.
- Delete old replica-status helpers and dead compatibility code.
- Update developer docs for the new ownership model.
- Validate that PR-sized cleanup does not alter the agreed external behavior.

Exit criteria:

- ITS is the only execution owner for h-scale.
- Legacy component-side execution paths are removed.

Status: `TODO`

## Immediate Gaps Already Observed

- `DataLoad` is represented in state, but workload-controller execution coverage is incomplete.
- Classic ITS status rebuilding no longer preserves all previous `InstanceStatus.Configs` behavior.
- Some delete paths remain too strict when Pod state disappears before lifecycle cleanup.
- The only valid landing zone is classic `InstanceSet`; any `Instance` or ITS2 work must be treated as non-binding exploration unless it directly serves classic ITS.

These should be treated as priority fixes during WS3 to WS5, not as polish work.

## Baseline Audit On `main` (2026-03-24)

This audit is the authoritative starting point for migration work on branch `support/hscale-to-its`.
It supersedes earlier observations made on top of `support/hscale-in-its`.

### Component Controller

Current baseline on `main` is relatively strong for user-facing h-scale semantics.

Covered:

- change replicas to match desired count
- scale-in to zero
- scale-in to zero with min replicas limit
- scale-out from 1 to 3
- scale-in from 3 to 1
- scale-in to zero with PVC retention expectation
- h-scale with data actions
- h-scale on stopped component
- h-scale on stopped component with data actions

Assessment:

- `Component` baseline is good enough to serve as the top-level semantic guardrail.
- No immediate need to rewrite this layer before starting migration.

### Classic `InstanceSet` Controller

Current baseline on `main` covers:

- generic provision behavior
- PVC retention policy on delete and scale-in
- reconfigure flows

Current baseline on `main` does not yet cover:

- richer scale-out and scale-in acceptance scenarios beyond generic replica count changes
- lifecycle-gated readiness semantics
- any membership or data-load semantics, because they are not part of the classic ITS workload API on `main`
- any `Provisioned`, `DataLoaded`, or `MemberJoined` status contract, because those fields do not exist in classic ITS status on `main`

Assessment:

- This controller still matters for backward compatibility, but it cannot be used as the first membership/data-load baseline anchor.
- WS1 should keep its existing observable behavior covered, while treating membership/data-load as post-downshift coverage.

### `InstanceSet2` Controller

Current baseline on `main` covers:

- provision status
- instance status role propagation
- ordered ready pod management
- rolling update
- basic scale-in and scale-out
- PVC retention on scale-in

Current baseline on `main` does not yet cover:

- membership/data-load semantics as a workload contract, because the public workload API on `main` does not yet expose these actions in a testable way
- joined vs unjoined semantics during shrink
- lifecycle-driven readiness gating

Assessment:

- `InstanceSet2` already has a usable operational baseline, but not the lifecycle baseline needed for downshift work.
- It is a better landing zone for post-downshift lifecycle baseline than classic ITS, but those cases should be added only once the workload API/state is actually present on the branch.

### `Instance` Controller

Current baseline on `main` covers:

- create/delete
- readiness and role status
- PVC retention
- recreate/in-place update
- switchover

Current baseline on `main` does not yet cover:

- membership and data-load semantics, because the current `Instance` workload API/status on `main` does not expose those fields
- reconfigure

Assessment:

- `Instance` should remain a supporting baseline layer.
- Membership/data-load cases belong after the new workload contract lands, not before.

### Recommended WS1 Order

1. Lock and document `Component` user-facing h-scale behavior as the top-level semantic baseline
2. Preserve classic `InstanceSet` observable contracts that already exist on `main`, especially `Configs`, PVC policy, reconfigure, and generic replica transitions
3. Introduce new workload lifecycle baseline only in the same PRs that add the corresponding workload API/state contract
4. Prefer `InstanceSet2` plus `Instance` as the first place to add membership/data-load assertions after the contract exists
5. Only add new `Component` cases when a migration step exposes a user-visible semantic gap not already covered

## Suggested PR Breakdown

### PR1. Baseline And Contracts

- Add or unskip core h-scale behavior tests.
- Document status compatibility expectations.
- Restore any unintentionally dropped observable status fields.

### PR2. ITS State Ownership

- Finish ITS lifecycle state composition.
- Keep component behavior unchanged.
- Prove status compatibility.

### PR3. ITS Scale-Out Execution

- Move data load and member join execution fully under ITS.
- Keep component as compatibility shell.

### PR4. ITS Scale-In And Delete Execution

- Move member leave and switchover semantics under ITS.
- Make failure handling and retry behavior explicit.

### PR5. Component Cleanup

- Remove old component-side execution logic.
- Keep status translation only.

### PR6. ITS2 Alignment And Final Cleanup

- Reconcile ITS2 semantics.
- Remove leftover duplicated code and docs drift.

## Tracking Table

| ID | Workstream | Owner | Status | Notes |
| --- | --- | --- | --- | --- |
| WS1 | Behavioral Baseline | TBD | TODO | Lock down `main` behavior before more migration |
| WS2 | Common Lifecycle Domain Layer | TBD | IN_PROGRESS | `Replica` abstraction already introduced |
| WS3 | State Downshift Into ITS | TBD | IN_PROGRESS | Needs status compatibility cleanup |
| WS4 | Scale-Out Execution Downshift | TBD | IN_PROGRESS | Data load path still incomplete |
| WS5 | Scale-In And Deletion Downshift | TBD | TODO | Need explicit degraded-path semantics |
| WS6 | Component Compatibility Shell | TBD | IN_PROGRESS | Component still partially coupled |
| WS7 | ITS2 Convergence | TBD | IN_PROGRESS | Headless service and assistant objects moved |
| WS8 | Final Cutover And Cleanup | TBD | TODO | Only after parity is proven |

## Update Rules

When updating this file:

- Change workstream status only when exit criteria materially move.
- Record behavior regressions under "Immediate Gaps Already Observed".
- Prefer adding evidence in terms of tests or explicit semantic decisions.
- Do not mark cleanup complete before compatibility is proven.
