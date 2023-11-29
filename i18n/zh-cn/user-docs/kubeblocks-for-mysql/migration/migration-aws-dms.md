---
title: 通过 AWS DMS 将数据迁移到 ApeCloud MySQL
description: 如何通过 AWS DMS 将数据迁移到 ApeCloud MySQL
keywords: [mysql, 迁移, aws dms]
sidebar_position: 1
sidebar_label: 通过 AWS DMS 迁移
---

# 通过 AWS DMS 将数据迁移到 ApeCloud MySQL

:::note

* 使用公共网络和网络负载均衡器可能会产生费用。
* 本文档适用于在 AWS EKS 上部署 ApeCloud MySQL 的情况。使用其他 Kubernetes 集群部署 ApeCloud MySQL 不适用于本文档。

:::

## 网络配置

### 暴露目标 ApeCloud MySQL 网络

在 EKS 环境中，ApeCloud MySQL 的 Kubernetes ClusterIP 默认是暴露的。由于 DMS（数据库迁移服务）的迁移任务是在独立的 Replication Instance 中运行的，虽然可以设置与 Kubernetes 集群使用相同的 VPC，但访问 ClusterIP 仍然会失败。这个解决方案旨在连接此部分网络。

#### KubeBlocks 自带解决方案

***开始之前***

* [安装 kbcli](./../../installation/install-with-kbcli/install-kbcli.md)
* [安装 KubeBlocks](链接): 你可以用 kbcli 或 Helm 来安装 KubeBlocks。
* 开启 AWS Load Balancer Controller 插件。

   ```bash
   kbcli addon list

   kbcli addon enable aws-load-balancer-controller
   >
   addon.extensions.kubeblocks.io/aws-load-balancer-controller enabled
   ```

   负载均衡器插件依赖于 EKS 环境，如果负载均衡器未成功开启，可能与环境有关。

   检查 EKS 环境并重新启用此插件。详情请参阅[启用插件](链接)。

***步骤***

1. 在 AWS 上创建 ApeCloud MySQL 集群。详情请参阅[创建 ApeCloud MySQL 集群](链接)。
2. 填写集群名称，并执行以下命令暴露集群的外部 IP。

   ```bash
   kbcli cluster expose mysql-cluster --enable=true --type='vpc'
   ```

   :::note

   For the above `kbcli cluster expose` command, the available value for `--type` are `vpc` and `internet`. Use `--type=vpc` for access within the same VPC and `--type=internet` for cross VPC access under the public network.
   在 `kbcli cluster expose` 命令中，`--type` 的可用值为 `vpc` 和 `internet`。对于同一 VPC 内的访问，请使用 `--type=vpc`；如果是公共网络下的跨 VPC 访问，请使用 `--type=internet`。

   :::

   执行以下命令，查看到外部 IP:Port 地址，该地址可以被 EKS 集群外的同一 VPC 的机器访问。

   ```bash
   kbcli cluster describe mysql-cluster | grep -A 3 Endpoints
   >
   Endpoints:
   COMPONENT       MODE            INTERNAL                EXTERNAL
   mysql           ReadWrite       10.100.51.xxx:3306      172.31.35.xxx:3306 
   ```

3. 将外部 IP: Port 配置为 AWS DMS 上的目标终端节点。

   此操作将在 EC2 上生成一个 ENI（弹性网络接口）。如果低规格机型的 Quota 较小，请注意 ENI 的可用水位。

   有关EC2 机型对应的 ENI 规格，请参阅[弹性网络接口](https://docs.aws.amazon.com/zh_cn/AWSEC2/latest/UserGuide/using-eni.html)。

#### 使用 Network Load Balancer (NLB) 暴露服务

1. Install Load Balancer Controller on EKS.

   For installation details, refer to [Installing the AWS Load Balancer Controller add-on](https://docs.aws.amazon.com/eks/latest/userguide/aws-load-balancer-controller.html).

   For how to create NLB in a cluster, refer to [Network load balancing on Amazon EKS](https://docs.aws.amazon.com/eks/latest/userguide/network-load-balancing.html).
2. Create a service that uses NLB to expose the ApeCloud MySQL service.

   Configure `metadata.name`, `metadata.annotations`, `metadata.labels`, and `spec.selector` according to your actual environment.

   ```yaml
   cat <<EOF | kubectl apply -f -
   kind: Service
   apiVersion: v1
   metadata:
       name: apecloud-mysql-service
       annotations:
           service.beta.kubernetes.io/aws-load-balancer-type: nlb-ip
           alb.ingress.kubernetes.io/scheme: internet-facing
           service.beta.kubernetes.io/aws-load-balancer-subnets: <subnet name1>,<subnet name2>
       labels:
         apps.kubeblocks.io/component-name: mysql
         app.kubernetes.io/instance: <apecloud-mysql clustername>
         app.kubernetes.io/managed-by: kubeblocks
         app.kubernetes.io/name: apecloud-mysql     
   spec:
       externalTrafficPolicy: Cluster 
       type: LoadBalancer
       selector:
         apps.kubeblocks.io/component-name: mysql
         app.kubernetes.io/instance: <apecloud-mysql clustername>
         app.kubernetes.io/managed-by: kubeblocks
         kubeblocks.io/role: leader
       ports:
           - name: http
             protocol: TCP
             port: 3306
             targetPort: mysql 
   EOF
   ```

3. Check whether this new service and NLB run normally.

   ```bash
   kubectl get svc 
   >
   NAME                           TYPE           CLUSTER-IP       EXTERNAL-IP                                        PORT(S)  
   apecloud-mysql-service         LoadBalancer   10.100.xx.xx     k8s-xx-xx-xx.elb.cn-northwest-1.amazonaws.com.cn   3306:xx/TCP
   ```

   Make sure the server runs normally and can generate EXTERNAL-IP. Meanwhile, verify whether the NLB state is `Active` by the AWS console, then you can access the cluster by EXTERNAL-IP:Port.

   ![NLB-active](./../../../img/mysql_migration_active.png)

### Expose the source network

There exist four different conditions for the source network. Choose one method to expose the source network according to your actual environment.

* Alibaba Cloud ApsaraDB RDS
  
   Use the public network. Refer to [Apply for or release a public endpoint for an ApsaraDB RDS for MySQL instance](https://www.alibabacloud.com/help/en/apsaradb-for-rds/latest/apply-for-or-release-a-public-endpoint-for-an-apsaradb-rds-for-mysql-instance) to release a public endpoint then create an endpoint in AWS DMS.

* RDS within the same VPC in AWS
  
   You only need to specify an RDS when creating an endpoint in DMS and no extra operation is required.

   For creating an endpoint, refer to step 2 in [Configure AWS DMS tasks](#configure-aws-dms-tasks).

* RDS within different VPCs in AWS
  
   Use the public network to create an endpoint. Refer to [this document](https://aws.amazon.com/premiumsupport/knowledge-center/aurora-mysql-connect-outside-vpc/?nc1=h_ls) to make public network access available, then create an endpoint in AWS DMS.

   For creating an endpoint, refer to step 2 in [Configure AWS DMS tasks](#configure-aws-dms-tasks).

* MySQL in AWS EKS
  
   Use NLB to expose the service.

  1. Install Load Balancer Controller.

     For installation details, refer to [Installing the AWS Load Balancer Controller add-on](https://docs.aws.amazon.com/eks/latest/userguide/aws-load-balancer-controller.html).

     For how to create NLB in a cluster, refer to [Network load balancing on Amazon EKS](https://docs.aws.amazon.com/eks/latest/userguide/network-load-balancing.html).
  2. Create the service using NLB.

     Make sure the value of `some.label.key` in `metadata.labels` is consistent with the value of ApeCloud MySQL you created.

     Configure `port` and `targetPort` in `spec.ports` according to your current environment.

     ```yaml
     cat <<EOF | kubectl apply -f -
     kind: Service
     apiVersion: v1
     metadata:
         name: mysql-local-service
         annotations:
             service.beta.kubernetes.io/aws-load-balancer-type: nlb-ip
             alb.ingress.kubernetes.io/scheme: internet-facing
             service.beta.kubernetes.io/aws-load-balancer-subnets: ${subnet name1},${subnet name2}
         labels:
         some.label.key: some-label-value    
     spec:
         externalTrafficPolicy: Cluster 
         type: LoadBalancer
         selector:
         some.label.key: some-label-value  
         ports:
             - name: http
               protocol: TCP
               port: 3306
               targetPort: 3306 
     EOF
     ```

  3. Make sure Service and NLB run normally.

     Refer to step 3 in [Use Network Load Balancer (NLB) to expose the service](#use-network-load-balancer-nlb-to-expose-the-service) for details.

## Configure AWS DMS tasks

Pay attention to the following potential issues during the migration task.

* Double write
  
   During the migration, make sure no business is writing to the target data instance. Otherwise, double write occurs.

* Disk space of the target instance
  
   Since the transfer tool uses a concurrent write model when writing to the target database, out-of-order writes may occur, which may trigger page splitting and cause the data space of the target database to be slightly enlarged compared with that of the original instance. It is recommended to plan appropriately when allocating the storage size of the target database, for example, at least 1.5 times the current storage size of the source database.

* DDL and onlineDDL
  
   Locked structure changes often affect the speed of data migration.

   The lock-free structure change is based on the rename of the temporary table in principle, which causes data problems if the migration object is not the whole database migration.

   For example, if the migration object chooses to migrate db1.table1 to the target, and an onlineDDL is performed on db1.table1 on the source database during the process, the data of db1.table1 on the target database will be inconsistent with the source database.

   It should be noted that the way some database management tools initiate DDL is performed by using lock-free mutation by default.

   Migration is a short-term behavior. To avoid unnecessary troubles, it is recommended not to perform DDL operations during the migration process.

* BinLog retention hours

   The incrementally migrating process of data transmission relies on the BinLog of the source database.

   It is recommended to extend the BinLog retention hours to avoid a long-term interruption and the situation that the BinLog of the source database is cleared during recovery, resulting in the migration not being resumed.

   For example, in AWS RDS, connect to the database and run the command below:

   ```bash
   # View configuration
   # Input: 
   call mysql.rds_show_configuration;

   # Output: Pay attention to the BinLog retention hours.
   +------------------------+-------+-----------------------------------------------------------------------------------------------------------+
   | name                   | value | description                                                                                               |
   +------------------------+-------+-----------------------------------------------------------------------------------------------------------+
   | binlog retention hours | 8     | binlog retention hours specifies the duration in hours before binary logs are automatically deleted.      |
   | source delay           | 0     | source delay specifies replication delay in seconds between current instance and its master.              |
   | target delay           | 0     | target delay specifies replication delay in seconds between current instance and its future read-replica. |
   +------------------------+-------+-----------------------------------------------------------------------------------------------------------+

   # Adjust the retention hours to 72 hours
   # Input:
   call mysql.rds_set_configuration('binlog retention hours', 72);
   ```

***Steps:***

1. Create a Replication Instance for migration.

   Go to **DMS** -> **Replication Instance** and click **Create replication instance**.

   :::caution

   Select the VPC that you have configured in EKS.

   :::

   ![Create replication instance](./../../../img/mysql_migration_replication_instance.png)

2. Create endpoints.

   Go to **DMS** -> **Endpoints** and click **Create endpoint**.

   ![Create endpoint](./../../../img/mysql_migration_create_endpoints.png)

   Create the source endpoint and target endpoint respectively. If the target endpoint is the RDS instance, check **Select RDS DB instance** to configure it.

   ![Select RDS DB instance](./../../../img/mysql_migration_select_rds_db_instance.png)

   After configuration, specify a replication instance to test the connection.

   ![Test connection](./../../../img/mysql_migration_test_connection.png)

3. Create migration tasks.

   ![Create task](./../../../img/mysql_migration_create_task.png)

   Click **Create task** and configure the task according to the instructions.

   Pay attention to the following parameters.

   * Migration Type

     ![Migration type](./../../../img/mysql_migration_migration_type.png)

     AWS DMS provides three migration types:

     * Migrate existing data: AWS DMS migrates only your existing data. Changes to your source data aren’t captured and applied to your target.
     * Migrate existing data and replicate ongoing changes: AWS DMS migrates both existing data and ongoing data changes, i.e. the existing data before the migration task and the data changes during the migration task will be synchronized to the target instance.
     * Replicate data changes only: AWS DMS only migrates the ongoing data changes. If you select this type, you can use **CDC start mode for source transactions** to specify a location and migrate the data changes.
    For this tutorial, select **Migrate existing data and replicate ongoing changes**.

   * Target table preparation mode

     ![Target table preparation mode](./../../../img/mysql_migration_target_table_preparation_mode.png)

     The target table preparation mode specifies the initial mode of the data structure. You can click the Info link beside the options to view the definition of each mode. For example, if ApeCloud MySQL is a newly created empty instance, you can select **Do nothing** mode.

     In addition, create a database on ApeCloud MySQL before migration because AWS DMS does not create a database.

   * Turn on validation
  
     It is recommended to enable this function.

     ![Turn on validation](./../../../img/mysql_migration_turn_on_validation.png)

   * Batch-optimized apply
  
     It is recommended to enable this function as this function enables you to write target instances in batch and can improve the write speed.

     ![Batch-optimized apply](./../../../img/mysql_migration_batch_optimized_apply.png)

   * Full load tuning settings: Maximum number of tables to load in parallel

     This number decides how many concurrencies DMS uses to get source table data. Theoretically speaking, this can cause pressure on the source table during the full-load migration. Lower this number when the business in the source table is delicate.

     ![Full load tuning settings](./../../../img/mysql_migration_full_load_tuning_settings.png)

   * Table Mapping

     Table mapping decides which tables in the database are used for migration and can also apply easy conversions. It is recommended to enable **Wizard** mode to configure this parameter.
4. Start the migration task.

## Switch applications

***Before you start***

* Make sure DMS migration tasks run normally. If you perform a validation task, make sure the results are as expected.
* To differentiate conversation and improve data security, it is recommended to create and authorize a database account solely for migration.
* It is recommended to switch applications during business off-peak hours because for safety concerns during the switching process, it is necessary to stop business write.

***Steps:***

1. Make sure the transmission task runs normally.

   Pay attention to **Status**, **Last updated in Table statistics**, and **CDC latency target** in **CloudWatch metrics**.

   You can also refer to [this document](https://aws.amazon.com/premiumsupport/knowledge-center/dms-stuck-task-progress/?nc1=h_ls) to verify the migration task.

   ![Status](./../../../img/mysql_migration_application_status.png)

   ![CDC](./../../../img/mysql_migration_application_cdc.png)

2. Pause business and prohibit new business write in the source database.
3. Verify the transmission task status again to make sure the task runs normally and the running status lasts at least 1 minute.

   Refer to step 1 above to observe whether the link is normal and whether latency exists.
4. Use the target database to resume business.
5. Verify the migration with business.
