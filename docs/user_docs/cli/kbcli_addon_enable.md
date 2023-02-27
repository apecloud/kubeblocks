## kbcli addon enable

Enable an addon

```
kbcli addon enable ADDON_NAME [flags]
```

### Examples

```
  # Enabled "prometheus" addon
  kbcli addon enable prometheus
  
  # Enabled "prometheus" addon with custom resources settings
  kbcli addon enable prometheus --memory 512Mi:4Gi --storage 8Gi --replicas 2
  
  # Enabled "prometheus" addon and its extra alertmanager component with custom resources settings
  kbcli addon enable prometheus --memory 512Mi:4Gi --storage 8Gi --replicas 2 \
  --memory alertmanager:16Mi:256Mi --storage: alertmanager:1Gi --replicas alertmanager:2
  
  # Enabled "prometheus" addon with tolerations
  kbcli addon enable prometheus --tolerations '[[{"key":"taintkey","operator":"Equal","effect":"NoSchedule","value":"true"}]]' \
  --tolerations 'alertmanager:[[{"key":"taintkey","operator":"Equal","effect":"NoSchedule","value":"true"}]]'
```

### Options

```
      ----cpu stringArray             Sets addon CPU resource values (--cpu [extraName:]<request>:<limit>) (can specify multiple if has extra items))
      ----memory stringArray          Sets addon memory resource values (--memory [extraName:]<request>:<limit>) (can specify multiple if has extra items))
      ----replicas stringArray        Sets addon component replica count (--replicas [extraName:]<N>) (can specify multiple if has extra items))
      ----storage stringArray         Sets addon storage size (--storage [extraName:]<request>) (can specify multiple if has extra items))
      ----storage-class stringArray   Sets addon storage class name (--storage-class [extraName:]<SC name>) (can specify multiple if has extra items))
      ----tolerations stringArray     Sets addon pod tolerations (--tolerations [extraName:]<toleration JSON list items>) (can specify multiple if has extra items))
  -h, --help                          help for enable
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

* [kbcli addon](kbcli_addon.md)	 - Addon command

