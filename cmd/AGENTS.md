# Binaries

`cmd/` contains the three KubeBlocks binaries. Each subdirectory has a single `main.go` — there is no shared code under `cmd/`; all reusable logic lives in `pkg/`.

## Layout

- `manager/`: main control-plane binary. Registers all CRD controllers (apps, workloads, operations, extensions, experimental, trace, parameters) and webhooks. Feature-gate flags control which controller groups are enabled.
- `dataprotection/`: standalone data-protection binary. Registers backup/restore/schedule/repo/storage-provider/GC/log-collection controllers.
- `kbagent/`: node-side sidecar agent. Runs an HTTP server (not a controller-runtime manager) providing action/probe/streaming/task services for day-2 operations.

## Entry-Point Conventions

- `manager` and `dataprotection` share the same controller-runtime scaffold: `flagName` type with `String()`/`viperName()` conversion (kebab-case flags → snake_case viper keys), `init()` for scheme registration + viper defaults, `ctrl.NewManager()`, `multicluster.Setup()`, controller registration via `(&Reconciler{...}).SetupWithManager(mgr)`, `healthz`/`readyz` checks, and `ctrl.SetupSignalHandler()`.
- `kbagent` uses a different pattern: `pflag` bound directly to `server.Config`, `automaxprocs.Set()`, and `kbagent.Launch(logger, serverConfig)`. It listens on SIGTERM/os.Interrupt manually.
- Shared dependencies for all three: `pkg/viperx` (thread-safe viper), `pkg/constant` (flag keys). Controller binaries also use `pkg/controllerutil`, `pkg/controller/multicluster`, `pkg/metrics`.

## Editing Rules

- Do not add shared code under `cmd/`; put reusable logic in `pkg/`.
- When adding a new controller registration, follow the existing `featureGate` guard pattern in `manager/main.go` and register the scheme in `init()`.
- When adding a new binary, follow the `manager`/`dataprotection` pattern for controller binaries or the `kbagent` pattern for agent binaries.
- Flag keys use kebab-case (`health-probe-bind-address`); viper keys are auto-converted to snake_case via `flagName.viperName()`.
- Controller recorder names follow the `{resource}-controller` convention (e.g. `"cluster-controller"`, `"backup-controller"`).
- Leader election IDs use the format `{prefix}.kubeblocks.io` (manager: `001c317f`, dataprotection: `abd03fda`).
