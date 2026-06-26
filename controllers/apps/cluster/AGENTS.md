# Cluster Controllers

`controllers/apps/cluster/` reconciles `Cluster` resources by normalizing spec, computing placement and sharding, injecting restore intent, aggregating component status, and managing deletion. It uses the same transformer DAG pipeline as `component/`.

## Layout

- `cluster_controller.go`, `cluster_plan_builder.go`: reconciler entry and plan builder.
- `transformer_cluster_*.go`: 13 transformers covering init, deletion, normalization, placement, sharding, sharding-account, service, component, component-status, status, termination-policy.
- `restore_intent.go`: cluster restore intent annotation handling, consumed by dataprotection restore code.
- `multicluster.go`: multi-cluster placement logic.
- `cluster_status_conditions.go`: status condition helpers.
- `scheme.go`, `utils.go`, `suite_test.go`.

## Editing Rules

- Follow the transformer chain pattern — see `controllers/apps/component/AGENTS.md` for the contract.
- Restore intent is a cross-package contract: changes to annotation keys or values in `restore_intent.go` must be coordinated with `controllers/dataprotection/` restore code. Test cross-namespace and default-namespace cases.
- Cluster normalization (`transformer_cluster_normalization.go`) runs early in the chain and fills defaults that downstream transformers depend on — do not skip it.
- Component status aggregation (`transformer_cluster_component_status.go`) reads status from child `Component` objects — preserve the label/selector contract that links clusters to components.
- Multi-cluster placement (`multicluster.go`, `transformer_cluster_placement.go`) interacts with `pkg/controller/multicluster` — preserve the local client contract unless explicitly opting into remote behavior.
- Status updates use `Status().Patch(..., client.MergeFrom(...))` — keep conflict-safe patch patterns.

## Testing

- Cover normalization, component graph changes, deletion/finalizer behavior, status updates, and sharding scenarios.
- For restore intent changes, add tests for both cross-namespace and same-namespace restore flows.
