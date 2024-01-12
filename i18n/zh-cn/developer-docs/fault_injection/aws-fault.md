---
title: 模拟 AWS 故障
description: 模拟 AWS 故障
sidebar_position: 10
sidebar_label: 模拟 AWS 故障
---

# 模拟 AWS 故障

AWSChaos 实验能够模拟指定的 AWS 实例发生故障的场景。目前，AWSChaos 实验支持以下几种故障类型：

* EC2 Stop：停止指定的 EC2 实例。
* EC2 Restart：重启指定的 EC2 实例。
* Detach Volume：从指定的 EC2 实例中卸载存储卷。

## 开始之前

* 默认已导入本地代码的 AWS 身份验证信息。如果尚未导入，请按照[前提条件](./prerequisite.md#check-your-permission)文档中的步骤进行操作。

* 为了方便连接到 AWS 集群，可以提前创建一个 Kubernetes Secret 文件存储身份验证信息。`Secret` 文件的示例如下：

    ```yaml
    apiVersion: v1
    kind: Secret
    metadata:
      name: cloud-key-secret-aws
      namespace: default
    type: Opaque
    stringData:
      aws_access_key_id: your-aws-access-key-id
      aws_secret_access_key: your-aws-secret-access-key
    ```

  * `name` 表示 Kubernetes Secret 对象的名字。
  * `namespace` 表示 Kubernetes Secret 对象的命名空间。
  * `aws_access_key_id` 表示存储 AWS 集群的访问密钥 ID。
  * `aws_secret_access_key` 表示存储 AWS 集群的秘密访问密钥。

## 使用 kbcli 模拟故障注入

### Stop

执行以下命令，向指定的 EC2 实例中注入 `instance-stop` 故障，使该实例在 3 分钟内不可用。

```bash
kbcli fault node stop [node1] -c=aws --region=cn-northwest-1 --duration=3m
```

### Restart

执行以下命令，向指定的 EC2 实例中注入 `instance-restart` 故障，使得该实例重启。

```bash
kbcli fault node restart [node1] -c=aws --region=cn-northwest-1 --duration=3m
```

### Detach volume

执行以下命令，向两个指定的 EC2 实例中注入 `detach-volume` 故障，使这两个实例在 1 分钟内与它们的指定存储卷分离。

```bash
kbcli fault node detach-volume [node1] -c=aws --region=cn-northwest-1 --duration=1m --volume-id=vol-xxx --device-name=/dev/xvdaa
```

你也可以添加多个节点及其卷，例如：

```bash
kbcli fault node detach-volume [node1] [node2] -c=aws --region=cn-northwest-1 --duration=1m --volume-id=vol-xxx,vol-xxx --device-name=/dev/sda,/dev/sdb
```

## 使用 YAML 文件模拟故障注入

本节介绍如何使用 YAML 文件模拟故障注入。你可以参考 [Chaos Mesh 官方文档](https://chaos-mesh.org/zh/docs/next/simulate-aws-chaos/#使用-yaml-方式创建实验)获取更详细的信息。

### AWS stop 示例

1. 将实验配置写入到 `aws-stop.yaml` 文件中。

   在下例中，Chaos Mesh 将向指定的 EC2 实例中注入 `ec2-stop` 故障，使该实例在 3 分钟内不可用。

   ```yaml
   apiVersion: chaos-mesh.org/v1alpha1
   kind: AWSChaos
   metadata:
     creationTimestamp: null
     generateName: node-chaos-
     namespace: default
   spec:
     action: ec2-stop
     awsRegion: cn-northwest-1
     duration: 3m
     ec2Instance: i-037b1f38debb59bd7
     secretName: cloud-key-secret-aws
   ```

2. 使用 `kubectl` 创建实验。

   ```bash
   kubectl apply -f ./aws-stop.yaml
   ```

### AWS restart 示例

1. 将实验配置写入到 `aws-restart.yaml` 文件中。

   在下例中，Chaos Mesh 将向指定的 EC2 实例中注入 `ec2-restart` 故障，使得该实例重启。

   ```yaml
   apiVersion: chaos-mesh.org/v1alpha1
   kind: AWSChaos
   metadata:
     creationTimestamp: null
     generateName: node-chaos-
     namespace: default
   spec:
     action: ec2-restart
     awsRegion: cn-northwest-1
     duration: 3m
     ec2Instance: i-037b1f38debb59bd7
     secretName: cloud-key-secret-aws
   ```

2. 使用 `kubectl` 创建实验。

   ```bash
   kubectl apply -f ./aws-restart.yaml
   ```

### AWS detach volume 示例

1. 将实验配置写入到 `aws-detach-volume.yaml` 文件中。

   在下例中，Chaos Mesh 将向两个指定的 EC2 实例中注入 `detach-volume` 故障，使这两个实例在 1 分钟内与它们的指定存储卷分离。

   ```yaml
   apiVersion: chaos-mesh.org/v1alpha1
   kind: AWSChaos
   metadata:
     creationTimestamp: null
     generateName: node-chaos-
     namespace: default
   spec:
     action: detach-volume
     awsRegion: cn-northwest-1
     deviceName: /dev/xvda
     duration: 1m
     ec2Instance: i-0e368667e544fa955
     secretName: cloud-key-secret-aws
     volumeID: vol-01b3d68c074cd93a9
   status:
     experiment: {}
   apiVersion: chaos-mesh.org/v1alpha1
   kind: AWSChaos
   metadata:
     creationTimestamp: null
     generateName: node-chaos-
     namespace: default
   spec:
     action: detach-volume
     awsRegion: cn-northwest-1
     deviceName: /dev/xvdaa
     duration: 1m
     ec2Instance: i-01da8eef32743b5de
     secretName: cloud-key-secret-aws
     volumeID: vol-0f1ecf66cb8d0328e
   ```

2. 使用 `kubectl` 创建实验。

   ```bash
   kubectl apply -f ./aws-detach-volume.yaml
   ```

### 字段说明

下表介绍以上 YAML 配置文件中的字段。

| 参数 | 类型 | 说明 | 默认值 | 是否必填 |
| :--- | :--- | :--- | :--- | :--- |
| action | string | 指定要注入的故障类型，仅支持 `ec2-stop`、`ec2-restart` 和 `detach-volume`。 | `ec2-stop` | 是 |
| mode | string | 指定实验的运行方式，可选项包括：`one`（表示随机选出一个符合条件的 Pod）、`all`（表示选出所有符合条件的 Pod）、`fixed`（表示选出指定数量且符合条件的 Pod）、`fixed-percent`（表示选出占符合条件的 Pod 中指定百分比的 Pod）和 `random-max-percent`（表示选出占符合条件的 Pod 中不超过指定百分比的 Pod）。| 无 | 是 |
| value | string | 取决于 `mode` 的配置，为 `mode` 提供对应的参数。例如，当你将 `mode` 配置为 `fixed-percent` 时，`value` 用于指定 Pod 的百分比。 | 无 | 否 |
| secretName | string | 指定存储 AWS 认证信息的 Kubernetes Secret 名字。 | 无 | 否 |
| awsRegion | string | 指定 AWS 区域。 | 无 | 是 | us-east-2 |
| ec2Instance | string | 指定 EC2 实例的 ID。 | 无 | 是 |
| volumeID | string | 当 `action` 为 `detach-volume` 时必填，指定 EBS 卷的 ID。 | 无 | 否 |
| deviceName | string | 当 `action` 为 `detach-volume` 时必填，指定设备名。 | 无 | 否 | 
| duration | string | 指定实验的持续时间。| 无 | 是 |
