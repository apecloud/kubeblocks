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
* [kbcli addon list](kbcli_addon_list.md)	 - List addons.


## [alert](kbcli_alert.md)

Manage alert receiver, include add, list and delete receiver.

* [kbcli alert add-receiver](kbcli_alert_add-receiver.md)	 - Add alert receiver, such as email, slack, webhook and so on.
* [kbcli alert delete-receiver](kbcli_alert_delete-receiver.md)	 - Delete alert receiver.
* [kbcli alert list-receivers](kbcli_alert_list-receivers.md)	 - List all alert receivers.


## [bench](kbcli_bench.md)

Run a benchmark.

* [kbcli bench tpcc](kbcli_bench_tpcc.md)	 - Run a TPCC benchmark.


## [class](kbcli_class.md)

Manage classes

* [kbcli class create](kbcli_class_create.md)	 - Create a class
* [kbcli class list](kbcli_class_list.md)	 - List classes
* [kbcli class template](kbcli_class_template.md)	 - Generate class definition template


## [cluster](kbcli_cluster.md)

Cluster command.

* [kbcli cluster backup](kbcli_cluster_backup.md)	 - Create a backup for the cluster.
* [kbcli cluster connect](kbcli_cluster_connect.md)	 - Connect to a cluster or instance.
* [kbcli cluster create](kbcli_cluster_create.md)	 - Create a cluster.
* [kbcli cluster create-account](kbcli_cluster_create-account.md)	 - Create account for a cluster
* [kbcli cluster delete](kbcli_cluster_delete.md)	 - Delete clusters.
* [kbcli cluster delete-account](kbcli_cluster_delete-account.md)	 - Delete account for a cluster
* [kbcli cluster delete-backup](kbcli_cluster_delete-backup.md)	 - Delete a backup.
* [kbcli cluster delete-ops](kbcli_cluster_delete-ops.md)	 - Delete an OpsRequest.
* [kbcli cluster describe](kbcli_cluster_describe.md)	 - Show details of a specific cluster.
* [kbcli cluster describe-account](kbcli_cluster_describe-account.md)	 - Describe account roles and related information
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
* [kbcli cluster reconfigure](kbcli_cluster_reconfigure.md)	 - Reconfigure parameters with the specified components in the cluster.
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

* [kbcli clusterdefinition list](kbcli_clusterdefinition_list.md)	 - List ClusterDefinitions.


## [clusterversion](kbcli_clusterversion.md)

ClusterVersion command.

* [kbcli clusterversion list](kbcli_clusterversion_list.md)	 - List ClusterVersions.


## [dashboard](kbcli_dashboard.md)

List and open the KubeBlocks dashboards.

* [kbcli dashboard list](kbcli_dashboard_list.md)	 - List all dashboards.
* [kbcli dashboard open](kbcli_dashboard_open.md)	 - Open one dashboard.


## [fault](kbcli_fault.md)

Inject faults to pod.

* [kbcli fault network](kbcli_fault_network.md)	 - Network chaos.


## [kubeblocks](kbcli_kubeblocks.md)

KubeBlocks operation commands.

* [kbcli kubeblocks config](kbcli_kubeblocks_config.md)	 - KubeBlocks config.
* [kbcli kubeblocks describe-config](kbcli_kubeblocks_describe-config.md)	 - describe KubeBlocks config.
* [kbcli kubeblocks install](kbcli_kubeblocks_install.md)	 - Install KubeBlocks.
* [kbcli kubeblocks list-versions](kbcli_kubeblocks_list-versions.md)	 - List KubeBlocks versions.
* [kbcli kubeblocks preflight](kbcli_kubeblocks_preflight.md)	 - Run and retrieve preflight checks for KubeBlocks.
* [kbcli kubeblocks status](kbcli_kubeblocks_status.md)	 - Show list of resource KubeBlocks uses or owns.
* [kbcli kubeblocks uninstall](kbcli_kubeblocks_uninstall.md)	 - Uninstall KubeBlocks.
* [kbcli kubeblocks upgrade](kbcli_kubeblocks_upgrade.md)	 - Upgrade KubeBlocks.


## [migration](kbcli_migration.md)

Data migration between two data sources.

* [kbcli migration create](kbcli_migration_create.md)	 - Create a migration task.
* [kbcli migration describe](kbcli_migration_describe.md)	 - Show details of a specific migration task.
* [kbcli migration list](kbcli_migration_list.md)	 - List migration tasks.
* [kbcli migration logs](kbcli_migration_logs.md)	 - Access migration task log file.
* [kbcli migration templates](kbcli_migration_templates.md)	 - List migration templates.
* [kbcli migration terminate](kbcli_migration_terminate.md)	 - Delete migration task.


## [options](kbcli_options.md)

Print the list of flags inherited by all commands.



## [playground](kbcli_playground.md)

Bootstrap a playground KubeBlocks in local host or cloud.

* [kbcli playground destroy](kbcli_playground_destroy.md)	 - Destroy the playground kubernetes cluster.
* [kbcli playground guide](kbcli_playground_guide.md)	 - Display playground cluster user guide.
* [kbcli playground init](kbcli_playground_init.md)	 - Bootstrap a kubernetes cluster and install KubeBlocks for playground.


## [plugin](kbcli_plugin.md)

Provides utilities for interacting with plugins.

 Plugins provide extended functionality that is not part of the major command-line distribution.

* [kbcli plugin list](kbcli_plugin_list.md)	 - List all visible plugin executables on a user's PATH


## [version](kbcli_version.md)

Print the version information, include kubernetes, KubeBlocks and kbcli version.



