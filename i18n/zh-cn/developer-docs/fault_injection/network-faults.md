---
title: 模拟网络故障
description: 模拟网络故障
sidebar_position: 4
sidebar_label: 模拟网络故障
---

# 模拟网络故障

网络故障包括 Partition、Net Emulation（包括丢包、延迟、重复和损坏）以及 Bandwidth 几种类型。

* Partition：网络断开或分区；
* Net emulation：模拟网络质量较差的情况，如高延迟、高丢包率、包乱序等；
* Bandwidth：限制节点之间通信的带宽。

## 开始之前

* 在进行网络注入的过程中，请保证 Controller Manager 与 Chaos Daemon 之间连接通畅，否则将无法恢复。
* 如果使用 Net Emulation 功能，请确保 Linux 内核中已安装 `NET_SCH_NETEM` 模块。如果使用的是 CentOS，可以通过 kernel-modules-extra 包安装，大部分其他 Linux 发行版已默认安装相应模块。

## 使用 kbcli 模拟故障注入

下表介绍所有网络故障类型的常见字段。

📎 Table 1. kbcli 网络故障参数说明

| 参数                   | 说明               | 默认值 | 是否必填 |
| :----------------------- | :------------------------ | :------------ | :------- |
| `pod name`  | 指定注入故障的 Pod 名称。例如，<br /> 在命令中添加 Pod 名称 `mysql-cluster-mysql-0`，完整命令为  `kbcli fault pod kill mysql-cluster-mysql-0`。  | 默认 | 否 |
| `--direction` | 指示目标数据包的方向。可用值包括 `from`（从目标发出的数据包）、`to`（发送到目标的数据包）和 `both`（全部选中）。 | `to` | 否 |
| `-e`,`--external-target` |指示 Kubernetes 外部的网络目标，可以是 IPv4 地址或域名。该参数仅在 `direction: to` 时有效。 | 无 | 否 |
| `--target-mode` | 指定目标的模式。如果指定了目标，则需一起指定 `target-mode`。可选项包括：`one`（表示随机选出一个符合条件的 Pod）、`all`（表示选出所有符合条件的 Pod）、`fixed`（表示选出指定数量且符合条件的 Pod）、`fixed-percent`（表示选出占符合条件的 Pod 中指定百分比的 Pod）、`random-max-percent`（表示选出占符合条件的 Pod 中不超过指定百分比的 Pod）。 | 无 | 否 |
| `--target-value` | 指定目标的值。| 无 | 否 |
| `--target-label` | 指定目标的标签。 | 无 | 否 |
| `--duration` | 指定分区的持续时间。 | 无 | 否 |
| `--target-ns-fault` | 指定目标的命名空间。| 无 | 否 |

### Partition

执行以下命令，将 `network-partition` 注入到 Pod 中，使 Pod `mycluster-mysql-1` 与 Kubernetes 的内外部网络分离。

```bash
kbcli fault network partition mycluster-mysql-1
```

### Net emulation

Net Emulation 模拟网络质量较差的情况，如高延迟、高丢包率、包乱序等。

#### 丢包

执行以下命令，将 `network-loss` 注入到 Pod `mycluster-mysql-1` 中，使得其与外界通信丢包率为 50%。

```bash
kbcli fault network loss mycluster-mysql-1 -e=kubeblocks.io --loss=50
```

📎 Table 2. kbcli 网络丢包故障参数说明

| 参数                   | 说明               | 默认值 | 是否必填 |
| :----------------------- | :------------------------ | :------------ | :------- |
| `--loss` | 表示丢包发生的概率。 | 无 | 是 |
| `-c`, `--correlation` | 表示丢包发生的概率与前一次是否发生的相关性。取值范围：[0, 100]。 | 无 | 否 |

#### 延迟

执行以下命令，将 `network-delay` 注入到 Pod `mycluster-mysql-1` 中，使指定 Pod 的网络连接延迟 15 秒。

```bash
kbcli fault network delay mycluster-mysql-1 --latency=15s -c=100 --jitter=0ms
```

📎 Table 3. kbcli 网络延迟故障参数说明

| 参数                   | 说明               | 默认值 | 是否必填 |
| :----------------------- | :------------------------ | :------------ | :------- |
| `--latency` | 表示延迟的时间长度。         | 无          | 是      |
| `--jitter` | 表示延迟时间的变化范围。  | 0 ms          | 否       |
| `-c`, `--correlation` | 表示延迟时间的时间长度与前一次延迟时长的相关性。取值范围：[0, 100]。| 无 | 否 |

#### 包重复

执行以下命令，向指定的 Pod 注入包重复的混乱，持续时间为 1 分钟，重复率为 50%。

`--duplicate` 指定了包重复包的比率，取值范围为 [0,100]。

```bash
kbcli fault network duplicate mysql-cluster-mysql-1 --duplicate=50
```

📎 Table 4. kbcli 包重复故障参数说明

| 参数                   | 说明               | 默认值 | 是否必填 |
| :----------------------- | :------------------------ | :------------ | :------- |
| `--duplicate`         | 表示包重复发生的概率。取值范围：[0, 100]。 | 无 | 是 |
| `-c`, `--correlation` | 表示包重复发生的概率与前一次是否发生的相关性。取值范围：[0, 100]。 | 无 | 否 |

#### 包损坏

执行以下命令，向指定的 Pod 注入包损坏的混乱，持续时间为 1 分钟，包损坏率为 50%。

```bash
kbcli fault network corrupt mycluster-mysql-1 --corrupt=50 --correlation=100 --duration=1m
```

### Bandwidth

执行以下命令，设置指定 Pod 与外部环境之间的带宽为 1 Kbps，持续时间为 1 分钟。

```bash
kbcli fault network bandwidth mycluster-mysql-1 --rate=1kbps --duration=1m
```

📎 Table 4. kbcli Bandwidth 故障参数说明

| 参数                   | 说明               | 默认值 | 是否必填 |
| :----------------------- | :------------------------ | :------------ | :------- |
| `--rate` | 表示带宽限制的速率。 | 无 | 是 |
| `--limit` | 表示在队列中等待的字节数。 | 1 | 否 |
| `--buffer` | 表示能够瞬间发送的最大字节数。| 1 | 否 |
| `--prakrate` | 表示 `bucket` 的最大消耗率。 | 0 | 否 |
| `--minburst` | 表示 `peakrate bucket` 的大小。 | 0 | 否 |

## 使用 YAML 文件模拟故障注入

本节介绍如何使用 YAML 文件模拟故障注入。你可以在上述 kbcli 命令的末尾添加 `--dry-run` 命令来查看 YAML 文件，还可以参考 [Chaos Mesh 官方文档](https://chaos-mesh.org/zh/docs/next/simulate-network-chaos-on-kubernetes/#使用-yaml-方式创建实验)获取更详细的信息。

### Partition 示例

1. 将实验配置写入到 `network-partition.yaml` 文件中。

    在以下示例中，Chaos Mesh 将 `network-partition` 注入到 Pod 中，使 Pod `mycluster-mysql-1` 与 Kubernetes 的内外部网络分离。

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: NetworkChaos
    metadata:
      creationTimestamp: null
      generateName: network-chaos-
      namespace: default
    spec:
      action: partition
      direction: to
      duration: 10s
      mode: all
      selector:
        namespaces:
        - default
        pods:
          default:
          - mycluster-mysql-1
    ```

2. 使用 `kubectl` 创建实验。

   ```bash
   kubectl apply -f ./network-partition.yaml
   ```

### Net emulation 示例

#### 丢包示例

1. 将实验配置写入到 `network-loss.yaml` 文件中。

    在以下示例中，Chaos Mesh 将 `network-loss` 注入到 Pod `mycluster-mysql-1` 中，使得其与外界通信丢包率为 50%。

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: NetworkChaos
    metadata:
      creationTimestamp: null
      generateName: network-chaos-
      namespace: default
    spec:
      action: loss
      direction: to
      duration: 10s
      externalTargets:
      - kubeblocks.io
      loss:
        loss: "50"
      mode: all
      selector:
        namespaces:
        - default
        pods:
          default:
          - mycluster-mysql-1
    ```

2. 使用 `kubectl` 创建实验。

   ```bash
   kubectl apply -f ./network-loss.yaml
   ```

#### 延迟示例

1. 将实验配置写入到 `network-delay.yaml` 文件中。

    在以下示例中，Chaos Mesh 将 `network-delay` 注入到 Pod `mycluster-mysql-1` 中，使指定 Pod 的网络连接延迟 15 秒。

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: NetworkChaos
    metadata:
      creationTimestamp: null
      generateName: network-chaos-
      namespace: default
    spec:
      action: delay
      delay:
        correlation: "100"
        jitter: 0ms
        latency: 15s
      direction: to
      duration: 10s
      mode: all
      selector:
        namespaces:
        - default
        pods:
          default:
          - mycluster-mysql-1
    ```

2. 使用 `kubectl` 创建实验。

   ```bash
   kubectl apply -f ./network-delay.yaml
   ```

#### 包重复示例

1. 将实验配置写入到 `network-duplicate.yaml` 文件中。

    在以下示例中，Chaos Mesh 向指定的 Pod 注入包重复的混乱，持续时间为 1 分钟，重复率为 50%。

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: NetworkChaos
    metadata:
      creationTimestamp: null
      generateName: network-chaos-
      namespace: default
    spec:
      action: duplicate
      direction: to
      duplicate:
        duplicate: "50"
      duration: 10s
      mode: all
      selector:
        namespaces:
        - default
        pods:
          default:
          - mysql-cluster-mysql-1
    ```

2. 使用 `kubectl` 创建实验。

   ```bash
   kubectl apply -f ./network-duplicate.yaml
   ```

#### 包损坏示例

1. 将实验配置写入到 `network-corrupt.yaml` 文件中。

    在以下示例中，Chaos Mesh 向指定的 Pod 注入包损坏的混乱，持续时间为 1 分钟，包损坏率为 50%。

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: NetworkChaos
    metadata:
      creationTimestamp: null
      generateName: network-chaos-
      namespace: default
    spec:
      action: corrupt
      corrupt:
        correlation: "100"
        corrupt: "50"
      direction: to
      duration: 1m
      mode: all
      selector:
        namespaces:
        - default
        pods:
        default:
        - mycluster-mysql-1
    ```

2. 使用 `kubectl` 创建实验。

   ```bash
   kubectl apply -f ./network-corrupt.yaml
   ```

### Bandwidth 示例

1. 将实验配置写入到 `network-bandwidth.yaml` 文件中。

    在以下示例中，Chaos Mesh 设置指定 Pod 与外部环境之间的带宽为 1 Kbps，持续时间为 1 分钟。

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: NetworkChaos
    metadata:
      creationTimestamp: null
      generateName: network-chaos-
      namespace: default
    spec:
      action: bandwidth
      bandwidth:
        buffer: 1
        limit: 1
        rate: 1kbps
      direction: to
      duration: 1m
      mode: all
      selector:
        namespaces:
        - default
        pods:
          default:
          - mycluster-mysql-1
    ```

2. 使用 `kubectl` 创建实验。

   ```bash
   kubectl apply -f ./network-bandwidth.yaml
   ```

### 字段说明

下表介绍以上 YAML 配置文件中的字段。

| 参数 | 类型  | 说明 | 默认值 | 是否必填 | 示例 |
| :---      | :---  | :---        | :---          | :---     | :---    |
| action | string | 指定故障类型。如 `partition`、`loss`、`delay`、`duplicate`、`corrupt` 和 `bandwidth`。| 无 | 是 | `bandwidth` |
| duration | string | 指定实验的持续时间。 | 无 | 是 | 10s |
| mode | string | 指定实验的运行方式，可选项包括：`one`（表示随机选出一个符合条件的 Pod）、`all`（表示选出所有符合条件的 Pod）、`fixed`（表示选出指定数量且符合条件的 Pod）、`fixed-percent`（表示选出占符合条件的 Pod 中指定百分比的 Pod）和 `random-max-percent`（表示选出占符合条件的 Pod 中不超过指定百分比的 Pod）。 | 无 | 是 | `fixed-percent` |
| value | string | 取决于 `mode` 的配置，为 `mode` 提供对应的参数。例如，当你将 `mode` 配置为 `fixed-percent` `时，value` 用于指定 Pod 的百分比。 | 无 | 否 | 50 |
| selector | struct | 通过定义节点和标签来指定目标 Pod。| 无 | 是 <br /> 如果未指定，系统将终止默认命名空间下的所有 Pod。|
| duration | string | 指定实验的持续时间。 | 无 | 是 | 30s |
