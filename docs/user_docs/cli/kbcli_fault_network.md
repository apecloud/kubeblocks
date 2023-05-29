---
title: kbcli fault network
---

Network chaos.

### Options

```
  -h, --help   help for network
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

* [kbcli fault](kbcli_fault.md)	 - Inject faults to pod.
* [kbcli fault network bandwidth](kbcli_fault_network_bandwidth.md)	 - Limit the bandwidth that pods use to communicate with other objects.
* [kbcli fault network corrupt](kbcli_fault_network_corrupt.md)	 - Distorts the messages a pod communicates with other objects.
* [kbcli fault network delay](kbcli_fault_network_delay.md)	 - Make pods communicate with other objects lazily.
* [kbcli fault network dns](kbcli_fault_network_dns.md)	 - Inject faults into DNS server.
* [kbcli fault network duplicate](kbcli_fault_network_duplicate.md)	 - Make pods communicate with other objects to pick up duplicate packets.
* [kbcli fault network http](kbcli_fault_network_http.md)	 - Intercept HTTP requests and responses.
* [kbcli fault network loss](kbcli_fault_network_loss.md)	 - Cause pods to communicate with other objects to drop packets.
* [kbcli fault network partition](kbcli_fault_network_partition.md)	 - Make a pod network partitioned from other objects.

#### Go Back to [CLI Overview](cli.md) Homepage.

