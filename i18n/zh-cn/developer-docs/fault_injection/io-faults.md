---
title: 模拟 I/O 故障
description: 模拟 I/O 故障
sidebar_position: 7
sidebar_label: 模拟 I/O 故障
---

# 模拟 I/O 故障

IOChaos 实验能够模拟文件系统发生故障的场景。目前，I/OChaos 实验支持以下几种故障类型：

* Latency：延迟文件系统调用；
* Fault：使文件系统调用返回错误；
* AttrOverride：修改文件属性；
* Mistake：使文件读到或写入错误的值。

## 开始之前

* I/O 故障注入只能在 Linux 上执行。
* 实验结果需要进入容器内部查看，且要指定 volume 挂载路径。
* 建议只对 Write 和 Read 操作注入 I/O 故障。

## 使用 kbcli 模拟故障注入

下表介绍 I/O 故障类型的常见字段。

📎 Table 1. kbcli I/O 故障参数说明

| 参数                   | 说明               | 默认值 | 是否必填 |
| :----------------------- | :------------------------ | :------------ | :------- |
| `--volume-path` | 指定 volume 在目标容器内的挂载点，必须为挂载的根目录。 | 无 | 是 |
| `--path` | 指定注入故障的生效范围，可以是通配符，也可以是单个文件。| * | 否 |
| `--percent` | 指定每次操作发生故障的概率，单位为 %。 | 100 | 否 |
| `--container`, `-c` | 指定注入故障的容器名称。| 无 | 否 |
| `--method` | 指定 I/O 操作，支持 `read` 和 `write`。 | * | 否 |

### Latency

执行以下命令，向 `/data` 目录注入延迟故障，使该目录下的所有文件内容产生 10 秒延迟。即，延迟 Read 操作。

`--delay` 指定具体的延迟时长，必填。

```bash
kbcli fault io latency --delay=10s --volume-path=/data
```

### Fault

常见的错误号：

* 1: Operation not permitted
* 2: No such file or directory
* 5: I/O error
* 6: No such device or address
* 12: Out of memory
* 16: Device or resource busy
* 17: File exists
* 20: Not a directory
* 22: Invalid argument
* 24: Too many open files
* 28: No space left on device

点击参考[完整的错误编号列表](https://raw.githubusercontent.com/torvalds/linux/master/include/uapi/asm-generic/errno-base.h)。

执行以下命令，向 `/data` 目录注入文件错误故障，使该目录下的所有文件系统 100% 出现故障，并返回错误码 22（invalid argument）。

`--errno` 指定系统应该返回的错误号，必填。

```bash
kbcli fault io errno --volume-path=/data --errno=22
```

### Attribute override

执行以下命令，向目录 `/data` 注入 attrOverride 故障，使得该目录下的所有文件系统操作 100% 将目标文件的权限更改为 72（即八进制中的 110）。这将导致文件只能由所有者和其所在的组执行，无权进行其他操作。

```bash
kbcli fault io attribute --volume-path=/data --perm=72
```

你可以使用以下参数来修改相关属性。

📎 Table 2. kbcli AttrOverride 参数说明

| 参数                   | 说明               | 默认值 | 是否必填 |
| :----------------------- | :------------------------ | :------------ | :------- |
| `--blocks` | 文件占用块数 | 无 | 否 |
| `--ino` | ino 号 | 无 | 否 |
| `--nlink` | 硬链接数量 | 无 | 否 |
| `--perm` | 文件权限的十进制表示 | 无 | 否 |
| `--size` |文件大小 | 无 | 否 |
| `--uid` | 所有者的用户 ID | 无 | 否 |
| `--gid` | 所有者的组 ID | 无 | 否 |

### Mistake

执行以下命令，向目录 `/data` 注入读写错误故障，使该目录下的读写操作有 10% 的概率发生错误。在此过程中，将随机选择一个最大长度为 10 个字节的位置，并将其替换为 0 字节。

```bash
kbcli fault io mistake --volume-path=/data --filling=zero --max-occurrences=10 --max-length=1
```

📎 Table 3. kbcli Mistake 参数说明

| 参数                   | 说明               | 默认值 | 是否必填 |
| :----------------------- | :------------------------ | :------------ | :------- |
| `--filling` | 错误数据的填充内容，只能为 zero（填充 0）或 random（填充随机字节）。 | 无 | 是 |
| `max-occurrences` | 错误在每一次操作中最多出现次数。 | 无 | 是 |
| `--max-length` | 每次错误的最大长度（单位为字节）。 | 无 |  是 |

:::warning

不建议在除了 READ 和 WRITE 之外的文件系统调用上使用 mistake 错误。这可能会导致文件系统损坏、程序崩溃等后果。

:::

## 使用 YAML 文件模拟故障注入

本节介绍如何使用 YAML 文件模拟故障注入。你可以在上述 kbcli 命令的末尾添加 `--dry-run` 命令来查看 YAML 文件，还可以参考 [Chaos Mesh 官方文档](https://chaos-mesh.org/zh/docs/next/simulate-io-chaos-on-kubernetes/#使用-yaml-文件创建实验)获取更详细的信息。

### Fault latency 示例

1. 将实验配置写入到 `fault-latency.yaml` 文件中。

    在下例中，Chaos Mesh 将向 `/data` 目录注入延迟故障，使该目录下的所有文件产生 10 秒延迟。即，延迟读取操作。

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: IOChaos
    metadata:
      creationTimestamp: null
      generateName: io-chaos-
      namespace: default
    spec:
      action: latency
      delay: 10s
      duration: 10s
      mode: all
      percent: 100
      selector:
        namespaces:
        - default
      volumePath: /data
    ```

2. 使用 `kubectl` 创建实验。

   ```bash
   kubectl apply -f ./fault-latency.yaml
   ```

### Fault fault 示例

1. 将实验配置写入到 `fault-fault.yaml` 文件中。

    在下例中，Chaos Mesh 将向 `/data` 目录注入文件错误故障，使该目录下的所有文件系统 100% 出现故障，并返回错误码 22（invalid argument）。

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: IOChaos
    metadata:
      creationTimestamp: null
      generateName: io-chaos-
      namespace: default
    spec:
      action: fault
      duration: 10s
      errno: 22
      mode: all
      percent: 100
      selector:
        namespaces:
        - default
      volumePath: /data
    ```

2. 使用 `kubectl` 创建实验。

   ```bash
   kubectl apply -f ./fault-fault.yaml
   ```

### Fault attrOverride 示例

1. 将实验配置写入到 `fault-attrOverride.yaml` 文件中。

    在下例中，Chaos Mesh 将向目录 `/data` 注入 attrOverride 故障，使得该目录下的所有文件系统操作 100% 将目标文件的权限更改为 72（即八进制中的 110）。这将导致文件只能由所有者和其所在的组执行，无权进行其他操作。

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: IOChaos
    metadata:
      creationTimestamp: null
      generateName: io-chaos-
      namespace: default
    spec:
      action: attrOverride
      attr:
        perm: 72
      duration: 10s
      mode: all
      percent: 100
      selector:
        namespaces:
        - default
      volumePath: /data
    ```

2. 使用 `kubectl` 创建实验。

   ```bash
   kubectl apply -f ./fault-attrOverride.yaml
   ```

### Fault mistake 示例

1. 将实验配置写入到 `fault-mistake.yaml` 文件中。

    在下例中，Chaos Mesh 将向目录 `/data` 注入读写错误故障，使该目录下的读写操作有 10% 的概率发生错误。在此过程中，将随机选择一个最大长度为 10 个字节的位置，将其替换为 0 字节。

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: IOChaos
    metadata:
      creationTimestamp: null
      generateName: io-chaos-
      namespace: default
    spec:
      action: mistake
      duration: 10s
      mistake:
        filling: zero
        maxLength: 1
        maxOccurrences: 10
      mode: all
      percent: 100
      selector:
        namespaces:
        - default
      volumePath: /data
    ```

2. 使用 `kubectl` 创建实验。

   ```bash
   kubectl apply -f ./fault-mistake.yaml
   ```
