# ParameterView Notes

## Goal

`ParameterView` is a user-facing editing/view resource for parameterized runtime configuration.

It is intended to solve a different problem from `ComponentParameter`:

- `ComponentParameter` remains the parameter domain object and execution object.
- `ParameterView` provides a more direct what-you-see-is-what-you-edit entry for users.

The long-term direction is to keep these two roles decoupled:

- `ComponentParameter`: source of truth in the parameter domain, execution coordination, validation/apply pipeline.
- `ParameterView`: editable projection over effective configuration content.

## Why A Separate Resource

The current `ComponentParameter` model is still incremental:

- `spec.init`
- `spec.desired`
- `spec.configItemDetails`

These fields are useful for parameter management and execution, but they are not a true WYSIWYG editing surface.

A separate resource is preferred because it:

- decouples the user editing model from the execution model;
- avoids overloading `ComponentParameter` with UI/view-specific semantics;
- allows future evolution of the view model without destabilizing the execution model.

## Core Design Principles

1. `ParameterView` is associated with one `ComponentParameter`.
2. `ParameterView` must provide version protection to avoid stale overwrite.
3. `ParameterView` supports read-only and read-write modes.
4. The initial scope should be file-oriented view/editing.
5. The actual rendered/runtime content can be read from the generated ConfigMap, so the view object does not need to duplicate a rendered snapshot in status.

## What ParameterView Needs

### 1. Reference To ComponentParameter

`ParameterView` must explicitly point to the `ComponentParameter` it operates on.

This can later be modeled as:

- object reference, or
- `{ namespace, name }`

The resource should not become an independent configuration source.

### 2. Version Protection

`ParameterView` should include a source version marker to prevent stale edits from blindly overwriting newer runtime state.

Candidate forms:

- source generation
- source hash
- both

Current preference:

- generation for object-level conflict detection
- optional hash for content-level conflict detection

### 3. Mode

The object should support at least:

- `ReadOnly`
- `ReadWrite`

The mode determines whether user edits are allowed to be translated into patches against `ComponentParameter`.

### 4. View Granularity

The first version should focus on file view.

Reasoning:

- the final runtime object consumed by users is usually file content;
- file view is closer to WYSIWYG editing;
- template view is more abstract and can be introduced later if needed.

So the initial design should treat `ParameterView` as a file-level editing object.

### 5. Current Content

The object needs to carry the editable content that the user is working on.

This is expected to be the main write surface for the view resource.

## What ParameterView Does Not Need

### 1. Rendered Snapshot In Status

The rendered/effective content can be fetched from the actual generated ConfigMap.

With generation or hash protection in place, a second full rendered copy in status is unnecessary and introduces duplication.

That means the current direction is:

- read the effective content from ConfigMap;
- use `ParameterView` as an editing/session object;
- translate the diff into a patch request for `ComponentParameter`.

## High-Level Workflow

1. User opens or creates a `ParameterView` for a `ComponentParameter`.
2. The system reads the effective file content from the current generated ConfigMap.
3. The user edits the file content through the `ParameterView`.
4. A `ParameterView` controller compares the edited content with the effective content.
5. The controller translates the change into a patch/update request against `ComponentParameter`.
6. `ComponentParameter` continues to own validation, merge, rendering, and runtime apply.

## Non-Goals For The First Iteration

- do not redesign `ComponentParameter` again;
- do not move source-of-truth semantics away from `ComponentParameter` in this step;
- do not add template-view editing in the first iteration unless file view proves insufficient;
- do not duplicate runtime rendered state in `ParameterView.status`.

## Initial Naming Choice

The resource name is currently:

- `ParameterView`

Reasoning:

- neutral and user-facing;
- does not expose implementation details like `Component`;
- leaves room for future extension beyond a single entity kind.

## Open Questions

1. Should source protection use only generation, or generation plus content hash?
2. Should the object keep the editable content in `spec`, or split draft content and applied content?
3. What is the right patch format from `ParameterView` to `ComponentParameter`?
4. Should `ParameterView` be created explicitly by users, or generated on demand by tooling/UI?
5. How should conflict handling be reported back to the user?
