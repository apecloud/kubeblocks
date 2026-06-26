# Data Protection Packages

`pkg/dataprotection/` implements backup, restore, and action execution logic used by `controllers/dataprotection/`. Controllers orchestrate state; this package implements the mechanics.

## Layout

- `action/`: action execution manager — runs backup/restore actions defined by `ActionSet` CRDs, including exec-based and job-based actions.
- `backup/`: backup execution — creates backup jobs, manages backup status phases, handles incremental/continuous backups.
- `restore/`: restore execution — creates restore jobs, manages restore status phases, handles volume snapshot restore.
- `errors/`: typed errors for backup/restore flows (e.g. `ErrBackupInProgress`, `ErrRestoreInProgress`).
- `types/`: shared types and constants (backup phase, restore phase, config keys).
- `utils/`: utility functions (storage provider config, credential building, PVC handling).

## Editing Rules

- Keep this package controller-agnostic — no imports from `controllers/`. Controllers call into this package; not the reverse.
- Action execution is driven by `ActionSet` definitions. When adding new action types, extend the action manager, not the controller.
- Backup and restore flows use phase-based state machines — preserve phase transition ordering and never skip phases.
- Error types in `errors/` should be sentinel errors matched with `errors.Is()`, not string comparisons.
- Return wrapped errors with `fmt.Errorf("...: %w", err)` — preserve the original error chain.
- Pass `context.Context` first in functions that perform I/O or Kubernetes API calls.
- Credential and storage provider config building in `utils/` must handle all supported backends (S3, OSS, COS, OBS, Azure, Minio, GCS, NFS, PVC, FTP) — test with each backend's config shape.

## Testing

- Add table-driven tests for phase transitions, action execution, and storage provider config building.
- For restore, test both volume-snapshot and job-based restore paths.
