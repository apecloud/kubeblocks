---
title: kbcli cluster create mysql
---

Create a MySQL cluster.

```
kbcli cluster create mysql [flags]
```

### Options

```
      --available-policy string     The available policy of cluster (default "single")
      --cpu float                   cpu (default 1)
  -h, --help                        help for mysql
      --host-network-accessible     The hostNetworkAccessible of cluster (default true)
      --memory float                memory, the unit is Gi (default 1)
      --mode string                 mysql cluster topology (default "standalone")
      --monitor                     Enable monitor or not (default true)
      --name string                 cluster name (default "mycluster")
      --port int                    The port of cluster (default 3306)
      --proxy-enabled               Enable proxy or not
      --publicly-accessible         The publiclyAccessible of cluster
      --replicas int                replicas (default 1)
      --storage-engine string       The storage engine of cluster (default "innodb")
      --storage-size float          storage size, the unit is Gi (default 20)
      --tenancy string              The tenancy of cluster (default "DedicatedNode")
      --termination-policy string   The termination policy of cluster (default "Delete")
      --version string              mysql version (default "ac-mysql-8.0.30")
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
      --dry-run string[="unchanged"]   Must be "client", or "server". If with client strategy, only print the object that would be sent, and no data is actually sent. If with server strategy, submit the server-side request, but no data is persistent. (default "none")
      --insecure-skip-tls-verify       If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string              Path to the kubeconfig file to use for CLI requests.
      --match-server-version           Require server version to match client version
  -n, --namespace string               If present, the namespace scope for this CLI request
  -o, --output format                  prints the output in the specified format. Allowed values: table, json, yaml, wide (default table)
      --request-timeout string         The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
  -s, --server string                  The address and port of the Kubernetes API server
      --tls-server-name string         Server name to use for server certificate validation. If it is not provided, the hostname used to contact the server is used
      --token string                   Bearer token for authentication to the API server
      --user string                    The name of the kubeconfig user to use
```

### SEE ALSO

* [kbcli cluster create](kbcli_cluster_create.md)	 - Create a cluster.

#### Go Back to [CLI Overview](cli.md) Homepage.

