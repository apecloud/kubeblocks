# KubeBlocks Maturity Levels

KubeBlocks uses a three-tier maturity model to communicate the stability and
production-readiness of the project. The current level is shown as a badge in
[`README.md`](../README.md).

## Levels

### Alpha

- **Meaning**: Early development. APIs and CRDs may change in breaking ways
  between minor releases. Features may be incomplete or experimental.
- **Color**: `red`
- **Criteria**:
  - Core CRDs exist but may still be in `v1beta1` or earlier.
  - Limited production deployments.
  - Test coverage is growing but not comprehensive.
  - Upgrade paths are not guaranteed to be smooth.

### Beta

- **Meaning**: Feature-complete and production-tried. APIs are mostly stable;
  breaking changes are discouraged and require a deprecation notice.
- **Color**: `orange`
- **Criteria**:
  - Core CRDs are at `v1` (or `v1beta1` with a clear path to `v1`).
  - Multiple production deployments across different industries.
  - CI covers unit, integration (envtest), and e2e tests with acceptable
    coverage.
  - Upgrade and rollback have been exercised in production.
  - Security policy is published and vulnerabilities are tracked.

### Stable / GA (General Availability)

- **Meaning**: Production-grade. APIs are stable with backward-compatibility
  guarantees following semantic versioning.
- **Color**: `green`
- **Criteria**:
  - All core CRDs are at `v1` and no breaking changes are planned.
  - Widespread production adoption with documented case studies.
  - Comprehensive test coverage including upgrade/downgrade matrices.
  - Formal release process with LTS support windows.
  - Security audits performed; SBOM published per release.

## How to Change the Maturity Level

1. Open a GitHub issue proposing the level change with evidence that the
   target level's criteria are met.
2. Maintainers review the proposal. At least two maintainer approvals are
   required.
3. On approval, update the badge line in `README.md`:

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

API version snapshot:

| API group | Version | Key resources |
|-----------|---------|---------------|
| `apps.kubeblocks.io` | `v1` | `Cluster`, `ClusterDefinition`, `ClusterVersion` |
| `workloads.kubeblocks.io` | `v1` | `InstanceSet` |
| `dataprotection.kubeblocks.io` | `v1alpha1` | `Backup`, `Restore`, `BackupPolicy`, `BackupSchedule` |
| `operations.kubeblocks.io` | `v1alpha1` | `OpsRequest`, `OpsDefinition` |
| `parameters.kubeblocks.io` | `v1alpha1` | `ParametersDefinition`, `ConfigConstraint` |
| `extensions.kubeblocks.io` | `v1alpha1` | `AddOn` |

Promoting the remaining `v1alpha1` APIs to `v1` and obtaining maintainer
approval per the process above are prerequisites for moving to **Beta**.
