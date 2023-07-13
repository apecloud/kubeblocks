---
title: kbcli cluster
---

Cluster command.

### Options

```
  -h, --help   help for cluster
```

### Options inherited from parent commands

```
      --as string                      Username to impersonate for the operation. User could be a regular user or a service account in a namespace.
      --as-group stringArray           Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --as-uid string                  UID to impersonate for the operation.
      --cache-dir string               Default cache directory (default "$HOME/.kube/cache")
      --certificate-authority string   Path to a cert file for the certificate authority
      --client-certificate string      Path to a client certificate file for TLS
      --client-key string              Path to a client key file for TLS
      --cluster string                 The name of the kubeconfig cluster to use
      --context string                 The name of the kubeconfig context to use
      --disable-compression            If true, opt-out of response compression for all requests to the server
      --insecure-skip-tls-verify       If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string              Path to the kubeconfig file to use for CLI requests.
      --match-server-version           Require server version to match client version
  -n, --namespace string               If present, the namespace scope for this CLI request
      --request-timeout string         The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
  -s, --server string                  The address and port of the Kubernetes API server
      --tls-server-name string         Server name to use for server certificate validation. If it is not provided, the hostname used to contact the server is used
      --token string                   Bearer token for authentication to the API server
      --user string                    The name of the kubeconfig user to use
```

### SEE ALSO


* [kbcli cluster backup](kbcli_cluster_backup.md)	 - Create a backup for the cluster.
* [kbcli cluster cancel-ops](kbcli_cluster_cancel-ops.md)	 - Cancel the pending/creating/running OpsRequest which type is vscale or hscale.
* [kbcli cluster configure](kbcli_cluster_configure.md)	 - Configure parameters with the specified components in the cluster.
* [kbcli cluster connect](kbcli_cluster_connect.md)	 - Connect to a cluster or instance.
* [kbcli cluster create](kbcli_cluster_create.md)	 - Create a cluster.
* [kbcli cluster create-account](kbcli_cluster_create-account.md)	 - Create account for a cluster
* [kbcli cluster delete](kbcli_cluster_delete.md)	 - Delete clusters.
* [kbcli cluster delete-account](kbcli_cluster_delete-account.md)	 - Delete account for a cluster
* [kbcli cluster delete-backup](kbcli_cluster_delete-backup.md)	 - Delete a backup.
* [kbcli cluster delete-ops](kbcli_cluster_delete-ops.md)	 - Delete an OpsRequest.
* [kbcli cluster describe](kbcli_cluster_describe.md)	 - Show details of a specific cluster.
* [kbcli cluster describe-account](kbcli_cluster_describe-account.md)	 - Describe account roles and related information
* [kbcli cluster describe-backup](kbcli_cluster_describe-backup.md)	 - Describe a backup.
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
* [kbcli cluster restart](kbcli_cluster_restart.md)	 - Restart the specified components in the cluster.
* [kbcli cluster restore](kbcli_cluster_restore.md)	 - Restore a new cluster from backup.
* [kbcli cluster revoke-role](kbcli_cluster_revoke-role.md)	 - Revoke role from account
* [kbcli cluster start](kbcli_cluster_start.md)	 - Start the cluster if cluster is stopped.
* [kbcli cluster stop](kbcli_cluster_stop.md)	 - Stop the cluster and release all the pods of the cluster.
* [kbcli cluster update](kbcli_cluster_update.md)	 - Update the cluster settings, such as enable or disable monitor or log.
* [kbcli cluster upgrade](kbcli_cluster_upgrade.md)	 - Upgrade the cluster version.
* [kbcli cluster volume-expand](kbcli_cluster_volume-expand.md)	 - Expand volume with the specified components and volumeClaimTemplates in the cluster.
* [kbcli cluster vscale](kbcli_cluster_vscale.md)	 - Vertically scale the specified components in the cluster.

#### Go Back to [CLI Overview](cli.md) Homepage.

