# Rollout Controllers

`controllers/apps/rollout/` reconciles `Rollout` resources for progressive delivery across database clusters. It uses the same transformer DAG pipeline as `component/` and `cluster/`, with transformers organized by rollout lifecycle phase.

## Layout

- `rollout_controller.go`, `rollout_plan_builder.go`: reconciler entry and plan builder.
- `transformer_rollout_init.go`, `transformer_rollout_load.go`, `transformer_rollout_meta.go`: initialization, resource loading, and metadata setup.
- `transformer_rollout_create.go`, `transformer_rollout_update.go`, `transformer_rollout_replace.go`, `transformer_rollout_inplace.go`: rollout strategy transformers — one per strategy type.
- `transformer_rollout_setup.go`, `transformer_rollout_teardown.go`, `transformer_rollout_deletion.go`: setup and cleanup flows.
- `transformer_rollout_status.go`: status aggregation and phase transitions.
- `type.go`, `scheme.go`: types and scheme registration.

## Editing Rules

- Follow the transformer chain pattern — see `controllers/apps/component/AGENTS.md` for the contract.
- One transformer per rollout phase/strategy — do not combine strategies in a single transformer.
- Rollout strategy selection happens in the plan builder; transformers should not re-select strategies.
- Status phase transitions (`transformer_rollout_status.go`) are user-visible — update status conditions and tests together.
- Teardown and deletion transformers must clean up in the correct order — preserve finalizer semantics.

## Testing

- Cover each strategy (create, update, replace, inplace) with its own test case.
- Test teardown path to ensure cleanup runs even on partial failures.
