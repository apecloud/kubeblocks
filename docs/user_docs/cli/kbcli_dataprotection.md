---
title: kbcli dataprotection
---

Data protection command.

### Options

```
  -h, --help   help for dataprotection
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

#### Go Back to [CLI Overview](cli.md) Homepage.

