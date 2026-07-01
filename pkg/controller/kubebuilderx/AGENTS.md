# KubeBuilderX Framework

`pkg/controller/kubebuilderx/` is a next-generation reconciliation framework that separates reconciliation into a pure computation phase (generating an execution plan) and an execution phase (applying changes). It uses an ObjectTree abstraction for object snapshots.

## Layout

- `doc.go`: framework documentation — describes the two-phase model and rationale.
- `controller.go`: controller scaffolding integrating with controller-runtime.
- `reconciler.go`: reconciler interface and base implementation.
- `plan_builder.go`: execution plan builder for the computation phase.
- `utils.go`: framework utilities.

## Editing Rules

- This is an **early-stage** framework (per `doc.go`) — changes here affect all controllers adopting it.
- Preserve the two-phase separation: the computation phase must not perform API calls; the execution phase applies the plan.
- The ObjectTree snapshot is the source of truth during computation — do not bypass it with direct client calls.
- When adding plan operations, extend `plan_builder.go` rather than the reconciler.
- Keep the reconciler interface minimal — controllers should implement it, not extend it.

## Testing

- Test plan builders with mock ObjectTrees — verify computation without side effects.
- Test execution phase separately with a fake client.
