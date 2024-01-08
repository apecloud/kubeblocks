---
title: 模拟 GCP 故障
description: 模拟 GCP 故障
sidebar_position: 11
sidebar_label: 模拟 GCP 故障
---

# 模拟 GCP 故障

GCPChaos 实验能够模拟指定的 GCP 实例发生故障的场景。目前，GCPChaos 实验支持以下几种故障类型：

* Node Stop：使指定的 GCP 实例进入停止状态。
* Node Restart：重置指定的 GCP 实例。
* Disk Loss：从指定的 GCP 实例中卸载存储卷。

## 开始之前

* 默认已导入本地代码的 GCP 身份验证信息。如果尚未导入，请按照[前提条件](./prerequisite.md#check-your-permission)文档中的步骤进行操作。

* 为了方便连接到 GCP 集群，可以提前创建一个 Kubernetes Secret 文件存储身份验证信息。`Secret` 文件的示例如下：
  
  ```yaml
  apiVersion: v1
  kind: Secret
  metadata:
    name: cloud-key-secret-gcp
    namespace: default
  type: Opaque
  stringData:
    service_account: your-gcp-service-account-base64-encode
  ```
  
  * `name` 表示 Kubernetes Secret 对象的名字。
  * `namespace` 表示 Kubernetes Secret 对象的命名空间。
  * `service_account` 存储 GCP 集群的服务账号密钥。请注意，你需要对 GCP 集群的服务账号密钥进行 Base64 编码。如需了解 GCP 服务账号密钥，请参阅[创建和管理服务帐号密钥](https://cloud.google.com/iam/docs/keys-create-delete?hl=zh-cn)。

## 使用 kbcli 模拟故障注入

### Stop

执行以下命令，向指定的 GCP 实例中注入 `node-stop` 故障，使该实例在 3 分钟内处于不可用的状态。

```bash
kbcli fault node stop [node1] [node2] -c=gcp --region=us-central1-c --duration=3m
```

执行上述命令后，`node-stop` 命令会创建资源，包括 Secret `cloud-key-secret-gcp` 和 GCPChaos `node-chaos-w98j5`。执行 `kubectl describe node-chaos-w98j5` 命令，可以验证是否成功注入了 `node-stop` 故障。

:::caution

在更改集群权限、更新密钥或更改集群 context 时，必须删除 `cloud-key-secret-gcp`。注入 `node-stop` 后，将根据新的密钥创建一个新的 `cloud-key-secret-gcp`。

:::

### Restart

执行以下命令，向指定的 GCP 实例中注入 `node-restart` 故障，使得该实例重启。

```bash
kbcli fault node restart [node1] [node2] -c=gcp --region=us-central1-c
```

### Detach volume

执行以下命令，向指定的 GCP 实例中注入 `detach-volume` 故障，使该实例与指定存储卷分离。

```bash
kbcli fault node detach-volume [node1] -c=gcp --region=us-central1-c --device-name=/dev/sdb
```

## 使用 YAML 文件模拟故障注入

### GCP stop 示例

1. 将实验配置写入到 `gcp-stop.yaml` 文件中。

   在下例中，Chaos Mesh 将向指定的 GCP 实例中注入 `node-stop` 故障，使该实例在 30 秒内不可用。

   ```yaml
   apiVersion: chaos-mesh.org/v1alpha1
   kind: GCPChaos
   metadata:
     creationTimestamp: null
     generateName: node-chaos-
     namespace: default
   spec:
     action: node-stop
     duration: 30s
     instance: gke-yjtest-default-pool-c2ee710b-fs5q
     project: apecloud-platform-engineering
     secretName: cloud-key-secret-gcp
     zone: us-central1-c
   ```

2. 使用 `kubectl` 创建实验。

   ```bash
   kubectl apply -f ./aws-detach-volume.yaml
   ```

### GCP restart 示例

1. 将实验配置写入到 `gcp-restart.yaml` 文件中。

   在下例中，Chaos Mesh 将向指定的 GCP 实例中注入 `node-reset` 故障，使得该实例重启。

   ```yaml
   apiVersion: chaos-mesh.org/v1alpha1
   kind: GCPChaos
   metadata:
     creationTimestamp: null
     generateName: node-chaos-
     namespace: default
   spec:
     action: node-reset
     duration: 30s
     instance: gke-yjtest-default-pool-c2ee710b-fs5q
     project: apecloud-platform-engineering
     secretName: cloud-key-secret-gcp
     zone: us-central1-c
   ```

2. 使用 `kubectl` 创建实验。

   ```bash
   kubectl apply -f ./aws-detach-volume.yaml
   ```

### GCP detach volume 示例

1. 将实验配置写入到 `gcp-detach-volume.yaml` 文件中。

   在下例中，Chaos Mesh 将向指定的 GCP 实例中注入 `disk-loss` 故障，使该实例在 30 秒内与指定存储卷分离。

   ```yaml
   apiVersion: chaos-mesh.org/v1alpha1
   kind: GCPChaos
   metadata:
     creationTimestamp: null
     generateName: node-chaos-
     namespace: default
   spec:
     action: disk-loss
     deviceNames:
     - /dev/sdb
     duration: 30s
     instance: gke-yjtest-default-pool-c2ee710b-fs5q
     project: apecloud-platform-engineering
     secretName: cloud-key-secret-gcp
     zone: us-central1-c
   ```

2. 使用 `kubectl` 创建实验。

   ```bash
   kubectl apply -f ./aws-detach-volume.yaml
   ```

### 字段说明

下表介绍以上 YAML 配置文件中的字段。

| 参数 | 类型 | 说明 | 默认值 | 是否必填 |
| :--- | :--- | :--- | :--- | :--- |
| action | string | 指定要注入的故障类型，支持 `node-stop`、`node-reset` 和 `disk-loss`。 | `node-stop` | 是 |
| mode | string | 指定实验的运行方式，可选项包括：`one`（表示随机选出一个符合条件的 Pod）、`all`（表示选出所有符合条件的 Pod）、`fixed`（表示选出指定数量且符合条件的 Pod）、`fixed-percent`（表示选出占符合条件的 Pod 中指定百分比的 Pod）和 `random-max-percent`（表示选出占符合条件的 Pod 中不超过指定百分比的 Pod）。 | 无 | 是 |
| value | string | 取决于 `mode` 的配置，为 `mode` 提供对应的参数。例如，当你将 `mode` 配置为 `fixed-percent` 时，`value` 用于指定 Pod 的百分比。 | 无 | 否 |
| secretName | string | 指定存储 GCP 认证信息的 Kubernetes Secret 名字。| 无 | 否 |
| project | string | 指定 GCP 项目的 ID。 | 无 | 是 | real-testing-project |
| zone | string | 指定 GCP 实例的区域。 | 无 | 是 |
| instance | string | 指定 GCP 实例的名称。| 无 | 是 |
| deviceNames | []string | 当 `action` 为 `disk-loss` 时必填，指定设备磁盘 ID。 | 无 | 否 |
| duration | string | 指定实验的持续时间。 | 无 | 是 |
