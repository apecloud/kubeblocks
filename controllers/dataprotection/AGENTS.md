# Data Protection Controllers

`controllers/dataprotection/` contains 11 controllers for backup, restore, scheduling, repos, storage providers, GC, and log collection. Unlike `controllers/apps/`, these use traditional reconciler patterns (no transformer DAG) and delegate domain logic to `pkg/dataprotection/`.

## Layout

- `backup_controller.go`, `restore_controller.go`, `backuppolicy_controller.go`, `backuppolicytemplate_controller.go`, `backupschedule_controller.go`, `backuprepo_controller.go`, `storageprovider_controller.go`, `actionset_controller.go`, `log_collection_controller.go`, `garbage_collection_controller.go`, `volumepopulator_controller.go`.
- `cluster_backup_controller.go`: cluster-level backup orchestration.
- `backuppolicy_driver_controller.go`: backup policy driver reconciliation.
- `scheme.go`, `suite_test.go`.

## Editing Rules

- Use traditional reconciler patterns — do not introduce transformer DAG here unless migrating the entire package.
- Delegate backup/restore/action execution logic to `pkg/dataprotection/` (action, backup, restore, utils). Controllers should orchestrate state transitions and status, not implement backup mechanics.
- Status updates use `Status().Patch(..., client.MergeFrom(...))` — preserve conflict-safe patterns.
- Finalizers: backup and restore controllers manage finalizers for cleanup of external artifacts (backup files, restore jobs). Remove finalizers only after cleanup succeeds.
- `BackupRepo` and `StorageProvider` support multi-cluster — use the multi-cluster client pattern when available.
- `ActionSet` changes can affect backup/restore behavior — test actionset reconciliation with matching backup policy scenarios.

## Testing

- Keep controller tests close to the reconciler package.
- For backup/restore, cover status phase transitions, finalizer cleanup, and error requeue behavior.
- For `BackupSchedule`, test schedule parsing and next-run logic.
