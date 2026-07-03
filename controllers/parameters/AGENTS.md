# Parameter Controllers

`controllers/parameters/` reconciles parameter and configuration resources. It handles both the new `ComponentParameter`/`ParameterView`/`ParametersDefinition` API and legacy `Parameter`/`ParamConfigRenderer` controllers that are being phased out.

## Layout

- `componentparameter_controller.go`, `parameterview_controller.go`, `parametersdefinition_controller.go`: new API controllers.
- `componentdrivenparameter_controller.go`, `parametertemplateextension_controller.go`: controllers that bridge component-driven parameter flows and template extensions.
- `legacy_parameter_controller.go`, `legacy_paramconfigrenderer_controller.go`, `reconfigure_controller.go`: legacy parameter and reconfigure controllers.
- `reconfigure/`: sync/restart helpers for reconfigure operations (`sync.go`, `restart.go`, `auto.go`, `legacy_compat.go`).
- `revision.go`, `utils.go`, `parameterview_markerline.go`, `componentparameter_controller_utils.go`: shared helpers for revision tracking, marker handling, and controller support.
- `suite_test.go`: Ginkgo test suite setup for the package.

## Editing Rules

- `pkg/parameters` is the active parameter implementation layer. Do not reintroduce the removed `configmanager` or proto client paths.
- New controllers (`ComponentParameter`, `ParameterView`, `ParametersDefinition`) should keep validation, rendering, and patch logic delegated to `pkg/parameters`.
- Legacy controllers (`legacy_*.go`) exist for backward compatibility. Do not add new features; fix only critical bugs. Plan removal when migration is complete.
- `ParameterView` is user-editable: controllers must handle spec-content initialization, conflict/invalid phases, and submission status transitions. Keep these covered by tests.
- `reconfigure/` contains the sync/restart logic shared between legacy and new paths. Changes here affect both — test both code paths.
- When changing parameter validation or schema logic, ensure `pkg/parameters/validate/` and `pkg/parameters/openapi/` stay consistent.

## Testing

- For `ParameterView`, cover spec initialization, conflict detection, invalid phase handling, and submission status transitions.
- For `reconfigure/`, test both legacy and new API code paths.
- For legacy controllers, maintain existing test coverage; do not reduce it during migration.
