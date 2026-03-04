# KubeBlocks AGENTS.md

**Generated:** 2026-03-02
**Go Version:** 1.24.0
**Project:** Kubernetes Database Control Plane

## OVERVIEW

KubeBlocks is a Kubernetes operator that manages multiple database engines through unified APIs. It abstracts common database operations (lifecycle, backup, monitoring) into CRDs like `Cluster`, `Component`, and `OpsRequest`.

## STRUCTURE

```
.
├── apis/              # CRD type definitions (Cluster, Component, OpsRequest...)
├── controllers/       # Kubernetes controllers (reconcilers)
│   ├── apps/         # Cluster & Component controllers
│   ├── dataprotection/ # Backup & restore controllers
│   └── operations/   # OpsRequest controllers
├── pkg/              # Shared libraries
│   ├── controller/   # Controller building blocks
│   ├── operations/   # Operation implementations
│   └── common/       # Utilities
├── cmd/              # Main entry points
│   ├── manager/      # Main operator
│   ├── dataprotection/ # Backup operator
│   └── kbagent/      # Node agent
├── config/           # Kubernetes manifests
└── deploy/helm/      # Helm charts
```

## WHERE TO LOOK

| Task | Location | Notes |
|------|----------|-------|
| Add new CRD | `apis/{group}/{version}/` | Run `make generate manifests` after changes |
| Add controller | `controllers/{group}/` | Follow existing reconciler patterns |
| Fix reconcile bug | `controllers/*/controller.go` | Look for `Reconcile()` method |
| API type changes | `apis/*/v1*/types*.go` | Then regenerate clients |
| Build operator | `cmd/manager/` | Entry point for main controller |
| Build tools | `cmd/kbagent/`, `cmd/dataprotection/` | Separate binaries |
| Shared utilities | `pkg/common/`, `pkg/controllerutil/` | Cross-cutting concerns |
| Tests | `*_test.go` alongside source | Use `make test-fast` |

## CONVENTIONS

### Code Generation
- **MUST** run `make generate` after modifying API types (generates deepcopy)
- **MUST** run `make manifests` after API changes (generates CRD YAMLs)
- Generated clients in `pkg/client/` (run `make client-sdk-gen`)
- Skip generated files in linting: `zz_generated.*`, `pkg/client/`

### Imports
```go
// Group imports: stdlib -> third-party -> kubeblocks
import (
    "context"
    
    "k8s.io/apimachinery/pkg/types"
    "sigs.k8s.io/controller-runtime/pkg/client"
    
    appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)
```

### Testing
- Standard Go tests: `*_test.go`
- BDD tests with Ginkgo/Gomega in `test/` directory
- Mock generation with `go generate`

## ANTI-PATTERNS

| Don't | Do Instead |
|-------|------------|
| Edit `config/crd/bases/*.yaml` directly | Run `make manifests` |
| Edit `zz_generated.deepcopy.go` | Run `make generate` |
| Import controllers from pkg/ | Import only APIs and utils |
| Use `k8s.io/client-go` directly | Use controller-runtime client |
| Ignore `make build-checks` failures | Fix lint/format issues first |

## COMMANDS

```bash
# Development
make generate          # Generate deepcopy code
make manifests         # Generate CRDs
make build-checks      # fmt + vet + lint-fast
make test-fast         # Run unit tests

# Building
make manager           # Build main operator binary
make dataprotection    # Build backup operator
make kbagent          # Build node agent

# Deployment
make install          # Install CRDs to cluster
make deploy           # Deploy operator
```

## NOTES

- Controller-runtime patterns throughout
- Heavy use of CUE for configuration (see `pkg/configuration/`)
- Multiple binary outputs (not a single binary)
- Helm charts auto-generated from config/
- License header check enforced (`make check-license-header`)
