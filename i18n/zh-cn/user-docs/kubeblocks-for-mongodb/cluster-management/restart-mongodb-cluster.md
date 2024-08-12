---
title: 重启集群
description: 如何重启集群
keywords: [mongodb, 重启集群]
sidebar_position: 4
sidebar_label: 重启
---

# 重启集群

KubeBlocks 支持重启集群中的所有 Pod。当数据库出现异常时，也可以尝试重启集群。

## 步骤

1. 使用 `kbcli cluster restart` 命令重启集群，然后再次输入集群名称。

    ```bash
    kbcli cluster restart mongodb-cluster
    >
    OpsRequest mongodb-cluster-restart-pzsbj created successfully, you can view the progress:
          kbcli cluster describe-ops mongodb-cluster-restart-pzsbj -n default
    ```

2. 查看集群状态，验证重启操作。

   ```bash
   kbcli cluster list mongodb-cluster
   >
   NAME                   NAMESPACE        CLUSTER-DEFINITION        VERSION            TERMINATION-POLICY        STATUS         CREATED-TIME
   mongodb-cluster        default          mongodb                   mongodb-5.0        Delete                    Running        Apr 26,2023 12:50 UTC+0800
   ```

   - STATUS=Updating 表示集群正在重启中。
   - STATUS=Running 表示集群已重启。

   您也可以使用在步骤 1 中随机生成的请求代码（本例中为 `pzsbj`）验证重启操作是否成功。

    ```bash
    kbcli cluster describe-ops mongodb-cluster-restart-pzsbj -n default
    ```
