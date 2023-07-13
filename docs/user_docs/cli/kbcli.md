---
title: kbcli
---

KubeBlocks CLI.

### Synopsis

```

=============================================
 __    __ _______   ______  __       ______ 
|  \  /  \       \ /      \|  \     |      \
| ▓▓ /  ▓▓ ▓▓▓▓▓▓▓\  ▓▓▓▓▓▓\ ▓▓      \▓▓▓▓▓▓
| ▓▓/  ▓▓| ▓▓__/ ▓▓ ▓▓   \▓▓ ▓▓       | ▓▓  
| ▓▓  ▓▓ | ▓▓    ▓▓ ▓▓     | ▓▓       | ▓▓  
| ▓▓▓▓▓\ | ▓▓▓▓▓▓▓\ ▓▓   __| ▓▓       | ▓▓  
| ▓▓ \▓▓\| ▓▓__/ ▓▓ ▓▓__/  \ ▓▓_____ _| ▓▓_ 
| ▓▓  \▓▓\ ▓▓    ▓▓\▓▓    ▓▓ ▓▓     \   ▓▓ \
 \▓▓   \▓▓\▓▓▓▓▓▓▓  \▓▓▓▓▓▓ \▓▓▓▓▓▓▓▓\▓▓▓▓▓▓

=============================================
A Command Line Interface for KubeBlocks
```

```
kbcli [flags]
```

### Options

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
  -h, --help                           help for kbcli
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

* [kbcli addon](kbcli_addon.md)	 - Addon command.
* [kbcli alert](kbcli_alert.md)	 - Manage alert receiver, include add, list and delete receiver.
* [kbcli backuprepo](kbcli_backuprepo.md)	 - BackupRepo command.
* [kbcli bench](kbcli_bench.md)	 - Run a benchmark.
* [kbcli builder](kbcli_builder.md)	 - builder command.
* [kbcli class](kbcli_class.md)	 - Manage classes
* [kbcli cluster](kbcli_cluster.md)	 - Cluster command.
* [kbcli clusterdefinition](kbcli_clusterdefinition.md)	 - ClusterDefinition command.
* [kbcli clusterversion](kbcli_clusterversion.md)	 - ClusterVersion command.
* [kbcli dashboard](kbcli_dashboard.md)	 - List and open the KubeBlocks dashboards.
* [kbcli fault](kbcli_fault.md)	 - Inject faults to pod.
* [kbcli infra](kbcli_infra.md)	 - infra command
* [kbcli kubeblocks](kbcli_kubeblocks.md)	 - KubeBlocks operation commands.
* [kbcli migration](kbcli_migration.md)	 - Data migration between two data sources.
* [kbcli options](kbcli_options.md)	 - Print the list of flags inherited by all commands.
* [kbcli playground](kbcli_playground.md)	 - Bootstrap or destroy a playground KubeBlocks in local host or cloud.
* [kbcli plugin](kbcli_plugin.md)	 - Provides utilities for interacting with plugins.
* [kbcli report](kbcli_report.md)	 - report kubeblocks or cluster info.
* [kbcli version](kbcli_version.md)	 - Print the version information, include kubernetes, KubeBlocks and kbcli version.

#### Go Back to [CLI Overview](cli.md) Homepage.

