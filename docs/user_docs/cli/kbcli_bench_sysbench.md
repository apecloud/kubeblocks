---
title: kbcli bench sysbench
---

run a SysBench benchmark

### Options

```
      --database string   database name
      --driver string     database driver
  -h, --help              help for sysbench
      --host string       the host of database
      --password string   the password of database
      --port int          the port of database
      --size int          the data size of per table (default 20000)
      --tables int        the number of tables (default 10)
      --times int         the number of test times (default 100)
      --type string       sysbench type (default "oltp_read_write_pct")
      --user string       the user of database
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
```

### SEE ALSO

* [kbcli bench](kbcli_bench.md)	 - Run a benchmark.
* [kbcli bench sysbench cleanup](kbcli_bench_sysbench_cleanup.md)	 - Cleanup the data of SysBench for cluster
* [kbcli bench sysbench prepare](kbcli_bench_sysbench_prepare.md)	 - Prepare the data of SysBench for a cluster
* [kbcli bench sysbench run](kbcli_bench_sysbench_run.md)	 - Run  SysBench on cluster

#### Go Back to [CLI Overview](cli.md) Homepage.

