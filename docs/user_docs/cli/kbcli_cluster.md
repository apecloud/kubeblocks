## kbcli cluster

Database cluster operation command

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

* [kbcli](kbcli.md)	 - KubeBlocks CLI
* [kbcli cluster backup](kbcli_cluster_backup.md)	 - Create a backup
* [kbcli cluster connect](kbcli_cluster_connect.md)	 - connect to a database cluster
* [kbcli cluster create](kbcli_cluster_create.md)	 - Create a database cluster
* [kbcli cluster delete](kbcli_cluster_delete.md)	 - Delete a cluster
* [kbcli cluster delete-backup](kbcli_cluster_delete-backup.md)	 - Delete a backup job.
* [kbcli cluster delete-ops](kbcli_cluster_delete-ops.md)	 - Delete a OpsRequest
* [kbcli cluster delete-restore](kbcli_cluster_delete-restore.md)	 - Delete a restore job.
* [kbcli cluster describe](kbcli_cluster_describe.md)	 - Describe database cluster info
* [kbcli cluster horizontal-scaling](kbcli_cluster_horizontal-scaling.md)	 - horizontal scaling the specified components in the cluster
* [kbcli cluster list](kbcli_cluster_list.md)	 - List all cluster.
* [kbcli cluster list-backups](kbcli_cluster_list-backups.md)	 - List backup jobs.
* [kbcli cluster list-logs](kbcli_cluster_list-logs.md)	 - List supported log files in cluster
* [kbcli cluster list-ops](kbcli_cluster_list-ops.md)	 - List all opsRequest.
* [kbcli cluster list-restores](kbcli_cluster_list-restores.md)	 - List all restore jobs.
* [kbcli cluster logs](kbcli_cluster_logs.md)	 - Access cluster log file
* [kbcli cluster restart](kbcli_cluster_restart.md)	 - restart the specified components in the cluster
* [kbcli cluster restore](kbcli_cluster_restore.md)	 - Restore a new cluster from backup
* [kbcli cluster update](kbcli_cluster_update.md)	 - Update the cluster
* [kbcli cluster upgrade](kbcli_cluster_upgrade.md)	 - upgrade the cluster
* [kbcli cluster vertical-scaling](kbcli_cluster_vertical-scaling.md)	 - vertical scaling the specified components in the cluster
* [kbcli cluster volume-expansion](kbcli_cluster_volume-expansion.md)	 - expand volume with the specified components and volumeClaimTemplates in the cluster

