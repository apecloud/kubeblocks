---
title: 集群扩缩容
description: 如何对集群进行扩缩容操作？
keywords: [postgresql, 垂直扩缩容]
sidebar_position: 2
sidebar_label: 扩缩容
---

# PostgreSQL 集群扩缩容

KubeBlocks 支持对 PostgreSQL 集群进行垂直扩缩容。

## 垂直扩缩容

你可以通过更改资源需求和限制（CPU 和存储）来垂直扩展集群。例如，如果你需要将资源类别从 1C2G 更改为 2C4G，就需要进行垂直扩容。

:::note

垂直扩容将触发 Pod 并行重启。重启后，主节点可能会发生变化。

:::

### 开始之前

确保集群处于 `Running` 状态，否则以下操作可能会失败。

```bash
kbcli cluster list <name>
```

***示例***

```bash
kbcli cluster list pg-cluster
>
NAME         NAMESPACE   CLUSTER-DEFINITION           VERSION             TERMINATION-POLICY   STATUS    CREATED-TIME
pg-cluster   default     postgresql-cluster           postgresql-14.7.0   Delete               Running   Mar 03,2023 18:00 UTC+0800
```

### 步骤

1. 更改配置，共有 3 种方式。

   **选项 1.** （**推荐**） 使用 kbcli

   配置参数 `--components`、`--memory` 和 `--cpu`，并执行以下命令。

   ***示例***

   ```bash
   kbcli cluster vscale pg-cluster \
   --components="pg-replication" \
   --memory="4Gi" --cpu="2" \
   ```

   - `--components` 表示可进行垂直扩容的组件名称。
   - `--memory` 表示组件请求和限制的内存大小。
   - `--cpu` 表示组件请求和限制的 CPU 大小。
  
   **选项 2.** 创建 OpsRequest
  
   将 OpsRequest 应用于指定的集群，根据需求配置参数。

   ```bash
   kubectl apply -f - <<EOF
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: ops-vertical-scaling
   spec:
     clusterRef: pg-cluster
     type: VerticalScaling 
     verticalScaling:
     - componentName: pg-replication
       requests:
         memory: "2Gi"
         cpu: "1000m"
       limits:
         memory: "4Gi"
         cpu: "2000m"
   EOF
   ```
  
   **选项 3.** 修改集群的 YAML 文件

   修改 YAML 文件中 `spec.components.resources` 的配置。`spec.components.resources` 控制资源需求和相关限制，更改配置将触发垂直扩容。

   ***示例***

   ```YAML
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: Cluster
   metadata:
     name: pg-cluster
     namespace: default
   spec:
     clusterDefinitionRef: postgresql-cluster
     clusterVersionRef: postgre-14.7.0
     componentSpecs:
     - name: pg-replication
       componentDefRef: postgresql
       replicas: 1
       resources: # 修改资源值
         requests:
           memory: "2Gi"
           cpu: "1000m"
         limits:
           memory: "4Gi"
           cpu: "2000m"
       volumeClaimTemplates:
       - name: data
         spec:
           accessModes:
             - ReadWriteOnce
           resources:
             requests:
               storage: 1Gi
     terminationPolicy: Halt
   ```

2. 验证垂直扩容。

    执行以下命令检查集群状态，验证垂直扩容。

    ```bash
    kbcli cluster list <name>
    ```

    ***示例***

    ```bash
    kbcli cluster list pg-cluster
    >
    NAME              NAMESPACE        CLUSTER-DEFINITION            VERSION                TERMINATION-POLICY   STATUS    CREATED-TIME
    pg-cluster        default          postgresql-cluster            postgresql-14.7.0      Delete               Running   Mar 03,2023 18:00 UTC+0800
    ```

   - STATUS=VerticalScaling 表示正在进行垂直扩容。
   - STATUS=Running 表示垂直扩容已完成。
   - STATUS=Abnormal 表示垂直扩容异常。原因可能是正常实例的数量少于总实例数，或者 Leader 实例正常运行而其他实例异常。
     > 你可以手动检查是否由于资源不足而导致报错。如果 Kubernetes 集群支持 AutoScaling，系统在资源充足的情况下会执行自动恢复。或者你也可以创建足够的资源，并使用 `kubectl describe` 命令进行故障排除。

    :::note

    垂直扩容不会同步与 CPU 和内存相关的参数，需要手动调用配置的 OpsRequest 来进行更改。详情请参考[配置](./../configuration/configuration.md)。

    :::

3. 检查资源是否已经发生更改。

    ```bash
    kbcli cluster describe pg-cluster
    ```
