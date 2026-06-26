# Parameters Packages

`pkg/parameters/` is the active parameter implementation layer for configuration management. It handles parameter metadata, schema validation, template rendering, and OpenAPI/CUE validation.

## Layout

- Top-level: key implementation files — `patch_merger.go`, `parameter_assignment.go`, `parameter_schema.go`, `parameter_utils.go`, `template_render.go`, `template_validate.go`, `template_merger.go`, `config_metadata.go`, `config_util.go`, `mutation.go`, `remove_marker.go`, `resource_wrapper.go`, `value_transformer.go`.
- `core/`: configuration patch and query logic — `config.go`, `config_patch.go`, `config_patch_option.go`, `config_patch_util.go`, `config_query.go`, `config_util.go`, `constraint.go`, `reconfigure_util.go`, `type.go`, `error.go`.
- `openapi/`: OpenAPI v3 schema generation from CUE — `cue_gen_openapi.go`, `flatten.go`, `runtime.go`, `utils.go`.
- `validate/`: CUE-based parameter validation — `config_validate.go`, `cue_util.go`, `cue_visitor.go`, `cuelang_expansion.go`, `utils.go`.
- `util/`: low-level utilities — `file_util.go`, `jsonpath.go`, `math.go`, `pointer.go`, `set.go`, `shell_util.go`, `unstructured.go`.

## Editing Rules

- This is the active parameter layer. Do not reintroduce the removed `configmanager` or proto client paths (see `pkg/AGENTS.md`).
- API types (`ComponentParameter`, `ParametersDefinition`, `ParameterView`) live in `apis/parameters/v1alpha1/`, not here. This package owns implementation logic only.
- Parameter validation flows: CUE validation (`validate/`) → OpenAPI schema (`openapi/`) → patch merge (top-level `patch_merger.go` + `core/config_patch.go`). Preserve this ordering.
- Patch merge logic (`patch_merger.go`, `core/config_patch.go`) must handle both JSON merge patch and strategic merge patch semantics — test both.
- CUE validation in `validate/` must handle all parameter data types (string, int, float, bool, enum, array). When adding new types, extend the CUE schema generator.
- OpenAPI schema in `openapi/` is exposed via the `ParameterView` API — schema changes are user-visible API changes.
- Do not hand-edit generated mocks under `pkg/configuration/**/mocks/` — update the source interface and regenerate.

## Testing

- Add table-driven tests for CUE validation, OpenAPI schema generation, and patch merge logic.
- For patch merge changes, add tests that verify backward compatibility of serialized parameter patches.
