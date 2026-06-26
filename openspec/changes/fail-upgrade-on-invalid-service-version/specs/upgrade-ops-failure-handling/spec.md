## ADDED Requirements

### Requirement: Invalid upgrade service-version resolution fails the OpsRequest
The system SHALL mark an Upgrade OpsRequest as `Failed` when running reconciliation cannot resolve target images because the requested service version is invalid or unmatched against compatible ComponentVersion releases.

#### Scenario: Invalid service version during running upgrade
- **WHEN** an Upgrade OpsRequest is `Running` and requests a service version that cannot be resolved from compatible ComponentVersion releases
- **THEN** the OpsRequest phase becomes `Failed` with a failed condition that includes the resolution error

### Requirement: Transient ComponentVersion lookup errors remain retryable
The system SHALL keep transient errors encountered while reading compatible ComponentVersions retryable during Upgrade OpsRequest reconciliation.

#### Scenario: ComponentVersion lookup fails before resolution
- **WHEN** an Upgrade OpsRequest is reconciling and compatible ComponentVersions cannot be read
- **THEN** the reconcile error remains retryable and does not immediately convert the OpsRequest to `Failed`
