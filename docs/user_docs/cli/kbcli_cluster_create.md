---
title: kbcli cluster create
---

Create a cluster.

```
kbcli cluster create [NAME] [flags]
```

### Examples

```
  # Create a cluster with cluster definition apecloud-mysql and cluster version ac-mysql-8.0.30
  kbcli cluster create mycluster --cluster-definition apecloud-mysql --cluster-version ac-mysql-8.0.30
  
  # --cluster-definition is required, if --cluster-version is not specified, pick the most recently created version
  kbcli cluster create mycluster --cluster-definition apecloud-mysql
  
  # Output resource information in YAML format, without creation of resources.
  kbcli cluster create mycluster --cluster-definition apecloud-mysql --dry-run -o yaml
  
  # Output resource information in YAML format, the information will be sent to the server
  # but the resources will not be actually created.
  kbcli cluster create mycluster --cluster-definition apecloud-mysql --dry-run=server -o yaml
  
  # Create a cluster and set termination policy DoNotTerminate that prevents the cluster from being deleted
  kbcli cluster create mycluster --cluster-definition apecloud-mysql --termination-policy DoNotTerminate
  
  # Delete resources such as statefulsets, deployments, services, pdb, but keep PVCs
  # when deleting the cluster, use termination policy Halt
  kbcli cluster create mycluster --cluster-definition apecloud-mysql --termination-policy Halt
  
  # Delete resource such as statefulsets, deployments, services, pdb, and including
  # PVCs when deleting the cluster, use termination policy Delete
  kbcli cluster create mycluster --cluster-definition apecloud-mysql --termination-policy Delete
  
  # Delete all resources including all snapshots and snapshot data when deleting
  # the cluster, use termination policy WipeOut
  kbcli cluster create mycluster --cluster-definition apecloud-mysql --termination-policy WipeOut
  
  # Create a cluster and set cpu to 1 core, memory to 1Gi, storage size to 20Gi and replicas to 3
  kbcli cluster create mycluster --cluster-definition apecloud-mysql --set cpu=1,memory=1Gi,storage=20Gi,replicas=3
  
  # Create a cluster and set storageClass to csi-hostpath-sc, if storageClass is not specified,
  # the default storage class will be used
  kbcli cluster create mycluster --cluster-definition apecloud-mysql --set storageClass=csi-hostpath-sc
  
  # Create a cluster with replicationSet workloadType and set switchPolicy to Noop
  kbcli cluster create mycluster --cluster-definition postgresql --set switchPolicy=Noop
  
  # Create a cluster with more than one component, use "--set type=component-name" to specify the component,
  # if not specified, the main component will be used, run "kbcli cd list-components CLUSTER-DEFINITION-NAME"
  # to show the components in the cluster definition
  kbcli cluster create mycluster --cluster-definition redis --set type=redis,cpu=1 --set type=redis-sentinel,cpu=200m
  
  # Create a cluster and use a URL to set cluster resource
  kbcli cluster create mycluster --cluster-definition apecloud-mysql \
  --set-file https://kubeblocks.io/yamls/apecloud-mysql.yaml
  
  # Create a cluster and load cluster resource set from stdin
  cat << EOF | kbcli cluster create mycluster --cluster-definition apecloud-mysql --set-file -
  - name: my-test ...
  
  # Create a cluster scattered by nodes
  kbcli cluster create --cluster-definition apecloud-mysql --topology-keys kubernetes.io/hostname \
  --pod-anti-affinity Required
  
  # Create a cluster in specific labels nodes
  kbcli cluster create --cluster-definition apecloud-mysql \
  --node-labels '"topology.kubernetes.io/zone=us-east-1a","disktype=ssd,essd"'
  
  # Create a Cluster with two tolerations
  kbcli cluster create --cluster-definition apecloud-mysql --tolerations \ '"engineType=mongo:NoSchedule","diskType=ssd:NoSchedule"'
  
  # Create a cluster, with each pod runs on their own dedicated node
  kbcli cluster create --cluster-definition apecloud-mysql --tenancy=DedicatedNode
  
  # Create a cluster with backup to restore data
  kbcli cluster create --backup backup-default-mycluster-20230616190023
  
  # Create a cluster with time to restore from point in time
  kbcli cluster create --restore-to-time "Jun 16,2023 18:58:53 UTC+0800" --source-cluster mycluster
```

### Options

```
      --backup string                  Set a source backup to restore data
      --cluster-definition string      Specify cluster definition, run "kbcli cd list" to show all available cluster definitions
      --cluster-version string         Specify cluster version, run "kbcli cv list" to show all available cluster versions, use the latest version if not specified
      --dry-run string[="unchanged"]   Must be "client", or "server". If with client strategy, only print the object that would be sent, and no data is actually sent. If with server strategy, submit the server-side request, but no data is persistent. (default "none")
      --edit                           Edit the API resource before creating
      --enable-all-logs                Enable advanced application all log extraction, set to true will ignore enabledLogs of component level, default is false
  -h, --help                           help for create
      --monitor                        Set monitor enabled and inject metrics exporter (default true)
      --node-labels stringToString     Node label selector (default [])
  -o, --output format                  Prints the output in the specified format. Allowed values: JSON and YAML (default yaml)
      --pod-anti-affinity string       Pod anti-affinity type, one of: (Preferred, Required) (default "Preferred")
      --rbac-enabled                   Specify whether rbac resources will be created by kbcli, otherwise KubeBlocks server will try to create rbac resources
      --restore-to-time string         Set a time for point in time recovery
      --set stringArray                Set the cluster resource including cpu, memory, replicas and storage, each set corresponds to a component.(e.g. --set cpu=1,memory=1Gi,replicas=3,storage=20Gi or --set class=general-1c1g)
  -f, --set-file string                Use yaml file, URL, or stdin to set the cluster resource
      --source-cluster string          Set a source cluster for point in time recovery
      --tenancy string                 Tenancy options, one of: (SharedNode, DedicatedNode) (default "SharedNode")
      --termination-policy string      Termination policy, one of: (DoNotTerminate, Halt, Delete, WipeOut) (default "Delete")
      --tolerations strings            Tolerations for cluster, such as "key=value:effect, key:effect", for example '"engineType=mongo:NoSchedule", "diskType:NoSchedule"'
      --topology-keys stringArray      Topology keys for affinity
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
* [kbcli cluster create kafka](kbcli_cluster_create_kafka.md)	 - Create a kafka cluster.
* [kbcli cluster create mongodb](kbcli_cluster_create_mongodb.md)	 - Create a mongodb cluster.
* [kbcli cluster create mysql](kbcli_cluster_create_mysql.md)	 - Create a mysql cluster.
* [kbcli cluster create postgresql](kbcli_cluster_create_postgresql.md)	 - Create a postgresql cluster.
* [kbcli cluster create redis](kbcli_cluster_create_redis.md)	 - Create a redis cluster.

#### Go Back to [CLI Overview](cli.md) Homepage.

