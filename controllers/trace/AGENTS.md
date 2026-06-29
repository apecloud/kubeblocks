# Trace Controllers

`controllers/trace/` implements the `ReconciliationTrace` debug subsystem. It observes Cluster object trees during reconciliation, supports dry-run previews, and captures state changes. It is the only controller package that reuses other controllers (cluster, component, workloads, dataprotection, parameters) for dry-run plan generation.

## Layout

- `reconciliationtrace_controller.go`: main reconciler — builds a 6-step handler chain via `kubebuilderx.NewController().Prepare().Do()×6.Commit()`. Scheme registration in `init()` via `model.AddScheme()`.
- `type.go`: core types — `OwnershipRule`, `OwnedResource`, `OwnershipCriteria`, and `fullKBOwnershipRules` (the resource-tree ownership table that drives object discovery).
- Handler files (each implements `kubebuilderx.Reconciler` with `PreCondition` + `Reconcile`):
  - `resources_loader.go` (Prepare step): loads trace object + i18n ConfigMap into ObjectTree.
  - `resources_validator.go` (Do #1): validates target Cluster exists.
  - `finalizer_handler.go` (Do #2): ensures finalizer `trace.kubeblocks.io/finalizer` is present.
  - `deletion_handler.go` (Do #3): cleans up `ObjectRevisionStore` and removes finalizer on deletion.
  - `dry_run_handler.go` (Do #4): applies `Spec.DryRun.DesiredSpec` via strategic merge patch, runs dry-run plan generation.
  - `current_state_handler.go` (Do #5): computes object/event changes, writes to `ObjectRevisionStore`.
  - `desired_state_handler.go` (Do #6): evaluates state via CEL expressions, locates nearest reconcile cycle, generates desired plan.
- Infrastructure files: `plan_generator.go`, `reconciler_tree.go`, `object_tree_root_finder.go`, `object_revision_store.go`, `change_capture_store.go`, `informer_manager.go`, `i18n_resources_manager.go`, `mock_client.go`, `mock_event_recorder.go`, `util.go`.

## Editing Rules

- All handlers implement `kubebuilderx.Reconciler` (`PreCondition` + `Reconcile`) — add `var _ kubebuilderx.Reconciler = &xxxHandler{}` compile-time assertion.
- Handler chain order is fixed in `Reconcile()` — new handlers must consider finalizer/deletion short-circuit logic (`PreCondition` returns `ConditionUnsatisfied`).
- `fullKBOwnershipRules` in `type.go` is the core data table defining the Cluster→Component→InstanceSet→Pod resource tree. When adding a new tracked resource type, update this table AND `reconcilerFuncMap` in `reconciler_tree.go` AND informer watch list in `informer_manager.go`.
- `ObjectRevisionStore` is explicitly not thread-safe — use only within the single trace controller.
- Scheme registration uses `model.AddScheme(tracev1.AddToScheme)` in `init()` — no `scheme.go` file.
- `reconciler_tree.go` contains mock reconcilers for K8s native resources (pod, pvc, pv, sts, job, volumeSnapshot) and `doNothingReconciler` for resources that need no simulation (secret, service, configmap, etc.).
- Dry-run has a 5-second timeout (`plan_generator.go`) — do not increase without coordination.

## Testing

- Ginkgo/Gomega + envtest, via `suite_test.go`.
- Use `initKBOwnershipRulesForTest` to filter unsupported resources in test environment.
- Each handler should have a paired `_test.go` covering its `PreCondition` short-circuit and `Reconcile` logic.
