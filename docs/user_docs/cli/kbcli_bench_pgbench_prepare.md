---
title: kbcli bench pgbench prepare
---

Prepare pgbench test data for a PostgreSQL cluster

```
kbcli bench pgbench prepare [ClusterName] [flags]
```

### Examples

```
  # pgbench prepare data on a cluster
  kbcli bench pgbench prepare pgcluster --database postgres --user xxx --password xxx --scale 100
```

### Options

```
  -h, --help        help for prepare
      --scale int   The scale factor to use for pgbench (default 1)
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
      --database string                database name
      --disable-compression            If true, opt-out of response compression for all requests to the server
      --host string                    the host of database
      --insecure-skip-tls-verify       If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string              Path to the kubeconfig file to use for CLI requests.
      --match-server-version           Require server version to match client version
  -n, --namespace string               If present, the namespace scope for this CLI request
      --password string                the password of database
      --port int                       the port of database
      --request-timeout string         The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
  -s, --server string                  The address and port of the Kubernetes API server
      --tls-server-name string         Server name to use for server certificate validation. If it is not provided, the hostname used to contact the server is used
      --token string                   Bearer token for authentication to the API server
      --user string                    the user of database
```

### SEE ALSO

* [kbcli bench pgbench](kbcli_bench_pgbench.md)	 - Run pgbench against a PostgreSQL cluster

#### Go Back to [CLI Overview](cli.md) Homepage.

