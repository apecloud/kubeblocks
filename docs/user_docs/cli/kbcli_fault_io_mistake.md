---
title: kbcli fault io mistake
---

Alters the contents of the file, distorting the contents of the file.

```
kbcli fault io mistake [flags]
```

### Examples

```
  # Affects the first container in default namespace's all pods. Delay all IO operations under the /data path by 10s.
  kbcli fault io latency --delay=10s --volume-path=/data
  
  # Affects the first container in mycluster-mysql-0 pod.
  kbcli fault io latency mycluster-mysql-0 --delay=10s --volume-path=/data
  
  # Affects the mysql container in mycluster-mysql-0 pod.
  kbcli fault io latency mycluster-mysql-0 --delay=10s --volume-path=/data -c=mysql
  
  # There is a 50% probability of affecting the read IO operation of the test.txt file under the /data path.
  kbcli fault io latency mycluster-mysql-0 --delay=10s --volume-path=/data --path=test.txt --percent=50 --method=READ -c=mysql
  
  # Same as above.Make all IO operations under the /data path return the specified error number 22 (Invalid argument).
  kbcli fault io errno --volume-path=/data --errno=22
  
  # Same as above.Modify the IO operation permission attribute of the files under the /data path to 72.(110 in octal).
  kbcli fault io attribute --volume-path=/data --perm=72
  
  # Modify all files so that random positions of 1's with a maximum length of 10 bytes will be replaced with 0's.
  kbcli fault io mistake --volume-path=/data --filling=zero --max-occurrences=10 --max-length=1
```

### Options

```
      --annotation stringToString      Select the pod to inject the fault according to Annotation. (default [])
  -c, --container stringArray          The name of the container, such as mysql, prometheus.If it's empty, the first container will be injected.
      --dry-run string[="unchanged"]   Must be "client", or "server". If with client strategy, only print the object that would be sent, and no data is actually sent. If with server strategy, submit the server-side request, but no data is persistent. (default "none")
      --duration string                Supported formats of the duration are: ms / s / m / h. (default "10s")
      --filling string                 The filling content of the error data can only be zero (filling with 0) or random (filling with random bytes).
  -h, --help                           help for mistake
      --label stringToString           label for pod, such as '"app.kubernetes.io/component=mysql, statefulset.kubernetes.io/pod-name=mycluster-mysql-0. (default [])
      --max-length int                 The maximum length (in bytes) of each error. (default 1)
      --max-occurrences int            The maximum number of times an error can occur per operation. (default 1)
      --method stringArray             The file system calls that need to inject faults. For example: WRITE READ
      --mode string                    You can select "one", "all", "fixed", "fixed-percent", "random-max-percent", Specify the experimental mode, that is, which Pods to experiment with. (default "all")
      --node stringArray               Inject faults into pods in the specified node.
      --node-label stringToString      label for node, such as '"kubernetes.io/arch=arm64,kubernetes.io/hostname=minikube-m03,kubernetes.io/os=linux. (default [])
      --ns-fault stringArray           Specifies the namespace into which you want to inject faults. (default [default])
  -o, --output format                  Prints the output in the specified format. Allowed values: JSON and YAML (default yaml)
      --path string                    The effective scope of the injection error can be a wildcard or a single file.
      --percent int                    Probability of failure per operation, in %. (default 100)
      --phase stringArray              Specify the pod that injects the fault by the state of the pod.
      --value string                   If you choose mode=fixed or fixed-percent or random-max-percent, you can enter a value to specify the number or percentage of pods you want to inject.
      --volume-path string             The mount point of the volume in the target container must be the root directory of the mount.
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

* [kbcli fault io](kbcli_fault_io.md)	 - IO chaos.

#### Go Back to [CLI Overview](cli.md) Homepage.

