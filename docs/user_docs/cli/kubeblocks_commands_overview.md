# KubeBlocks commands overview

This section lists all the KubeBlocks commands for configuring and managing a database cluster. 

## Commands

| **Command**                                                               |  **Usage**                                       |
| :--                                                                       | :--                                              |
| [`kbcli`](kbcli.md)                                                       | KubeBlocks CLI.                                  |
| [`kbcli_backup-config`](kbcli_backup-config.md)                           | Backup configuration command.                    |
| [`kbcli_bench`](kbcli_bench.md)                                           | Run a benchmark.                                 |
| [`dbcel_bench_tpcc`](kbcli_bench_tpcc.md)                                 | Run a TPCC benchmark.                            |
| [`kbcli_bench_tpcc_cleanup`](kbcli_bench_tpcc_cleanup.md)                 | Clean up data for TPCC.                          |
| [`kbcli_bench_tpcc_prepare`](kbcli_bench_tpcc_prepare.md)                 | Prepare data for TPCC.                           |
| [`kbcli_bench_tpcc_run`](kbcli_bench_tpcc_run.md)                         | Run workload for TPCC.                           |
| [`kbcli_cluster`](kbcli_cluster.md)                                       | Database cluster operation commands.             |
| [`kbcli_cluster_backup`](kbcli_cluster_backup.md)                         | Create a cluster backup task.                    |
| [`kbcli_cluster_connect`](kbcli_cluster_connect.md)                       | Connect to a database cluster.                   |
| [`kbcli_cluster_create`](kbcli_cluster_create.md)                         | Create a database cluster.                       |
| [`kbcli_cluster_delete-backup`](kbcli_cluster_delete-backup.md)           | Delete a cluster backup.                     |
| [`kbcli_cluster_delete-ops`](kbcli_cluster_delete-ops.md)                 | Delete an OpsRequest.                            |
| [`kbcli_cluster_delete-restore`](kbcli_cluster_delete-restore.md)         | Delete a cluster restoring job.                  |
| [`kbcli_cluster_delete`](kbcli_cluster_delete.md)                         | Delete a cluster.                                |
| [`kbcli_cluster_describe`](kbcli_cluster_describe.md)                     | Describe information of a specified database cluster. |
| [`kbcli_cluster_horizontal-scaling`](kbcli_cluster_horizontal-scaling.md) | Horizontally scale the specified components in a cluster. |
| [`kbcli_cluster_list-backups`](kbcli_cluster_list-backups.md)             | List all backup.                            |
| [`kbcli_cluster_list-logs-type`](kbcli_cluster_list-logs-type.md)         | List the supported logs file types in a cluster. |
| [`kbcli_cluster_list-ops`](kbcli_cluster_list-ops.md)                     | List all opsRequests.                            |
| [`kbcli_cluster_list-restores`](kbcli_cluster_list-restores.md)           | List all restoring jobs.                         |
| [`kbcli_cluster_list`](kbcli_cluster_list.md)                             | List all clusters.                               |
| [`kbcli_cluster_logs`](kbcli_cluster_logs.md)                             | Access up-to-date cluster log files.             |
| [`kbcli_cluster_restart`](kbcli_cluster_restart.md)                       | Restart a specified components in the cluster.   |
| [`kbcli_cluster_restore`](kbcli_cluster_restore.md)                       | Restore a new cluster from backup.               |
| [`kbcli_cluster_upgrade`](kbcli_cluster_upgrade.md)                       | Upgrade the cluster.                             |
| [`kbcli_cluster_vertical-scaling`](kbcli_cluster_vertical-scaling.md)     | Vertically scale a specified components in a cluster. |
| [`kbcli_cluster_volume-expansion`](kbcli_cluster_volume-expansion.md)     | Expand the volume with the specified components and volumeClaimTemplates in the cluster. |
| [`kbcli_kubenlocks`](kbcli_kubeblocks.md)                                 | KubeBlocks operation commands.                   |
| [`kbcli_kubeblocks_install`](kbcli_kubeblocks_install.md)                 | Install KubeBlocks.                              |
| [`kbcli_kubeblocks_unistall`](kbcli_kubeblocks_uninstall.md)              | Unistall KubeBlocks.                             |
| [`kbcli_options`](kbcli_options.md)                                       | Print the list of flags inherited by all commands. |
| [`kbcli_playground`](kbcli_playground.md)                                 | Bootstrap a KubeBlocks in a local host.          |
| [`kbcli_playground_destroy`](kbcli_playground_destroy.md)                 | Destroy the playground cluster.                  |
| [`kbcli_playground_guide`](kbcli_playground_guide.md)                     | Display the playground cluster user guide.       |
| [`kbcli_playground_init`](kbcli_playground_init.md)                       | Bootstrap a KubeBlocks for playground.           |
| [`kbcli_version`](kbcli_version.md)                                       | Print the kbcli version.                         |
