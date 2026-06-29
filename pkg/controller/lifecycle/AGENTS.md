# Lifecycle Package

`pkg/controller/lifecycle/` implements lifecycle action execution and kbagent integration. It runs pre/post-provision, pre/post-terminate, and member-reconfiguration actions defined in `ComponentDefinition` lifecycle hooks.

## Layout

- `lifecycle.go`: core lifecycle action orchestration and phase management.
- `lfa_component.go`, `lfa_member.go`, `lfa_replica.go`, `lfa_account.go`, `lfa_udf.go`: lifecycle action (LFA) implementations by scope — component-level, member-level, replica-level, account-level, and user-defined functions.
- `kbagent.go`: kbagent integration — sends action requests to the node-side agent.
- `errors.go`: typed lifecycle errors (e.g. action not defined, action failed, action timeout).

## Editing Rules

- Lifecycle actions are defined in `ComponentDefinition` specs — this package executes them, it does not define them.
- `kbagent.go` communicates with `pkg/kbagent/` — action request/response types must match the kbagent contract.
- Errors in `errors.go` should be sentinel errors matched with `errors.Is()`, not string comparisons.
- LFA implementations are scoped by level (component/member/replica/account/udf) — add new scopes as a new `lfa_{scope}.go` file.
- Do not block inside lifecycle execution — use timeouts and return requeue behavior for async actions.
- Preserve action ordering: pre-provision → (workload) → post-provision; pre-terminate → (workload) → post-terminate.

## Testing

- Test each LFA scope with mock kbagent responses.
- Test timeout and error requeue behavior.
- Test action ordering to ensure hooks fire in the correct sequence.
