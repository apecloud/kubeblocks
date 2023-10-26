---
title: kbcli bench sysbench prepare
---

Prepare the data of SysBench for a cluster

```
kbcli bench sysbench prepare [ClusterName] [flags]
```

### Examples

```
  # sysbench prepare data on a cluster
  kbcli bench sysbench prepare mycluster --user xxx --password xxx --database mydb
  
  # sysbench prepare data on a cluster with specified tables and size
  kbcli bench sysbench prepare mycluster --user xxx --password xxx --database mydb --tables 10 --size 25000
```

### Options

```
  -h, --help   help for prepare
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
      --flag int                       the flag of sysbench, 0(normal), 1(long), 2(three nodes)
      --host string                    the host of database
      --insecure-skip-tls-verify       If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string              Path to the kubeconfig file to use for CLI requests.
      --match-server-version           Require server version to match client version
  -n, --namespace string               If present, the namespace scope for this CLI request
      --password string                the password of database
      --port int                       the port of database
      --read-percent int               the percent of read, only useful when type is oltp_read_write_pct
      --request-timeout string         The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
  -s, --server string                  The address and port of the Kubernetes API server
      --size int                       the data size of per table (default 25000)
      --tables int                     the number of tables (default 10)
      --threads ints                   the number of threads, you can set multiple values, like 4,8 (default [4])
      --times int                      the number of test times (default 60)
      --tls-server-name string         Server name to use for server certificate validation. If it is not provided, the hostname used to contact the server is used
      --token string                   Bearer token for authentication to the API server
      --type strings                   sysbench type, you can set multiple values (default [oltp_read_write])
      --user string                    the user of database
      --write-percent int              the percent of write, only useful when type is oltp_read_write_pct
```

### SEE ALSO

* [kbcli bench sysbench](kbcli_bench_sysbench.md)	 - run a SysBench benchmark

#### Go Back to [CLI Overview](cli.md) Homepage.

