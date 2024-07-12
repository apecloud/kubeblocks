---
title: kbcli cluster logs
---

Access cluster log file.

```
kbcli cluster logs NAME [flags]
```

### Examples

```
  # Return snapshot logs from cluster mycluster with default primary instance (stdout)
  kbcli cluster logs mycluster
  
  # Display only the most recent 20 lines from cluster mycluster with default primary instance (stdout)
  kbcli cluster logs mycluster --tail=20
  
  # Display stdout info of specific instance my-instance-0 (cluster name comes from annotation app.kubernetes.io/instance)
  kbcli cluster logs --instance my-instance-0
  
  # Return snapshot logs from cluster mycluster with specific instance my-instance-0 (stdout)
  kbcli cluster logs mycluster --instance my-instance-0
  
  # Return snapshot logs from cluster mycluster with specific instance my-instance-0 and specific container
  # my-container (stdout)
  kbcli cluster logs mycluster --instance my-instance-0 -c my-container
  
  # Return slow logs from cluster mycluster with default primary instance
  kbcli cluster logs mycluster --file-type=slow
  
  # Begin streaming the slow logs from cluster mycluster with default primary instance
  kbcli cluster logs -f mycluster --file-type=slow
  
  # Return the specific file logs from cluster mycluster with specific instance my-instance-0
  kbcli cluster logs mycluster --instance my-instance-0 --file-path=/var/log/yum.log
  
  # Return the specific file logs from cluster mycluster with specific instance my-instance-0 and specific
  # container my-container
  kbcli cluster logs mycluster --instance my-instance-0 -c my-container --file-path=/var/log/yum.log
```

### Options

```
  -c, --container string    Container name.
      --file-path string    Log-file path. File path has a priority over file-type. When file-path and file-type are unset, output stdout/stderr of target container.
      --file-type string    Log-file type. List them with list-logs cmd. When file-path and file-type are unset, output stdout/stderr of target container.
  -f, --follow              Specify if the logs should be streamed.
  -h, --help                help for logs
      --ignore-errors       If watching / following pod logs, allow for any errors that occur to be non-fatal. Only take effect for stdout&stderr.
  -i, --instance string     Instance name.
      --limit-bytes int     Maximum bytes of logs to return.
      --prefix              Prefix each log line with the log source (pod name and container name). Only take effect for stdout&stderr.
  -p, --previous            If true, print the logs for the previous instance of the container in a pod if it exists. Only take effect for stdout&stderr.
      --since duration      Only return logs newer than a relative duration like 5s, 2m, or 3h. Defaults to all logs. Only one of since-time / since may be used. Only take effect for stdout&stderr.
      --since-time string   Only return logs after a specific date (RFC3339). Defaults to all logs. Only one of since-time / since may be used. Only take effect for stdout&stderr.
      --tail int            Lines of recent log file to display. Defaults to -1 for showing all log lines. (default -1)
      --timestamps          Include timestamps on each line in the log output. Only take effect for stdout&stderr.
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

