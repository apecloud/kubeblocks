---
title: kbcli kubeblocks install
---

Install KubeBlocks.

```
kbcli kubeblocks install [flags]
```

### Examples

```
  # Install KubeBlocks, the default version is same with the kbcli version, the default namespace is kb-system
  kbcli kubeblocks install
  
  # Install KubeBlocks with specified version
  kbcli kubeblocks install --version=0.4.0
  
  # Install KubeBlocks with ignoring preflight checks
  kbcli kubeblocks install --force
  
  # Install KubeBlocks with specified namespace, if the namespace is not present, it will be created
  kbcli kubeblocks install --namespace=my-namespace --create-namespace
  
  # Install KubeBlocks with other settings, for example, set replicaCount to 3
  kbcli kubeblocks install --set replicaCount=3
```

### Options

```
      --check                        Check kubernetes environment before installation (default true)
      --create-namespace             Create the namespace if not present
      --force                        If present, just print fail item and continue with the following steps
  -h, --help                         help for install
      --node-labels stringToString   Node label selector (default [])
      --pod-anti-affinity string     Pod anti-affinity type, one of: (Preferred, Required)
      --set stringArray              Set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
      --set-file stringArray         Set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)
      --set-json stringArray         Set JSON values on the command line (can specify multiple or separate values with commas: key1=jsonval1,key2=jsonval2)
      --set-string stringArray       Set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
      --timeout duration             Time to wait for installing KubeBlocks, such as --timeout=10m (default 5m0s)
      --tolerations strings          Tolerations for Kubeblocks, such as '"dev=true:NoSchedule,large=true:NoSchedule"'
      --topology-keys stringArray    Topology keys for affinity
  -f, --values strings               Specify values in a YAML file or a URL (can specify multiple)
      --version string               KubeBlocks version
      --wait                         Wait for KubeBlocks to be ready, including all the auto installed add-ons. It will wait for a --timeout period (default true)
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

* [kbcli kubeblocks](kbcli_kubeblocks.md)	 - KubeBlocks operation commands.

#### Go Back to [CLI Overview](cli.md) Homepage.

