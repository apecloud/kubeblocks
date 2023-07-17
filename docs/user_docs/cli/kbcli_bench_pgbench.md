---
title: kbcli bench pgbench
---

Run pgbench against a PostgreSQL cluster

```
kbcli bench pgbench [flags]
```

### Examples

```
  # pgbench run on a cluster
  kbcli bench pgbench mytest --cluster pgcluster --database postgres --user xxx --password xxx
  
  # pgbench run on a cluster with different threads and different client
  kbcli bench sysbench mytest --cluster pgcluster --user xxx --password xxx --database xxx --clients 5 --threads 5
  
  # pgbench run on a cluster with specified transactions
  kbcli bench pgbench mytest --cluster pgcluster --database postgres --user xxx --password xxx --transactions 1000
  
  # pgbench run on a cluster with specified seconds
  kbcli bench pgbench mytest --cluster pgcluster --database postgres --user xxx --password xxx --duration 60
  
  # pgbench run on a cluster with select only
  kbcli bench pgbench mytest --cluster pgcluster --database postgres --user xxx --password xxx --select
```

### Options

```
      --clients ints       The number of clients to use for pgbench (default [1])
      --cluster string     the cluster of database
      --database string    database name
      --duration int       The seconds to run pgbench for (default 60)
  -h, --help               help for pgbench
      --host string        the host of database
      --password string    the password of database
      --port int           the port of database
      --scale int          The scale factor to use for pgbench (default 1)
      --select             Run pgbench with select only
      --threads int        The number of threads to use for pgbench (default 1)
      --transactions int   The number of transactions to run for pgbench
      --user string        the user of database
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
```

### SEE ALSO

* [kbcli bench](kbcli_bench.md)	 - Run a benchmark.

#### Go Back to [CLI Overview](cli.md) Homepage.

