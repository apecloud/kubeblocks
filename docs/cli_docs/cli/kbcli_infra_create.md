---
title: kbcli infra create
---

create kubernetes cluster.

```
kbcli infra create [flags]
```

### Examples

```
  # Create kubernetes cluster with specified config yaml
  kbcli infra create -c cluster.yaml
  
  # example cluster.yaml
  cat cluster.yaml
  metadata:
  name: kb-k8s-test-cluster
  user:
  name: user1
  privateKeyPath: ~/.ssh/test.pem
  nodes:
  - name: kb-infra-node-0
  address: 1.1.1.1
  internalAddress: 10.128.0.19
  - name: kb-infra-node-1
  address: 1.1.1.2
  internalAddress: 10.128.0.20
  - name: kb-infra-node-2
  address: 1.1.1.3
  internalAddress: 10.128.0.21
  options:
  hugePageFeature:
  hugePageSize: 10GB
  roleGroup:
  etcd:
  - kb-infra-node-0
  - kb-infra-node-1
  - kb-infra-node-2
  master:
  - kb-infra-node-0
  worker:
  - kb-infra-node-1
  - kb-infra-node-2
  
  kubernetes:
  containerManager: containerd
  # apis/kubeadm/types.Networking
  networking:
  plugin: cilium
  dnsDomain: cluster.local
  podSubnet: 10.233.64.0/18
  serviceSubnet: 10.233.0.0/18
  controlPlaneEndpoint:
  domain: lb.kubeblocks.local
  port: 6443
  cri:
  containerRuntimeType: "containerd"
  containerRuntimeEndpoint: "unix:///run/containerd/containerd.sock"
  sandBoxImage: "k8s.gcr.io/pause:3.8"
  addons:
  - name: openebs
  namespace: kube-blocks
  sources:
  chart:
  name: openebs
  version: 3.7.0
  repo: https://openebs.github.io/charts
  options:
  values:
  - "localprovisioner.basePath=/mnt/disks"
  - "localprovisioner.hostpathClass.isDefaultClass=true"
```

### Options

```
  -c, --config string              Specify infra cluster config file. [option]
      --container-runtime string   Specify kubernetes container runtime. default is containerd (default "containerd")
      --debug                      set debug mode
      --etcd strings               Specify etcd nodes
  -h, --help                       help for create
      --master strings             Specify master nodes
      --name string                Specify kubernetes cluster name
      --nodes strings              List of machines on which kubernetes is installed. [require]
      --output-kubeconfig string   Specified output kubeconfig. [option] (default "$HOME/.kube/config")
  -p, --password string            Specify the password for the account to execute sudo. [option]
      --private-key string         The PrimaryKey for ssh to the remote machine. [option]
      --private-key-path string    Specify the file PrimaryKeyPath of ssh to the remote machine. default ~/.ssh/id_rsa.
      --sandbox-image string       Specified sandbox-image will not be used by the cri. [option] (default "k8s.gcr.io/pause:3.8")
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

