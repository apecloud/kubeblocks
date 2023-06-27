---
title: kbcli report cluster
---

Report Cluster information

```
kbcli report cluster NAME [-f file] [-with-logs] [-mask] [flags]
```

### Examples

```
  # report KubeBlocks status
  kbcli report cluster mycluster
  
  # report KubeBlocks cluster information to file
  kbcli report cluster mycluster -f filename
  
  # report KubeBlocks cluster information with logs
  kbcli report cluster mycluster --with-logs
  
  # report KubeBlocks cluster information with logs and mask sensitive info
  kbcli report cluster mycluster --with-logs --mask
  
  # report KubeBlocks cluster information with logs since 1 hour ago
  kbcli report cluster mycluster --with-logs --since 1h
  
  # report KubeBlocks cluster information with logs since given time
  kbcli report cluster mycluster --with-logs --since-time 2023-05-23T00:00:00Z
  
  # report KubeBlocks cluster information with logs for all containers
  kbcli report cluster mycluster --with-logs --all-containers
```

### Options

```
      --all-containers      Get all containers' logs in the pod(s). Byt default, only the main container (the first container) will have logs recorded.
  -f, --file string         zip file for output
  -h, --help                help for cluster
      --mask                mask sensitive info for secrets and configmaps (default true)
  -o, --output string       Output format. One of: json|yaml. (default "json")
      --since duration      Only return logs newer than a relative duration like 5s, 2m, or 3h. Defaults to all logs. Only one of since-time / since may be used.
      --since-time string   Only return logs after a specific date (RFC3339). Defaults to all logs. Only one of since-time / since may be used.
      --with-logs           include pod logs
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

* [kbcli report](kbcli_report.md)	 - report kubeblocks or cluster info.

#### Go Back to [CLI Overview](cli.md) Homepage.

