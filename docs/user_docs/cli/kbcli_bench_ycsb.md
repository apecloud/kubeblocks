---
title: kbcli bench ycsb
---

Run YCSB benchmark on a cluster

```
kbcli bench ycsb [Step] [BenchmarkName] [flags]
```

### Examples

```
  # ycsb on a cluster,  that will exec for all steps, cleanup, prepare and run
  kbcli bench ycsb mytest --cluster mycluster --user xxx --password xxx --database mydb
  
  # ycsb on a cluster, but with cpu and memory limits set
  kbcli bench ycsb mytest --cluster mycluster --user xxx --password xxx --database mydb --limit-cpu 1 --limit-memory 1Gi
  
  # ycsb on a cluster with cleanup, only cleanup by deleting the testdata
  kbcli bench ycsb cleanup mytest --cluster mycluster --user xxx --password xxx --database mydb
  
  # ycsb on a cluster with prepare, just prepare by creating the testdata
  kbcli bench ycsb prepare mytest --cluster mycluster --user xxx --password xxx --database mydb
  
  # ycsb on a cluster with run, just run by running the test
  kbcli bench ycsb run mytest --cluster mycluster --user xxx --password xxx --database mydb
  
  # ycsb on a cluster with thread counts
  kbcli bench ycsb mytest --cluster mycluster --user xxx --password xxx --database mydb --threads 4,8
  
  # ycsb on a cluster with record number and operation number
  kbcli bench ycsb mytest --cluster mycluster --user xxx --password xxx --database mydb --record-count 10000 --operation-count 10000
  
  # ycsb on a cluster mixed read/write
  kbcli bench ycsb mytest --cluster mycluster --user xxx --password xxx --database mydb --read-proportion 50 --update-proportion 50
```

### Options

```
      --cluster string                     the cluster of database
      --database string                    database name
      --driver string                      the driver of database
      --extra-args strings                 extra arguments for benchmark
  -h, --help                               help for ycsb
      --host string                        the host of database
      --insert-proportion int              the percentage of insert operations in benchmark
      --limit-cpu string                   the limit cpu of benchmark
      --limit-memory string                the limit memory of benchmark
      --operation-count int                the number of operations to use during the run phase (default 1000)
      --password string                    the password of database
      --port int                           the port of database
      --read-modify-write-proportion int   the percentage of read-modify-write operations in benchmark, which read a record, modify it, and write it back
      --read-proportion int                the percentage of read operations in benchmark
      --record-count int                   the number of records to use (default 1000)
      --request-cpu string                 the request cpu of benchmark
      --request-memory string              the request memory of benchmark
      --scan-proportion int                the percentage of scan operations in benchmark
      --threads ints                       the number of threads to use (default [1])
      --tolerations strings                Tolerations for benchmark, such as '"dev=true:NoSchedule,large=true:NoSchedule"'
      --update-proportion int              the percentage of update operations in benchmark
      --user string                        the user of database
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

