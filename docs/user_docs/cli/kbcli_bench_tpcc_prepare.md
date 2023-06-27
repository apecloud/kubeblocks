---
title: kbcli bench tpcc prepare
---

Prepare data for TPCC.

```
kbcli bench tpcc prepare [flags]
```

### Options

```
  -h, --help                      help for prepare
      --no-check                  TPCC prepare check, default false
      --output-dir string         Output directory for generating file if specified
      --output-type string        Output file type. If not set, database generates the data itself. Current only support csv
      --retry-count int           Retry count when errors occur (default 50)
      --retry-interval duration   The interval for each retry (default 5s)
      --tables string             Specified tables for generating file, separated by ','. Valid only if output is set. If not set, generate all tables by default
```

### Options inherited from parent commands

```
      --as string                      Username to impersonate for the operation. User could be a regular user or a service account in a namespace.
      --as-group stringArray           Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --as-uid string                  UID to impersonate for the operation.
      --cache-dir string               Default cache directory (default "$HOME/.kube/cache")
      --certificate-authority string   Path to a cert file for the certificate authority
      --check-all                      Run all consistency checks
      --client-certificate string      Path to a client certificate file for TLS
      --client-key string              Path to a client key file for TLS
      --cluster string                 The name of the kubeconfig cluster to use
      --context string                 The name of the kubeconfig context to use
      --count int                      Total execution count, 0 means infinite
  -D, --db string                      Database name (default "kb_test")
      --disable-compression            If true, opt-out of response compression for all requests to the server
  -d, --driver string                  Database driver: mysql (default "mysql")
      --dropdata                       Cleanup data before prepare
  -H, --host string                    Database host (default "127.0.0.1")
      --ignore-error                   Ignore error when running workload
      --insecure-skip-tls-verify       If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --interval duration              Output interval time (default 5s)
      --kubeconfig string              Path to the kubeconfig file to use for CLI requests.
      --match-server-version           Require server version to match client version
      --max-procs int                  runtime.GOMAXPROCS
  -n, --namespace string               If present, the namespace scope for this CLI request
      --partition-type int             Partition type (1 - HASH, 2 - RANGE, 3 - LIST (like HASH), 4 - LIST (like RANGE) (default 1)
      --parts int                      Number to partition warehouses (default 1)
  -p, --password string                Database password (default "sakila")
  -P, --port int                       Database port (default 3306)
      --request-timeout string         The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
  -s, --server string                  The address and port of the Kubernetes API server
      --silence                        Don't print error when running workload
  -T, --threads int                    Thread concurrency (default 1)
      --time duration                  Total execution time (default 2562047h47m16.854775807s)
      --tls-server-name string         Server name to use for server certificate validation. If it is not provided, the hostname used to contact the server is used
      --token string                   Bearer token for authentication to the API server
  -U, --user string                    Database user (default "root")
      --warehouses int                 Number of warehouses (default 4)
```

### SEE ALSO

* [kbcli bench tpcc](kbcli_bench_tpcc.md)	 - Run a TPCC benchmark.

#### Go Back to [CLI Overview](cli.md) Homepage.

