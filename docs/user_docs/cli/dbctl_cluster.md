## dbctl cluster

Database cluster operation command

### Options

```
  -h, --help   help for cluster
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

* [dbctl](dbctl.md)	 - KubeBlocks CLI
* [dbctl cluster connect](dbctl_cluster_connect.md)	 - connect to a database cluster
* [dbctl cluster create](dbctl_cluster_create.md)	 - Create a database cluster
* [dbctl cluster delete](dbctl_cluster_delete.md)	 - Delete a cluster
* [dbctl cluster delete-ops](dbctl_cluster_delete-ops.md)	 - Delete a OpsRequest
* [dbctl cluster describe](dbctl_cluster_describe.md)	 - Describe database cluster info
* [dbctl cluster horizontal-scaling](dbctl_cluster_horizontal-scaling.md)	 - horizontal scaling the specified components in the cluster
* [dbctl cluster list](dbctl_cluster_list.md)	 - List all cluster.
* [dbctl cluster list-ops](dbctl_cluster_list-ops.md)	 - List all opsRequest.
* [dbctl cluster restart](dbctl_cluster_restart.md)	 - restart the specified components in the cluster
* [dbctl cluster upgrade](dbctl_cluster_upgrade.md)	 - upgrade the cluster
* [dbctl cluster vertical-scaling](dbctl_cluster_vertical-scaling.md)	 - vertical scaling the specified components in the cluster
* [dbctl cluster volume-expansion](dbctl_cluster_volume-expansion.md)	 - expand volume with the specified components and volumeClaimTemplates in the cluster

