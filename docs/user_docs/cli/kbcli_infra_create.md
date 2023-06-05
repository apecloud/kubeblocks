---
title: kbcli infra create
---

create kubernetes cluster.

```
kbcli infra create [flags]
```

### Examples

```


```

### Options

```
      --container-runtime string   Specify kubernetes container runtime. default is containerd (default "containerd")
      --debug                      set debug mode
      --etcd strings               Specify etcd nodes
  -h, --help                       help for create
      --master strings             Specify master nodes
      --name string                Specify kubernetes cluster name
      --nodes strings              List of machines on which kubernetes is installed. [require]
  -p, --password string            Specify the password for the account to execute sudo. [option]
      --private-key string         The PrimaryKey for ssh to the remote machine. [option]
      --private-key-path string    Specify the file PrimaryKeyPath of ssh to the remote machine. default ~/.ssh/id_rsa.
  -t, --timeout int                Specify the ssh timeout.[option] (default 30)
  -u, --user string                Specify the account to access the remote server. [require]
      --version string             Specify install kubernetes version. default version is v1.26.5 (default "v1.26.5")
      --worker strings             Specify worker nodes
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
```

### SEE ALSO

* [kbcli infra](kbcli_infra.md)	 - infra command

#### Go Back to [CLI Overview](cli.md) Homepage.

