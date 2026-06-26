## Why

Upgrade OpsRequests can remain in `Running` when service-version image resolution fails for deterministic user input, such as a PostgreSQL upgrade using `serviceVersion: "17.5"` while releases are published as semantic versions like `17.5.0`. This leaves users unable to correct or delete the operation without forceful cleanup.

## What Changes

- Treat deterministic service-version resolution failures during upgrade reconciliation as terminal operation failures.
- Preserve retry behavior for transient Kubernetes/client reads while resolving compatible ComponentVersions.
- Add focused regression coverage for invalid upgrade service-version input.

## Capabilities

### New Capabilities
- `upgrade-ops-failure-handling`: Defines terminal failure behavior for invalid Upgrade OpsRequest service-version resolution.

### Modified Capabilities

## Impact

- Affected code: upgrade operation reconciliation in `pkg/operations/upgrade.go`.
- Affected tests: upgrade operation tests in `pkg/operations/upgrade_test.go`.
- Public APIs, CRDs, generated clients, and dependencies are unchanged.
