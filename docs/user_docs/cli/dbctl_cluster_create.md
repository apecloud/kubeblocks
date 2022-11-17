## dbctl cluster create

Create a database cluster

```
dbctl cluster create NAME --termination-policy=DoNotTerminate|Halt|Delete|WipeOut --components=file-path [flags]
```

### Examples

```
  # Create a cluster using component file component.yaml and termination policy DoNotDelete that will prevent
  # the cluster from being deleted
  dbctl cluster create mycluster --components=component.yaml --termination-policy=DoNotDelete
  
  # In scenarios where you want to delete resources such as sts, deploy, svc, pdb, but keep pvcs when deleting
  # the cluster, use termination policy Halt
  dbctl cluster create mycluster --components=component.yaml --termination-policy=Halt
  
  # In scenarios where you want to delete resource such as sts, deploy, svc, pdb, and including pvcs when
  # deleting the cluster, use termination policy Delete
  dbctl cluster create mycluster --components=component.yaml --termination-policy=Delete
  
  # In scenarios where you want to delete all resources including all snapshots and snapshot data when deleting
  # the cluster, use termination policy WipeOut
  dbctl cluster create mycluster --components=component.yaml --termination-policy=WipeOut
```

### Options

```
      --app-version string           AppVersion reference (default "wesql-8.0.30")
      --cluster-definition string    ClusterDefinition reference (default "apecloud-wesql")
      --components string            Use yaml file to specify the cluster components
      --enable-all-logs              Enable advanced application all log extraction, and true will ignore enabledLogs of component level
  -h, --help                         help for create
      --monitor                      Set monitor enabled (default false)
      --node-labels stringToString   Node label selector (default [])
      --pod-anti-affinity string     Pod anti-affinity type (default "Preferred")
      --termination-policy string    Termination policy, one of: (DoNotTerminate, Halt, Delete, WipeOut)
      --topology-keys stringArray    Topology keys for affinity
```

### Options inherited from parent commands

```
      --as string                      Username to impersonate for the operation. User could be a regular user or a service account in a namespace.
      --as-group stringArray           Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --as-uid string                  UID to impersonate for the operation.
      --cache-dir string               Default cache directory (default "/Users/ldm/.kube/cache")
      --certificate-authority string   Path to a cert file for the certificate authority
      --client-certificate string      Path to a client certificate file for TLS
      --client-key string              Path to a client key file for TLS
      --cluster string                 The name of the kubeconfig cluster to use
      --context string                 The name of the kubeconfig context to use
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

* [dbctl cluster](dbctl_cluster.md)	 - Database cluster operation command

