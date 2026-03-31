# ParameterView Controller Plan

## Goal

`ParameterView` is a user-facing single-file view/edit resource built on top of `ComponentParameter`.

The controller should make `ParameterView` usable as:

- a readable projection of one effective config file;
- an editable surface for supported modes and content types;
- a guarded translation layer back into `ComponentParameter`.

`ComponentParameter` remains the source of truth and execution object.

## Controller Responsibilities

### 1. View Construction

The controller should:

- resolve `spec.parameterRef` to the target `ComponentParameter`;
- locate the target file by `spec.templateName` and `spec.fileName`;
- validate `spec.fileFormat`;
- capture source version metadata such as `spec.sourceGeneration`;
- optionally compute and persist `spec.contentHash`;
- build `spec.content`;
- maintain `status.phase`, `status.conditions`, and `status.message`.

### 2. Edit Validation

The controller should:

- enforce `ReadOnly` vs `ReadWrite`;
- reject stale writes when source generation or content hash no longer matches;
- validate `spec.content.type`;
- validate content syntax for each content type;
- report user-visible errors through conditions and status.

### 3. Patch Translation

The controller should:

- normalize view content into raw file content;
- diff user content against source content;
- translate file changes into a patch against `ComponentParameter`;
- apply patch results back into `ParameterView.status`.

### 4. Drift Handling

The controller should:

- react when the referenced `ComponentParameter` changes;
- distinguish between refreshable views and conflicting edited views;
- eventually support sync from generated/effective runtime content sources.

## Implementation Phases

## Progress

- Phase 1: In Progress
- Phase 2: In Progress
- Phase 3: Not Started
- Phase 4: Not Started
- Phase 5: Not Started

### Phase 1: Minimal Read Path

Objective:

- make `ParameterView` resolvable and readable.

Scope:

- validate `parameterRef`, `templateName`, and `fileName`;
- support only `content.type=PlainText`;
- populate `spec.content.text`;
- populate `status.observedGeneration`;
- set `status.phase` to `Ready`, `Pending`, or `Invalid`;
- enforce `ReadOnly` semantics at status level only;
- perform source generation conflict detection.

Done when:

- creating a `ParameterView` results in a populated single-file view and stable status.

Current status:

- controller resolves `parameterRef`, `templateName`, and `fileName`;
- controller infers and backfills `fileFormat`;
- controller currently supports `content.type=PlainText` only;
- controller backfills `sourceGeneration`, `contentHash`, and `content.text`;
- controller marks invalid references and unsupported content types as `Invalid`;
- controller detects source drift and marks edited stale views as `Conflict`;
- controller reports readiness through `status.phase`, `status.message`, and conditions.

### Phase 2: PlainText Write Path

Objective:

- make `ParameterView` editable with `PlainText`.

Scope:

- detect user edits in `spec.content.text`;
- normalize to raw file content;
- define and implement translation from file content to `ComponentParameter` patch;
- update `status.phase` for success, invalid input, and conflict;
- preserve source version protection.

Done when:

- editing `ParameterView.spec.content.text` can drive a valid `ComponentParameter` update.

Current status:

- `PlainText` edits are translated into `ComponentParameter.spec.configItemDetails[].configFileParams[file].content`;
- new write requests are marked as `Applying`;
- already-submitted desired content that has not yet reached the runtime ConfigMap is also marked as `Applying`;
- `ReadOnly` mode rejects write attempts;
- Phase 2 controller behavior is covered by focused unit tests for write, read-only rejection, and pending-apply handling.

### Phase 3: MarkerLine Support

Objective:

- support marker-annotated content as a richer editing surface.

Scope:

- define marker grammar;
- render marker-based content from source file content;
- parse marker-based content back into raw file content;
- protect immutable or unsupported regions;
- classify lines such as dynamic, static, immutable, and unmanaged.

Done when:

- the controller supports both `PlainText` and `MarkerLine`.

### Phase 4: Source Drift Sync

Objective:

- keep views aligned with evolving source objects.

Scope:

- watch `ComponentParameter`;
- requeue related `ParameterView` objects;
- refresh untouched views automatically;
- mark edited stale views as `Conflict`;
- optionally incorporate generated ConfigMap changes as a source trigger.

Done when:

- source updates are reflected in linked views without manual intervention.

### Phase 5: Hardening

Objective:

- raise the implementation to production quality.

Scope:

- refine conditions and reasons;
- improve idempotency and retry behavior;
- add reconcile events;
- add unit and controller tests for view build, conflict detection, and write-back;
- decide whether admission validation/defaulting is needed.

Done when:

- the controller behavior is predictable, test-covered, and diagnosable.

## Recommended Delivery Order

Recommended sequence:

1. Phase 1
2. Phase 2
3. Phase 3
4. Phase 4
5. Phase 5

Reasoning:

- the largest risk is not marker rendering, but the write-back contract from single-file content into `ComponentParameter`;
- `PlainText` should be the first end-to-end writable mode;
- marker support should be added after the patch translation model is stable.

## Current Open Decisions

The following items still need to be fixed before or during implementation:

1. Where the controller should read effective raw file content from during initialization.
2. The exact write-back mapping from one edited file into `ComponentParameter`.
3. The conflict policy when source content changes after a view is created.
4. The exact `MarkerLine` grammar and what edits are legal.
