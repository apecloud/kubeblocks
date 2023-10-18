---
title: kbcli backuprepo create
---

Create a backup repo

```
kbcli backuprepo create [NAME] [flags]
```

### Examples

```
  # Create a default backup repo using S3 as the backend
  kbcli backuprepo create \
  --provider s3 \
  --region us-west-1 \
  --bucket test-kb-backup \
  --access-key-id <ACCESS KEY> \
  --secret-access-key <SECRET KEY> \
  --default
  
  # Create a non-default backup repo with a specified name
  kbcli backuprepo create my-backup-repo \
  --provider s3 \
  --region us-west-1 \
  --bucket test-kb-backup \
  --access-key-id <ACCESS KEY> \
  --secret-access-key <SECRET KEY>
```

### Options

```
      --access-method string       Specify the access method for the backup repository, "Tool" is preferred if not specified. options: ["Mount" "Tool"]
      --default                    Specify whether to set the created backup repo as default
  -h, --help                       help for create
      --provider string            Specify storage provider
      --pv-reclaim-policy string   Specify the reclaim policy for PVs created by this backup repo, the value can be "Retain" or "Delete" (default "Retain")
      --volume-capacity string     Specify the capacity of the new created PVC" (default "100Gi")
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

* [kbcli backuprepo](kbcli_backuprepo.md)	 - BackupRepo command.

#### Go Back to [CLI Overview](cli.md) Homepage.

