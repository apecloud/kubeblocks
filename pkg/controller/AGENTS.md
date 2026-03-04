# pkg/controller/ AGENTS.md

**Generated:** 2026-03-02
**Purpose:** Controller Building Blocks

## OVERVIEW

Reusable controller components for building K8s resources. Provides builders, state management, and resource lifecycle utilities.

## STRUCTURE

```
pkg/controller/
├── builder/        # Resource builders
├── instanceset/    # InstanceSet controller logic
├── component/      # Component utilities
├── configuration/  # Config management
├── scheduling/     # Scheduling utilities
├── graph/          # Dependency graph
├── multicluster/   # Multi-cluster support
└── plan/           # Execution plans
```

## WHERE TO LOOK

| Task | Location | Notes |
|------|----------|-------|
| Build Deployments | `builder/` | Builder pattern |
| Build Services | `builder/` | Service builders |
| InstanceSet logic | `instanceset/` | Pod lifecycle |
| Config templates | `configuration/` | CUE processing |
| Scheduling | `scheduling/` | Affinity/Tolerations |
| Execution plans | `plan/` | Ordered operations |

## CONVENTIONS

- Builder pattern for complex objects
- Functional options pattern
- Immutable builders
- Error aggregation for builders

## ANTI-PATTERNS

| Don't | Do Instead |
|-------|------------|
| Inline object creation | Use builders |
| Modify after Build() | Create new builder |
| Ignore builder errors | Check each step |
