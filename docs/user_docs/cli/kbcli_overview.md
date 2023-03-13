---
title: Kubeblocks CLI Overview
description: KubeBlocks CLI overview
sidebar_position: 1
---

## [addon](kbcli_addon.md)

Addon command.

* [describe](kbcli_addon_describe.md)	 - Describe an addon specification.
* [disable](kbcli_addon_disable.md)	 - Disable an addon.
* [enable](kbcli_addon_enable.md)	 - Enable an addon.
* [list](kbcli_addon_list.md)	 - List addons.


## [alert](kbcli_alert.md)

Manage alert receiver, include add, list and delete receiver.

* [add-receiver](kbcli_alert_add-receiver.md)	 - Add alert receiver, such as email, slack, webhook and so on.
* [delete-receiver](kbcli_alert_delete-receiver.md)	 - Delete alert receiver.
* [list-receivers](kbcli_alert_list-receivers.md)	 - List all alert receivers.


## [app](kbcli_app.md)

Manage external applications related to KubeBlocks.

* [install](kbcli_app_install.md)	 - Install the application with the specified name.
* [uninstall](kbcli_app_uninstall.md)	 - Uninstall the application with the specified name.


## [backup-config](kbcli_backup-config.md)

KubeBlocks backup config.



## [bench](kbcli_bench.md)

Run a benchmark.

* [tpcc](kbcli_bench_tpcc.md)	 - Run a TPCC benchmark.


## [cluster](kbcli_cluster.md)

Cluster command.

* [backup](kbcli_cluster_backup.md)	 - Create a backup.
* [configure](kbcli_cluster_configure.md)	 - Reconfigure parameters with the specified components in the cluster.
* [connect](kbcli_cluster_connect.md)	 - Connect to a cluster or instance.
* [create](kbcli_cluster_create.md)	 - Create a cluster.
* [delete](kbcli_cluster_delete.md)	 - Delete clusters.
* [delete-backup](kbcli_cluster_delete-backup.md)	 - Delete a backup.
* [delete-ops](kbcli_cluster_delete-ops.md)	 - Delete an OpsRequest.
* [delete-restore](kbcli_cluster_delete-restore.md)	 - Delete a restore job.
* [describe](kbcli_cluster_describe.md)	 - Show details of a specific cluster.
* [describe-configure](kbcli_cluster_describe-configure.md)	 - Show details of a specific reconfiguring.
* [describe-ops](kbcli_cluster_describe-ops.md)	 - Show details of a specific OpsRequest.
* [diff-configure](kbcli_cluster_diff-configure.md)	 - Show the difference in parameters between the two submitted OpsRequest.
* [explain-configure](kbcli_cluster_explain-configure.md)	 - List the constraint for supported configuration params.
* [expose](kbcli_cluster_expose.md)	 - Expose a cluster.
* [hscale](kbcli_cluster_hscale.md)	 - Horizontally scale the specified components in the cluster.
* [list](kbcli_cluster_list.md)	 - List clusters.
* [list-accounts](kbcli_cluster_list-accounts.md)	 - List cluster accounts.
* [list-backups](kbcli_cluster_list-backups.md)	 - List backups.
* [list-components](kbcli_cluster_list-components.md)	 - List cluster components.
* [list-events](kbcli_cluster_list-events.md)	 - List cluster events.
* [list-instances](kbcli_cluster_list-instances.md)	 - List cluster instances.
* [list-logs](kbcli_cluster_list-logs.md)	 - List supported log files in cluster.
* [list-ops](kbcli_cluster_list-ops.md)	 - List all opsRequests.
* [list-restores](kbcli_cluster_list-restores.md)	 - List all restore jobs.
* [logs](kbcli_cluster_logs.md)	 - Access cluster log file.
* [restart](kbcli_cluster_restart.md)	 - Restart the specified components in the cluster.
* [restore](kbcli_cluster_restore.md)	 - Restore a new cluster from backup.
* [start](kbcli_cluster_start.md)	 - Start the cluster if cluster is stopped.
* [stop](kbcli_cluster_stop.md)	 - Stop the cluster and release all the pods of the cluster.
* [update](kbcli_cluster_update.md)	 - Update the cluster settings, such as enable or disable monitor or log.
* [upgrade](kbcli_cluster_upgrade.md)	 - Upgrade the cluster version.
* [volume-expand](kbcli_cluster_volume-expand.md)	 - Expand volume with the specified components and volumeClaimTemplates in the cluster.
* [vscale](kbcli_cluster_vscale.md)	 - Vertically scale the specified components in the cluster.


## [clusterdefinition](kbcli_clusterdefinition.md)

ClusterDefinition command.

* [list](kbcli_clusterdefinition_list.md)	 - List ClusterDefinitions.


## [clusterversion](kbcli_clusterversion.md)

ClusterVersion command.

* [list](kbcli_clusterversion_list.md)	 - List ClusterVersions.


## [dashboard](kbcli_dashboard.md)

List and open the KubeBlocks dashboards.

* [list](kbcli_dashboard_list.md)	 - List all dashboards.
* [open](kbcli_dashboard_open.md)	 - Open one dashboard.


## [kubeblocks](kbcli_kubeblocks.md)

KubeBlocks operation commands.

* [install](kbcli_kubeblocks_install.md)	 - Install KubeBlocks.
* [list-versions](kbcli_kubeblocks_list-versions.md)	 - List KubeBlocks versions.
* [preflight](kbcli_kubeblocks_preflight.md)	 - Run and retrieve preflight checks for KubeBlocks.
* [status](kbcli_kubeblocks_status.md)	 - Show list of resource KubeBlocks uses or owns.
* [uninstall](kbcli_kubeblocks_uninstall.md)	 - Uninstall KubeBlocks.
* [upgrade](kbcli_kubeblocks_upgrade.md)	 - Upgrade KubeBlocks.


## [options](kbcli_options.md)

Print the list of flags inherited by all commands.



## [playground](kbcli_playground.md)

Bootstrap a playground KubeBlocks in local host or cloud.

* [destroy](kbcli_playground_destroy.md)	 - Destroy the playground kubernetes cluster.
* [guide](kbcli_playground_guide.md)	 - Display playground cluster user guide.
* [init](kbcli_playground_init.md)	 - Bootstrap a kubernetes cluster and install KubeBlocks for playground.


## [version](kbcli_version.md)

Print the version information, include kubernetes, KubeBlocks and kbcli version.



