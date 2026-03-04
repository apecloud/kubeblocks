# pkg/ AGENTS.md

**Generated:** 2026-03-02
**Purpose:** Shared Libraries & Utilities

## OVERVIEW

Reusable packages shared across KubeBlocks binaries. Contains controller utilities, client wrappers, and domain-specific logic.

## STRUCTURE

```
pkg/
├── controller/      # Controller building blocks
│   ├── builder/     # Resource builders
│   ├── instanceset/ # InstanceSet logic
│   └── component/   # Component utilities
├── controllerutil/  # Controller-runtime helpers
├── operations/      # Operation implementations
├── configuration/   # CUE-based config handling
├── common/          # General utilities
├── client/          # Generated K8s clients
├── constant/        # Constants
└── testutil/        # Test helpers
```

## WHERE TO LOOK

| Task | Location | Notes |
|------|----------|-------|
| Build K8s resources | `pkg/controller/builder/` | Builder pattern |
| InstanceSet logic | `pkg/controller/instanceset/` | Pod management |
| Config processing | `pkg/configuration/` | CUE templates |
| Operation logic | `pkg/operations/` | Day-2 operations |
| Shared constants | `pkg/constant/` | Reused constants |
| K8s client | `pkg/client/` | Generated clientset |
| Test mocks | `pkg/testutil/` | Test fixtures |

## CONVENTIONS

- Pure functions preferred (no side effects)
- Context as first parameter
- Error wrapping with `fmt.Errorf("...: %w", err)`
- Interface definitions for testability
- `util` suffix for helper packages

## ANTI-PATTERNS

| Don't | Do Instead |
|-------|------------|
| Import from controllers/ | Keep pkg/ dependency-free |
| Direct k8s client-go | Use controller-runtime client |
| Panic in library code | Return errors |
| Global state | Pass dependencies explicitly |
