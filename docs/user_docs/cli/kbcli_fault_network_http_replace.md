---
title: kbcli fault network http replace
---

Replace the HTTP request and response.

```
kbcli fault network http replace [flags]
```

### Examples

```
  # By default, the method of GET from port 80 is blocked.
  kbcli fault network http abort --duration=1m
  
  # Block the method of GET from port 4399.
  kbcli fault network http abort --port=4399 --duration=1m
  
  # Block the method of POST from port 4399.
  kbcli fault network http abort --port=4399 --method=POST --duration=1m
  
  # Delays post requests from port 4399.
  kbcli fault network http delay --port=4399 --method=POST --delay=15s
  
  # Replace the GET method sent from port 80 with the PUT method.
  kbcli fault network http replace --replace-method=PUT --duration=1m
  
  # Replace the GET method sent from port 80 with the PUT method, and replace the request body.
  kbcli fault network http replace --body="you are good luck" --replace-method=PUT --duration=2m
  
  # Replace the response content "you" from port 80.
  kbcli fault network http replace --target=Response --body=you --duration=30s
  
  # Append content to the body of the post request sent from port 4399, in JSON format.
  kbcli fault network http patch --method=POST --port=4399 --body="you are good luck" --type=JSON --duration=30s
```

### Options

```
      --annotation stringToString      Select the pod to inject the fault according to Annotation. (default [])
      --body string                    The content of the request body or response body to replace the failure.
      --code int32                     The status code responded by target.
      --dry-run string[="unchanged"]   Must be "client", or "server". If with client strategy, only print the object that would be sent, and no data is actually sent. If with server strategy, submit the server-side request, but no data is persistent. (default "none")
      --duration string                Supported formats of the duration are: ms / s / m / h. (default "10s")
  -h, --help                           help for replace
      --label stringToString           label for pod, such as '"app.kubernetes.io/component=mysql, statefulset.kubernetes.io/pod-name=mycluster-mysql-0. (default [])
      --method string                  The HTTP method of the target request method. For example: GET, POST, PUT, DELETE, HEAD, OPTIONS, PATCH. (default "GET")
      --mode string                    You can select "one", "all", "fixed", "fixed-percent", "random-max-percent", Specify the experimental mode, that is, which Pods to experiment with. (default "all")
      --node stringArray               Inject faults into pods in the specified node.
      --node-label stringToString      label for node, such as '"kubernetes.io/arch=arm64,kubernetes.io/hostname=minikube-m03,kubernetes.io/os=linux. (default [])
      --ns-fault stringArray           Specifies the namespace into which you want to inject faults. (default [default])
  -o, --output format                  Prints the output in the specified format. Allowed values: JSON and YAML (default yaml)
      --path string                    The URI path of the target request. Supports Matching wildcards. (default "*")
      --phase stringArray              Specify the pod that injects the fault by the state of the pod.
      --port int32                     The TCP port that the target service listens on. (default 80)
      --replace-method string          The replaced content of the HTTP request method.
      --replace-path string            The URI path used to replace content.
      --target string                  Specifies whether the target of fault injection is Request or Response. The target-related fields should be configured at the same time. (default "Request")
      --value string                   If you choose mode=fixed or fixed-percent or random-max-percent, you can enter a value to specify the number or percentage of pods you want to inject.
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

* [kbcli fault network http](kbcli_fault_network_http.md)	 - Intercept HTTP requests and responses.

#### Go Back to [CLI Overview](cli.md) Homepage.

