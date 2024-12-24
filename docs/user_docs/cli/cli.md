---
title: KubeBlocks CLI Overview
description: KubeBlocks CLI overview
sidebar_position: 1
---

## [addon](kbcli_addon.md)

Addon command.

* [kbcli addon describe](kbcli_addon_describe.md)	 - Describe an addon specification.
* [kbcli addon disable](kbcli_addon_disable.md)	 - Disable an addon.
* [kbcli addon enable](kbcli_addon_enable.md)	 - Enable an addon.
* [kbcli addon index](kbcli_addon_index.md)	 - Manage custom addon indexes
* [kbcli addon install](kbcli_addon_install.md)	 - Install KubeBlocks addon
* [kbcli addon list](kbcli_addon_list.md)	 - List addons.
* [kbcli addon search](kbcli_addon_search.md)	 - Search the addon from index
* [kbcli addon uninstall](kbcli_addon_uninstall.md)	 - Uninstall an existed addon
* [kbcli addon upgrade](kbcli_addon_upgrade.md)	 - Upgrade an existed addon to latest version or a specified version


## [backuprepo](kbcli_backuprepo.md)

BackupRepo command.

* [kbcli backuprepo create](kbcli_backuprepo_create.md)	 - Create a backup repository
* [kbcli backuprepo delete](kbcli_backuprepo_delete.md)	 - Delete a backup repository.
* [kbcli backuprepo describe](kbcli_backuprepo_describe.md)	 - Describe a backup repository.
* [kbcli backuprepo list](kbcli_backuprepo_list.md)	 - List Backup Repositories.
* [kbcli backuprepo update](kbcli_backuprepo_update.md)	 - Update a backup repository.


## [bench](kbcli_bench.md)

Run a benchmark.

* [kbcli bench delete](kbcli_bench_delete.md)	 - Delete a benchmark.
* [kbcli bench describe](kbcli_bench_describe.md)	 - Describe a benchmark.
* [kbcli bench list](kbcli_bench_list.md)	 - List all benchmarks.
* [kbcli bench pgbench](kbcli_bench_pgbench.md)	 - Run pgbench against a PostgreSQL cluster
* [kbcli bench redis-benchmark](kbcli_bench_redis-benchmark.md)	 - Run redis-benchmark on a cluster
* [kbcli bench sysbench](kbcli_bench_sysbench.md)	 - run a SysBench benchmark
* [kbcli bench tpcc](kbcli_bench_tpcc.md)	 - Run tpcc benchmark
* [kbcli bench tpcds](kbcli_bench_tpcds.md)	 - Run TPC-DS benchmark
* [kbcli bench tpch](kbcli_bench_tpch.md)	 - Run tpch benchmark
* [kbcli bench ycsb](kbcli_bench_ycsb.md)	 - Run YCSB benchmark on a cluster


## [cluster](kbcli_cluster.md)

Cluster command.

* [kbcli cluster backup](kbcli_cluster_backup.md)	 - Create a backup for the cluster.
* [kbcli cluster cancel-ops](kbcli_cluster_cancel-ops.md)	 - Cancel the pending/creating/running OpsRequest which type is vscale or hscale.
* [kbcli cluster configure](kbcli_cluster_configure.md)	 - Configure parameters with the specified components in the cluster.
* [kbcli cluster connect](kbcli_cluster_connect.md)	 - Connect to a cluster or instance.
* [kbcli cluster create](kbcli_cluster_create.md)	 - Create a cluster.
* [kbcli cluster create-account](kbcli_cluster_create-account.md)	 - Create account for a cluster
* [kbcli cluster custom-ops](kbcli_cluster_custom-ops.md)	 - 
* [kbcli cluster delete](kbcli_cluster_delete.md)	 - Delete clusters.
* [kbcli cluster delete-account](kbcli_cluster_delete-account.md)	 - Delete account for a cluster
* [kbcli cluster delete-backup](kbcli_cluster_delete-backup.md)	 - Delete a backup.
* [kbcli cluster delete-ops](kbcli_cluster_delete-ops.md)	 - Delete an OpsRequest.
* [kbcli cluster describe](kbcli_cluster_describe.md)	 - Show details of a specific cluster.
* [kbcli cluster describe-account](kbcli_cluster_describe-account.md)	 - Describe account roles and related information
* [kbcli cluster describe-backup](kbcli_cluster_describe-backup.md)	 - Describe a backup.
* [kbcli cluster describe-backup-policy](kbcli_cluster_describe-backup-policy.md)	 - Describe backup policy
* [kbcli cluster describe-config](kbcli_cluster_describe-config.md)	 - Show details of a specific reconfiguring.
* [kbcli cluster describe-ops](kbcli_cluster_describe-ops.md)	 - Show details of a specific OpsRequest.
* [kbcli cluster diff-config](kbcli_cluster_diff-config.md)	 - Show the difference in parameters between the two submitted OpsRequest.
* [kbcli cluster edit-backup-policy](kbcli_cluster_edit-backup-policy.md)	 - Edit backup policy
* [kbcli cluster edit-config](kbcli_cluster_edit-config.md)	 - Edit the config file of the component.
* [kbcli cluster explain-config](kbcli_cluster_explain-config.md)	 - List the constraint for supported configuration params.
* [kbcli cluster expose](kbcli_cluster_expose.md)	 - Expose a cluster with a new endpoint, the new endpoint can be found by executing 'kbcli cluster describe NAME'.
* [kbcli cluster grant-role](kbcli_cluster_grant-role.md)	 - Grant role to account
* [kbcli cluster hscale](kbcli_cluster_hscale.md)	 - Horizontally scale the specified components in the cluster.
* [kbcli cluster label](kbcli_cluster_label.md)	 - Update the labels on cluster
* [kbcli cluster list](kbcli_cluster_list.md)	 - List clusters.
* [kbcli cluster list-accounts](kbcli_cluster_list-accounts.md)	 - List accounts for a cluster
* [kbcli cluster list-backup-policy](kbcli_cluster_list-backup-policy.md)	 - List backups policies.
* [kbcli cluster list-backups](kbcli_cluster_list-backups.md)	 - List backups.
* [kbcli cluster list-components](kbcli_cluster_list-components.md)	 - List cluster components.
* [kbcli cluster list-events](kbcli_cluster_list-events.md)	 - List cluster events.
* [kbcli cluster list-instances](kbcli_cluster_list-instances.md)	 - List cluster instances.
* [kbcli cluster list-logs](kbcli_cluster_list-logs.md)	 - List supported log files in cluster.
* [kbcli cluster list-ops](kbcli_cluster_list-ops.md)	 - List all opsRequests.
* [kbcli cluster logs](kbcli_cluster_logs.md)	 - Access cluster log file.
* [kbcli cluster promote](kbcli_cluster_promote.md)	 - Promote a non-primary or non-leader instance as the new primary or leader of the cluster
* [kbcli cluster rebuild-instance](kbcli_cluster_rebuild-instance.md)	 - Rebuild the specified instances in the cluster.
* [kbcli cluster register](kbcli_cluster_register.md)	 - Pull the cluster chart to the local cache and register the type to 'create' sub-command
* [kbcli cluster restart](kbcli_cluster_restart.md)	 - Restart the specified components in the cluster.
* [kbcli cluster restore](kbcli_cluster_restore.md)	 - Restore a new cluster from backup.
* [kbcli cluster revoke-role](kbcli_cluster_revoke-role.md)	 - Revoke role from account
* [kbcli cluster start](kbcli_cluster_start.md)	 - Start the cluster if cluster is stopped.
* [kbcli cluster stop](kbcli_cluster_stop.md)	 - Stop the cluster and release all the pods of the cluster.
* [kbcli cluster update](kbcli_cluster_update.md)	 - Update the cluster settings, such as enable or disable monitor or log.
* [kbcli cluster upgrade](kbcli_cluster_upgrade.md)	 - Upgrade the cluster version.
* [kbcli cluster volume-expand](kbcli_cluster_volume-expand.md)	 - Expand volume with the specified components and volumeClaimTemplates in the cluster.
* [kbcli cluster vscale](kbcli_cluster_vscale.md)	 - Vertically scale the specified components in the cluster.


## [clusterdefinition](kbcli_clusterdefinition.md)

ClusterDefinition command.

* [kbcli clusterdefinition describe](kbcli_clusterdefinition_describe.md)	 - Describe ClusterDefinition.
* [kbcli clusterdefinition list](kbcli_clusterdefinition_list.md)	 - List ClusterDefinitions.
* [kbcli clusterdefinition list-components](kbcli_clusterdefinition_list-components.md)	 - List cluster definition components.
* [kbcli clusterdefinition list-service-reference](kbcli_clusterdefinition_list-service-reference.md)	 - List cluster references declared in a cluster definition.


## [clusterversion](kbcli_clusterversion.md)

ClusterVersion command.

* [kbcli clusterversion list](kbcli_clusterversion_list.md)	 - List ClusterVersions.
* [kbcli clusterversion set-default](kbcli_clusterversion_set-default.md)	 - Set the clusterversion to the default clusterversion for its clusterdefinition.
* [kbcli clusterversion unset-default](kbcli_clusterversion_unset-default.md)	 - Unset the clusterversion if it's default.


## [dashboard](kbcli_dashboard.md)

List and open the KubeBlocks dashboards.

* [kbcli dashboard list](kbcli_dashboard_list.md)	 - List all dashboards.
* [kbcli dashboard open](kbcli_dashboard_open.md)	 - Open one dashboard.


## [dataprotection](kbcli_dataprotection.md)

Data protection command.

* [kbcli dataprotection backup](kbcli_dataprotection_backup.md)	 - Create a backup for the cluster.
* [kbcli dataprotection delete-backup](kbcli_dataprotection_delete-backup.md)	 - Delete a backup.
* [kbcli dataprotection describe-backup](kbcli_dataprotection_describe-backup.md)	 - Describe a backup
* [kbcli dataprotection describe-backup-policy](kbcli_dataprotection_describe-backup-policy.md)	 - Describe a backup policy
* [kbcli dataprotection list-backup-policy](kbcli_dataprotection_list-backup-policy.md)	 - List backup policies
* [kbcli dataprotection list-backups](kbcli_dataprotection_list-backups.md)	 - List backups.
* [kbcli dataprotection restore](kbcli_dataprotection_restore.md)	 - Restore a new cluster from backup


## [kubeblocks](kbcli_kubeblocks.md)

KubeBlocks operation commands.

* [kbcli kubeblocks compare](kbcli_kubeblocks_compare.md)	 - List the changes between two different version KubeBlocks.
* [kbcli kubeblocks config](kbcli_kubeblocks_config.md)	 - KubeBlocks config.
* [kbcli kubeblocks describe-config](kbcli_kubeblocks_describe-config.md)	 - Describe KubeBlocks config.
* [kbcli kubeblocks install](kbcli_kubeblocks_install.md)	 - Install KubeBlocks.
* [kbcli kubeblocks list-versions](kbcli_kubeblocks_list-versions.md)	 - List KubeBlocks versions.
* [kbcli kubeblocks preflight](kbcli_kubeblocks_preflight.md)	 - Run and retrieve preflight checks for KubeBlocks.
* [kbcli kubeblocks status](kbcli_kubeblocks_status.md)	 - Show list of resource KubeBlocks uses or owns.
* [kbcli kubeblocks uninstall](kbcli_kubeblocks_uninstall.md)	 - Uninstall KubeBlocks.
* [kbcli kubeblocks upgrade](kbcli_kubeblocks_upgrade.md)	 - Upgrade KubeBlocks.


## [options](kbcli_options.md)

Print the list of flags inherited by all commands.



## [playground](kbcli_playground.md)

Bootstrap or destroy a playground KubeBlocks in local host or cloud.

* [kbcli playground destroy](kbcli_playground_destroy.md)	 - Destroy the playground KubeBlocks and kubernetes cluster.
* [kbcli playground init](kbcli_playground_init.md)	 - Bootstrap a kubernetes cluster and install KubeBlocks for playground.


## [plugin](kbcli_plugin.md)

Provides utilities for interacting with plugins.

 Plugins provide extended functionality that is not part of the major command-line distribution.

* [kbcli plugin describe](kbcli_plugin_describe.md)	 - Describe a plugin
* [kbcli plugin index](kbcli_plugin_index.md)	 - Manage custom plugin indexes
* [kbcli plugin install](kbcli_plugin_install.md)	 - Install kbcli or kubectl plugins
* [kbcli plugin list](kbcli_plugin_list.md)	 - List all visible plugin executables on a user's PATH
* [kbcli plugin search](kbcli_plugin_search.md)	 - Search kbcli or kubectl plugins
* [kbcli plugin uninstall](kbcli_plugin_uninstall.md)	 - Uninstall kbcli or kubectl plugins
* [kbcli plugin upgrade](kbcli_plugin_upgrade.md)	 - Upgrade kbcli or kubectl plugins


## [report](kbcli_report.md)

Report kubeblocks or cluster info.

* [kbcli report cluster](kbcli_report_cluster.md)	 - Report Cluster information
* [kbcli report kubeblocks](kbcli_report_kubeblocks.md)	 - Report KubeBlocks information, including deployments, events, logs, etc.


## [version](kbcli_version.md)

Print the version information, include kubernetes, KubeBlocks and kbcli version.



