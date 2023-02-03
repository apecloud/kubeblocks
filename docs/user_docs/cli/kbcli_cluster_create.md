## kbcli cluster create

Create a cluster

```
kbcli cluster create [NAME] [flags]
```

### Examples

```
  # Create a cluster using cluster definition my-cluster-def and cluster version my-version
  kbcli cluster create mycluster --cluster-definition=my-cluster-def --cluster-version=my-version
  
  # --cluster-definition is required, if --cluster-version is not specified, will use the latest cluster version
  kbcli cluster create mycluster --cluster-definition=my-cluster-def
  
  # Create a cluster using file my.yaml and termination policy DoNotDelete that will prevent
  # the cluster from being deleted
  kbcli cluster create mycluster --cluster-definition=my-cluster-def --set=my.yaml --termination-policy=DoNotDelete
  
  # In scenarios where you want to delete resources such as sts, deploy, svc, pdb, but keep pvcs when deleting
  # the cluster, use termination policy Halt
  kbcli cluster create mycluster --cluster-definition=my-cluster-def --set=my.yaml --termination-policy=Halt
  
  # In scenarios where you want to delete resource such as sts, deploy, svc, pdb, and including pvcs when
  # deleting the cluster, use termination policy Delete
  kbcli cluster create mycluster --cluster-definition=my-cluster-def --set=my.yaml --termination-policy=Delete
  
  # In scenarios where you want to delete all resources including all snapshots and snapshot data when deleting
  # the cluster, use termination policy WipeOut
  kbcli cluster create mycluster --cluster-definition=my-cluster-def --set=my.yaml --termination-policy=WipeOut
  
  # In scenarios where you want to load components data from website URL
  kbcli cluster create mycluster --cluster-definition=my-cluster-def --set=https://kubeblocks.io/yamls/my.yaml
  
  # In scenarios where you want to load components data from stdin
  cat << EOF | kbcli cluster create mycluster --cluster-definition=my-cluster-def --set -
  - name: my-test ...
  
  # Create a cluster forced to scatter by node
  kbcli cluster create --cluster-definition=my-cluster-def --topology-keys=kubernetes.io/hostname --pod-anti-affinity=Required
  
  # Create a cluster in specific labels nodes
  kbcli cluster create --cluster-definition=my-cluster-def --node-labels='"topology.kubernetes.io/zone=us-east-1a","disktype=ssd,essd"'
  
  # Create a Cluster with two tolerations
  kbcli cluster create --cluster-definition=my-cluster-def --tolerations='"key=engineType,value=mongo,operator=Equal,effect=NoSchedule","key=diskType,value=ssd,operator=Equal,effect=NoSchedule"'
```

### Options

```
      --backup string                Set a source backup to restore data
      --cluster-definition string    Specify cluster definition, run "kbcli cluster-definition list" to show all available cluster definition
      --cluster-version string       Specify cluster version, run "kbcli cluster-version list" to show all available cluster version, use the latest version if not specified
      --enable-all-logs              Enable advanced application all log extraction, and true will ignore enabledLogs of component level (default true)
  -h, --help                         help for create
      --monitor                      Set monitor enabled and inject metrics exporter (default true)
      --node-labels stringToString   Node label selector (default [])
      --pod-anti-affinity string     Pod anti-affinity type (default "Preferred")
      --set string                   Use yaml file, URL, or stdin to set the cluster parameters
      --termination-policy string    Termination policy, one of: (DoNotTerminate, Halt, Delete, WipeOut) (default "Delete")
      --tolerations strings          Tolerations for cluster, such as '"key=engineType,value=mongo,operator=Equal,effect=NoSchedule"'
      --topology-keys stringArray    Topology keys for affinity
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

* [kbcli cluster](kbcli_cluster.md)	 - Cluster command

