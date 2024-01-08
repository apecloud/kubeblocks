---
title: 模拟时间故障
description: 模拟时间故障
sidebar_position: 9
sidebar_label: 模拟时间故障
---

# 模拟时间故障

TimeChaos 实验用于模拟时间发生偏移的场景。本文档将介绍如何创建 TimeChaos 实验。

:::note

TimeChaos 只影响容器中 PID 命名空间的 PID `1` 进程及其子进程，通过 `kubectl exec` 启动的进程不受其影响。

:::

| 参数                   | 缩写 | 说明               | 默认值 | 是否必填 |
| :----------------------- | :------- | :------------------------ | :------------ | :------- |
| `--time-offset` | 无 | 指定时间偏移的长度。 | 无 | 是 |
| `--clock-id` | 无 | 指定时间偏移作用的时钟，详见 [clock_gettime 文档](https://man7.org/linux/man-pages/man2/clock_gettime.2.html)。| CLOCK_REALTIME | 否 |
| `--container` | -c | 指定注入故障的容器名称。 | 无 | 否 |

## 使用 kbcli 模拟故障注入

执行以下命令，向指定的 Pod 中注入时间故障，将该 Pod 中进程的时间向前推移 5 秒。一旦时间故障注入，该 Pod 就会处于不可用状态，并立即重启。

```bash
kbcli fault time --time-offset=-5s
```

## 使用 YAML 文件模拟故障注入

本节介绍如何使用 YAML 文件模拟故障注入。你可以参考 [Chaos Mesh 官方文档](https://chaos-mesh.org/zh/docs/next/simulate-time-chaos-on-kubernetes/#使用-yaml-方式创建实验) 获取更详细的信息。

1. 将实验配置写入到 `time.yaml` 文件中。

    在下例中，Chaos Mesh 向指定的 Pod 中注入时间故障，将该 Pod 中进程的时间向前推移 5 秒。一旦时间故障注入，该 Pod 就会处于不可用状态，并立即重启。

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: TimeChaos
    metadata:
      creationTimestamp: null
      generateName: time-chaos-
      namespace: default
    spec:
      duration: 10s
      mode: all
      selector:
        namespaces:
        - default
      timeOffset: -5s
    ```

2. 使用 `kubectl` 创建实验。

   ```bash
   kubectl apply -f ./time.yaml
   ```

### 字段说明

下表介绍以上 YAML 配置文件中的字段。

| 参数 | 类型 | 说明 | 默认值 | 是否必填 |
| :--- | :--- | :--- | :--- | :--- |
| timeOffset | string | 指定时间偏移的长度。| 无 | 是 | 
| clockIds | []string | 指定时间偏移作用的时钟，详见 [clock_gettime 文档](https://man7.org/linux/man-pages/man2/clock_gettime.2.html)。 | `["CLOCK_REALTIME"]` | 否 |
| mode | string | 指定实验的运行方式，可选项包括：`one`（表示随机选出一个符合条件的 Pod）、`all`（表示选出所有符合条件的 Pod）、`fixed`（表示选出指定数量且符合条件的 Pod）、`fixed-percent`（表示选出占符合条件的 Pod 中指定百分比的 Pod）和 `random-max-percent`（表示选出占符合条件的 Pod 中不超过指定百分比的 Pod）。 | 无 | 是 |
| value | string | 取决于 `mode` 的配置，为 `mode` 提供对应的参数。例如，当你将 `mode` 配置为 `fixed-percent` `时，value` 用于指定 Pod 的百分比。 | 无 | 否 |
| containerNames | []string | 指定注入故障的容器名称。 | 无 | 否 |
| selector | struct | 指定目标 Pod。 | 无 | 是 |
