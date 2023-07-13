---
title: kbcli cluster create kafka
---

Create a kafka cluster.

```
kbcli cluster create kafka NAME [flags]
```

### Examples

```
  # Create a cluster with the default values
  kbcli cluster create kafka
  
  # Create a cluster with the specified cpu, memory and storage
  kbcli cluster create kafka --cpu 1 --memory 2 --storage 10
```

### Options

```
      --availability-policy string   The availability policy of cluster. Legal values [none, node, zone]. (default "node")
      --broker-heap string           Kafka broker's jvm heap setting. (default "-XshowSettings:vm -XX:MaxRAMPercentage=100 -Ddepth=64")
      --broker-replicas int          The number of Kafka broker replicas for separated mode. Value range [1, 100]. (default 1)
      --controller-heap string       Kafka controller's jvm heap setting for separated mode (default "-XshowSettings:vm -XX:MaxRAMPercentage=100 -Ddepth=64")
      --controller-replicas int      The number of Kafka controller replicas for separated mode. Legal values [1, 3, 5]. (default 1)
      --cpu float                    CPU cores. Value range [0.5, 64]. (default 0.5)
  -h, --help                         help for kafka
      --host-network-accessible      Specify whether the cluster can be accessed from within the VPC.
      --memory float                 Memory, the unit is Gi. Value range [0.5, 1000]. (default 0.5)
      --meta-storage float           Metadata Storage size, the unit is Gi. Value range [1, 10000]. (default 5)
      --meta-storage-class string    The StorageClass for Kafka Metadata Storage.
      --mode string                  Mode for Kafka kraft cluster, 'combined' is combined Kafka controller and broker,'separated' is broker and controller running independently. Legal values [combined, separated]. (default "combined")
      --monitor-enable               Enable monitor for Kafka.
      --monitor-replicas int         The number of Kafka monitor replicas. Value range [1, 5]. (default 1)
      --monitoring-interval int      The monitoring interval of cluster, 0 is disabled, the unit is second. Value range [0, 60].
      --publicly-accessible          Specify whether the cluster can be accessed from the public internet.
      --rbac-enabled                 Specify whether rbac resources will be created by client, otherwise KubeBlocks server will try to create rbac resources.
      --replicas int                 The number of Kafka broker replicas for combined mode. Legal values [1, 3, 5]. (default 1)
      --sasl-enable                  Enable authentication using SASL/PLAIN for Kafka.
      --storage float                Data Storage size, the unit is Gi. Value range [1, 10000]. (default 10)
      --storage-class string         The StorageClass for Kafka Data Storage.
      --storage-enable               Enable storage for Kafka.
      --tenancy string               The tenancy of cluster. Legal values [SharedNode, DedicatedNode]. (default "SharedNode")
      --termination-policy string    The termination policy of cluster. Legal values [DoNotTerminate, Halt, Delete, WipeOut]. (default "Delete")
      --version string               Cluster version.
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
      --edit                           Edit the API resource before creating
      --insecure-skip-tls-verify       If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string              Path to the kubeconfig file to use for CLI requests.
      --match-server-version           Require server version to match client version
  -n, --namespace string               If present, the namespace scope for this CLI request
  -o, --output format                  Prints the output in the specified format. Allowed values: JSON and YAML (default yaml)
      --request-timeout string         The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
  -s, --server string                  The address and port of the Kubernetes API server
      --tls-server-name string         Server name to use for server certificate validation. If it is not provided, the hostname used to contact the server is used
      --token string                   Bearer token for authentication to the API server
      --user string                    The name of the kubeconfig user to use
```

### SEE ALSO

* [kbcli cluster create](kbcli_cluster_create.md)	 - Create a cluster.

#### Go Back to [CLI Overview](cli.md) Homepage.

