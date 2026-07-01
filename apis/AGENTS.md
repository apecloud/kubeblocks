# APIs

This tree defines KubeBlocks CRDs and their Kubernetes API behavior. API edits have a wide blast radius because generated clients, CRDs, webhooks, and controllers depend on these types.

## Layout

- `apps/`: core database APIs such as `Cluster`, `Component`, definitions, versions, service descriptors, sidecars, and sharding definitions.
- `workloads/`: workload APIs, including `InstanceSet` and `Instance`.
- `operations/`: `OpsRequest` and operation definition APIs.
- `dataprotection/`: backup, restore, repo, schedule, storage provider, and action APIs.
- `parameters/`: configuration and parameter APIs, including `ParameterView` user-editable configuration views and initial-parameter annotation helpers.
- `extensions/`, `trace/`, `experimental/`: extension, reconciliation trace, and experimental APIs.

## Editing Rules

- Resource files use the local `<resource>_types.go` convention, with supporting webhook, conversion, validation, condition, and registration files beside them.
- Keep versioned API packages self-contained. Do not create casual cross-version or cross-group dependencies; use explicit conversion code when versions need to interoperate.
- Preserve kubebuilder markers when moving or renaming fields. Validation, defaults, print columns, subresources, and categories are generated from comments.
- Any field that is persisted in a CRD must remain JSON-compatible and backward compatible. Prefer pointer fields or omitempty semantics when introducing optional behavior.
- Annotation-backed API helpers, such as initial-parameter serialization on `Cluster`, are still part of the API contract. Keep encode/decode helpers and tests with the API package that owns the annotation schema.
- Do not edit `zz_generated.deepcopy.go`; run `make generate`.
- After API changes, run `make manifests`; run `make client-sdk-gen` when typed client output must be refreshed.

## Review Checklist

- Confirm the CRD schema expresses required/optional/default behavior correctly.
- Check whether webhooks, conversions, status conditions, tests, and sample manifests need matching updates.
- For new resources, update registration, RBAC, samples, generated clients, and controller setup together.
- For status changes, verify controllers write the new fields and tests cover transition behavior.
