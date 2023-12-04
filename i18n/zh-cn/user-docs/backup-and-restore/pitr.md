# 时间点恢复（PITR）
:::note

MySQL 和 MongoDB 的 PITR 功能还处于测试阶段。

:::

1. 查看集群可以恢复到的时间戳。
```
kbcli cluster describe mysql-cluster
...
Data Protection:
AUTO-BACKUP   BACKUP-SCHEDULE   TYPE       BACKUP-TTL   LAST-SCHEDULE                RECOVERABLE-TIME
Enabled       0 18 * * *        datafile   7d           Jul 25,2023 19:36 UTC+0800   Jul 25,2023 14:53:00 UTC+0800 ~ Jul 25,2023 19:07:38 UTC+0800
 ```
   `RECOVERABLE-TIME` 表示集群可以恢复到的时间范围。

2. 执行以下命令将集群恢复到指定的时间点。
```
kbcli cluster restore mysql-cluster-pitr --restore-to-time 'Jul 25,2023 18:52:53 UTC+0800' --source-cluster mysql-cluster
```
3. 查看新集群的状态。
若状态显示为 Running，则恢复成功。
```
kbcli cluster list mysql-cluster-pitr
>
NAME                 NAMESPACE   CLUSTER-DEFINITION   VERSION           TERMINATION-POLICY   STATUS    CREATED-TIME
mysql-cluster-pitr   default     apecloud-mysql       ac-mysql-8.0.30   Delete               Running   Jul 25,2023 19:42 UTC+0800
```
