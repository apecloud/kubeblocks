# KBAgent Packages

`pkg/kbagent/` implements the node-side agent that runs as a sidecar in database pods. It provides HTTP/gRPC services for executing lifecycle actions, probes, streaming, and tasks on behalf of the control plane.

## Layout

- `server/`: HTTP server setup, request routing, and `Config` struct consumed by `cmd/kbagent/main.go`.
- `service/`: service implementations for each API (action, probe, streaming, task) — handles HTTP requests and delegates to the appropriate executor.
- `client/`: Go client for the kbagent HTTP API — used by the control plane to call agent endpoints.
- `proto/`: protocol definitions (request/response types, error codes) shared between client and server.
- `util/`: shared utilities (logging, signal handling, config parsing).

## Editing Rules

- The agent runs inside database pods — keep dependencies minimal and the binary lightweight.
- `server/Config` is the entry point for `cmd/kbagent/main.go`. When adding new server options, extend `Config` and bind via `pflag` in the cmd.
- HTTP API changes in `service/` must be reflected in `client/` and `proto/` — keep all three in sync.
- The agent communicates with the control plane via HTTP (port 3501) and streaming (port 3502). When adding endpoints, register them in the server router and add corresponding client methods.
- Lifecycle action execution integrates with `pkg/controller/lifecycle/` — action request/response types must match the lifecycle contract.
- Error codes in `proto/` should be sentinel values, not string comparisons. Use typed errors matched with `errors.Is()`.
- The agent uses `automaxprocs` — do not override `GOMAXPROCS` manually.

## Testing

- Add table-driven tests for service handlers using mock executors.
- For client methods, test against a mock HTTP server.
- For streaming, test backpressure and connection handling.
