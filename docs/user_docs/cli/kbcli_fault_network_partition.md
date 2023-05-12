---
title: kbcli fault network partition
---

Make a pod network partitioned from other objects.

```
kbcli fault network partition [flags]
```

### Examples

```
  # Isolate all pods network under the default namespace from the outside world, including the k8s internal network.
  kbcli fault network partition
  
  # The specified pod is isolated from the k8s external network "kubeblocks.io".
  kbcli fault network partition --label=statefulset.kubernetes.io/pod-name=mycluster-mysql-1 --external-targets=kubeblocks.io
  
  # Isolate the network between two pods.
  kbcli fault network partition --label=statefulset.kubernetes.io/pod-name=mycluster-mysql-1 --target-label=statefulset.kubernetes.io/pod-name=mycluster-mysql-2
  
  // Like the partition command, the target can be specified through --target-label or --external-targets. The pod only has obstacles in communicating with this target. If the target is not specified, all communication will be blocked.
  # Block all pod communication under the default namespace, resulting in a 50% packet loss rate.
  kbcli fault network loss --loss=50
  
  # Block the specified pod communication, so that the packet loss rate is 50%.
  kbcli fault network loss --label=statefulset.kubernetes.io/pod-name=mysql-cluster-mysql-2 --loss=50
  
  kbcli fault network corrupt --corrupt=50
  
  # Blocks specified pod communication with a 50% packet corruption rate.
  kbcli fault network corrupt --label=statefulset.kubernetes.io/pod-name=mysql-cluster-mysql-2 --corrupt=50
  
  kbcli fault network duplicate --duplicate=50
  
  # Block specified pod communication so that the packet repetition rate is 50%.
  kbcli fault network duplicate --label=statefulset.kubernetes.io/pod-name=mysql-cluster-mysql-2 --duplicate=50
  
  kbcli fault network delay --latency=10s
  
  # Block the communication of the specified pod, causing its network delay for 10s.
  kbcli fault network delay --label=statefulset.kubernetes.io/pod-name=mysql-cluster-mysql-2 --latency=10s
```

### Options

```
      --direction string                   You can select "to"" or "from"" or "both"". (default "to")
      --dry-run string[="unchanged"]       Must be "client", or "server". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource. (default "none")
      --duration string                    Supported formats of the duration are: ms / s / m / h. (default "10s")
  -e, --external-targets stringArray       a network target outside of Kubernetes, which can be an IPv4 address or a domain name,
                                           	 such as "www.baidu.com". Only works with direction: to.
  -h, --help                               help for partition
      --label stringToString               label for pod, such as '"app.kubernetes.io/component=mysql, statefulset.kubernetes.io/pod-name=mycluster-mysql-0"' (default [])
      --mode string                        You can select "one", "all", "fixed", "fixed-percent", "random-max-percent", Specify the experimental mode, that is, which Pods to experiment with. (default "all")
      --namespace-selector stringArray     Specifies the namespace into which you want to inject faults. (default [default])
  -o, --output format                      prints the output in the specified format. Allowed values: JSON and YAML (default yaml)
      --target-label stringToString        label for pod, such as '"app.kubernetes.io/component=mysql, statefulset.kubernetes.io/pod-name=mycluster-mysql-0"' (default [])
      --target-mode string                 You can select "one", "all", "fixed", "fixed-percent", "random-max-percent", Specify the experimental mode, that is, which Pods to experiment with. (default "all")
      --target-namespace-selector string   Specifies the namespace into which you want to inject faults. (default "default")
      --target-value string                If you choose mode=fixed or fixed-percent or random-max-percent, you can enter a value to specify the number or percentage of pods you want to inject.
      --value string                       If you choose mode=fixed or fixed-percent or random-max-percent, you can enter a value to specify the number or percentage of pods you want to inject.
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

* [kbcli fault network](kbcli_fault_network.md)	 - Network chaos.

#### Go Back to [CLI Overview](cli.md) Homepage.

