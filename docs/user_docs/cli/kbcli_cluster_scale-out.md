---
title: kbcli cluster scale-out
---

scale out replicas of the specified components in the cluster.

```
kbcli cluster scale-out Replicas [flags]
```

### Examples

```
  # scale out 2 replicas
  kbcli cluster scale-out mycluster --components=mysql --replicas=2
  
  # to bring the offline instances specified in compSpec.offlineInstances online.
  kbcli cluster scale-out mycluster --components=mysql --online-instances pod1
  
  # scale out 2 replicas, one of which is an instance that has already been taken offline.
  kbcli cluster scale-out mycluster --components=mysql --replicas=2 --online-instances pod1
```

### Options

```
      --auto-approve                   Skip interactive approval before horizontally scaling the cluster
      --components strings             Component names to this operations
      --dry-run string[="unchanged"]   Must be "client", or "server". If with client strategy, only print the object that would be sent, and no data is actually sent. If with server strategy, submit the server-side request, but no data is persistent. (default "none")
      --edit                           Edit the API resource before creating
      --force                           skip the pre-checks of the opsRequest to run the opsRequest forcibly
  -h, --help                           help for scale-out
      --name string                    OpsRequest name. if not specified, it will be randomly generated
      --online-instances strings       online the specified instances which have been offline
  -o, --output format                  Prints the output in the specified format. Allowed values: JSON and YAML (default yaml)
      --replicas string                Replica changes with the specified components
      --ttlSecondsAfterSucceed int     Time to live after the OpsRequest succeed
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

