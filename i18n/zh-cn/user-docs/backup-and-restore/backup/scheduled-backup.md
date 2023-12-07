# 定时备份

## 按计划备份
你可以通过修改相关参数来自定义备份计划。

:::caution
由 kbcli 或 kubectl 创建的备份会被永久保存。如果想删除备份，请手动删除。
:::

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```
kbcli cluster edit-backup-policy mysql-cluster-mysql-backup-policy
>
spec:
  ...
  schedule:
    datafile:
      # UTC 时区，下面的示例代表每周一凌晨 2 点
      cronExpression: "0 18 * * 0"
      # 启用功能
      enable: true
```
</TabItem>

<TabItem value="kubectl" label="kubectl">

```
kubectl edit cluster -n default mysql-cluster
>
spec:
  ...
  backup:
    # 启动自动备份
    enabled: true
    # UTC 时区，下面的示例代表每周一凌晨 2 点
    cronExpression: 0 18 * * *
    # 指定备份方法。下面是 backupTool 的示例。如果支持快照存储，可以将其更改为 snapshot
    method: backupTool
    # 禁用 PITR。如果启用，自动备份也会相应启用
    pitrEnabled: false
    # 备份集保留时长
    retentionPeriod: 1d
```
</TabItem>

</Tabs>

## 自动日志备份
KubeBlocks 仅支持自动日志备份。

### 在开始之前

以 MySQL 为例，准备一个用于测试备份和恢复功能的集群。

1. 创建集群。
```
kbcli cluster create mysql mysql-cluster
```
2. 查看备份策略。
```
kbcli cluster list-backup-policy mysql-cluster
```
默认情况下，所有备份都存储在默认的全局仓库中。你可以通过[编辑 BackupPolicy 资源](./backup-repo.md#optional-change-the-backup-repository-for-a-cluster)来指定新的仓库。


### 创建备份

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

目前，KubeBlocks 仅支持自动日志备份。
1. 执行以下命令启用自动日志备份。
```
kbcli cluster edit-backup-policy mysql-cluster-mysql-backup-policy --set schedule.logfile.enable=true
```
2. 检查备份是否成功。
```
kbcli cluster list-backups
```
</TabItem>

<TabItem value="kubectl" label="kubectl">
在集群的 yaml 配置文件中将 pitrEnabled 设置为 true，启用自动日志备份。
```
kubectl edit cluster -n default mysql-cluster
>
spec:
  ...
  backup:
    ...
    # 如果值为 true，将自动启用日志备份
    pitrEnabled: true
```
</TabItem>
</Tabs>

