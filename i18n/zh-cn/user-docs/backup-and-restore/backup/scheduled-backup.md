---
title: 定时备份
description: 如何进行定时备份
keywords: [备份恢复, 定时, 自动备份, 定时备份]
sidebar_position: 3
sidebar_label: 定时备份
---

# 定时备份

KubeBlocks 支持用 kubectl 为集群配置自动备份。

您可以修改 Cluster 中的 backup 字段。以 mysql-cluster 为例，参考命令如下：

```bash
kubectl edit cluster -n default mysql-cluster
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
    # 备份集保留时长
    retentionPeriod: 7d
    # 备份仓库
    repoName: my-repo
```

在上述 YAML 文件中，您可以根据需要设置是否开启自动备份，指定备份方法、仓库名称、保留时长、备份周期等。

开启自动备份后，可以看一下是否有 CronJob 对象被创建：

```bash
kubectl get cronjob
>
NAME                                        SCHEDULE     SUSPEND   ACTIVE   LAST SCHEDULE   AGE
96523399-mysql-cluster-default-xtrabackup   0 18 * * *   False     0        <none>          57m
```