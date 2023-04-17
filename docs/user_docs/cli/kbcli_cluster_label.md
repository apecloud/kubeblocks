---
title: kbcli cluster label
---

Update the labels on cluster

```
kbcli cluster label NAME [flags]
```

### Examples

```
  # list label for clusters with specified name
  kbcli cluster label mycluster --list
  
  # add label 'env' and value 'dev' for clusters with specified name
  kbcli cluster label mycluster env=dev
  
  # add label 'env' and value 'dev' for all clusters
  kbcli cluster label env=dev --all
  
  # add label 'env' and value 'dev' for the clusters that match the selector
  kbcli cluster label env=dev -l type=mysql
  
  # update cluster with the label 'env' with value 'test', overwriting any existing value
  kbcli cluster label mycluster --overwrite env=test
  
  # delete label env for clusters with specified name
  kbcli cluster label mycluster env-
```

### Options

```
      --all                            Select all cluster
      --dry-run string[="unchanged"]   Must be "none", "server", or "client". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource. (default "none")
  -h, --help                           help for label
      --list                           If true, display the labels of the clusters
      --overwrite                      If true, allow labels to be overwritten, otherwise reject label updates that overwrite existing labels.
  -l, --selector string                Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2). Matching objects must satisfy all of the specified label constraints.
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

#### Go Back to [CLI Overview](cli.md) Homepage.

