---
title: kbcli cluster rebuild-instance
---

Rebuild the specified instances in the cluster.

```
kbcli cluster rebuild-instance NAME [flags]
```

### Examples

```
  # rebuild instance by creating new instances and remove the specified instances after the new instances are ready.
  kbcli cluster rebuild-instance mycluster --instances pod1,pod2
  
  # rebuild instance to a new node.
  kbcli cluster rebuild-instance mycluster --instances pod1 --node nodeName.
  
  # rebuild instance with the same pod name.
  kbcli cluster rebuild-instance mycluster --instances pod1 --in-place
  
  # rebuild instance from backup and with the same pod name
  kbcli cluster rebuild-instance mycluster --instances pod1,pod2 --backupName <backup> --in-place
```

### Options

```
      --auto-approve                   Skip interactive approval before rebuilding the instances.gi
      --backup string                  instances will be rebuild by the specified backup.
      --dry-run string[="unchanged"]   Must be "client", or "server". If with client strategy, only print the object that would be sent, and no data is actually sent. If with server strategy, submit the server-side request, but no data is persistent. (default "none")
      --edit                           Edit the API resource before creating
      --force                           skip the pre-checks of the opsRequest to run the opsRequest forcibly
  -h, --help                           help for rebuild-instance
      --in-place                       rebuild the instance with the same pod name. if not set, will create a new instance by horizontalScaling and remove the instance after the new instance is ready
      --instances strings              instances which need to rebuild.
      --name string                    OpsRequest name. if not specified, it will be randomly generated
      --node strings                   specified the target node which rebuilds the instance on the node otherwise will rebuild on a random node. format: insName1=nodeName,insName2=nodeName
  -o, --output format                  Prints the output in the specified format. Allowed values: JSON and YAML (default yaml)
      --restore-env stringArray        provide the necessary env for the 'Restore' operation from the backup. format: key1=value, key2=value
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

