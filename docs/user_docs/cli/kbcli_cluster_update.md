---
title: kbcli cluster update
---

Update the cluster settings, such as enable or disable monitor or log.

```
kbcli cluster update NAME [flags]
```

### Examples

```
  # update cluster mycluster termination policy to Delete
  kbcli cluster update mycluster --termination-policy=Delete
  
  # enable cluster monitor
  kbcli cluster update mycluster --monitor=true
  
  # enable all logs
  kbcli cluster update mycluster --enable-all-logs=true
  
  # update cluster topology keys and affinity
  kbcli cluster update mycluster --topology-keys=kubernetes.io/hostname --pod-anti-affinity=Required
  
  # update cluster tolerations
  kbcli cluster update mycluster --tolerations='"key=engineType,value=mongo,operator=Equal,effect=NoSchedule","key=diskType,value=ssd,operator=Equal,effect=NoSchedule"'
  
  # edit cluster
  kbcli cluster update mycluster --edit
  
  # enable cluster monitor and edit
  # kbcli cluster update mycluster --monitor=true --edit
  
  # enable cluster auto backup
  kbcli cluster update mycluster --backup-enabled=true
  
  # update cluster backup retention period
  kbcli cluster update mycluster --backup-retention-period=1d
  
  # update cluster backup method
  kbcli cluster update mycluster --backup-method=snapshot
  
  # update cluster backup cron expression
  kbcli cluster update mycluster --backup-cron-expression="0 0 * * *"
  
  # update cluster backup starting deadline minutes
  kbcli cluster update mycluster --backup-starting-deadline-minutes=10
  
  # update cluster backup repo name
  kbcli cluster update mycluster --backup-repo-name=repo1
  
  # update cluster backup pitr enabled
  kbcli cluster update mycluster --pitr-enabled=true
```

### Options

```
      --allow-missing-template-keys            If true, ignore any errors in templates when a field or map key is missing in the template. Only applies to golang and jsonpath output formats. (default true)
      --backup-cron-expression string          the cron expression for schedule, the timezone is in UTC. see https://en.wikipedia.org/wiki/Cron.
      --backup-enabled                         Specify whether enabled automated backup
      --backup-method string                   the backup method, view it by "kbcli cd describe <cluster-definition>", if not specified, the default backup method will be to take snapshots of the volume
      --backup-repo-name string                the backup repository name
      --backup-retention-period string         a time string ending with the 'd'|'D'|'h'|'H' character to describe how long the Backup should be retained (default "1d")
      --backup-starting-deadline-minutes int   the deadline in minutes for starting the backup job if it misses its scheduled time for any reason
      --dry-run string[="unchanged"]           Must be "none", "server", or "client". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource. (default "none")
      --edit                                   Edit the API resource
      --enable-all-logs                        Enable advanced application all log extraction, set to true will ignore enabledLogs of component level, default is false
  -h, --help                                   help for update
      --monitoring-interval uint8              The monitoring interval of cluster, 0 is disabled, the unit is second, any non-zero value means enabling monitoring.
      --node-labels stringToString             Node label selector (default [])
  -o, --output string                          Output format. One of: (json, yaml, name, go-template, go-template-file, template, templatefile, jsonpath, jsonpath-as-json, jsonpath-file).
      --pitr-enabled                           Specify whether enabled point in time recovery
      --pod-anti-affinity string               Pod anti-affinity type, one of: (Preferred, Required) (default "Preferred")
      --show-managed-fields                    If true, keep the managedFields when printing objects in JSON or YAML format.
      --template string                        Template string or path to template file to use when -o=go-template, -o=go-template-file. The template format is golang templates [http://golang.org/pkg/text/template/#pkg-overview].
      --tenancy string                         Tenancy options, one of: (SharedNode, DedicatedNode) (default "SharedNode")
      --termination-policy string              Termination policy, one of: (DoNotTerminate, Halt, Delete, WipeOut) (default "Delete")
      --tolerations strings                    Tolerations for cluster, such as "key=value:effect, key:effect", for example '"engineType=mongo:NoSchedule", "diskType:NoSchedule"'
      --topology-keys stringArray              Topology keys for affinity
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

* [kbcli cluster](kbcli_cluster.md)	 - Cluster command.

#### Go Back to [CLI Overview](cli.md) Homepage.

