---
title: kbcli bench tpch
---

Run tpch benchmark

```
kbcli bench tpch [Step] [BenchmarkName] [flags]
```

### Examples

```
  # tpch on a cluster, that will exec for all steps, cleanup, prepare and run
  kbcli bench tpch mytest --cluster mycluster --user xxx --password xxx --database mydb
  
  # tpch on a cluster with run, just run by running the test
  kbcli bench tpch run mytest --cluster mycluster --user xxx --password xxx --database mydb
```

### Options

```
      --cluster string        the cluster of database
      --database string       database name
      --driver string         the driver of database
      --extra-args strings    extra arguments for benchmark
  -h, --help                  help for tpch
      --host string           the host of database
      --password string       the password of database
      --port int              the port of database
      --tolerations strings   Tolerations for benchmark, such as '"dev=true:NoSchedule,large=true:NoSchedule"'
      --user string           the user of database
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

