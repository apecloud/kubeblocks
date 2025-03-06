---
title: kbcli trace create
---

create a trace.

```
kbcli trace create trace-name [flags]
```

### Examples

```
  # create a trace for cluster has the same name 'pg-cluster'
  kbcli trace create pg-cluster
  
  # create a trace for cluster has the name of 'pg-cluster'
  kbcli trace create pg-cluster-trace --cluster-name pg-cluster
  
  # create a trace with custom locale, stateEvaluationExpression
  kbcli trace create pg-cluster-trace --locale zh_cn --cel-state-evaluation-expression "has(object.status.phase) && object.status.phase == \"Running\""
```

### Options

```
      --cel-state-evaluation-expression string   Specify CEL state evaluation expression.
      --cluster-name string                      Specify target cluster name.
      --depth int                                Specify object tree depth to display.
  -h, --help                                     help for create
      --locale string                            Specify locale.
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

* [kbcli trace](kbcli_trace.md)	 - trace management command

#### Go Back to [CLI Overview](cli.md) Homepage.

