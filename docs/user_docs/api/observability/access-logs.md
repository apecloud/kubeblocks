---
title: Access logs
description: The API of accessing logs
sidebar_position: 1
---

# Access logs

## API definition

Add the log-related specification to the API file to enable this function for a cluster.

### Cluster (for users)

The `enabledLogs` string is added in `spec.components` to mark whether to enable the log-related function of a cluster.

***Example***

Add the `enabledLogs` key and fill its value with a log type defined by the provider to enable the log function.

```
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: mysql-cluster-01
  namespace: default
spec:
  clusterDefinitionRef: mysql-cluster-definition
  clusterVersionRef: clusterversion-mysql-latest
  components:
  - name: replicasets
    type: replicasets
    enabledLogs:
      - slow 
    volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
    - name: log
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
```

### ClusterDefinition (for providers)

The `logsConfigs` string is used to search log files. Fill the `name` with the custom log type and `filePathPattern` with the path of the log file. `name` can be defined by providers and is the only identifier. 
Fill the value of `configTemplateRefs` with the kernel parameters.

***Example***

Here is an example of configuring the error log and slow log.

```
apiVersion: apps.kubeblocks.io/v1alpha1
kind: ClusterDefinition
metadata:
  name: mysql-cluster-definition
spec:
  componentDefs:
  - name: replicasets
    characterType: mysql
    monitor:
      builtIn: true
    logConfigs: 
      - name: error
        filePathPattern: /log/mysql/log/mysqld.err
      - name: slow
        filePathPattern: /log/mysql/mysqld-slow.log 
    configTemplateRefs: 
      - name: mysql-tree-node-template-8.0
        volumeName: mysql-config
    workloadType: Consensus
    consensusSpec:
      leader:
        name: leader
        accessMode: ReadWrite
      followers:
        - name: follower
          accessMode: Readonly
    podSpec:
      containers:
      - name: mysql
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 3306
          protocol: TCP
          name: mysql
        - containerPort: 13306
          protocol: TCP
          name: paxos
        volumeMounts:
          - mountPath: /data
            name: data
          - mountPath: /log
            name: log
          - mountPath: /data/config/mysql
            name: mysql-config
        env:
          - name: "MYSQL_ROOT_PASSWORD"
            valueFrom:
              secretKeyRef:
                name: $(KB_SECRET_NAME)
                key: password
        command: ["/usr/bin/bash", "-c"]
        args:
          - >
            cluster_info="";
            for (( i=0; i<$KB_REPLICASETS_N; i++ )); do
              if [ $i -ne 0 ]; then
                cluster_info="$cluster_info;";
              fi;
              host=$(eval echo \$KB_REPLICASETS_"$i"_HOSTNAME)
              cluster_info="$cluster_info$host:13306";
            done;
            idx=0;
            while IFS='-' read -ra ADDR; do
              for i in "${ADDR[@]}"; do
                idx=$i;
              done;
            done <<< "$KB_POD_NAME";
            echo $idx;
            cluster_info="$cluster_info@$(($idx+1))";
            echo $cluster_info;
            mkdir -p /data/mysql/log;
            mkdir -p /data/mysql/data;
            mkdir -p /data/mysql/std_data;
            mkdir -p /data/mysql/tmp;
            mkdir -p /data/mysql/run;
            chmod +777 -R /data/mysql;
            docker-entrypoint.sh mysqld --defaults-file=/data/config/mysql/my.cnf --cluster-start-index=1 --cluster-info="$cluster_info" --cluster-id=1
```

#### How to configure `name` and `filePath` under different conditions

- Multiple files under one path
  
  Here is an example of how to write three files at the same time in the internal PostgreSQL audit file of Alibaba Cloud. 

  ```
  logsConfig: 
    
    # `name` is customized by the provider and is the only identifier.
    - name: audit
      # The path information of the log file.
      filePath: /postgresql/log/postgresql_[0-2]_audit.log
  ```

- Multiple paths (including a path under which there are single or multiple files) 
  
  For the log which is sent to multiple paths and is separated into multiple types, the configurations are as follows:

  ```
  logsConfig: 
    # The following is the audit log of configuring multiple paths.
    # `name` is customized by the provider and is the only identifier.
    - name: audit1
      # The path information of the log file.
      filePath: /var/log1/postgresql_*_audit.log
    - name: audit2
      # The path information of the log file.
      filePath: /var/log2/postgresql_*_audit.log
  ```

### ConfigTemplate (for providers)

When opening a certain log of a certain engine, write the related kernel configuration in `ConfigTemplate` to make sure the log file can be output correctly.

***Example***

Here is an example.

```
apiVersion: v1
kind: ConfigMap
metadata:
  name: mysql-tree-node-template-8.0
data:
  my.cnf: |-
    [mysqld]
    loose_query_cache_type          = OFF
    loose_query_cache_size          = 0
    loose_innodb_thread_concurrency = 0
    loose_concurrent_insert         = 0
    loose_gts_lease                 = 2000
    loose_log_bin_use_v1_row_events = off
    loose_binlog_checksum           = crc32
    
    {{- if mustHas "error" $.Component.EnabledLogs }}
    # Mysql error log
    log_error={{ $log_root }}/mysqld.err
    {{- end }}

    {{- if mustHas "slow" $.Component.EnabledLogs }}
    # MySQL Slow log
    slow_query_log=ON
    long_query_time=5
    log_output=FILE
    slow_query_log_file={{ $log_root }}/mysqld-slow.log
    {{- end }}
...
```

### Status

The log-related function, similar to a warning, neither affects the main flow of control and management nor changes `Phase` or `Generation`. It adds a `conditions` field in `cluster API status` to store the warning of a cluster.

```
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  ...
spec: 
  ...
status:
  # metav1.Condition[] 
  conditions:
  - key: spec.components[replicasets].logs
    reason: 'xxx' is invalid 

  components:
    # component name 
    replicasets:
      phase: Failed
      message: Volume snapshot not support
```

Run `kbcli describe cluster <cluster-name>` and its output information is as follows: 

```
Status:
  Cluster Def Generation:  3
  Components:
    Replicasets:
      Phase:  Running
  Conditions:
    Last Transition Time:  2022-11-11T03:57:42Z
    Message:               EnabledLogs of cluster component replicasets has invalid value [errora slowa] which isn't defined in cluster definition component replicasets
    Reason:                EnabledLogsListValidateFail
    Status:                False
    Type:                  ValidateEnabledLogs
  Observed Generation:     2
  Operations:
    Horizontal Scalable:
      Name:  replicasets
    Restartable:
      replicasets
    Vertical Scalable:
      replicasets
  Phase:  Running
Events:
  Type     Reason                      Age   From                Message
  ----     ------                      ----  ----                -------
  Normal   Creating                    49s   cluster-controller  Start Creating in Cluster: release-name-error
  Warning  EnabledLogsListValidateFail  49s   cluster-controller  EnabledLogs of cluster component replicasets has invalid value [errora slowa] which isn't defined in cluster definition component replicasets
  Normal   Running                     36s   cluster-controller  Cluster: release-name-error is ready, current phase is Running
```