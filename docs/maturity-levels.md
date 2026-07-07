# KubeBlocks Maturity Levels

KubeBlocks uses a three-tier maturity model to communicate API stability and
release-process maturity. The current level is shown as a badge in
[`README.md`](../README.md). The badge does not measure production adoption by
itself; adoption evidence is one input to maturity changes.

The criteria below separate hard gates from evidence requirements. API version,
security, release-process, and approval requirements are hard gates. Adoption,
test coverage, and upgrade history require linked evidence and maintainer
judgment during the level-change review.

## Levels

### Alpha

- **Meaning**: Early development. APIs and CRDs may change in breaking ways
  between minor releases. Features may be incomplete or experimental.
- **Color**: `red`
- **Criteria**:
  - Some served CRD groups may still be `v1alpha1` or `v1beta1`.
  - Production deployments may be limited or concentrated in early adopters.
  - Test coverage is growing but has not met the Beta evidence bar.
  - Upgrade and rollback paths may not yet have broad production evidence.

### Beta

- **Meaning**: Feature-complete and production-tried. APIs are mostly stable;
  breaking changes are discouraged and require a deprecation notice.
- **Color**: `orange`
- **Criteria**:
  - Core CRDs are at `v1`. Core CRDs are the `apps.kubeblocks.io/v1` and
    `workloads.kubeblocks.io/v1` resources listed in the current API snapshot.
  - Multiple production deployments across different industries are documented
    in the level-change issue.
  - CI covers unit, integration (envtest), and e2e tests, with coverage accepted
    by the reviewing maintainers.
  - Upgrade and rollback have linked production or release-validation evidence.
  - [`SECURITY.md`](../SECURITY.md) is published and vulnerabilities are tracked.

### Stable / GA (General Availability)

- **Meaning**: Production-grade. APIs are stable with backward-compatibility
  guarantees following semantic versioning.
- **Color**: `green`
- **Criteria**:
  - All stable-track CRD groups are at `v1`, and no breaking changes are
    planned.
  - Widespread production adoption with documented case studies.
  - Comprehensive test coverage including upgrade/downgrade matrices.
  - Formal release process with LTS support windows.
  - Security audits performed; SBOM published per release.

## How to Change the Maturity Level

1. Open a GitHub issue proposing the level change with evidence that the
   target level's criteria are met.
2. Maintainers review the proposal. At least two maintainer approvals are
   required. Maintainers are listed in [`MAINTAINERS.md`](../MAINTAINERS.md).
3. On approval, update the badge line in `README.md` and the status text in
   this document:

   ```
   ![maturity](https://img.shields.io/static/v1?label=maturity&message=<level>&color=<color>)
   ```

   | Level | `message` | `color` |
   |-------|-----------|---------|
   | Alpha | `alpha`   | `red`    |
   | Beta  | `beta`    | `orange` |
   | Stable| `stable`  | `green`  |

4. The change is included in the next release notes.

## Current Status

KubeBlocks is currently at **Alpha**. The project is used in production by
internet companies, financial institutions, telecom carriers, and SaaS
providers, and the addon ecosystem spans 35+ database engines.

API version snapshot as of the `main` branch at the time this document was
added. The `apis/` directory and CRD manifests under `config/crd/bases/` remain
the source of truth.

| API group | Served versions | Key resources |
|-----------|-----------------|---------------|
| `apps.kubeblocks.io` | `v1` (primary); `v1beta1`, `v1alpha1` also served for legacy kinds | `Cluster`, `Component`, `ClusterDefinition`, `ComponentDefinition`, `ComponentVersion`, `ServiceDescriptor`, `ShardingDefinition`, `SidecarDefinition`; `ConfigConstraint` (`v1beta1`, `v1alpha1`); `Rollout`, `Configuration` (`v1alpha1`) |
| `workloads.kubeblocks.io` | `v1` | `InstanceSet`, `Instance` |
| `dataprotection.kubeblocks.io` | `v1alpha1` | `Backup`, `Restore`, `BackupPolicy`, `BackupSchedule`, `BackupPolicyTemplate`, `ActionSet`, `BackupRepo`, `StorageProvider` |
| `operations.kubeblocks.io` | `v1alpha1` | `OpsRequest`, `OpsDefinition` |
| `parameters.kubeblocks.io` | `v1alpha1` | `Parameter`, `ComponentParameter`, `ParametersDefinition`, `ParamConfigRenderer`, `ParameterView` |
| `extensions.kubeblocks.io` | `v1alpha1` | `Addon` |
| `experimental.kubeblocks.io` | `v1alpha1` | `NodeCountScaler` |
| `trace.kubeblocks.io` | `v1` | `ReconciliationTrace` |

The current core CRDs already meet the Beta API-version gate, but the project
remains **Alpha** until all Beta evidence requirements are reviewed and
approved. Promoting the remaining `v1alpha1` groups to `v1` is a Stable / GA
prerequisite tracked per group.
