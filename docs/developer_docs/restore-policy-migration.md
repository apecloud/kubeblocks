# Migrating Serial restores

Restore execution supports only parallel volume recovery. Omit the restore policy
setting or use `Parallel`. `Serial` is deprecated and rejected by the controllers.
Existing API fields, CRD enums, and the `Parallel` default remain available so that
old objects can still be read, updated, and deleted. An API server accepting an
object does not mean its restore policy passed controller validation.

Check these configuration locations:

- `Restore.spec.prepareDataConfig.volumeClaimRestorePolicy`, including Restores
  referenced by a PVC's `dataSourceRef`.
- `OpsRequest.spec.restore.volumeRestorePolicy` and the reserved policy parameter
  in `OpsRequest.spec.restore.parameters`.
- `Cluster.spec.restore.parameters[dataprotection.kubeblocks.io/volume-restore-policy]`.
- The same reserved parameter in a PVC's existing restore parameters annotation.

Allow in-progress Serial operations to finish before upgrading when possible.
After upgrading, an unfinished Serial operation starts no further restore Jobs or
actions. Already-created Jobs continue with their existing retry behavior. When
their restore containers exit, the controller stops their restore-manager
containers and waits for the Jobs to finish. It then reports the overall operation
as failed because Serial is unsupported. A pending Job can keep this operation
waiting; existing cancellation and deletion procedures still apply.

Completed historical results remain unchanged. The controller does not delete
target or helper data PVCs/PVs because of policy rejection, roll back completed
instances, or automatically retry with Parallel. Some members of a failed
operation may already have completed recovery.

Submit a new request without the policy or with `Parallel`. Use new recovery
targets for incomplete data; restore scripts are not guaranteed to support
replaying into partially restored volumes. `Cluster.spec.restore` remains
immutable. For scaling or rebuilding, select only the remaining targets rather
than repeating already-completed members.

Parallel recovery retains restore-manager to preserve the existing scheduling
protection between recovery Pods. It does not change business Pod
`OrderedReady` behavior, backup-chain dependencies, or action ordering.
