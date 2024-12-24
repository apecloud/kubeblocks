---
title: kbcli bench tpcc
---

Run tpcc benchmark

```
kbcli bench tpcc [Step] [BenchmarkName] [flags]
```

### Examples

```
  # tpcc on a cluster, that will exec for all steps, cleanup, prepare and run
  kbcli bench tpcc mytest --cluster mycluster --user xxx --password xxx --database mydb
  
  # tpcc on a cluster, but with cpu and memory limits set
  kbcli bench tpcc mytest --cluster mycluster --user xxx --password xxx --database mydb --limit-cpu 1 --limit-memory 1Gi
  
  # tpcc on a cluster with cleanup, only cleanup by deleting the testdata
  kbcli bench tpcc cleanup mytest --cluster mycluster --user xxx --password xxx --database mydb
  
  # tpcc on a cluster with prepare, just prepare by creating the testdata
  kbcli bench tpcc prepare mytest --cluster mycluster --user xxx --password xxx --database mydb
  
  # tpcc on a cluster with run, just run by running the test
  kbcli bench tpcc run mytest --cluster mycluster --user xxx --password xxx --database mydb
  
  # tpcc on a cluster with warehouse counts, which is the overall database size scaling parameter
  kbcli bench tpcc mytest --cluster mycluster --user xxx --password xxx --database mydb --warehouses 100
  
  # tpcc on a cluster with thread counts
  kbcli bench tpcc mytest --cluster mycluster --user xxx --password xxx --database mydb --threads 4,8
  
  # tpcc on a cluster with transactions counts
  kbcli bench tpcc mytest --cluster mycluster --user xxx --password xxx --database mydb --transactions 1000
  
  # tpcc on a cluster with duration 10 minutes
  kbcli bench tpcc mytest --cluster mycluster --user xxx --password xxx --database mydb --duration 10
```

### Options

```
      --cluster string          the cluster of database
      --database string         database name
      --delivery int            specify the percentage of transactions that should be delivery (default 4)
      --driver string           the driver of database
      --duration int            specify the number of minutes to run (default 1)
      --extra-args strings      extra arguments for benchmark
  -h, --help                    help for tpcc
      --host string             the host of database
      --limit-cpu string        the limit cpu of benchmark
      --limit-memory string     the limit memory of benchmark
      --limit-tx-per-min int    limit the number of transactions to run per minute, 0 means no limit
      --new-order int           specify the percentage of transactions that should be new orders (default 45)
      --order-status int        specify the percentage of transactions that should be order status (default 4)
      --password string         the password of database
      --payment int             specify the percentage of transactions that should be payments (default 43)
      --port int                the port of database
      --request-cpu string      the request cpu of benchmark
      --request-memory string   the request memory of benchmark
      --stock-level int         specify the percentage of transactions that should be stock level (default 4)
      --threads ints            specify the number of threads to use (default [1])
      --tolerations strings     Tolerations for benchmark, such as '"dev=true:NoSchedule,large=true:NoSchedule"'
      --transactions int        specify the number of transactions that each thread should run
      --user string             the user of database
      --warehouses int          specify the overall database size scaling parameter (default 1)
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

