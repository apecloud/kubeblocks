# Controllers

This tree contains controller-runtime reconcilers. Controllers should translate API state into Kubernetes objects and status while delegating reusable mechanics to `pkg/`.

## Layout

- `apps/`: core app reconcilers for clusters, components, rollouts, definitions, versions, service descriptors, sidecars, and sharding definitions.
- `workloads/`: `InstanceSet`, `Instance`, and workload event reconcilers.
- `operations/`: `OpsRequest` and operation definition reconcilers.
- `dataprotection/`: backup, restore, repository, schedule, GC, and data protection helpers.
- `parameters/`: parameter and configuration reconcilers.
- `extensions/`, `trace/`, `k8score`, `experimental/`: addon, trace, Kubernetes core wrapper, and experimental controllers.

## Reconcile Rules

- Reconcile methods should stay short enough to expose the high-level flow. Move reusable or domain-heavy logic to package helpers.
- Treat not-found root objects as successful deletion handling unless the existing controller has a specific reason to requeue.
- Do not block inside reconcile for external systems or long-running work. Record intent/status and requeue or rely on watches.
- Preserve finalizer ordering: add finalizers before creating dependent resources that need cleanup, and remove them only after cleanup succeeds.
- Status updates should be conflict-aware. Follow nearby `Status().Patch(..., client.MergeFrom(...))` patterns unless a controller deliberately uses `Status().Update()`.
- Set owner references or labels consistently so garbage collection, event handlers, and test cleanup continue to find dependent resources.

## Testing

- Keep controller tests close to the reconciler package unless the scenario requires broader envtest setup.
- When changing watches, predicates, owner references, labels, or finalizers, add tests that prove the reconcile is triggered and cleanup still works.
