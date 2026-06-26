# Parameters Packages

`pkg/parameters/` is the active parameter implementation layer for configuration management. It handles parameter metadata, schema validation, template rendering, and OpenAPI/CUE validation.

## Layout

- `core/`: core parameter types — `ComponentParameter`, `ParametersDefinition`, parameter metadata, and patch merge logic.
- `openapi/`: OpenAPI v3 schema generation and validation for parameter values.
- `validate/`: CUE-based parameter validation logic.
- `util/`: shared utilities — parameter assignment, mutation, conversion, and template rendering helpers.

## Editing Rules

- This is the active parameter layer. Do not reintroduce the removed `configmanager` or proto client paths (see `pkg/AGENTS.md`).
- Parameter validation flows: CUE validation (`validate/`) → OpenAPI schema (`openapi/`) → patch merge (`core/`). Preserve this ordering.
- `core/` types are consumed by `controllers/parameters/` — changing public types here may require controller updates.
- Patch merge logic in `core/` must handle both JSON merge patch and strategic merge patch semantics — test both.
- CUE validation in `validate/` must handle all parameter data types (string, int, float, bool, enum, array). When adding new types, extend the CUE schema generator.
- OpenAPI schema in `openapi/` is exposed via the `ParameterView` API — schema changes are user-visible API changes.
- Do not hand-edit generated mocks under `pkg/configuration/**/mocks/` — update the source interface and regenerate.

## Testing

- Add table-driven tests for CUE validation, OpenAPI schema generation, and patch merge logic.
- For `core/` type changes, add tests that verify backward compatibility of serialized parameter patches.
