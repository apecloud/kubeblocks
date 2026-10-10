# InstanceSet Stop and resume

`InstanceSet.spec.stop` has the same runtime and storage contract with either value of `enableInstanceAPI`.
The legacy controller manages Pods directly. With the Instance API enabled, the parent controller retains Instance objects and delegates runtime execution to their controllers.

## Runtime and storage contract

| Behavior | Both paths |
| --- | --- |
| Initially stopped | Create no Pods or PVCs. Continue assistant reconciliation. |
| Stop a running workload | Remove Pods, including unready Pods, without treating Stop as scale-down. |
| Stop admission | Use `podManagementPolicy`. OrderedReady drains in reverse creation order. Parallel uses its configured concurrency. |
| Readiness and update policy | MinReadySeconds, OnDelete, role readiness, and rolling-update budgets do not prevent Stop or admission of a fresh runtime on resume. |
| PVCs during Stop | Suspend creation, metadata updates, expansion, and deletion. Preserve existing claim identity and ownership. |
| Desired changes during Stop | Keep the latest templates and assistant payloads. Defer changes to allocated instance names until resume. |
| Resume | Treat nil and false Stop as running intent. Use the latest templates and reuse matching claims. |
| Terminating Pod | Wait for removal before creating its replacement or changing its PVCs. |
| Resume admission | OrderedReady starts in creation order and waits for predecessor availability. Parallel counts admitted, unavailable runtimes against concurrency. |
| Removed claim template | Retain its allocated PVC. Actual workload deletion owns cleanup. |
| Removed instance name | After resume, apply `persistentVolumeClaimRetentionPolicy.whenScaled` to its claims, including names whose Pods disappeared during Stop. |
| Actual workload deletion | Apply `persistentVolumeClaimRetentionPolicy.whenDeleted`. ITS2 persists the current parent policy to children before requesting their deletion. |
| Stop-only revision changes | Preserve the Pod template revision. Stop does not set `ScaledDown`. |
| Completion | Report observed Pod states. Once every Pod is absent, runtime replica counts reach zero so Component and operation completion can progress. |

An Instance spec update hands off the latest desired template and clears Stop together when the parent admits resume.
The normal rolling updater cannot clear Stop on children that await admission.
After a partial commit, both controllers reload persisted resources and derive the remaining work.
There is no multi-object transaction or in-memory resume record.

## Representation differences

The legacy path has no Instance resource. ITS2 retains Instance identity, finalizers, assistants, and PVC ownership while its Pod is stopped.
Retained Instance objects do not count as running Pods.

ITS2 parent revision hashes describe desired Instance specs. Instance revision hashes describe Pod templates.
Consumers use the public convergence fields rather than compare hashes between these domains.

`InstanceStatus` preserves the shared desired/current-state contract.
Stop changes desired runtime assignments to Offline, while actual observations progress through Present, Terminating, and Absent.
Scheduling a delete does not establish Absent.

## Verification

The shared controller-entry tests in `controllers/workloads/instanceset_stop_test.go` exercise both API routes and their real Commit paths.
They inspect persisted Pods, claims, Instance objects, and public InstanceSet status after fresh controller invocations.
The cases include policy admission, template changes, partial writes, and actual deletion retention.

`controllers/workloads/instanceset_stop_envtest_test.go` runs the same API-server acceptance scenario with both values of `enableInstanceAPI`.
All registered controllers run under a manager. A held Pod finalizer exposes termination and early resume, then a replacement Pod proves the latest template and original PVC identity.

`controllers/workloads/instance_stop_test.go` covers lower-level update-action suppression and failures between assistant, Pod, PVC, and status writes.

The test commands use the repository wrapper:

```sh
/Users/leon/Workspace/github.com/apecloud/kubeblocks/scripts/codex-go-test.sh ./pkg/controller/instance ./pkg/controller/instanceset ./pkg/controller/instanceset2 -count=1
/Users/leon/Workspace/github.com/apecloud/kubeblocks/scripts/codex-go-test.sh ./controllers/workloads -count=1
```
