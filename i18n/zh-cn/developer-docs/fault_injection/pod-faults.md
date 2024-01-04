---
title: 模拟 Pod 故障
description: 模拟 Pod 故障
sidebar_position: 3
sidebar_label: 模拟 Pod 故障
---

# 模拟 Pod 故障

Pod 故障包括 Pod failure、Pod kill 和 Container kill。

- Pod failure：向指定的 Pod 注入故障，使得该 Pod 在一段时间内不可用；
- Pod kill：杀死指定的 Pod。为确保 Pod 能够成功重启，需要配置 ReplicaSet 或类似的机制；
- Container kill：杀死目标 Pod 中的指定容器。

## 使用限制

无论 Pod 是否绑定至 Deployment、StatefulSet、DaemonSet 或其他控制器，Chaos Mesh 都可以向任一 Pod 注入 PodChaos。当向独立的 Pod 注入 PodChaos 时，可能会发生不同的情况。比如，向独立的 Pod 注入 `pod-kill` 故障时，无法保证应用程序在故障发生后能够恢复正常。

## 开始之前

- 确保在目标 Pod 上没有运行 Chaos Mesh 的控制管理器。
- 如果故障类型是 `pod-kill`，请配置 ReplicaSet 或类似机制，确保 Pod 能够自动重启。

## 使用 kbcli 模拟故障注入

下表介绍所有 Pod 故障类型的常见字段。

📎 Table 1. Pod 故障参数说明

| 参数                  | 说明              | 默认值  | 是否必填 |
| :-----------------------| :------------------------| :------------ | :------- |
| `pod name`  | 指定注入故障的 Pod 名称。例如，<br /> 在命令中添加 Pod 名称 `mysql-cluster-mysql-0`，完整命令为  `kbcli fault pod kill mysql-cluster-mysql-0`。 | 默认 | 否 |
| `--namespace` | 指定创建 Chaos 的命名空间。 | 当前命名空间 | 否 |
| `--ns-fault` | 指定一个命名空间，使该命名空间中的所有 Pod 都无法使用。例如，<br /> `kbcli fault pod kill --ns-fault=kb-system`。 | 默认 | 否 |
| `--node`   | 指定一个节点，使该节点上的所有 Pod 都无法使用。例如，<br /> `kbcli fault pod kill --node=minikube-m02`。 | 无 | 否 |
| `--label`  | 指定一个标签，使默认命名空间中具有该标签的 Pod 无法使用。例如，<br /> `kbcli fault pod kill --label=app.kubernetes.io/component=mysql`。 | 无 | 否 |
| `--node-label` | 指定一个节点标签，使具有该节点标签的节点上的所有 Pod 都无法使用。例如，<br /> `kbcli fault pod kill --node-label=kubernetes.io/arch=arm64`。 | 无 | 否 |
| `--mode` | 指定实验的运行方式，可选择项包括：`one`（表示随机选出一个符合条件的 Pod）、`all`（表示选出所有符合条件的 Pod）、`fixed`（表示选出指定数量且符合条件的 Pod）、`fixed-percent`（表示选出占符合条件的 Pod 中指定百分比的 Pod）、`random-max-percent`（表示选出占符合条件的 Pod 中不超过指定百分比的 Pod）。 | `all` | 否 |
| `--value` | 取决于 `mode` 的配置，为 `mode` 提供对应的参数。例如，当你将 `mode` 配置为 `fixed-percent` 时，`value` 用于指定 Pod 的百分比。例如，<br />  `kbcli fault pod kill --mode=fixed-percent --value=50`。 | 无 | 否 |
| `--duration` | 指定实验的持续时间。 | 10s | 否 |

### Pod kill

执行以下命令，将 `pod-kill` 注入到默认命名空间中的所有 Pod 中，使这些 Pod 被杀死。

```bash
kbcli fault pod kill
```

### Pod failure

执行以下命令，将 `pod-failure` 注入到默认命名空间中的所有 Pod 中，并使这些 Pod 在 10 秒内不可用。

```bash
kbcli fault pod failure --duration=10s
```

### Container kill

执行以下命令，将 `container-kill` 注入到默认命名空间中所有 Pod 的容器中，并使这些容器被杀死。注意，`--container` 是必需的。

```bash
kbcli fault pod kill-container --container=mysql
```

你还可以添加多个容器。例如，在默认命名空间中杀死 `mysql` 和 `config-manager` 容器。

```bash
kbcli fault pod kill-container --container=mysql --container=config-manager
```

## 使用 YAML 文件模拟故障注入

本节介绍如何使用 YAML 文件模拟故障注入。你可以在上述 kbcli 命令的末尾添加 `--dry-run` 命令来查看 YAML 文件，还可以参考 [Chaos Mesh 官方文档](https://chaos-mesh.org/zh/docs/next/simulate-pod-chaos-on-kubernetes/#使用-yaml-配置文件创建实验)获取更详细的信息。

### Pod-kill 示例

1. 将实验配置写入到 `pod-kill.yaml` 文件中。

    在下例中，Chaos Mesh 向指定的 Pod 中注入了 `pod-kill` 故障，使该 Pod 被杀死。

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: PodChaos
    metadata:
      creationTimestamp: null
      generateName: pod-chaos-
      namespace: default
    spec:
      action: pod-kill
      duration: 10s
      mode: fixed-percent
      selector:
        namespaces:
        - default
        labelSelectors:
        'app.kubernetes.io/component': 'mysql'
      value: "50"
    ```

2. 使用 `kubectl` 创建实验。

   ```bash
   kubectl apply -f ./pod-kill.yaml
   ```

### Pod-failure 示例

1. 将实验配置写入到 `pod-failure.yaml` 文件中。

    在下例中，Chaos Mesh 向指定的 Pod 中注入了 `pod-failure` 故障，使该 Pod 在 30 秒内不可用。

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: PodChaos
    metadata:
      creationTimestamp: null
      generateName: pod-chaos-
      namespace: default
    spec:
      action: pod-failure
      duration: 30s
      mode: fixed-percent
      selector:
        namespaces:
        - default
        labelSelectors:
        'app.kubernetes.io/component': 'mysql'
      value: "50"
    ```

2. 使用 `kubectl` 创建实验。

   ```bash
   kubectl apply -f ./pod-kill.yaml
   ```

### Container-kill 示例

1. 将实验配置写入到 `container-kill.yaml` 文件中。

    在下例中，Chaos Mesh 向指定的 Pod 中注入了 `container-kill` 故障，使该 Container 被杀死。

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: PodChaos
    metadata:
      creationTimestamp: null
      generateName: pod-chaos-
      namespace: default
    spec:
      action: container-kill
      duration: 10s
      mode: fixed-percent
      selector:
        namespaces:
        - default
        labelSelectors:
        'app.kubernetes.io/component': 'mysql'
      value: "50"
    ```

2. 使用 `kubectl` 创建实验。

   ```bash
   kubectl apply -f ./pod-kill.yaml
   ```

### 字段说明

下表介绍以上 YAML 配置文件中的字段。

| 参数 | 类型  | 说明 | 默认值 | 是否必填 | 示例 |
| :---      | :---  | :---        | :---          | :---     | :---    |
| action | string | 指定要注入的故障类型，仅支持 `pod-failure`、`pod-kill` 和 `container-kill`。 | 无 | 是 | `pod-kill` |
| duration | string | 指定实验的持续时间。 | 无 | 是 | 10s |
| mode | string | 指定实验的运行方式，可选项包括：`one`（表示随机选出一个符合条件的 Pod）、`all`（表示选出所有符合条件的 Pod）、`fixed`（表示选出指定数量且符合条件的 Pod）、`fixed-percent`（表示选出占符合条件的 Pod 中指定百分比的 Pod）和 `random-max-percent`（表示选出占符合条件的 Pod 中不超过指定百分比的 Pod）。 | 无 | 是 | `fixed-percent` |
| value | string | 取决于 `mode` 的配置，为 `mode` 提供对应的参数。例如，当你将 `mode` 配置为 `fixed-percent` `时，value` 用于指定 Pod 的百分比。 | 无 | 否 | 50 |
| selector | struct | 通过定义节点和标签来指定目标 Pod。| 无 | 是 <br /> 如果未指定，系统将终止默认命名空间下的所有 Pod。 |  |
| containerNames | string | 当你将 `action` 配置为 `container-kill` 时，此配置为必填，用于指定注入故障的目标 Container 名。 | 无 | 否 | mysql |
| gracePeriod | int64 | 当你将 `action` 配置为 `pod-kill` 时，此配置为必填，用于指定删除 Pod 之前的持续时间。 | 0 | 否 | 0 |
| duration | string | 指定实验的持续时间。 | 无 | 是 | 30s |
