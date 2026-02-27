---
title: kbcli cluster custom-ops
---



```
kbcli cluster custom-ops OpsDef --cluster <clusterName> <your custom params> [flags]
```

### Examples

```
  # custom ops cli format
  kbcli cluster custom-ops <opsDefName> --cluster <clusterName> <your params of this opsDef>
  
  # example for kafka topic
  kbcli cluster custom-ops kafka-topic --cluster mycluster --type create --topic test --partition 3 --replicas 3
  
  # example for kafka acl
  kbcli cluster custom-ops kafka-user-acl --cluster mycluster --type add --operations "Read,Writer,Delete,Alter,Describe" --allowUsers client --topic "*"
  
  # example for kafka quota
  kbcli cluster custom-ops kafka-quota --cluster mycluster --user client --producerByteRate 1024 --consumerByteRate 2048
```

### Options

```
  -h, --help   help for custom-ops
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
* [kbcli cluster custom-ops add-arch-for-dm](kbcli_cluster_custom-ops_add-arch-for-dm.md)	 - Create a custom ops with opsDef add-arch-for-dm
* [kbcli cluster custom-ops hdfs-balancer](kbcli_cluster_custom-ops_hdfs-balancer.md)	 - Create a custom ops with opsDef hdfs-balancer
* [kbcli cluster custom-ops hive-server2-apply-account](kbcli_cluster_custom-ops_hive-server2-apply-account.md)	 - Create a custom ops with opsDef hive-server2-apply-account
* [kbcli cluster custom-ops kafka-quota](kbcli_cluster_custom-ops_kafka-quota.md)	 - Create a custom ops with opsDef kafka-quota
* [kbcli cluster custom-ops kafka-topic](kbcli_cluster_custom-ops_kafka-topic.md)	 - Create a custom ops with opsDef kafka-topic
* [kbcli cluster custom-ops kafka-user-acl](kbcli_cluster_custom-ops_kafka-user-acl.md)	 - Create a custom ops with opsDef kafka-user-acl
* [kbcli cluster custom-ops mongodb-shard-toggle-balancer](kbcli_cluster_custom-ops_mongodb-shard-toggle-balancer.md)	 - Create a custom ops with opsDef mongodb-shard-toggle-balancer
* [kbcli cluster custom-ops mssql-dynamic-modify-member](kbcli_cluster_custom-ops_mssql-dynamic-modify-member.md)	 - Create a custom ops with opsDef mssql-dynamic-modify-member
* [kbcli cluster custom-ops mssql-dynamic-modify-member-1.0.0](kbcli_cluster_custom-ops_mssql-dynamic-modify-member-1.0.0.md)	 - Create a custom ops with opsDef mssql-dynamic-modify-member-1.0.0
* [kbcli cluster custom-ops mssql-dynamic-remove-ag](kbcli_cluster_custom-ops_mssql-dynamic-remove-ag.md)	 - Create a custom ops with opsDef mssql-dynamic-remove-ag
* [kbcli cluster custom-ops mssql-dynamic-remove-ag-1.0.0](kbcli_cluster_custom-ops_mssql-dynamic-remove-ag-1.0.0.md)	 - Create a custom ops with opsDef mssql-dynamic-remove-ag-1.0.0
* [kbcli cluster custom-ops mssql-dynamic-remove-member](kbcli_cluster_custom-ops_mssql-dynamic-remove-member.md)	 - Create a custom ops with opsDef mssql-dynamic-remove-member
* [kbcli cluster custom-ops mssql-dynamic-remove-member-1.0.0](kbcli_cluster_custom-ops_mssql-dynamic-remove-member-1.0.0.md)	 - Create a custom ops with opsDef mssql-dynamic-remove-member-1.0.0
* [kbcli cluster custom-ops nebula-balance-data](kbcli_cluster_custom-ops_nebula-balance-data.md)	 - Create a custom ops with opsDef nebula-balance-data
* [kbcli cluster custom-ops ob-alter-unit](kbcli_cluster_custom-ops_ob-alter-unit.md)	 - Create a custom ops with opsDef ob-alter-unit
* [kbcli cluster custom-ops ob-switch-configserver](kbcli_cluster_custom-ops_ob-switch-configserver.md)	 - Create a custom ops with opsDef ob-switch-configserver
* [kbcli cluster custom-ops pg-update-standby-config](kbcli_cluster_custom-ops_pg-update-standby-config.md)	 - Create a custom ops with opsDef pg-update-standby-config
* [kbcli cluster custom-ops post-rebuild-for-clickhouse](kbcli_cluster_custom-ops_post-rebuild-for-clickhouse.md)	 - Create a custom ops with opsDef post-rebuild-for-clickhouse
* [kbcli cluster custom-ops post-scale-for-dmdb](kbcli_cluster_custom-ops_post-scale-for-dmdb.md)	 - Create a custom ops with opsDef post-scale-for-dmdb
* [kbcli cluster custom-ops post-scale-out-shard-for-clickhouse](kbcli_cluster_custom-ops_post-scale-out-shard-for-clickhouse.md)	 - Create a custom ops with opsDef post-scale-out-shard-for-clickhouse
* [kbcli cluster custom-ops redis-cluster-rebalance](kbcli_cluster_custom-ops_redis-cluster-rebalance.md)	 - Create a custom ops with opsDef redis-cluster-rebalance
* [kbcli cluster custom-ops redis-master-account-ops](kbcli_cluster_custom-ops_redis-master-account-ops.md)	 - Create a custom ops with opsDef redis-master-account-ops
* [kbcli cluster custom-ops redis-reset-master](kbcli_cluster_custom-ops_redis-reset-master.md)	 - Create a custom ops with opsDef redis-reset-master
* [kbcli cluster custom-ops redis-sentinel-account-ops](kbcli_cluster_custom-ops_redis-sentinel-account-ops.md)	 - Create a custom ops with opsDef redis-sentinel-account-ops
* [kbcli cluster custom-ops redis-shard-account-ops](kbcli_cluster_custom-ops_redis-shard-account-ops.md)	 - Create a custom ops with opsDef redis-shard-account-ops
* [kbcli cluster custom-ops remove-remote-arch](kbcli_cluster_custom-ops_remove-remote-arch.md)	 - Create a custom ops with opsDef remove-remote-arch
* [kbcli cluster custom-ops switchover-for-dm](kbcli_cluster_custom-ops_switchover-for-dm.md)	 - Create a custom ops with opsDef switchover-for-dm
* [kbcli cluster custom-ops update-license-for-dm](kbcli_cluster_custom-ops_update-license-for-dm.md)	 - Create a custom ops with opsDef update-license-for-dm
* [kbcli cluster custom-ops update-license-for-kingbase](kbcli_cluster_custom-ops_update-license-for-kingbase.md)	 - Create a custom ops with opsDef update-license-for-kingbase

#### Go Back to [CLI Overview](cli.md) Homepage.

