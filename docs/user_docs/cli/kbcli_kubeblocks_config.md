---
title: kbcli kubeblocks config
---

KubeBlocks config.

```
kbcli kubeblocks config [flags]
```

### Examples

```
  # Enable the snapshot-controller and volume snapshot, to support snapshot backup.
  kbcli kubeblocks config --set snapshot-controller.enabled=true
  
  Options Parameters:
  # If you have already installed a snapshot-controller, only enable the snapshot backup feature
  dataProtection.enableVolumeSnapshot=true
  
  # the global pvc name which persistent volume claim to store the backup data.
  # replace the pvc name when it is empty in the backup policy.
  dataProtection.backupPVCName=backup-data
  
  # the init capacity of pvc for creating the pvc, e.g. 10Gi.
  # replace the init capacity when it is empty in the backup policy.
  dataProtection.backupPVCInitCapacity=100Gi
  
  # the pvc storage class name. replace the storageClassName when it is unset in the backup policy.
  dataProtection.backupPVCStorageClassName=csi-s3
  
  # the pvc creation policy.
  # if the storageClass supports dynamic provisioning, recommend "IfNotPresent" policy.
  # otherwise, using "Never" policy. only affect the backupPolicy automatically created by KubeBlocks.
  dataProtection.backupPVCCreatePolicy=Never
  
  # the configmap name of the pv template. if the csi-driver does not support dynamic provisioning,
  # you can provide a configmap which contains key "persistentVolume" and value of the persistentVolume struct.
  dataProtection.backupPVConfigMapName=pv-template
  
  # the configmap namespace of the pv template.
  dataProtection.backupPVConfigMapNamespace=default
```

### Options

```
  -h, --help                     help for config
      --set stringArray          Set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
      --set-file stringArray     Set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)
      --set-json stringArray     Set JSON values on the command line (can specify multiple or separate values with commas: key1=jsonval1,key2=jsonval2)
      --set-string stringArray   Set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)
  -f, --values strings           Specify values in a YAML file or a URL (can specify multiple)
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

* [kbcli kubeblocks](kbcli_kubeblocks.md)	 - KubeBlocks operation commands.

#### Go Back to [CLI Overview](cli.md) Homepage.

