---
title: 配置集群参数
description: 如何配置集群参数
keywords: [pulsar, 参数, 配置, 再配置]
sidebar_position: 1
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# 配置集群参数

从 v0.6.0 版本开始，KubeBlocks 支持使用 `kbcli cluster configure` 和 `kbcli cluster edit-config` 两种方式来配置参数。它们的区别在于，`kbcli cluster configure ` 可以自动配置参数，而 `kbcli cluster edit-config` 则允许以可视化的方式直接编辑参数。

一共有 3 类参数：

1. 环境参数，比如 GC 相关的参数，`PULSAR_MEM` 和 `PULSAR_GC`。参数变更对每个组件都适用。
2. 配置参数，比如 `zookeeper` 或 `bookies.conf` 配置文件。可以通过 `env` 的方式做变更，变更会重启 pod。
3. 动态参数，比如 `brokers.conf` 中的配置文件。Pulsar 的 `broker` 支持两种变更模式：
     1. 一种是需要重启的参数变更，比如 `zookeeperSessionExpiredPolicy`。
     2. 另外一种是支持 dynamic 的参数，可以通过 `pulsar-admin brokers list-dynamic-config` 获取所有的动态参数列表。

:::note

`pulsar-admin `是 Pulsar 集群自带的管理工具，可以通过 `kubectl exec -it <pod-name> -- bash` 登录到对应的 Pod 中（pod-name 可通过 `kubectl get pods` 获取，选择名字中带有 `broker` 字样的 Pod 即可）。在 Pod 中的 `/pulsar/bin path` 路径下有对应的命令。关于 pulsar-admin 的更多信息，可参考[官方文档](https://pulsar.apache.org/docs/3.0.x/admin-api-tools/)。

:::

<Tabs>

<TabItem value="编辑配置文件" label="编辑配置文件">

1. 编辑 Pulsar 集群的 `broker.conf` 文件。本文实例修改了名为 `pulsar-broker-broker-config` 的文件。

   ```bash
   kubectl edit cm pulsar-broker-broker-config -n demo
   ```

2. 按需配置参数。

3. 查看配置是否生效。

   ```bash
   kubectl get pod -l app.kubernetes.io/name=pulsar-broker
   ```

:::note

如果您无法找到集群的配置文件，您可以使用 `kbcli` 查看集群当前的配置文件。

```bash
kbcli cluster describe-config mycluster -n demo
```

:::

</TabItem>

<TabItem value="OpsRequest" label="OpsRequest">

1. 在名为 `mycluster-configuring-demo.yaml` 的 YAML 文件中定义 OpsRequest，并修改参数。如下示例中，`lostBookieRecoveryDelay` 参数修改为 `1000`。

   ```bash
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: mycluster-configuring-demo
     namespace: demo
   spec:
     clusterName: mycluster
     reconfigure:
       componentName: bookies
       configurations:
       - keys:
         - key: bookkeeper.conf
           parameters:
           - key: lostBookieRecoveryDelay
             value: "1000"
         name: bookies-config
     preConditionDeadlineSeconds: 0
     type: Reconfiguring
   EOF
   ```

   | 字段                                                    | 定义     |
   |--------------------------------------------------------|--------------------------------|
   | `metadata.name`                                        | 定义了 OpsRequest 的名称。 |
   | `metadata.namespace`                                   | 定义了集群所在的 namespace。 |
   | `spec.clusterName`                                     | 定义了本次运维操作指向的集群名称。 |
   | `spec.reconfigure`                                     | 定义了需配置的 component 及相关配置更新内容。 |
   | `spec.reconfigure.componentName`                       | 定义了改集群的 component 名称。  |
   | `spec.configurations`                                  | 包含一系列 ConfigurationItem 对象，定义了 component 的配置模板名称、更新策略、参数键值对。 |
   | `spec.reconfigure.configurations.keys.key`             | 定义了 configuration map。 |
   | `spec.reconfigure.configurations.keys.parameters`      | 定义了单个参数文件的键值对列表。 |
   | `spec.reconfigure.configurations.keys.parameter.key`   | 代表您需要编辑的参数名称。|
   | `spec.reconfigure.configurations.keys.parameter.value` | 代表了将要更新的参数值。如果设置为 nil，Key 字段定义的参数将会被移出配置文件。  |
   | `spec.reconfigure.configurations.name`                 | 定义了配置模板名称。  |
   | `preConditionDeadlineSeconds`                          | 定义了本次 OpsRequest 中止之前，满足其启动条件的最长等待时间（单位为秒）。如果设置为 0（默认），则必须立即满足启动条件，OpsRequest 才能继续。|

2. 应用配置 OpsRequest。

   ```bash
   kubectl apply -f mycluster-configuring-demo.yaml -n demo
   ```

3. 确认配置是否生效。

   1. 查看配置进度。

      ```bash
      kubectl get ops -n demo
      ```

   2. 查看配置是否生效。

      ```bash
      kubectl get pod -l app.kubernetes.io/name=pulsar -n demo
      ```

:::note

如果您无法找到集群的配置文件，您可以使用 `kbcli` 查看集群当前的配置文件。

```bash
kbcli cluster describe-config mycluster -n demo
```

:::

</TabItem>

<TabItem value="kbcli" label="kbcli">

## 查看参数信息

* 查看集群的当前配置文件。

  ```bash
  kbcli cluster describe-config mycluster -n demo 
  ```

* 查看当前配置文件的详细信息。

  ```bash
  kbcli cluster describe-config mycluster -n demo --show-detail
  ```

* 查看参数描述。

  ```bash
  kbcli cluster explain-config mycluster -n demo | head -n 20
  ```

## 配置参数

### 使用 configure 命令配置参数

#### 配置环境参数

***步骤：***

1. 获取 Pulsar 集群组件名称，配置参数。

   ```bash
   kbcli cluster list-components mycluster -n demo 
   >
   NAME               NAMESPACE   CLUSTER      TYPE               IMAGE
   proxy              demo        mycluster    pulsar-proxy       docker.io/apecloud/pulsar:2.11.2
   broker             demo        mycluster    pulsar-broker      docker.io/apecloud/pulsar:2.11.2
   bookies-recovery   demo        mycluster    bookies-recovery   docker.io/apecloud/pulsar:2.11.2
   bookies            demo        mycluster    bookies            docker.io/apecloud/pulsar:2.11.2
   zookeeper          demo        mycluster    zookeeper          docker.io/apecloud/pulsar:2.11.2
   ```

2. 配置参数。

   下面以 `zookeeper` 为例。

   ```bash
   kbcli cluster configure mycluster -n demo --components=zookeeper --set PULSAR_MEM="-XX:MinRAMPercentage=50 -XX:MaxRAMPercentage=70"  
   ```

3. 验证配置。

   a. 检查配置进度。

   ```bash
   kubectl get ops -n demo
   ```

   b. 检查配置是否完成。

   ```bash
   kubectl get pod -l app.kubernetes.io/name=pulsar -n demo
   ```

#### 配置其他参数

下面以配置动态参数 `brokerShutdownTimeoutMs` 为例。

***步骤：***

1. 获取配置信息。

   ```bash
   kbcli cluster desc-config mycluster -n demo --components=broker
   >
   ConfigSpecs Meta:
   CONFIG-SPEC-NAME         FILE                   ENABLED   TEMPLATE                   CONSTRAINT                   RENDERED                                  COMPONENT   CLUSTER
   agamotto-configuration   agamotto-config.yaml   false     pulsar-agamotto-conf-tpl                                mycluster-broker-agamotto-configuration   broker      mycluster
   broker-env               conf                   true      pulsar-broker-env-tpl      pulsar-env-constraints       mycluster-broker-broker-env               broker      mycluster
   broker-config            broker.conf            true      pulsar-broker-config-tpl   brokers-config-constraints   mycluster-broker-broker-config            broker      mycluster
   ```

2. 配置参数。

   ```bash
   kbcli cluster configure mycluster -n demo --components=broker --config-specs=broker-config --set brokerShutdownTimeoutMs=66600
   >
   Will updated configure file meta:
     ConfigSpec: broker-config          ConfigFile: broker.conf        ComponentName: broker        ClusterName: mycluster
   OpsRequest mycluster-reconfiguring-qxw8s created successfully, you can view the progress:
           kbcli cluster describe-ops mycluster-reconfiguring-qxw8s -n demo
   ```

3. 检查配置进度。

   使用上述命令打印的 ops name 进行检查。

   ```bash
   kbcli cluster describe-ops mycluster-reconfiguring-qxw8s -n demo
   >
   Spec:
     Name: mycluster-reconfiguring-qxw8s        NameSpace: demo        Cluster: mycluster        Type: Reconfiguring

   Command:
     kbcli cluster configure mycluster --components=broker --config-specs=broker-config --config-file=broker.conf --set brokerShutdownTimeoutMs=66600 --namespace=demo

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
   kbcli cluster edit-config mycluster -n demo
   ```

   :::note

   如果集群中有多个组件，请使用 `--components` 参数指定一个组件。

   :::

2. 查看参数配置状态。

   ```bash
   kbcli cluster describe-ops mycluster-reconfiguring-nqxw8s -n demo
   ```

3. 连接到数据库，验证参数是否按预期配置。

   ```bash
   kbcli cluster connect mycluster -n demo
   ```

   :::note

   1. `edit-config` 不能同时编辑静态参数和动态参数。
   2. KubeBlocks 未来将支持删除参数。

   :::

## 查看历史记录并比较参数差异

配置完成后，你可以搜索历史配置并比较参数差异。

查看参数配置历史记录。

```bash
kbcli cluster describe-config mycluster -n demo --components=zookeeper               
```

从上面可以看到，有三个参数被修改过。

比较这些改动，查看不同版本中配置的参数和参数值。

```bash
kbcli cluster diff-config mycluster-reconfiguring-qxw8s mycluster-reconfiguring-mwbnw
```

</TabItem>

</Tabs>
