---
title: 配置集群参数
description: 如何配置集群参数
keywords: [pulsar, 参数, 配置]
sidebar_position: 1
siderbar_label: 配置
---

# Configure cluster parameters

本文档演示了如何配置集群参数。

一共有 3 类参数：

1. 环境参数，比如 GC 相关的参数，`PULSAR_MEM` 和 `PULSAR_GC`。参数变更对每个组件都适用。
2. 配置参数，比如 `zookeeper` 或 `bookies.conf` 配置文件。可以通过 `env` 的方式做变更，变更会重启 pod。
3. 动态参数，比如 `brokers.conf` 中的配置文件。Pulsar 的 `broker` 支持两种变更模式：
     1. 一种是需要重启的参数变更，比如 `zookeeperSessionExpiredPolicy`。
     2. 另外一种是支持 dynamic 的参数，可以通过 `pulsar-admin brokers list-dynamic-config` 获取所有的动态参数列表。

:::note

`pulsar-admin` 是 Pulsar 集群自带的管理工具，可以通过 `kubectl exec -it <pod-name> -- bash` 登录到对应的 Pod 中（pod-name 可通过 `kubectl get pods` 获取，选择名字中带有 `broker` 字样的 Pod 即可）。在 Pod 中的 `/pulsar/bin path` 路径下有对应的命令。关于 pulsar-admin 的更多信息，可参考[官方文档](https://pulsar.apache.org/docs/3.0.x/admin-api-tools/)。

:::

## 开始之前

1. [安装 KubeBlocks](./../../installation/install-kubeblocks.md)。
2. [创建 Pulsar 集群](./../cluster-management/create-pulsar-cluster-on-kubeblocks.md)。

## 通过编辑配置文件配置参数

1. 编辑 Pulsar 集群的 `broker.conf` 文件。本文实例修改了名为 `pulsar-broker-broker-config` 的文件。

   ```bash
   kubectl edit cm pulsar-broker-broker-config -n demo
   ```

2. 按需配置参数。

3. 查看配置是否生效。

   ```bash
   kubectl get pod -l app.kubernetes.io/name=pulsar-broker
   ```

## 通过 OpsRerquest 配置参数

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
   | `metadata.name`                                        | 定义了 Opsrequest 的名称。 |
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
   kubectl apply -f mycluster-configuring-demo.yaml
   ```

3. 确认配置是否生效。

   1. 查看配置进度。

      ```bash
      kubectl get ops
      ```

   2. 查看配置是否生效。

      ```bash
      kubectl get pod -l app.kubernetes.io/name=pulsar
      ```

:::note

如果您无法找到集群的配置文件，您可以使用 `kbcli` 查看集群当前的配置文件。

```bash
kbcli cluster describe-config mycluster -n demo
```

从元信息中可以看到，集群 `mycluster` 的配置文件名称。

你也可以查看此配置文件和参数的详细信息。

* 查看当前配置文件的详细信息。

   ```bash
   kbcli cluster describe-config mycluster --show-detail -n demo
   ```

* 查看参数描述。

  ```bash
  kbcli cluster explain-config mycluster -n demo | head -n 20
  ```

* 查看指定参数的使用文档。
  
  ```bash
  kbcli cluster explain-config mycluster --param=lostBookieRecoveryDelay --config-specs=bookies-config -n demo
  ```

  如果集群支持多个模板，你可以通过 `--config-specs` 来指定一个配置模板。执行 `kbcli cluster describe-config mycluster` 查看所有模板的名称。

:::
