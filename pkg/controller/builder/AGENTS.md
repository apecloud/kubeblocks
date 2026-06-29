# Builder Package

`pkg/controller/builder/` provides typed Kubernetes object builders with functional options. All builders inherit from a generic `BaseBuilder` and use chainable methods that return the concrete builder type.

## Layout

- `builder_base.go`: generic `BaseBuilder[T, PT, B]` — handles labels, annotations, owner references, finalizers. `T` = object type, `PT` = pointer type, `B` = concrete builder type for chainable returns.
- `builder_cluster.go`, `builder_component.go`, `builder_component_definition.go`, `builder_component_parameter.go`: KubeBlocks CRD builders.
- `builder_backup.go`: data protection object builder.
- `builder_configmap.go`, `builder_container.go`, `builder_event.go`, `builder_pod.go`, `builder_pvc.go`, `builder_secret.go`, `builder_service.go`, `builder_service_account.go`, `builder_role.go`, `builder_role_binding.go`, `builder_cluster_role.go`, `builder_job.go`, `builder_instance.go`, `builder_instance_set.go`, `builder_node_count_scaler.go`, `builder_service_descriptor.go`, `builder_parameter.go`: Kubernetes native object builders.
- `builder_volume.go`, `builder_base_test.go`: volume builder and base tests.

## Editing Rules

- All builders extend `BaseBuilder` — do not bypass the base by constructing objects directly.
- Builder methods must return `*B` (the concrete builder), not `*BaseBuilder`, to preserve chainability.
- File naming: `builder_{object_kind_snake_case}.go` (e.g. `builder_service_account.go`).
- Builders should be deterministic — tests compare full object graphs and are sensitive to name, label, and annotation drift.
- Do not perform API calls inside builders — they only construct objects. Validation and creation happen in reconcilers/transformers.
- Each builder should have a paired `builder_{object}_test.go` using table-driven tests.

## Testing

- Test that generated objects have stable names, labels, annotations, and owner references.
- Test functional option chaining produces correct final objects.
