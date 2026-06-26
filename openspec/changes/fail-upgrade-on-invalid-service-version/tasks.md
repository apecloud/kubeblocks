## 1. OpenSpec

- [x] 1.1 Create the `fail-upgrade-on-invalid-service-version` OpenSpec change.
- [x] 1.2 Document the proposal, design, requirements, and implementation checklist.

## 2. Implementation

- [x] 2.1 Update upgrade reconciliation to classify deterministic service-version image-resolution errors as fatal.
- [x] 2.2 Preserve retry behavior for compatible ComponentVersion lookup errors.

## 3. Verification

- [x] 3.1 Add regression coverage for an invalid upgrade service version that fails the OpsRequest.
- [x] 3.2 Run focused Go tests for upgrade operations. Verified with `setup-envtest` 1.26.1 assets: `pkg/operations` upgrade suite and `pkg/controller/component` tests pass.
- [x] 3.3 Validate the OpenSpec change.
