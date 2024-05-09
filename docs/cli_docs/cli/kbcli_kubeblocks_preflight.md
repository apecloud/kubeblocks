---
title: kbcli kubeblocks preflight
---

Run and retrieve preflight checks for KubeBlocks.

```
kbcli kubeblocks preflight [flags]
```

### Examples

```
  # Run preflight provider checks against the default rules automatically
  kbcli kubeblocks preflight
  
  # Run preflight provider checks and output more verbose info
  kbcli kubeblocks preflight --verbose
  
  # Run preflight checks against the customized rules of preflight-check.yaml
  kbcli kubeblocks preflight preflight-check.yaml
  
  # Run preflight checks and display AnalyzeResults with interactive mode
  kbcli kubeblocks preflight preflight-check.yaml --interactive=true
```

### Options

```
      --collect-without-permissions   always run preflight checks even if some required permissions that preflight does not have (default true)
      --collector-image string        the full name of the collector image to use
      --collector-pullpolicy string   the pull policy of the collector image
      --debug                         enable debug logging
      --format string                 output format, one of json, yaml. only used when interactive is set to false, default format is yaml (default "yaml")
  -h, --help                          help for preflight
  -n, --namespace string              If present, the namespace scope for this CLI request
  -o, --output string                 specify the output file path for the preflight checks
      --selector string               selector (label query) to filter remote collection nodes on.
      --since string                  force pod logs collectors to return logs newer than a relative duration like 5s, 2m, or 3h.
      --since-time string             force pod logs collectors to return logs after a specific date (RFC3339)
      --verbose                       print more verbose logs, default value is false
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
      --request-timeout string         The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
  -s, --server string                  The address and port of the Kubernetes API server
      --tls-server-name string         Server name to use for server certificate validation. If it is not provided, the hostname used to contact the server is used
      --token string                   Bearer token for authentication to the API server
      --user string                    The name of the kubeconfig user to use
```

### SEE ALSO

* [kbcli kubeblocks](kbcli_kubeblocks.md)	 - KubeBlocks operation commands.

#### Go Back to [CLI Overview](cli.md) Homepage.

