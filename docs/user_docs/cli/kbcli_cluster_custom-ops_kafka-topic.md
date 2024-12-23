---
title: kbcli cluster custom-ops kafka-topic
---

Create a custom ops with opsDef kafka-topic

```
kbcli cluster custom-ops kafka-topic <ClusterName> [flags]
```

### Examples

```
  # Create a kafka-topic ops
  kbcli cluster custom-ops kafka-topic <clusterName> --component <componentName> --topic=<topic> --type=<type>
```

### Options

```
      --auto-approve                   Skip interactive approval before promote the instance
      --component string               Specify the component name of the cluster. if not specified, using the first component which referenced the defined componentDefinition.
      --config string                  A topic configuration override for the topic being created or altered.
      --dry-run string[="unchanged"]   Must be "client", or "server". If with client strategy, only print the object that would be sent, and no data is actually sent. If with server strategy, submit the server-side request, but no data is persistent. (default "none")
      --edit                           Edit the API resource before creating
      --force                           skip the pre-checks of the opsRequest to run the opsRequest forcibly
  -h, --help                           help for kafka-topic
      --name string                    OpsRequest name. if not specified, it will be randomly generated
  -o, --output format                  Prints the output in the specified format. Allowed values: JSON and YAML (default yaml)
      --partitions int                 The number of partitions for the topic being created or altered (WARNING:
                                       If partitions are increased for a topic that has a key, the partition logic or ordering
                                       of the messages will be affected). If not supplied for create, defaults to the cluster default.
                                        Value range [1, 10000].
      --replicas int                   The replication factor for each partition in the topic being
                                       created. If not supplied, defaults to the cluster default.
                                        Value range [1, 10].
      --topic string                   The topic to create, alter or delete. It also accepts a regular
                                       expression, except for --create option. Put topic name in double quotes and
                                       use the '\' prefix to escape regular expression symbols; e.g. "test\.topic".
                                       
      --ttlSecondsAfterSucceed int     Time to live after the OpsRequest succeed
      --type string                    operation type, supports value: [create, alter, delete]. Legal values [create, alter, delete].
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

* [kbcli cluster custom-ops](kbcli_cluster_custom-ops.md)	 - 

#### Go Back to [CLI Overview](cli.md) Homepage.

