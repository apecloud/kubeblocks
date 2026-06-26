# Operations Packages

`pkg/operations/` implements day-2 operation logic used by `controllers/operations/` for `OpsRequest` reconciliation. Each operation type (restart, upgrade, scale, switchover, backup, etc.) has its own implementation file.

## Layout

- `type.go`: operation type definitions and the `OpsInterface` that each operation type implements.
- `ops_comp_helper.go`: `ComponentOpsInterface` and shared component-operation helpers.
- `ops_manager.go`, `ops_runtime.go`, `ops_util.go`, `ops_progress_util.go`, `queue_util.go`: ops request lifecycle, validation, progress tracking, and queue management.
- `restart.go`, `upgrade.go`, `horizontal_scaling.go`, `vertical_scaling.go`, `volume_expansion.go`, `switchover.go`, `start.go`, `stop.go`, `expose.go`, `reconfigure.go`, `rebuild_instance.go`, `rebuild_instance_inplace.go`, `restore.go`, `backup.go`: per-operation implementations.
- `custom/`: custom operation support for `OpsDefinition`-based user-defined operations (`action.go`, `action_exec.go`, `action_workload.go`, `workload_job.go`, `workload_pod.go`).
- `util/`: shared helpers (`common_util.go`).

## Editing Rules

- Each operation type implements `OpsInterface` — add new operations as a new `{operation}.go` file implementing the interface, not by extending existing operation files.
- Operations should not directly mutate Kubernetes resources outside the `OpsRequest` status and the target cluster/component. Use the reconciler's client and recorder.
- Operation validation and progress tracking go in `ops_util.go` / `ops_progress_util.go` — keep operation implementations focused on execution.
- Custom operations (`custom/`) are user-defined via `OpsDefinition` — do not hardcode engine-specific logic here.
- Return wrapped errors with `%w` — preserve error chains for the reconciler to set status conditions.
- When an operation is asynchronous (e.g. scaling, upgrade), return the appropriate requeue behavior through the interface contract.

## Testing

- Each operation implementation should have a paired `_test.go` covering validation, execution, and status transition.
- For custom operations, test the `OpsDefinition` dispatch mechanism.
