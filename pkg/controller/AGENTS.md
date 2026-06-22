# Controller Building Blocks

`pkg/controller/` holds reusable reconciliation building blocks: object graph modeling, plan execution, builders, scheduling, lifecycle, component synthesis, and workload helpers.

## Layout

- `builder/`: typed Kubernetes object builders and functional options.
- `model/`, `graph/`, `kubebuilderx/`: desired/current object graph modeling, transform pipelines, and reconciler scaffolding.
- `component/`: component synthesis helpers shared outside `controllers/apps/component`.
- `instance/`, `instanceset/`, `instanceset2/`, `instancetemplate/`: workload and instance reconciliation support.
- `lifecycle/`: lifecycle action execution and kbagent integration helpers.
- `plan/`: reusable plan helpers, including restore and TLS flows.
- `render/`: template rendering and built-in render functions.
- `scheduling/`, `sharding/`, `multicluster/`, `handler/`, `factory/`: shared scheduling, sharding, multicluster client, event handler, and factory helpers.

## Editing Rules

- Preserve the graph/transformer separation: transformers should describe desired object changes, while plan/reconciler code applies them.
- Builders should validate or surface errors through the existing builder error path; do not silently produce partially valid objects.
- Keep object builders and synthesis helpers deterministic. Reconcile tests often compare generated objects and depend on stable names, labels, and ordering.
- If changing InstanceSet or Instance behavior, check both legacy and newer paths (`instanceset`, `instanceset2`, and `instance`) for shared contracts before editing only one side.
- Template/render changes can affect configuration, lifecycle, and component generation; add focused render tests when changing built-ins or input semantics.
- Multicluster client changes must preserve the local controller-runtime client contract unless the caller explicitly opts into remote behavior.

## Testing

- Add table-driven unit tests for builders, graph transforms, scheduling, rendering, and instance template behavior.
- For code that mutates Kubernetes objects, test finalizers, owner refs, labels, and status/resource version assumptions explicitly.
