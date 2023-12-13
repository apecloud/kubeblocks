---
title: 配置集群参数
description: 如何配置集群参数
keywords: [pulsar, 参数, 配置, 再配置]
sidebar_position: 1
---

# 配置集群参数

从 v0.6.0 版本开始，KubeBlocks 支持使用 `kbcli cluster configure` 和 `kbcli cluster edit-config` 两种方式来配置参数。它们的区别在于，`kbcli cluster configure `可以自动配置参数，而 `kbcli cluster edit-config` 则允许以可视化的方式直接编辑参数。

一共有 3 类参数：

1. 环境参数，比如 GC 相关的参数，`PULSAR_MEM` 和 `PULSAR_GC`。参数变更对每个组件都适用。
2. 配置参数，比如 `zookeeper` 或 `bookies.conf` 配置文件。可以通过 `env` 的方式做变更，变更会重启 pod。
3. 动态参数，比如 `brokers.conf` 中的配置文件。Pulsar 的 `broker` 支持两种变更模式：
     1. 一种是需要重启的参数变更，比如 `zookeeperSessionExpiredPolicy`。
     2. 另外一种是支持 dynamic 的参数，可以通过 `pulsar-admin brokers list-dynamic-config` 获取所有的动态参数列表。

:::note

`pulsar-admin `是 Pulsar 集群自带的管理工具，可以通过 `kubectl exec -it <pod-name> -- bash` 登录到对应的 Pod 中（pod-name 可通过 `kubectl get pods` 获取，选择名字中带有 `broker` 字样的 Pod 即可）。在 Pod 中的 `/pulsar/bin path` 路径下有对应的命令。关于 pulsar-admin 的更多信息，可参考[官方文档](https://pulsar.apache.org/docs/3.0.x/admin-api-tools/)。

:::

## 查看参数信息

* 查看集群的当前配置文件。

   ```bash
   kbcli cluster describe-config pulsar  
   ```

* 查看当前配置文件的详细信息。

  ```bash
  kbcli cluster describe-config pulsar --show-detail
  ```

* 查看参数描述。

  ```bash
  kbcli cluster explain-config pulsar | head -n 20
  ```

## 配置参数

### 使用 configure 命令配置参数

#### 配置环境参数

***步骤：***

1. 获取 Pulsar 集群组件名称，配置参数。

   ```bash
   kbcli cluster list-components pulsar 
   ```

   ***示例***

   ```bash
   kbcli cluster list-components pulsar 

   NAME               NAMESPACE   CLUSTER   TYPE               IMAGE
   proxy              default     pulsar    pulsar-proxy       docker.io/apecloud/pulsar:2.11.2
   broker             default     pulsar    pulsar-broker      docker.io/apecloud/pulsar:2.11.2
   bookies-recovery   default     pulsar    bookies-recovery   docker.io/apecloud/pulsar:2.11.2
   bookies            default     pulsar    bookies            docker.io/apecloud/pulsar:2.11.2
   zookeeper          default     pulsar    zookeeper          docker.io/apecloud/pulsar:2.11.2
   ```

2. 配置参数。

   下面以 `zookeeper` 为例。

   ```bash
   kbcli cluster configure pulsar --component=zookeeper --set PULSAR_MEM="-XX:MinRAMPercentage=50 -XX:MaxRAMPercentage=70" 
   ```

3. 验证配置。

   a. 检查配置进度。

   ```bash
   kubectl get ops 
   ```

   b. 检查配置是否完成。

   ```bash
   kubectl get pod -l app.kubernetes.io/name=pulsar
   ```

#### 配置其他参数

下面以配置动态参数 `brokerShutdownTimeoutMs` 为例。

***步骤：***

1. 获取配置信息。

   ```bash
   kbcli cluster desc-config pulsar --component=broker
   
   ConfigSpecs Meta:
   CONFIG-SPEC-NAME         FILE                   ENABLED   TEMPLATE                   CONSTRAINT                   RENDERED                               COMPONENT   CLUSTER
   agamotto-configuration   agamotto-config.yaml   false     pulsar-agamotto-conf-tpl                                pulsar-broker-agamotto-configuration   broker      pulsar
   broker-env               conf                   true      pulsar-broker-env-tpl      pulsar-env-constraints       pulsar-broker-broker-env               broker      pulsar
   broker-config            broker.conf            true      pulsar-broker-config-tpl   brokers-config-constraints   pulsar-broker-broker-config            broker      pulsar
   ```

2. 配置参数。

   ```bash
   kbcli cluster configure pulsar --component=broker --config-spec=broker-config --set brokerShutdownTimeoutMs=66600
   >
   Will updated configure file meta:
     ConfigSpec: broker-config          ConfigFile: broker.conf        ComponentName: broker        ClusterName: pulsar
   OpsRequest pulsar-reconfiguring-qxw8s created successfully, you can view the progress:
           kbcli cluster describe-ops pulsar-reconfiguring-qxw8s -n default
   ```

3. 检查配置进度。

   使用上述命令打印的 ops name 进行检查。

   ```bash
   kbcli cluster describe-ops pulsar-reconfiguring-qxw8s -n default
   >
   Spec:
     Name: pulsar-reconfiguring-qxw8s        NameSpace: default        Cluster: pulsar        Type: Reconfiguring

   Command:
     kbcli cluster configure pulsar --components=broker --config-spec=broker-config --config-file=broker.conf --set brokerShutdownTimeoutMs=66600 --namespace=default

   Status:
     Start Time:         Jul 20,2023 09:53 UTC+0800
     Completion Time:    Jul 20,2023 09:53 UTC+0800
     Duration:           1s
     Status:             Succeed
     Progress:           2/2
                         OBJECT-KEY   STATUS   DURATION   MESSAGE
   ```

### 使用 edit-config 命令配置参数

KubeBlocks 提供了一个名为 `edit-config` 的工具，帮助以可视化的方式配置参数。

Linux 和 macOS 系统可以使用 vi 编辑器编辑配置文件，Windows 系统可以使用 notepad。

1. 编辑配置文件。

   ```bash
   kbcli cluster edit-config pulsar
   ```

   :::note

   如果集群中有多个组件，请使用 `--component` 参数指定一个组件。

   :::

2. 查看参数配置状态。

   ```bash
   kbcli cluster describe-ops xxx -n default
   ```

3. 连接到数据库，验证参数是否按预期配置。

   ```bash
   kbcli cluster connect pulsar
   ```

   :::note

   1. `edit-config` 不能同时编辑静态参数和动态参数。
   2. KubeBlocks 未来将支持删除参数。

   :::

### 使用 kubectl 配置参数

使用 kubectl 配置 Pulsar 集群时，需要修改配置文件。

***步骤：***

1. 获取包含配置文件的 configMap。下面以 `broker` 组件为例。
   
    ```bash
    kbcli cluster desc-config pulsar --component=broker

    ConfigSpecs Meta:
    CONFIG-SPEC-NAME         FILE                   ENABLED   TEMPLATE                   CONSTRAINT                   RENDERED                               COMPONENT   CLUSTER
    agamotto-configuration   agamotto-config.yaml   false     pulsar-agamotto-conf-tpl                                pulsar-broker-agamotto-configuration   broker      pulsar
    broker-env               conf                   true      pulsar-broker-env-tpl      pulsar-env-constraints       pulsar-broker-broker-env               broker      pulsar
    broker-config            broker.conf            true      pulsar-broker-config-tpl   brokers-config-constraints   pulsar-broker-broker-config            broker      pulsar
    ```

    在上述输出的 RENDERED 列中，可以看到 broker 的 configMap 名称为 `pulsar-broker-broker-config`。

2. 修改 `broker.conf` 文件。在本例中，就是 `pulsar-broker-broker-config`。
   
    ```bash
    kubectl edit cm pulsar-broker-broker-config
    ```
