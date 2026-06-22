# Shared Packages

`pkg/` contains reusable code shared by controllers and binaries. Code here should not depend on `controllers/`; it is the lower layer that controllers call into.

## Layout

- `controller/`: graph/model/reconciler helpers, builders, scheduling, lifecycle, component, instance, and InstanceSet support.
- `controllerutil/`: controller-runtime helper functions and common error handling.
- `operations/`: day-2 operation implementation logic used by OpsRequest reconciliation.
- `dataprotection/`: backup/restore/action managers and data protection utilities.
- `configuration/`, `parameters/`, `gotemplate/`: configuration rendering, parameter management, and template support.
- `client/`: generated Kubernetes clients; regenerate rather than editing by hand.
- `common/`, `constant/`, `generics/`, `apiutil/`, `unstructured/`, `viperx/`, `lru/`, `metrics/`: shared infrastructure utilities.
- `kbagent/`: shared code for the node-side agent.
- `testutil/`: envtest fixtures and typed cleanup/factory helpers.

## Editing Rules

- Keep package APIs narrow and controller-agnostic. If a helper needs reconciler-specific wiring, it probably belongs under `controllers/`.
- Return errors instead of panicking. Use wrapping that preserves the original error with `%w`.
- Pass `context.Context` first in functions that call Kubernetes, I/O, or long-running work.
- Avoid package-level mutable state unless the package already owns a clear singleton contract.
- Do not hand-edit `pkg/client/`; run `make client-sdk-gen`.
- When adding helpers used by multiple controllers, add package-level tests here rather than relying only on a controller integration test.

## Dependency Direction

`controllers/` may import `pkg/`, `apis/`, and shared utilities. `pkg/` may import `apis/` and external libraries, but must not import `controllers/`.
