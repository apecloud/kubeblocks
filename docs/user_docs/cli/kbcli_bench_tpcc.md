---
title: kbcli bench tpcc
---

Run tpcc benchmark

```
kbcli bench tpcc [flags]
```

### Examples

```
  # tpcc on a cluster
  kbcli bench tpcc mytest --cluster mycluster --user xxx --password xxx --database mydb
  
  # tpcc on a cluster with warehouses count, which is the overall database size scaling parameter
  kbcli bench tpcc mytest --cluster mycluster --user xxx --password xxx --database mydb --warehouses 100
  
  # tpcc on a cluster with threads count
  kbcli bench tpcc mytest --cluster mycluster --user xxx --password xxx --database mydb --threads 4,8
  
  # tpcc on a cluster with transactions count
  kbcli bench tpcc mytest --cluster mycluster --user xxx --password xxx --database mydb --transactions 1000
  
  # tpcc on a cluster with duration 10 minutes
  kbcli bench tpcc mytest --cluster mycluster --user xxx --password xxx --database mydb --duration 10
```

### Options

```
      --cluster string           the cluster of database
      --database string          database name
      --delivery int             specify the percentage of transactions that should be delivery (default 4)
      --driver string            the driver of database
      --duration int             specify the number of minutes to run (default 1)
      --extra-args stringArray   specify the extra arguments
  -h, --help                     help for tpcc
      --host string              the host of database
      --limit-tx-per-min int     limit the number of transactions to run per minute, 0 means no limit
      --new-order int            specify the percentage of transactions that should be new orders (default 45)
      --order-status int         specify the percentage of transactions that should be order status (default 4)
      --password string          the password of database
      --payment int              specify the percentage of transactions that should be payments (default 43)
      --port int                 the port of database
      --stock-level int          specify the percentage of transactions that should be stock level (default 4)
      --threads ints             specify the number of threads to use (default [1])
      --transactions int         specify the number of transactions that each thread should run
      --user string              the user of database
      --warehouses int           specify the overall database size scaling parameter (default 1)
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

