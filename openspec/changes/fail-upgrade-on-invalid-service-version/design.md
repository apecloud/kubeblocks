## Context

Upgrade OpsRequests update cluster component definitions and service versions, then reconcile progress by resolving the desired images from ComponentVersion releases. The OpsRequest manager already converts `Fatal` controller errors during running reconciliation into a terminal `Failed` phase, but upgrade image resolution currently returns plain errors, so deterministic invalid input is retried indefinitely.

## Goals / Non-Goals

**Goals:**
- Mark Upgrade OpsRequests as `Failed` when desired service-version image resolution cannot succeed without user correction.
- Keep the existing failed condition message visible to users.
- Keep transient client/list errors retryable.

**Non-Goals:**
- Change OpsRequest API validation, CRD schemas, or generated clients.
- Change failure handling for other OpsRequest types.
- Archive the OpenSpec change before implementation validation is complete.

## Decisions

- Classify only image-resolution errors after compatible ComponentVersions have been read as fatal. This separates deterministic content/input failures from transient Kubernetes client failures.
- Keep the conversion in the upgrade handler instead of broadening OpsRequest controller behavior, because the known bug is specific to upgrade image resolution.
- Reuse existing `intctrlutil.NewFatalError` and `OpsManager.Reconcile` completion handling so status conditions, events, and cleanup semantics stay consistent with other fatal operation failures.

## Risks / Trade-offs

- A ComponentVersion content issue can now fail an active upgrade instead of retrying until the ComponentVersion is fixed. Mitigation: only errors after successful compatible-version lookup are fatal, matching the user-correctable failure mode.
- The handler may resolve compatible ComponentVersions separately from image resolution. Mitigation: keep the helper narrowly scoped and covered by regression tests.
