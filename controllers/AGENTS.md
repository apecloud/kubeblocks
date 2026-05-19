# controllers/ AGENTS.md

**Generated:** 2026-03-02
**Purpose:** Kubernetes Controllers (Reconcilers)

## OVERVIEW

Implements Kubernetes controllers that reconcile CRDs to desired state. Uses controller-runtime framework with reconcile loops.

## STRUCTURE

```
controllers/
├── apps/           # Cluster & Component reconcilers (20+ subdirs)
├── dataprotection/ # Backup & restore controllers
├── operations/     # OpsRequest controllers
├── parameters/     # Configuration controllers
├── workloads/      # InstanceSet controller
├── trace/          # Trace controllers
├── extensions/     # Extension controllers
├── k8score/        # Core K8s resource wrappers
└── experimental/   # Experimental controllers
```

## WHERE TO LOOK

| Task | Location | Notes |
|------|----------|-------|
| Fix Cluster reconcile | `controllers/apps/cluster/` | Main cluster logic |
| Fix Component reconcile | `controllers/apps/component/` | Component lifecycle |
| Add new operation | `controllers/operations/` | Day-2 ops handlers |
| Fix backup issues | `controllers/dataprotection/` | Backup controllers |
| Find reconciler | `*/controller.go` or `*_controller.go` | Entry point |

## CONVENTIONS

- Controller files named `*_controller.go` or `controller.go`
- Reconcile method signature: `Reconcile(ctx, req)`
- Return `ctrl.Result{}, err` pattern
- Use `r.Client.Get()` to fetch resources
- Finalizers for cleanup logic

## ANTI-PATTERNS

| Don't | Do Instead |
|-------|------------|
| Long-running Reconcile | Return requeue with delay |
| Ignore not-found errors | Return nil for deleted resources |
| Direct status updates | Use `Status().Update()` |
| Skip owner references | Set ownerRef for GC |
| Block on external calls | Use conditions, not blocks |
