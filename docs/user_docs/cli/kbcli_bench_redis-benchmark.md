---
title: kbcli bench redis-benchmark
---

Run redis-benchmark on a cluster

```
kbcli bench redis-benchmark [flags]
```

### Examples

```
  # redis-benchmark run on a cluster
  kbcli bench redis-benchmark mytest --cluster rediscluster --clients 50 --requests 10000 --password xxx
  
  # redis-benchmark run on a cluster, but with cpu and memory limits set
  kbcli bench redis-benchmark mytest --cluster rediscluster --clients 50 --requests 10000 --limit-cpu 1 --limit-memory 1Gi --password xxx
  
  # redis-benchmark run on a cluster, just test set/get
  kbcli bench redis-benchmark mytest --cluster rediscluster --clients 50 --requests 10000 --tests set,get --password xxx
  
  # redis-benchmark run on a cluster, just test set/get with key space
  kbcli bench redis-benchmark mytest --cluster rediscluster --clients 50 --requests 10000 --tests set,get --key-space 100000 --password xxx
  
  # redis-benchmark run on a cluster, with pipeline
  kbcli bench redis-benchmark mytest --cluster rediscluster --clients 50 --requests 10000 --pipeline 10 --password xxx
  
  # redis-benchmark run on a cluster, with csv output
  kbcli bench redis-benchmark mytest --cluster rediscluster --clients 50 --requests 10000 --quiet false --extra-args "--csv" --password xxx
```

### Options

```
      --clients ints            number of parallel connections (default [50])
      --cluster string          the cluster of database
      --data-size int           data size of set/get value in bytes (default 3)
      --database string         database name
      --driver string           the driver of database
      --extra-args strings      extra arguments for benchmark
  -h, --help                    help for redis-benchmark
      --host string             the host of database
      --key-space int           use random keys for SET/GET/INCR, random values for SADD
      --limit-cpu string        the limit cpu of benchmark
      --limit-memory string     the limit memory of benchmark
      --password string         the password of database
      --pipeline int            pipelining num requests. Default 1 (no pipeline). (default 1)
      --port int                the port of database
      --quiet                   quiet mode. Just show query/sec values (default true)
      --request-cpu string      the request cpu of benchmark
      --request-memory string   the request memory of benchmark
      --requests int            total number of requests (default 10000)
      --tests string            only run the comma separated list of tests. The test names are the same as the ones produced as output.
      --tolerations strings     Tolerations for benchmark, such as '"dev=true:NoSchedule,large=true:NoSchedule"'
      --user string             the user of database
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

