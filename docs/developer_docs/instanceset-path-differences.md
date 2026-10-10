# InstanceSet controller path differences

This comparison covers `InstanceSet -> Pod` and `InstanceSet -> Instance -> Pod`.
The Stop and resume contract is described in [InstanceSet Stop and resume](instanceset-stop-parity.md).
This change verifies that contract through both controller entries and API-server tests.

The remaining rows come from source review. They require focused behavioral reproduction or a supported migration requirement before implementation.
Support for the same API fields does not establish equivalent behavior for every combination.

## Shared feature families

Both paths use the shared instance-template and name builders for default, named, compressed, discrete, and ranged templates.
Both provide OrderedReady and Parallel provisioning, PVC expansion, role observation, rolling updates, in-place updates, configuration actions, and deletion retention policies.
ITS2 introduces an additional spec handoff and runtime-observation step, which can affect admission and recovery.

## Follow-up differences

| Area | Difference found in source | Follow-up acceptance |
| --- | --- | --- |
| Pending Pod correction | Legacy deletes a changed Pending Pod before readiness admission. ITS2 requires readiness before handing the corrected spec to its Instance. | A template correction recovers a permanently Pending Pod through both real controller entries. |
| OnDelete | Legacy recreation reads the current ITS template. ITS2 recreation reads the persisted child spec, whose handoff still uses readiness and convergence budgets. | Change several replicas' templates, then manually delete each Pod. Every replacement uses the current template without another replica's pending convergence blocking spec delivery. |
| Flat ordinal allocation | Legacy revision reconciliation persists `AssignedOrdinals`. ITS2 uses the shared builder but has no corresponding production write. | Named-template expansion, removal, and reassignment preserve the supported ordinal allocation contract across retries and restart. |
| Peer-dependent actions | Legacy lifecycle construction receives all ITS Pods. Instance switchover and reconfiguration currently receive the child's Pod. | An action that selects another available member receives the required peer context through the public lifecycle API. |
| Retained PVC adoption | Legacy explicitly takes over an external claim without a controller reference. Instance merging has no equivalent owner-reference takeover. | Retire and recreate an identity under Retain, then verify claim UID, ownership, and subsequent deletion behavior. |
| Parent pause | Pausing ITS stops parent work. An already admitted Instance continues through its independent controller. | Define whether pause covers the subtree or only admission, then exercise an in-flight update under that contract. |
| One-shot node relocation | ITS2 validation explicitly rejects `NodeSelectorOnce`; legacy supports it. | Define the supported scheduling interface before enabling equivalent relocation for ITS2. |
| Deferred service-account migration | Legacy has proposed and in-use service-account handling. The new path has no matching mechanism. | Establish the released-version upgrade path, then verify replacement and live-Pod behavior for that path. |
| Deferred kbagent init migration | Legacy normalizes the old init command and preserves the live command during update. Instance does not use those helpers. | Establish the required upgrade path, then verify revision stability and replacement behavior. |

The Pending, OnDelete, and flat-ordinal cases affect ordinary workload execution.
The migration rows depend on released versions and supported upgrades; an earlier state of an unmerged PR is not a compatibility requirement.
These differences remain outside the Stop and resume change.

## Intentional representation differences

ITS2 retains Instance resources while stopped, supports additional assistant kinds, and routes Instance events across clusters.
Its parent revision hashes describe Instance specs, while child hashes describe Pod templates.
These representations do not need identical object counts or hash strings to satisfy the same runtime lifecycle contract.

## Source locations

The comparison follows `controllers/workloads/instanceset_controller.go`, `controllers/workloads/instanceset_controller_2.go`, and `controllers/workloads/instance_controller.go`.
The relevant owners are `pkg/controller/instanceset`, `pkg/controller/instanceset2`, `pkg/controller/instance`, and the shared `pkg/controller/instancetemplate` package.
The follow-up reproductions must exercise these entries and their Commit paths rather than infer recovery from a helper.
