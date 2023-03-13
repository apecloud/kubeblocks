---
title: KubeBlocks command line
description: KubeBlocks commands line, CLI
sidebar_position: 1
---

# KubeBlocks commands overview

This section lists all the KubeBlocks commands for configuring and managing a database cluster. 

## Commands for `kbcli cluster`

| **Command**                                                               |  **Usage**                                       |
| :--                                                                       | :--                                              |
| [`kbcli cluster`](./kbcli_cluster.md)                                       | Database cluster operation commands.           |
| **Basic cluster commands** |
| [`kbcli cluster create`](./kbcli_cluster_create.md)      | Create a database cluster.                                        |
| [`kbcli cluster connect`](./kbcli_cluster_connect.md)    | Connect to a cluster or instance.                                 |
| [`kbcli cluster describe`](./kbcli_cluster_describe.md)  | Show details of a specific cluster.                               |
| [`kbcli cluster list`](./kbcli_cluster_list.md)          | List clusters.                                                    |
| [`kbcli cluster list-instances`](./kbcli_cluster_list-instances.md)    | List cluster instances.                             |
| [`kbcli cluster list-components`](./kbcli_cluster_list-components.md)  | List cluster components.                            |
| [`kbcli cluster list-events`](./kbcli_cluster_list-events.md)          | List cluster events.                                |
| [`kbcli cluster list-accounts`](./kbcli_cluster_list-accounts.md)      | List cluster accounts.                              |
| [`kbcli cluster list-users`](./kbcli_cluster_list-users.md)            | List cluster users.                                 |
| [`kbcli cluster delete`](./kbcli_cluster_delete.md)                    | Delete clusters.                                    |
| **Cluster operation commands** |
| [`kbcli cluster update`](./kbcli_cluster_update.md)                    | Update the cluster settings, such as enable or disable monitor or log.                |
| [`kbcli cluster restart`](./kbcli_cluster_restart.md)                  | Delete a cluster backup.                     |
| [`kbcli cluster upgrade`](./kbcli_cluster_upgrade.md)                  | Upgrade the cluster version.                            |
| [`kbcli cluster volume-expand`](./kbcli_cluster_volume-expand.md)      | Expand volume with the specified components and volumeClaimTemplates in the cluster.  |
| [`kbcli cluster hscale`](./kbcli_cluster_hscale.md)                    | Horizontally scale the specified components in the cluster. |
| [`kbcli cluster describe-ops`](./kbcli_cluster_describe-ops.md)        | Show details of a specific OpsRequest. |
| [`kbcli cluster list-ops`](./kbcli_cluster_list-ops.md)                | List all opsRequests.                            |
| [`kbcli cluster delete-ops`](kbcli_cluster_delete-ops.md)              | Delete an OpsRequest. |
| [`kbcli cluster configure`](./kbcli_cluster_configure.md)              | Reconfigure parameters with the specified components in the cluster.                   |
| [`kbcli cluster expose`](./kbcli_cluster_expose.md)                    | Expose a cluster.                         |
| [`kbcli cluster describe-configure`](./kbcli_cluster_describe-configure.md)   | Show details of a specific reconfiguring.                               |
| [`kbcli cluster explain-configure`](./kbcli_cluster_explain-configure.md)     | List the constraint for supported configuration params.             |
| [`kbcli cluster diff-configure`](./kbcli_cluster_diff-configure.md)           | List the constraint for supported configuration params.   |
| [`kbcli cluster stop`](./kbcli_cluster_stop.md)                               | Stop the cluster and release all the pods of the cluster.               |
| [`kbcli cluster start`](./kbcli_cluster_start.md)                             | Start the cluster when cluster is stopped.                             |
| **Cluster backup/restore commands** |
| [`kbcli cluster backup`](./kbcli_cluster_backup.md)                  | Create a backup. |
| [`kbcli cluster list-backups`](kbcli_cluster_list-backups.md)        | List backups. |
| [`kbcli cluster delete-backup`](./kbcli_cluster_delete-backup.md)    | Delete a backup.                   |
| [`kbcli cluster restore`](./kbcli_cluster_restore.md)                | Restore a new cluster from backup.                              |
| [`kbcli cluster list-restores`](./kbcli_cluster_list-restores.md)    | List all restore job.                             |
| [`kbcli cluster delete-restore`](./kbcli_cluster_delete-restore.md)  | Delete a restore job. |
| **Cluster troubleshooting commands**
| [`kbcli cluster logs`](./kbcli_cluster_logs.md)                      | Access cluster log file.          |
| [`kbcli cluster list-logs`](./kbcli_cluster_list-logs.md)            | List supported log files in cluster.                  |

## Commands for other functions
| **Command**                                                               |  **Usage**                                       |
| :--                                                                       | :--                                              |
| [`kbcli`](./kbcli.md)                                                     | KubeBlocks CLI.                                  |
| [`kbcli dashboard`](./kbcli_dashboard.md)                                 | List and open the KubeBlocks dashboards.       |
| [`kbcli dashboard open`](./kbcli_dashboard_open.md)                       | Open one dashboard.           |
| [`kbcli dashboard list`](./kbcli_dashboard_list.md)                       | List all dashboards.                         |
| [`kbcli kubelocks`](./kbcli_kubeblocks.md)                                | KubeBlocks operation commands.   |
| [`kbcli kubeblocks install`](./kbcli_kubeblocks_install.md)               | Install KubeBlocks.     |
| [`kbcli kubeblocks list-versions`](./kbcli_kubeblocks_list-versions.md)   | List KubeBlocks versions.    |
| [`kbcli kubeblocks preflight`](./kbcli_kubeblocks_preflight.md)           | Run and retrieve preflight checks for KubeBlocks.   |
| [`kbcli kubeblocks status`](./kbcli_kubeblocks_status.md)                 | Show list of resource KubeBlocks uses or owns.  |
| [`kbcli kubeblocks uninstall`](kbcli_kubeblocks_uninstall.md)             | Uninstall KubeBlocks.   |
| [`kbcli kubeblocks upgrade`](./kbcli_kubeblocks_upgrade.md)               | Upgrade KubeBlocks.   |
| [`kbcli playground`](./kbcli_playground.md)                               | Bootstrap a playground KubeBlocks in local host or cloud.    |
| [`kbcli playground destroy`](./kbcli_playground_destroy.md)               | Destroy the playground kubernetes cluster.    |
| [`kbcli playground guide`](kbcli_playground_guide.md)                     | Display playground cluster user guide.   |
| [`kbcli playground init`](./kbcli_playground_init.md)                     | Bootstrap a kubernetes cluster and install KubeBlocks for playground.   |
| [`kbcli addon`](./kbcli_addon.md)                     | Addon command. |
| [`kbcli addon describe`](./kbcli_addon_describe.md)   | Describe an addon specification.   |
| [`kbcli addon disable`](./kbcli_addon_disable.md)     | Disable an addon.    |
| [`kbcli addon enable`](./kbcli_addon_enable.md)       | Enable an addon.     |
| [`kbcli addon list`](kbcli_addon_list.md)             | List addons.         |
| [`kbcli alert`](./kbcli_alert.md)                     | Manage alert receivers.  |
| [`kbcli alert add-receiver`](./kbcli_alert_add-receiver.md)        | Add alert receiver, such as email, slack, webhook and so on.   |
| [`kbcli alert delete-receiver`](./kbcli_alert_delete-receiver.md)  | Delete alert receiver.  |
| [`kbcli alert list-receiver`](./kbcli_alert_list-receivers.md)     | List all alert receivers.  |
| [`kbcli app`](./kbcli_app.md)                       | Manage external applications related to KubeBlocks.   |
| [`kbcli app install`](./kbcli_app_install.md)       | Install the application with the specified name.  |
| [`kbcli app uninstall`](./kbcli_app_uninstall.md)   | Uninstall the application with the specified name.   |
| [`kbcli backup-config`](./kbcli_backup-config.md)   | KubeBlocks backup configuration.   |
| [`kbcli bench`](./kbcli_bench.md)                             | Run a benchmark.    |
| [`kbcli bench tpcc`](./kbcli_bench_tpcc.md)                   | Run a TPCC benchmark.    |
| [`kbcli bench tpcc cleanup`](./kbcli_bench_tpcc_cleanup.md)   | Cleanup data for TPCC.   |
| [`kbcli bench tpcc prepare`](./kbcli_bench_tpcc_prepare.md)   | Prepare data for TPCC.   |
| [`kbcli bench tpcc run`](./kbcli_bench_tpcc_run.md)           | Run workload.            |
| [`kbcli clusterdefinition`](./kbcli_clusterdefinition.md)     | ClusterDefinition command.   |
| [`kbcli clusterdefinition list`](./kbcli_clusterdefinition_list.md)   | List ClusterDefinitions.   |
| [`kbcli clusterversion`](./kbcli_clusterversion.md)                   | ClusterVersion command.     |
| [`kbcli clusterversion list`](./kbcli_clusterversion_list.md)         | List ClusterVersions.   |
| [`kbcli options`](./kbcli_options.md)                                 | Print the list of flags inherited by all commands.   |
| [`kbcli troubleshoot`](./kbcli_troubleshoot.md)                       | Troubleshooting for KubeBlocks.    |
| [`kbcli troubleshoot preflight`](./kbcli_troubleshoot_preflight.md)   | Run and retrieve preflight checks for KubeBlocks.   |
| [`kbcli version`](./kbcli_version.md)                                 | Print the version information.   |
