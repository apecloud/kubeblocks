---
title: KubeBlocks CLI Overview
description: KubeBlocks CLI overview
sidebar_position: 1
---

## [addon](kbcli_addon.md)

Addon command.

* [kbcli addon delete-resources-with-version](kbcli_addon_delete-resources-with-version.md)	 - Delete the sub-resources of specified addon and versions
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
* [kbcli backuprepo list-storage-provider](kbcli_backuprepo_list-storage-provider.md)	 - List storage providers.
* [kbcli backuprepo update](kbcli_backuprepo_update.md)	 - Update a backup repository.


## [cluster](kbcli_cluster.md)

Cluster command.

* [kbcli cluster backup](kbcli_cluster_backup.md)	 - Create a backup for the cluster.
* [kbcli cluster cancel-ops](kbcli_cluster_cancel-ops.md)	 - Cancel the pending/creating/running OpsRequest which type is vscale or hscale.
* [kbcli cluster configure](kbcli_cluster_configure.md)	 - Configure parameters with the specified components in the cluster.
* [kbcli cluster connect](kbcli_cluster_connect.md)	 - Connect to a cluster or instance.
* [kbcli cluster create](kbcli_cluster_create.md)	 - Create a cluster.
* [kbcli cluster custom-ops](kbcli_cluster_custom-ops.md)	 - 
* [kbcli cluster delete](kbcli_cluster_delete.md)	 - Delete clusters.
* [kbcli cluster delete-backup](kbcli_cluster_delete-backup.md)	 - Delete a backup.
* [kbcli cluster delete-ops](kbcli_cluster_delete-ops.md)	 - Delete an OpsRequest.
* [kbcli cluster describe](kbcli_cluster_describe.md)	 - Show details of a specific cluster.
* [kbcli cluster describe-backup](kbcli_cluster_describe-backup.md)	 - Describe a backup.
* [kbcli cluster describe-backup-policy](kbcli_cluster_describe-backup-policy.md)	 - Describe backup policy
* [kbcli cluster describe-config](kbcli_cluster_describe-config.md)	 - Show details of a specific reconfiguring.
* [kbcli cluster describe-ops](kbcli_cluster_describe-ops.md)	 - Show details of a specific OpsRequest.
* [kbcli cluster describe-restore](kbcli_cluster_describe-restore.md)	 - Describe a restore
* [kbcli cluster diff-config](kbcli_cluster_diff-config.md)	 - Show the difference in parameters between the two submitted OpsRequest.
* [kbcli cluster edit-backup-policy](kbcli_cluster_edit-backup-policy.md)	 - Edit backup policy
* [kbcli cluster edit-config](kbcli_cluster_edit-config.md)	 - Edit the config file of the component.
* [kbcli cluster explain-config](kbcli_cluster_explain-config.md)	 - List the constraint for supported configuration params.
* [kbcli cluster expose](kbcli_cluster_expose.md)	 - Expose a cluster with a new endpoint, the new endpoint can be found by executing 'kbcli cluster describe NAME'.
* [kbcli cluster label](kbcli_cluster_label.md)	 - Update the labels on cluster
* [kbcli cluster list](kbcli_cluster_list.md)	 - List clusters.
* [kbcli cluster list-backup-policies](kbcli_cluster_list-backup-policies.md)	 - List backups policies.
* [kbcli cluster list-backups](kbcli_cluster_list-backups.md)	 - List backups.
* [kbcli cluster list-components](kbcli_cluster_list-components.md)	 - List cluster components.
* [kbcli cluster list-events](kbcli_cluster_list-events.md)	 - List cluster events.
* [kbcli cluster list-instances](kbcli_cluster_list-instances.md)	 - List cluster instances.
* [kbcli cluster list-logs](kbcli_cluster_list-logs.md)	 - List supported log files in cluster.
* [kbcli cluster list-ops](kbcli_cluster_list-ops.md)	 - List all opsRequests.
* [kbcli cluster list-restores](kbcli_cluster_list-restores.md)	 - List restores.
* [kbcli cluster logs](kbcli_cluster_logs.md)	 - Access cluster log file.
* [kbcli cluster promote](kbcli_cluster_promote.md)	 - Promote a non-primary or non-leader instance as the new primary or leader of the cluster
* [kbcli cluster rebuild-instance](kbcli_cluster_rebuild-instance.md)	 - Rebuild the specified instances in the cluster.
* [kbcli cluster register](kbcli_cluster_register.md)	 - Pull the cluster chart to the local cache and register the type to 'create' sub-command
* [kbcli cluster restart](kbcli_cluster_restart.md)	 - Restart the specified components in the cluster.
* [kbcli cluster restore](kbcli_cluster_restore.md)	 - Restore a new cluster from backup.
* [kbcli cluster scale-in](kbcli_cluster_scale-in.md)	 - scale in replicas of the specified components in the cluster.
* [kbcli cluster scale-out](kbcli_cluster_scale-out.md)	 - scale out replicas of the specified components in the cluster.
* [kbcli cluster start](kbcli_cluster_start.md)	 - Start the cluster if cluster is stopped.
* [kbcli cluster stop](kbcli_cluster_stop.md)	 - Stop the cluster and release all the pods of the cluster.
* [kbcli cluster update](kbcli_cluster_update.md)	 - Update the cluster settings, such as enable or disable monitor or log.
* [kbcli cluster upgrade](kbcli_cluster_upgrade.md)	 - Upgrade the service version(only support to upgrade minor version).
* [kbcli cluster volume-expand](kbcli_cluster_volume-expand.md)	 - Expand volume with the specified components and volumeClaimTemplates in the cluster.
* [kbcli cluster vscale](kbcli_cluster_vscale.md)	 - Vertically scale the specified components in the cluster.


## [clusterdefinition](kbcli_clusterdefinition.md)

ClusterDefinition command.

* [kbcli clusterdefinition describe](kbcli_clusterdefinition_describe.md)	 - Describe ClusterDefinition.
* [kbcli clusterdefinition list](kbcli_clusterdefinition_list.md)	 - List ClusterDefinitions.


## [componentdefinition](kbcli_componentdefinition.md)

ComponentDefinition command.

* [kbcli componentdefinition describe](kbcli_componentdefinition_describe.md)	 - Describe ComponentDefinition.
* [kbcli componentdefinition list](kbcli_componentdefinition_list.md)	 - List ComponentDefinition.


## [componentversion](kbcli_componentversion.md)

ComponentVersions command.

* [kbcli componentversion describe](kbcli_componentversion_describe.md)	 - Describe ComponentVersion.
* [kbcli componentversion list](kbcli_componentversion_list.md)	 - List ComponentVersion.


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
* [kbcli dataprotection describe-restore](kbcli_dataprotection_describe-restore.md)	 - Describe a restore
* [kbcli dataprotection edit-backup-policy](kbcli_dataprotection_edit-backup-policy.md)	 - Edit backup policy
* [kbcli dataprotection list-action-sets](kbcli_dataprotection_list-action-sets.md)	 - List actionsets
* [kbcli dataprotection list-backup-policies](kbcli_dataprotection_list-backup-policies.md)	 - List backup policies
* [kbcli dataprotection list-backup-policy-templates](kbcli_dataprotection_list-backup-policy-templates.md)	 - List backup policy templates
* [kbcli dataprotection list-backups](kbcli_dataprotection_list-backups.md)	 - List backups.
* [kbcli dataprotection list-restores](kbcli_dataprotection_list-restores.md)	 - List restores.
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


## [ops-definition](kbcli_ops-definition.md)

ops-definitions command.

* [kbcli ops-definition describe](kbcli_ops-definition_describe.md)	 - Describe OpsDefinition.
* [kbcli ops-definition list](kbcli_ops-definition_list.md)	 - List OpsDefinition.


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


## [trace](kbcli_trace.md)

trace management command

* [kbcli trace create](kbcli_trace_create.md)	 - create a trace.
* [kbcli trace delete](kbcli_trace_delete.md)	 - Delete a trace.
* [kbcli trace list](kbcli_trace_list.md)	 - list all traces.
* [kbcli trace update](kbcli_trace_update.md)	 - update a trace.
* [kbcli trace watch](kbcli_trace_watch.md)	 - watch a trace.


## [version](kbcli_version.md)

Print the version information, include kubernetes, KubeBlocks and kbcli version.



