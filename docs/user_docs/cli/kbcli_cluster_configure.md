---
title: kbcli cluster configure
---

Configure parameters with the specified components in the cluster.

```
kbcli cluster configure NAME --set key=value[,key=value] [--component=component-name] [--config-spec=config-spec-name] [--config-file=config-file] [flags]
```

### Examples

```
  # update component params
  kbcli cluster configure mycluster --component=mysql --config-spec=mysql-3node-tpl --config-file=my.cnf --set max_connections=1000,general_log=OFF
  
  # if only one component, and one config spec, and one config file, simplify the searching process of configure. e.g:
  # update mysql max_connections, cluster name is mycluster
  kbcli cluster configure mycluster --set max_connections=2000
```

### Options

```
      --auto-approve                   Skip interactive approval before reconfiguring the cluster
      --component string               Specify the name of Component to be updated. If the cluster has only one component, unset the parameter.
      --config-file string             Specify the name of the configuration file to be updated (e.g. for mysql: --config-file=my.cnf). For available templates and configs, refer to: 'kbcli cluster describe-config'.
      --config-spec string             Specify the name of the configuration template to be updated (e.g. for apecloud-mysql: --config-spec=mysql-3node-tpl). For available templates and configs, refer to: 'kbcli cluster describe-config'.
      --dry-run string[="unchanged"]   Must be "client", or "server". If with client strategy, only print the object that would be sent, and no data is actually sent. If with server strategy, submit the server-side request, but no data is persistent. (default "none")
  -h, --help                           help for configure
      --local-file string              Specify the local configuration file to be updated.
      --name string                    OpsRequest name. if not specified, it will be randomly generated 
  -o, --output format                  prints the output in the specified format. Allowed values: JSON and YAML (default yaml)
      --set strings                    Specify parameters list to be updated. For more details, refer to 'kbcli cluster describe-config'.
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

