---
title: 模拟压力场景
description: 模拟压力场景
sidebar_position: 8
sidebar_label: 模拟压力场景
---

# 模拟压力场景

StressChaos 实验用于模拟容器内压力的场景。本文档介绍如何创建 StressChaos 实验。

| 参数                   | 说明               | 默认值 | 是否必填 |
| :----------------------- | :------------------------ | :------------ | :------- |
| `--cpu-worker` | 指定施加 CPU 压力的线程个数。必须指定 `--cpu-worker` 或 `--memory-worker` 中的一个。| 无 | 否 |
| `--cpu-load` | 指定占据 CPU 的百分比。`0` 表示没有增加额外负载，`100` 表示满负载。总负载为 `workers * load`。 | 20 | 否 |
| `--memory-worker` | 指定施加内存压力的线程个数。必须指定 `--cpu-worker` 或 `--memory-worker` 中的一个。 | 无 | 否 |
| `--memory-size` | 指定分配内存的大小或是占总内存的百分比，分配内存的总和为 `size`。| 无 | 否 |
| `--container` | 指定容器名称，可以用于指定多个容器。如果未指定，则默认为 Pod 中的第一个容器。| 无 | 否 |

## 使用 kbcli 模拟故障注入

执行以下命令，在默认命名空间的所有 Pod 的第一个容器中创建进程，并持续进行 CPU 和内存的分配、读取和写入，最多占用 100MB 的内存，持续 10 秒钟。在此过程中，有 2 个线程施加 CPU 压力，1 个线程施加内存压力。

```bash
kbcli fault stress --cpu-worker=2 --cpu-load=50 --memory-worker=1 --memory-size=100Mi
```

## 使用 YAML 文件模拟故障注入

本节介绍如何使用 YAML 文件模拟故障注入。你可以在上述 kbcli 命令的末尾添加 --dry-run 命令来查看 YAML 文件，还可以参考 [Chaos Mesh 官方文档](https://chaos-mesh.org/zh/docs/next/simulate-heavy-stress-on-kubernetes/#使用-yaml-方式创建实验)获取更详细的信息。

### 压力示例

1. 将实验配置写入到 `stress.yaml` 文件中。

    在下例中，Chaos Mesh 在默认命名空间的所有 Pod 的第一个容器中创建了一个进程，并持续进行 CPU 和内存的分配、读取和写入，最多占用 100MB 的内存，持续 10 秒钟。在此过程中，有 2 个线程施加 CPU 压力，1 个线程施加内存压力。

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: StressChaos
    metadata:
      creationTimestamp: null
      generateName: stress-chaos-
      namespace: default
    spec:
      duration: 10s
      mode: all
      selector:
        namespaces:
        - default
      stressors:
        cpu:
          load: 50
          workers: 2
        memory:
          size: 100Mi
          workers: 1
    ```

2. 使用 `kubectl` 创建实验。

   ```bash
   kubectl apply -f ./stress.yaml
   ```
