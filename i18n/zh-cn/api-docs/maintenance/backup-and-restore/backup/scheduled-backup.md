---
title: 定时备份
description: 如何定时备份
keywords: [备份恢复, 定时, 自动备份, 定时备份]
sidebar_position: 3
sidebar_label: 定时备份
---

# 定时备份

KubeBlocks 支持为集群配置自动备份。

用 kubectl 命令修改 Cluster 中的 backup 字段，命令如下：

```bash
kubectl edit cluster -n default mycluster
>
spec:
  ...
  backup:
    # 开启自动备份
    enabled: true
    # UTC 时区, 下面示例是每周一凌晨 2 点
    cronExpression: 0 18 * * *
    # 使用 xtrabackup 进行备份，如果存储支持快照，可以指定为 volume-snapshot
    method: xtrabackup
    # 是否开启 PITR
    pitrEnabled: false
    # 备份集保留时长
    retentionPeriod: 7d
    # BackupRepo
    repoName: my-repo
```

您可在以上 YAML 文件中按需设置是否开启自动备份和 PITR，也可以指定备份方式、仓库名称、保留时长等。

开启自动备份后，可以执行如下命令查看是否有 CronJob 对象被创建：

```bash
kubectl get cronjob
>
NAME                                        SCHEDULE     SUSPEND   ACTIVE   LAST SCHEDULE   AGE
96523399-mycluster-default-xtrabackup       0 18 * * *   False     0        <none>          57m
```
