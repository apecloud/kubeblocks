# Component Synthesis Package

`pkg/controller/component/` provides component synthesis helpers shared outside `controllers/apps/component/`. It builds component objects, manages versions, handles sidecars, and integrates with kbagent for task events.

## Layout

- `component.go`, `synthesize_component.go`: core component synthesis logic.
- `component_version.go`: component version resolution and management.
- `available.go`, `replicas.go`: availability and replica computation.
- `sidecar.go`: sidecar container injection.
- `service_reference.go`: service reference resolution.
- `kbagent.go`, `kbagent_task_event.go`: kbagent integration for lifecycle task events.
- `mock_reader.go`: mock reader for testing.

## Editing Rules

- This package is shared by multiple controllers — keep APIs narrow and controller-agnostic.
- Component synthesis must be deterministic — reconciler tests compare full object graphs.
- `kbagent_task_event.go` integrates with `pkg/kbagent/` — keep the event type contract in sync.
- Version resolution (`component_version.go`) interacts with `apis/apps/v1/ComponentVersion` — preserve backward compatibility when changing resolution logic.
- Do not import from `controllers/` — this is the lower shared layer.

## Testing

- Add table-driven tests for synthesis, version resolution, and replica computation.
- Mock the reader using `mock_reader.go` for unit tests.
