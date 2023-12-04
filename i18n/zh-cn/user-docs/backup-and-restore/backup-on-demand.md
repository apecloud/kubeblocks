# 按需备份

## 在开始之前

以 MySQL 为例，首先准备一个用于测试备份恢复功能的集群。

1. 创建集群。
```
kbcli cluster create mysql mysql-cluster
```
2. 查看备份策略。
```
kbcli cluster list-backup-policy mysql-cluster
```
默认情况下，所有备份都存储在默认的全局仓库中。你可以通过[编辑 BackupPolicy 资源](./backup-repo.md#optional-change-the-backup-repository-for-a-cluster)来指定新的仓库。

## 创建备份
KubeBlocks 支持两种备份选项：备份工具备份和快照备份。

### 备份工具备份
KubeBlocks 同时支持 kbcli 和 kubectl。
<Tabs>

<TabItem value="kbcli" label="kbcli" default>

1. 查看集群，确保处于 Running 的状态。
```
kbcli cluster list mysql-cluster
```  
2. 创建备份。
```
kbcli cluster backup mysql-cluster --type=datafile
```
3. 查看备份集。
```
kbcli cluster list-backups mysql-cluster
```
</TabItem>

<TabItem value="kubectl" label="kubectl">
执行以下命令来创建名为 mybackup 的备份。
```
kubectl apply -f - <<-'EOF'
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: Backup
metadata:
  name: mybackup
  namespace: default
spec:
  backupPolicyName: mycluster-mysql-backup-policy
  backupType: datafile
EOF
```
</TabItem>

</Tabs>

### 快照备份
KubeBlocks 同时支持 kbcli 和 kubectl。

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

1. 查看集群，确保处于 Running 的状态。
```
kbcli cluster list mysql-cluster
```
2. 创建快照备份。
```
kbcli cluster backup mysql-cluster --type=snapshot
```
3. 查看备份集，检查备份是否成功。
```
kbcli cluster list-backups mysql-cluster
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

执行以下命令来创建名为 mysnapshot 的快照备份。
```
kubectl apply -f - <<-'EOF'
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: Backup
metadata:
  name: mysnapshot
  namespace: default
spec:
  backupPolicyName: mycluster-mysql-backup-policy
  backupType: snapshot
EOF
```
</TabItem>

</Tabs>

