# 从备份集中恢复数据

KubeBlocks 支持从备份集中恢复数据。

## 选项 1. 使用 kbcli cluster restore 命令恢复

1. 查看备份集。
```
kbcli cluster list-backups
```
2. 指定一个新的集群和备份集来恢复数据。
```
kbcli cluster restore new-mysql-cluster --backup backup-default-mysql-cluster-20230418124113
```
3. 查看新集群，确保处于 Running 状态。
```
kbcli cluster list new-mysql-cluster
```
## 选项 2. 使用 kbcli cluster create 命令恢复
```
kbcli cluster create --backup backup-default-mycluster-20230616190023
```
