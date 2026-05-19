# controllers/apps/ AGENTS.md

**Generated:** 2026-03-02
**Purpose:** Core Cluster & Component Controllers

## OVERVIEW

Implements the core Cluster and Component lifecycle controllers. Handles creation, update, scaling, and deletion of database clusters.

## STRUCTURE

```
controllers/apps/
├── cluster/        # Cluster reconciler
├── component/      # Component reconciler
├── configuration/  # Config handling
├── trace/          # Cluster trace
├── operations/     # Internal operations
├── transformer/    # Object transformers
├── lifecycle/      # Lifecycle utilities
├── scheduling/     # Scheduling logic
├── utils/          # App utilities
└── ...
```

## WHERE TO LOOK

| Task | Location | Notes |
|------|----------|-------|
| Fix Cluster reconcile | `cluster/` | Main reconciler |
| Fix Component reconcile | `component/` | Component logic |
| Transform objects | `transformer/` | K8s object transforms |
| Scheduling | `scheduling/` | Pod scheduling |
| Lifecycle hooks | `lifecycle/` | Pre/post hooks |
| Configuration | `configuration/` | Config rendering |
| Trace events | `trace/` | Cluster trace records |

## CONVENTIONS

- Separate concerns: cluster vs component
- Use transformers for K8s object generation
- Scheduling logic isolated in `scheduling/`
- Configuration via CUE templates

## ANTI-PATTERNS

| Don't | Do Instead |
|-------|------------|
| Mix cluster/component logic | Keep separate reconcilers |
| Generate objects inline | Use transformer pattern |
| Skip trace recording | Record all state changes |
