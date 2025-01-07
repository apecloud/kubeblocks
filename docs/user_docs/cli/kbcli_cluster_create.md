---
title: kbcli cluster create
---

Create a cluster.

```
kbcli cluster create [NAME] [flags]
```

### Examples

```
  # Create a postgresql
  kbcli cluster create postgresql my-cluster
  
  # Get the cluster yaml by dry-run
  kbcli cluster create postgresql my-cluster --dry-run
  
  # Edit cluster yaml before creation.
  kbcli cluster create mycluster --edit
```

### Options

```
  -h, --help   help for create
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
      --user string                    The name of the kubeconfig user to use
```

### SEE ALSO

* [kbcli cluster](kbcli_cluster.md)	 - Cluster command.
* [kbcli cluster create apecloud-mysql](kbcli_cluster_create_apecloud-mysql.md)	 - Create a apecloud-mysql cluster.
* [kbcli cluster create elasticsearch](kbcli_cluster_create_elasticsearch.md)	 - Create a elasticsearch cluster.
* [kbcli cluster create etcd](kbcli_cluster_create_etcd.md)	 - Create a etcd cluster.
* [kbcli cluster create kafka](kbcli_cluster_create_kafka.md)	 - Create a kafka cluster.
* [kbcli cluster create llm](kbcli_cluster_create_llm.md)	 - Create a llm cluster.
* [kbcli cluster create mongodb](kbcli_cluster_create_mongodb.md)	 - Create a mongodb cluster.
* [kbcli cluster create mysql](kbcli_cluster_create_mysql.md)	 - Create a mysql cluster.
* [kbcli cluster create postgresql](kbcli_cluster_create_postgresql.md)	 - Create a postgresql cluster.
* [kbcli cluster create qdrant](kbcli_cluster_create_qdrant.md)	 - Create a qdrant cluster.
* [kbcli cluster create redis](kbcli_cluster_create_redis.md)	 - Create a redis cluster.
* [kbcli cluster create xinference](kbcli_cluster_create_xinference.md)	 - Create a xinference cluster.

#### Go Back to [CLI Overview](cli.md) Homepage.

