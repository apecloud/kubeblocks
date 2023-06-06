---
title: kbcli playground init
---

Bootstrap a kubernetes cluster and install KubeBlocks for playground.

### Synopsis

Bootstrap a kubernetes cluster and install KubeBlocks for playground.

 If no cloud provider is specified, a k3d cluster named kb-playground will be created on local host, otherwise a kubernetes cluster will be created on the specified cloud. Then KubeBlocks will be installed on the created kubernetes cluster, and an apecloud-mysql cluster named mycluster will be created.

```
kbcli playground init [flags]
```

### Examples

```
  # create a k3d cluster on local host and install KubeBlocks
  kbcli playground init
  
  # create an AWS EKS cluster and install KubeBlocks, the region is required
  kbcli playground init --cloud-provider aws --region us-west-1
  
  # create an Alibaba cloud ACK cluster and install KubeBlocks, the region is required
  kbcli playground init --cloud-provider alicloud --region cn-hangzhou
  
  # create a Tencent cloud TKE cluster and install KubeBlocks, the region is required
  kbcli playground init --cloud-provider tencentcloud --region ap-chengdu
  
  # create a Google cloud GKE cluster and install KubeBlocks, the region is required
  kbcli playground init --cloud-provider gcp --region us-east1
  
  # after init, run the following commands to experience KubeBlocks quickly
  # list database cluster and check its status
  kbcli cluster list
  
  # get cluster information
  kbcli cluster describe mycluster
  
  # connect to database
  kbcli cluster connect mycluster
  
  # view the Grafana
  kbcli dashboard open kubeblocks-grafana
  
  # destroy playground
  kbcli playground destroy
```

### Options

```
      --auto-approve                Skip interactive approval during the initialization of playground
      --cloud-provider string       Cloud provider type, one of [local aws gcp alicloud tencentcloud] (default "local")
      --cluster-definition string   Specify the cluster definition, run "kbcli cd list" to get the available cluster definitions (default "apecloud-mysql")
      --cluster-version string      Specify the cluster version, run "kbcli cv list" to get the available cluster versions
  -h, --help                        help for init
      --region string               The region to create kubernetes cluster
      --timeout duration            Time to wait for init playground, such as --timeout=10m (default 5m0s)
      --version string              KubeBlocks version
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

* [kbcli playground](kbcli_playground.md)	 - Bootstrap or destroy a playground KubeBlocks in local host or cloud.

#### Go Back to [CLI Overview](cli.md) Homepage.

