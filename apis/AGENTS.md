# apis/ AGENTS.md

**Generated:** 2026-03-02
**Purpose:** CRD Type Definitions

## OVERVIEW

Defines Kubernetes Custom Resource Definitions (CRDs) for KubeBlocks. Each API group corresponds to a functional domain.

## STRUCTURE

```
apis/
├── apps/           # Core: Cluster, Component, ServiceDescriptor
├── dataprotection/ # Backup, Restore, ActionSet
├── operations/     # OpsRequest for day-2 operations
├── parameters/     # Configuration parameters
├── workloads/      # InstanceSet workloads
├── trace/          # Trace records
├── extensions/     # Extensions API
└── experimental/   # Experimental features
```

## WHERE TO LOOK

| Task | Location | Notes |
|------|----------|-------|
| Define new CRD | `apis/{group}/{version}/types_*.go` | Add to appropriate group |
| Add Cluster fields | `apis/apps/v1/cluster_types.go` | Core cluster definition |
| Add Component fields | `apis/apps/v1/component_types.go` | Component definition |
| Validation markers | `+kubebuilder:validation:*` | Use kubebuilder markers |
| Default values | `+kubebuilder:default:*` | Define defaults |

## CONVENTIONS

- Type files named `types_<resource>.go`
- Version subdirs: `v1`, `v1alpha1`, `v1beta1`
- Use kubebuilder markers for validation
- Status subresource always enabled for operators
- `zz_generated.deepcopy.go` auto-generated

## ANTI-PATTERNS

| Don't | Do Instead |
|-------|------------|
| Edit `zz_generated.deepcopy.go` | Run `make generate` |
| Skip `+genclient` marker | Required for client generation |
| Use `float` types | Use `resource.Quantity` or strings |
| Cross-import between groups | Keep groups independent |
