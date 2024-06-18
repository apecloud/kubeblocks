---
title: kbcli builder template
---

tpl - a developer tool integrated with KubeBlocks that can help developers quickly generate rendered configurations or scripts based on Helm templates, and discover errors in the template before creating the database cluster.

```
kbcli builder template [flags]
```

### Examples

```
  # builder template: Provides a mechanism to rendered template for ComponentConfigSpec and ComponentScriptSpec in the ClusterComponentDefinition.
  # builder template --helm deploy/redis --memory=64Gi --cpu=16 --replicas=3 --component-name=redis --config-spec=redis-replication-config
  
  # build all configspec
  kbcli builder template --helm deploy/redis -a
```

### Options

```
      --clean                       specify whether to clear the output dir
      --cluster string              the cluster yaml file
      --cluster-definition string   specify the cluster definition name
      --cluster-version string      specify the cluster version name
      --component-name string       specify the component name of the clusterdefinition
      --config-spec string          specify the config spec to be rendered
      --cpu string                  specify the cpu of the component
      --helm string                 specify the helm template dir
      --helm-output string          specify the helm template output dir
  -h, --help                        help for template
      --memory string               specify the memory of the component
  -o, --output-dir string           specify the output directory
  -r, --replicas int32              specify the replicas of the component (default 1)
      --volume-name string          specify the data volume name of the component
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

* [kbcli builder](kbcli_builder.md)	 - builder command.

#### Go Back to [CLI Overview](cli.md) Homepage.

