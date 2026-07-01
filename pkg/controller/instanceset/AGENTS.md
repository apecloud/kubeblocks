# InstanceSet Package

`pkg/controller/instanceset/` implements the core InstanceSet workload API — an enhanced StatefulSet with role-based updates, member reconfiguration, and in-place updates. It is the primary workload abstraction used by component controllers.

## Layout

- `doc.go`: package documentation describing the 7 key features (role update strategy, access mode, member reconfiguration, multi-template, designated pod deletion, in-place update).
- `types.go`: core types and constants (roles, update strategies, access modes).
- `reconciler_*.go`: split reconciler files by concern — `api_version`, `assistant_object`, `deletion`, `fix_meta`, `instance_alignment`, `revision_update`, `status`, `update`.
- `instance_util.go`, `in_place_update_util.go`: instance and in-place update helpers.
- `object_builder.go`: InstanceSet object construction.
- `tree_loader.go`: object tree loading for reconciliation.
- `revision_util.go`: revision computation and comparison.
- `update_plan.go`: update plan generation for rolling updates.

## Editing Rules

- Read `doc.go` before editing — it documents the 7 features and their interactions.
- Reconciler logic is split across `reconciler_*.go` files by concern — add new reconciler steps as a new `reconciler_{concern}.go` file, not by expanding existing ones.
- In-place update and revision helpers are mirrored across `instanceset/` and `instance/` — preserve their observable ordering and status assumptions when moving logic between packages.
- Check both `instanceset/` and `instanceset2/` for shared contracts before editing only one side — they represent legacy and newer code paths.
- `tree_loader.go` builds the object tree that reconcilers depend on — changes to loading order affect all downstream reconciler steps.
- Status assumptions (`reconciler_status.go`) are consumed by component controllers — preserve phase and condition semantics.

## Testing

- Add table-driven unit tests for reconciler steps, revision computation, and update plans.
- For in-place updates, test both the happy path and partial-failure requeue scenarios.
