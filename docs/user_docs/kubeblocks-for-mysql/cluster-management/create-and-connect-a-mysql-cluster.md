---
title: Create and connect to a MySQL Cluster
description: How to create and connect to a MySQL cluster
keywords: [mysql, create a mysql cluster, connect to a mysql cluster]
sidebar_position: 1
sidebar_label: Create and connect
---

# Create and connect to a MySQL cluster

This document shows how to create and connect to a MySQL cluster.

## Create a MySQL cluster

### Before you start

* [Install `kbcli`](./../../installation/install-kbcli.md).
* [Install KubeBlocks](./../../installation/introduction.md): Choose one guide that fits your actual environments.
* Make sure the ApeCloud MySQL addon is installed with `kbcli addon list`.
  
  ```bash
  kbcli addon list
  >
  NAME                           TYPE   STATUS     EXTRAS         AUTO-INSTALL   INSTALLABLE-SELECTOR
  ...
  apecloud-mysql                 Helm   Enabled                   true
  ...
  ```

* View all the database types and versions available for creating a cluster.

  ```bash
  kbcli clusterversion list
  ```

### (Recommended) Create a cluster on a tainted node

In actual scenarios, you are recommendend to create a cluster on nodes with taints and customized specification.

1. Taint your node.

   :::note

   If you have already some tainted node, you can skip this step.

   :::

   1. Get Kubernetes nodes.

      ```bash
      kubectl get node
      ```

   2. Place taints on the selected nodes.

      ```bash
      kubectl taint nodes <nodename> <taint1name>=true:NoSchedule
      ```

2. Create a MySQL cluster.

   The cluster creation command is simply `kbcli cluster create`. Use tolerances to deploy it on the tainted node. Further, you are recommended to create a cluster with specified class and customize your cluster settings as demanded.

   To create a cluster with specified class, you can use `--set` flag and specify your requirement.

   1. View and select a class for this cluster.

      ```bash
      kbcli class list --cluster-definition apecloud-mysql  
      ```

   :::note

   If there is no suitable class listed, you can [customize your own class](./../cluster-type/customize-class-type.md) template and apply the class here.

   Creating clusters that does not meet the constraints is invalid and system creates the cluster with the minimum CPU value specified.

   :::

   2. Create a cluster with specified class and add all taints on the current node in the `--toleration` flag to tolerate them.

      ```bash
      kbcli cluster create mysql-cluster --tolerations '"key=taint1name,value=true,operator=Equal,effect=NoSchedule","key=taint2name,value=true,operator=Equal,effect=NoSchedule"' --cluster-definition=apecloud-mysql --set class=general-2c2g,storageClass=<storageclassname> --namespace <name>
      ```

   Or change the corresponding parameters in the YAML file.

   ```bash
   kbcli cluster create mysql-cluster --tolerations '"key=taint1name,value=true,operator=Equal,effect=NoSchedule","key=taint2name,value=true,operator=Equal,effect=NoSchedule"' --cluster-definition=apecloud-mysql --namespace <name> --set-file -<<EOF
   - name: mysql-cluster
     replicas: 3
     componentDefRef: mysql
     volumeClaimTemplates:
     - name: data
       spec:
         storageClassName: <storageclassname>
         accessModes:
         - ReadWriteOnce
         resources:
           requests:
             cpu: 2000m
             memory: 2Gi
             storage: 10Gi
   EOF
   ```

See the table below for the detailed description for customizable parameters, setting the `--termination-policy` is necessary, and you are strongly recommended to turn on the monitor and enable all logs.

📎 Table 1. kbcli cluster create flags description

| Option                 | Description             |
|:-----------------------|:------------------------|
| `--cluster-definition` | It specifies the cluster definition. You can choose a database type. Run `kbcli cd list` to show all available cluster definitions.   |
| `--cluster-version`    | It specifies the cluster version. Run `kbcli cv list` to show all available cluster versions. If you do not specify a cluster version when creating a cluster, the latest version is applied by default.  |
| `--enable-all-logs`    | It enables you to view all application logs. When this function is enabled, enabledLogs of component level will be ignored. For logs settings, refer to [Access Logs](./../../observability/access-logs.md).  |
| `--help`               | It shows the help guide for `kbcli cluster create`. You can also use the abbreviated `-h`. |
| `--monitor`            | It is used to enable the monitor function and inject metrics exporter. It is set as true by default. |
| `--node-labels`        | It is a node label selector. Its default value is [] and means empty value. If you want set node labels, you can follow the example format: <br />`kbcli cluster create mysql-cluster --cluster-definition=apecloud-mysql --node-labels='"topology.kubernetes.io/zone=us-east-1a","disktype=ssd,essd"'`  |
| `--set`                | It sets the cluster resource including CPU, memory, replicas, and storage, each set corresponds to a component. For example, `--set cpu=1000m,memory=1Gi,replicas=1,storage=10Gi`.  |
| `--set-file`           | It uses a yaml file, URL, or stdin to set the cluster resource. |
| `--termination-policy` | It specifies how a cluster is deleted. Set the policy when creating a cluster. There are four available values, namely `DoNotTerminate`, `Halt`, `Delete`, and `WipeOut`. `Delete` is set as the default. <br /> - `DoNotTerminate`: DoNotTerminate blocks the delete operation. <br /> - `Halt`: Halt deletes workload resources such as statefulset, deployment workloads but keeps PVCs. <br /> - `Delete`: Delete is based on Halt and deletes PVCs. <br /> - `WipeOut`: WipeOut is based on Delete and wipes out all volume snapshots and snapshot data from backup storage location. |

If no flags are used and no information specified, you create a MySQL cluster with default settings.

```bash
kbcli cluster create mysql-cluster --cluster-definition=apecloud-mysql --tolerations '"key=taint1name,value=true,operator=Equal,effect=NoSchedule","key=taint2name,value=true,operator=Equal,effect=NoSchedule"' 
```

### Create a cluster on a node without taints

The cluster creation command is simply `kbcli cluster create`. Further, you are recommended to create a cluster with specified class and customize your cluster settings as demanded.

To create a cluster with specified class, you can use `--set` flag and specify your requirement.

1. View and select a class for this cluster.

   ```bash
   kbcli class list --cluster-definition apecloud-mysql  
   ```

   :::note

   If there is no suitable class listed, you can [customize your own class](./../cluster-type/customize-class-type.md) template and apply the class here.

   Creating clusters that does not meet the constraints is invalid and the system creates a cluster with the minimum CPU value specified.

   :::

2. Create a cluster with specified class and add all taints on the current node in the `--toleration` flag to tolerate them.

   ```bash
   kbcli cluster create mysql-cluster --cluster-definition=apecloud-mysql --set class=general-2c2g,storageClass=<storageclassname> --namespace <name>
   ```

   ***Result***

   A cluster is created in the namespace `default` with the specified class.

Or change the corresponding parameters in the YAML file.

```bash
kbcli cluster create mysql-cluster --cluster-definition=apecloud-mysql --set storageClass=<storageclassname> --namespace <name> --set-file -<<EOF
- name: mysql-cluster
  replicas: 3
  componentDefRef: mysql
  volumeClaimTemplates:
  - name: data
    spec:
    accessModes:
    - ReadWriteOnce
      resources:
        requests:
          cpu: 2000m
          memory: 2Gi
          storage: 10Gi
EOF
```

See the table below for the detailed description for customizable parameters, setting the `--termination-policy` is necessary, and you are strongly recommended to turn on the monitor and enable all logs.

📎 Table 1. kbcli cluster create flags description

| Option                 | Description             |
|:-----------------------|:------------------------|
| `--cluster-definition` | It specifies the cluster definition and you can choose the database type. Run `kbcli cd list` to show all available cluster definitions.   |
| `--cluster-version`    | It specifies the cluster version. Run `kbcli cv list` to show all available cluster versions. If you do not specify a cluster version when creating a cluster, the latest version is applied by default.  |
| `--enable-all-logs`    | It enables you to view all application logs. When this function is enabled, enabledLogs of component level will be ignored. For logs settings, refer to [Access Logs](./../../observability/access-logs.md).  |
| `--help`               | It shows the help guide for `kbcli cluster create`. You can also use the abbreviated `-h`. |
| `--monitor`            | It is used to enable the monitor function and inject metrics exporter. It is set as true by default. |
| `--node-labels`        | It is a node label selector. Its default value is [] and means empty value. If you want set node labels, you can follow the example format: <br />`kbcli cluster create mysql-cluster --cluster-definition=apecloud-mysql --node-labels='"topology.kubernetes.io/zone=us-east-1a","disktype=ssd,essd"'`  |
| `--set`                | It sets the cluster resource including CPU, memory, replicas, and storage, each set corresponds to a component. For example, `--set cpu=1000m,memory=1Gi,replicas=1,storage=10Gi`.  |
| `--set-file`           | It uses a yaml file, URL, or stdin to set the cluster resource. |
| `--termination-policy` | It specifies how a cluster is deleted. Set the policy when creating a cluster. There are four available values, namely `DoNotTerminate`, `Halt`, `Delete`, and `WipeOut`. `Delete` is set as the default. <br /> - `DoNotTerminate`: DoNotTerminate blocks the delete operation. <br /> - `Halt`: Halt deletes workload resources such as statefulset, deployment workloads but keeps PVCs. <br /> - `Delete`: Delete is based on Halt and deletes PVCs. <br /> - `WipeOut`: WipeOut is based on Delete and wipes out all volume snapshots and snapshot data from backup storage location. |

If no flags are used and no information specified, you create a MySQL cluster with default settings.

```bash
kbcli cluster create mysql-cluster --cluster-definition=apecloud-mysql
```

## Connect to a MySQL Cluster

```bash
kbcli cluster connect <clustername>  --namespace <name>
```

For the detailed database connection guide, refer to [Connect database](./../../connect_database/overview-of-database-connection.md).
