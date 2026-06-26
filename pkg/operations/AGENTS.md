# Operations Packages

`pkg/operations/` implements day-2 operation logic used by `controllers/operations/` for `OpsRequest` reconciliation. Each operation type (restart, upgrade, scale, switchover, backup, etc.) has its own implementation file.

## Layout

- `interface.go`: `OpsInterface` definition — each operation type implements this interface.
- `restart.go`, `upgrade.go`, `scaling.go`, `verticalscaling.go`, `volumesnapshotrestore.go`, `switchover.go`, `stop_or_start.go`, `expose.go`, `reconfigure.go`, `rebuild.go`, `promote.go`: per-operation implementations.
- `custom/`: custom operation support for `OpsDefinition`-based user-defined operations.
- `util/`: shared helpers (ops request validation, condition building, label/annotation management).

## Editing Rules

- Each operation type implements `OpsInterface` — add new operations as a new `{operation}.go` file implementing the interface, not by extending existing operation files.
- Operations should not directly mutate Kubernetes resources outside the `OpsRequest` status and the target cluster/component. Use the reconciler's client and recorder.
- Operation validation goes in `util/` — keep `OpsInterface` implementations focused on execution.
- Custom operations (`custom/`) are user-defined via `OpsDefinition` — do not hardcode engine-specific logic here.
- Return wrapped errors with `%w` — preserve error chains for the reconciler to set status conditions.
- When an operation is asynchronous (e.g. scaling, upgrade), return the appropriate requeue behavior through the interface contract.

## Testing

- Each operation implementation should have a paired `_test.go` covering validation, execution, and status transition.
- For custom operations, test the `OpsDefinition` dispatch mechanism.
