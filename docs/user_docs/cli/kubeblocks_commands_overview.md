# KubeBlcoks commands overview

This section lists all the KubeBlocks commands for configuring and managing a database cluster. 

## Commands

| **Command**                                                               |  **Usage**                                       |
| :--                                                                       | :--                                              |
| [`dbctl_backup_create`](dbctl_backup_create.md)                           | Create a new backup job.                         |
| [`dbctl_backup_list`](dbctl_backup_list.md)                               | List all database backup jobs.                   |
| [`dbctl_backup_restore`](dbctl_backup_restore.md)                         | Restore the specified database.                  |
| [`dbctl_backup_snapshot_create`](dbctl_backup_snapshot_create.md)         | Create a new volume snapshot backup.             |
| [`dbctl_backup_snapshot_list`](dbctl_backup_snapshot_list.md)             | List all database backup snapshots.              |
| [`dbctl_backup_snapshot_restore`](dbctl_backup_snapshot_restore.md)       | Restore a new database from volume snapshot.     |
| [`dbctl_backup_snapshot`](dbctl_backup_snapshot.md)                       | Backup snapshot command.                         |
| [`dbctl_backup-config`](dbctl_backup-config.md)                           | Backup configuration command.                    |
| [`dbctl_backup`](dbctl_backup.md)                                         | Backup operation command.                        |
| [`dbctl_bench_tpcc_cleanup`](dbctl_bench_tpcc_cleanup.md)                 | Clean up data for TPCC.                          |
| [`dbctl_bench_tpcc_prepare`](dbctl_bench_tpcc_prepare.md)                 | Prepare data for TPCC.                           |
| [`dbctl_bench_tpcc_run`](dbctl_bench_tpcc_run.md)                         | Run workload for TPCC.                           |
| [`dbcel_bench_tpcc`](dbctl_bench_tpcc.md)                                 | Run a TPCC benchmark.                            |
| [`dbctl_bench`](dbctl_bench.md)                                           | Run a benchmark.                                 |
| [`dbctl_cluster_backup`](dbctl_cluster_backup.md)                         | Create a cluster backup task.                    |
| [`dbctl_cluster_connect`](dbctl_cluster_connect.md)                       | Connect to a database cluster.                   |
| [`dbctl_cluster_create`](dbctl_cluster_create.md)                         | Create a database cluster.                       |
| [`dbctl_cluster_delete-backup`](dbctl_cluster_delete-backup.md)           | Delete a cluster backup job.                     |
| [`dbctl_cluster_delete-ops`](dbctl_cluster_delete-ops.md)                 | Delete a OpsRequest.                             |
| [`dbctl_cluster_delete-restore`](dbctl_cluster_delete-restore.md)         | Delete a cluster restoring job.                  |
| [`dbctl_cluster_delete`](dbctl_cluster_delete.md)                         | Delete a cluster.                                |
| [`dbctl_cluster_describe`](dbctl_cluster_describe.md)                     | Describe information of a specified database cluster. |
| [`dbctl_cluster_horizontal-scaling`](dbctl_cluster_horizontal-scaling.md) | Horizontally scale the specified components in a cluster. |
| [`dbctl_cluster_list-backups`](dbctl_cluster_list-backups.md)             | List all backup jobs.                            |
| [`dbctl_cluster_list-logs-type`](dbctl_cluster_list-logs-type.md)         | List the supported logs file types in a cluster. |
| [`dbctl_cluster_list-ops`](dbctl_cluster_list-ops.md)                     | List all opsRequests.                            |
| [`dbctl_cluster_list-restores`](dbctl_cluster_list-restores.md)           | List all restoring jobs.                         |
| [`dbctl_cluster_list`](dbctl_cluster_list.md)                             | List all clusters.                               |
| [`dbctl_cluster_logs`](dbctl_cluster_logs.md)                             | Access up-to-date cluster log files.             |
| [`dbctl_cluster_restart`](dbctl_cluster_restart.md)                       | Restart a specified components in the cluster.   |
| [`dbctl_cluster_restore`](dbctl_cluster_restore.md)                       | Restore a new cluster from backup.               |
| [`dbctl_cluster_upgrade`](dbctl_cluster_upgrade.md)                       | Upgrade the cluster.                             |
| [`dbctl_cluster_vertical-scaling`](dbctl_cluster_vertical-scaling.md)     | Vertically scale a specified components in a cluster. |
| [`dbctl_cluster_volume-expansion`](dbctl_cluster_volume-expansion.md)     | Expand the volume with the specified components and volumeClaimTemplates in the cluster. |
| [`dbctl_cluster`](dbctl_cluster.md)                                       | Database cluster operation commands.             |
| [`dbctl_dbass_install`](dbctl_dbaas_install.md)                           | Install KubeBlocks.                              |
| [`dbctl_dbass_unistall`](dbctl_dbaas_uninstall.md)                        | Uninstall KubeBlocks.                            |
| [`dbctl_dbass`](dbctl_dbaas.md)                                           | DBaaS (KubeBlocks) operation commands.           |
| [`dbctl_kubeblocks_install`](dbctl_kubeblocks_install.md)                 | Install KubeBlocks.                              |
| [`dbctl_kubeblocks_unistall`](dbctl_kubeblocks_uninstall.md)              | Unistall KubeBlocks.                             |
| [`dbctl_kubenlocks`](dbctl_kubeblocks.md)                                 | KubeBlocks operation commands.                   |
| [`dbctl_options`](dbctl_options.md)                                       | Print the list of flags inherited by all commands. |
| [`dbctl_playground_destroy`](dbctl_playground_destroy.md)                 | Destroy the playground cluster.                  |
| [`dbctl_playground_guide`](dbctl_playground_guide.md)                     | Display the playground cluster user guide.       |
| [`dbctl_playground_init`](dbctl_playground_init.md)                       | Bootstrap a KubeBlocks for playground.           |
| [`dbctl_playground`](dbctl_playground.md)                                 | Bootstrap a KubeBlocks in local host.            |
| [`dbctl_version`](dbctl_version.md)                                       | Print the dbctl version.                         |
| [`dbctl`](dbctl.md)                                                       | KubeBlocks CLI.                                  |