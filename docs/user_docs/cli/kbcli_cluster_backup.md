---
title: kbcli cluster backup
---

Create a backup for the cluster.

```
kbcli cluster backup NAME [flags]
```

### Examples

```
  # Create a backup for the cluster, use the default backup policy and volume snapshot backup method
  kbcli cluster backup mycluster
  
  # create a backup with a specified method, run "kbcli cluster desc-backup-policy mycluster" to show supported backup methods
  kbcli cluster backup mycluster --method volume-snapshot
  
  # create a backup with specified backup policy, run "kbcli cluster list-backup-policy mycluster" to show the cluster supported backup policies
  kbcli cluster backup mycluster --method volume-snapshot --policy <backup-policy-name>
  
  # create a backup from a parent backup
  kbcli cluster backup mycluster --parent-backup parent-backup-name
```

### Options

```
      --deletion-policy string    Deletion policy for backup, determine whether the backup content in backup repo will be deleted after the backup is deleted, supported values: [Delete, Retain] (default "Delete")
  -h, --help                      help for backup
      --method string             Backup methods are defined in backup policy (required), if only one backup method in backup policy, use it as default backup method, if multiple backup methods in backup policy, use method which volume snapshot is true as default backup method
      --name string               Backup name
      --parent-backup string      Parent backup name, used for incremental backup
      --policy string             Backup policy name, if not specified, use the cluster default backup policy
      --retention-period string   Retention period for backup, supported values: [1y, 1mo, 1d, 1h, 1m] or combine them [1y1mo1d1h1m], if not specified, the backup will not be automatically deleted, you need to manually delete it.
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

* [kbcli cluster](kbcli_cluster.md)	 - Cluster command.

#### Go Back to [CLI Overview](cli.md) Homepage.

