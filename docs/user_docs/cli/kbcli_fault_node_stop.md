---
title: kbcli fault node stop
---

Stop instance

```
kbcli fault node stop [flags]
```

### Examples

```
  # Stop a specified EC2 instance.
  kbcli fault node stop aws --secret-name=cloud-key-secret --region=cn-northwest-1 --instance=i-0a4986881adf30039 --duration=3m
  
  # Stop a specified EC2 instance.
  kbcli fault node stop -c=aws --secret-name=cloud-key-secret --region=cn-northwest-1 --instance=i-0a4986881adf30039 --duration=3m
  
  # Restart a specified EC2 instance.
  kbcli fault node restart aws --secret-name=cloud-key-secret --region=cn-northwest-1 --instance=i-0ff10a1487cf6bbac --duration=1m
  
  # Detach a specified volume from a specified EC2 instance.
  kbcli fault node detach-volume aws --secret-name=cloud-key-secret --region=cn-northwest-1 --instance=i-0df0732607d54dd8e --duration=1m --volume-id=vol-072f0940c28664f74 --device-name=/dev/xvdab
  
  # Stop a specified GCK instance.
  kbcli fault node stop gcp --region=us-central1-c --project=apecloud-platform-engineering --instance=gke-hyqtest-default-pool-2fe51a08-45rl --secret-name=cloud-key-secret
  
  # Stop a specified GCK instance.
  kbcli fault node stop -c=gcp --region=us-central1-c --project=apecloud-platform-engineering --instance=gke-hyqtest-default-pool-2fe51a08-45rl --secret-name=cloud-key-secret
  
  # Restart a specified GCK instance.
  kbcli fault node restart gcp --region=us-central1-c --project=apecloud-platform-engineering --instance=gke-hyqtest-default-pool-2fe51a08-d9 --secret-name=cloud-key-secret
  
  # Detach a specified volume from a specified GCK instance.
  kbcli fault node detach-volume gcp --region=us-central1-c --project=apecloud-platform-engineering --instance=gke-hyqtest-default-pool-2fe51a08-d9 --secret-name=cloud-key-secret --device-name=/dev/sdb
```

### Options

```
  -c, --cloud-provider string          Cloud provider type, one of [aws gcp]
      --dry-run string[="unchanged"]   Must be "client", or "server". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource. (default "none")
      --duration string                Supported formats of the duration are: ms / s / m / h. (default "30s")
  -h, --help                           help for stop
      --instance string                The instance id of the ec2.
  -o, --output format                  prints the output in the specified format. Allowed values: JSON and YAML (default yaml)
      --project string                 The ID of the GCP project.Only available when cloud-provider=gcp.
      --region string                  The region of the aws.
      --secret-name string             The name of the Kubernetes Secret that stores the AWS authentication information.
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

* [kbcli fault node](kbcli_fault_node.md)	 - Node chaos.

#### Go Back to [CLI Overview](cli.md) Homepage.

