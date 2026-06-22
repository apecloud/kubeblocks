# App Controllers

`controllers/apps/` implements the core database application reconcilers. It coordinates `Cluster`, `Component`, rollout, and definition resources by building desired object graphs and applying them through shared controller machinery.

## Layout

- `cluster/`: `Cluster` reconciliation, normalization, placement, sharding, TLS, service, restore, component status, and deletion flows.
- `component/`: `Component` reconciliation, workload generation, services, RBAC, accounts, variables, lifecycle hooks, TLS, restore, and status.
- `rollout/`: rollout reconcile planning and transformers for create/update/replace/in-place flows.
- `util/`: shared helpers for app controllers.
- Top-level `*_controller.go` files reconcile definition-like resources such as component definitions, component versions, sidecars, service descriptors, sharding definitions, and cluster definitions.

## Editing Rules

- Keep cluster-level orchestration in `cluster/` and component-level object synthesis in `component/`. Shared logic belongs in `util/` or `pkg/controller/*` only when it is reusable outside app controllers.
- The `cluster/`, `component/`, and `rollout/` packages use transformer and plan-builder flows. Add behavior as a transformer or plan step when nearby code follows that model.
- Do not inline large Kubernetes object construction in reconciler methods. Use existing builders, transformers, or `pkg/controller/model` patterns.
- Preserve labels, owner references, and finalizers used by app tests and cleanup helpers; small label drift can break watches and cascading cleanup.
- Trace/status behavior is part of the user-visible contract. If a state transition changes, update status conditions and tests together.

## Test Focus

- For cluster changes, cover normalization, component graph changes, deletion/finalizer behavior, and status updates.
- For component changes, cover generated workloads/services/RBAC plus lifecycle, restore, TLS, and variable behavior when touched.
- For rollout changes, cover the selected rollout strategy and teardown path.
