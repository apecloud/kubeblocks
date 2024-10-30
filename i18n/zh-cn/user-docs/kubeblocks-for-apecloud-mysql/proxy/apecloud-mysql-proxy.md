---
title: ApeCloud MySQL 代理
description: 如何使用 ApeCloud MySQL 代理
keywords: [apecloud mysql 代理, 代理]
sidebar_position: 2
sidebar_label: ApeCloud MySQL 代理
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# ApeCloud MySQL 代理

## 开始之前

1. [安装 kbcli](./../../installation/install-with-kbcli/install-kbcli.md)。
2. [安装 Helm](https://helm.sh/docs/intro/install/)。
3. 安装 KubeBlocks。

   可以执行 `kbcli playground init` 安装 k3d 集群和 KubeBlocks。详情请参考[在本地使用 KubeBlocks](./../../try-out-on-playground/try-kubeblocks-on-local-host.md) 或[在云上使用 KubeBlocks](./../../try-out-on-playground/try-kubeblocks-on-cloud.md)。

   ```bash
   kbcli playground init

   # 使用--version 指定版本
   kbcli playground init --version='x.y.z'
   ```

   如果已经有 Kubernetes 集群，可以通过 [kbcli](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md) 安装 KubeBlocks。
4. 准备一个名为 mycluster 的 ApeCloud MySQL 三节点集群，用于演示如何为现有集群启用代理功能。详情请参考[创建并连接到 MySQL 集群](./../cluster-management/create-and-connect-a-mysql-cluster.md)。

## 创建代理集群

1. 启用 etcd 引擎，创建 etcd 集群。

   1. 安装并启用 etcd 引擎。ectd 引擎默认未安装，需安装后在进行后续操作。可参考[使用引擎](./../../overview/database-engines-supported.md#使用引擎)。

       ```bash
       # 1. 查看是否已添加 KubeBlocks 引擎索引
       kbcli addon index list

       # 如果列表为空，请先添加索引
       kbcli addon index add kubeblocks https://github.com/apecloud/block-index.git

       # 2. 搜索 etcd 引擎
       kbcli addon search etcd

       # 3. 安装 etcd 引擎
       kbcli addon install etcd --index kubeblocks --version 0.9.0

       # 4. 启用 etcd 引擎
       kbcli addon enable etcd

       # 5. 确认 etcd 引擎是否已启用
       kbcli addon list
       ```

   2. 创建 etcd 集群。

       ```bash
       kbcli cluster create myetcd --cluster-definition etcd
       ```

   3. 查看 etcd 服务状态。

       ```bash
       kbcli cluster list myetcd
       ```

2. 查看 etcd 服务地址或 etcd pod 地址。

    ```bash
    kubectl get service
    >
    NAME                             TYPE        CLUSTER-IP       EXTERNAL-IP   PORT(S)                                                  AGE
    kubernetes                       ClusterIP   10.96.0.1        <none>        443/TCP                                                  85d
    myetcd-etcd                      ClusterIP   10.101.227.143   <none>        2379/TCP                                                 111s
    myetcd-etcd-headless             ClusterIP   None             <none>        2379/TCP,2380/TCP,3501/TCP,50001/TCP                     111s
    ```

3. 创建 ApeCloud MySQL Proxy 集群。

    ```bash
    helm repo add kubeblocks https://apecloud.github.io/helm-charts

    helm install myproxy kubeblocks/apecloud-mysql-cluster --set mode=raftGroup,proxyEnabled=true,etcd.serviceReference.endpoint="etcd-cluster-etcd.default.svc.cluster.local:2379"
    ```

4. 查看集群状态。

    ```bash
    kbcli get cluster

    kbcli get pods
    ```

    您也可进入 etcd 或者 wesql-scale 容器中查看wesql-scale 配置，或检查 etcd 服务可用性。

    ```bash
    etcdctl --endpoints=http://etcd-cluster-etcd.default.svc.cluster.local:2379 get /vitess --prefix --keys-only
    ```

## 动态启用代理

ApeCloud MySQL 代理本质是一个数据库代理。通过设置 `proxyEnabled=true`，可以将 ApeCloud MySQL 集群版切换为 ApeCloud MySQL 代理集群。

```bash
helm upgrade mycluster kubeblocks/apecloud-mysql-cluster --set mode=raftGroup,proxyEnabled=true
```

## 连接代理集群

ApeCloud MySQL 代理通过 `vtgate` 组件进行路由，MySQL 服务器访问 `vtgate` 的方式和访问 `mysqld` 很像。代理提供的外部 SQL 访问地址是 `vtgate` 的地址和端口，而 KubeBlocks 默认创建的 `vtgate` 地址是 `myproxy-cluster-vtgate-headless`，端口号为 `15306`。你可以通过与代理处于相同命名空间的任意 Pod 中的 MySQL 服务器访问 ApeCloud MySQL 代理。

### 通过 VTGate 连接代理集群

执行以下命令连接到代理集群。

```bash
kbcli cluster connect myproxy --components vtgate
```

### 通过 MySQL 服务器连接代理集群

执行以下命令连接到 MySQL 服务器。

```bash
kbcli cluster connect myproxy
```

:::note

如果要测试 MySQL 的故障切换功能，请先删除 Pod，然后进行端口转发。或者你可以编写一个 shell 脚本进行测试。比如，

如果使用 VTGate：

```bash
while true; do date; kubectl port-forward svc/vt-vtgate-headless 15306:15306; sleep 0.5; done
```

如果使用 MySQL：

```bash
while true; do date; kubectl port-forward svc/vt-mysql 3306:3306; sleep 0.5; done
```

:::

## 配置代理集群参数

VTGate、VTConsensus 和 VTTablet 都支持参数配置。你可以使用 `--components` 参数指定组件来配置 VTGate 和 VTConsensus，使用 `--components=mysql --config-specs=vttablet-config` 同时指定组件和配置文件模板来配置 VTTablet，因为 VTTablet 是 MySQL 组件的附属组件。

### 查看参数详情

* 查看当前配置文件的详细信息。

   ```bash
   # vtgate
   kbcli cluster describe-config myproxy --components vtgate --show-detai
   
   # vtcontroller
   kbcli cluster describe-config myproxy --components vtcontroller --show-detail
   
   # vttablet
   kbcli cluster describe-config myproxy --component mysql --show-detail --config-specs vttablet-config
   ```

* 查看参数描述。

   ```bash
   # vtgate
   kbcli cluster explain-config myproxy --components vtgate

   # vttablet
   kbcli cluster explain-config myproxy --components mysql --config-specs=vttablet-config
   ```

* 查看指定参数的定义。

   ```bash
   kbcli cluster explain-config myproxy --components vtgate --param=healthcheck_timeout
   ```

### 配置参数

1. 查看 MySQL 服务器中的当前值。

   ```bash
   kbcli cluster connect myproxy --components=vtgate
   ```

   ```bash
   mysql> show variables like '%healthcheck_timeout%';
   ```

   ```bash
   mysql> show variables like '%health_check_interval%';
   ```

2. 配置 VTGate 的 `healthcheck_timeout` 和 VTTablet 的 `health_check_interval`。

   你可以通过使用 `--set` 或编辑参数配置文件进行配置。

   * 使用 `--set`。

      ```bash
      # vtgate
      kbcli cluster configure myproxy --components vtgate --set=healthcheck_timeout=2s

      # vttablet
      kbcli cluster configure myproxy --set=health_check_interval=4s --components=mysql --config-spec=vttablet-config
      ```

   * 编辑参数配置文件。

      ```bash
      kbcli cluster edit-config myproxy --components vtgate
      ```

    :::note

    执行 `vtgate` 参数配置命令后，会启动一个新的 vtgate Pod，并终止旧的 vtgate Pod。你可以执行以下命令监视旧 Pod 是否被终止。

    ```bash
    kubectl get pod <vtgate-pod-name> -w
    ```

    :::

3. 执行以下命令查看配置状态。比如，

   ```bash
   kbcli cluster describe-ops myproxy -reconfiguring-lth8d -n default
   ```

   :::note

   关于参数配置的更多信息，请参考[配置](./../configuration/configuration.md)。

   :::

## 日志

你可以使用 kbcli 和 kubectl 来查看组件、Pod 和容器的日志文件。

查看不同组件的日志。

```bash
kbcli cluster list-logs myproxy
kbcli cluster list-logs myproxy --components vtgate
kbcli cluster list-logs myproxy --components vtcontroller
kbcli cluster list-logs myproxy --components mysql
```

查看 Pod 日志。

```bash
kbcli cluster logs myproxy --instance myproxy-vtgate-85bdcf99df-wbmnl
```

查看 Pod 中容器的日志。

```bash
kbcli cluster logs myproxy --instance myproxy-mysql-0 -c vttablet
```

## 监控

:::note

在生产环境安装 KubeBlocks 时，所有监控插件默认处于禁用状态。你可以自行启用这些插件，但强烈建议你构建自己的监控系统或购买第三方监控服务。详情请参考[监控](./../../observability/monitor-database.md)。

:::

1. 启用监控功能。

   ```bash
   kbcli cluster update myproxy --monitor=true
   ```

2. 查看插件列表并启用 Grafana 插件。

   ```bash
   kbcli addon list 
   
   kbcli addon enable grafana
   ```

3. 查看仪表盘列表。

   ```bash
   kbcli dashboard list
   ```

4. 打开 Grafana 仪表盘。

   ```bash
   kbcli dashboard open kubeblocks-grafana
   ```

## 读写分离

你可以启用读写分离功能。

```bash
kbcli cluster configure myproxy --components vtgate --set=read_write_splitting_policy=random
```

还可以设置读写分离的比例。如下是设置 70% 的流量导向只读节点的例子。

```bash
kbcli cluster configure myproxy --components vtgate --set=read_write_splitting_ratio=70
```

此外，你还可以[使用 Grafana](#监控) 或执行 `show workload` 来查看流量分布。

```bash
show workload;
```

## 透明的故障切换

执行以下命令，实现透明的故障切换。

```bash
kbcli cluster configure myproxy --components vtgate --set=enable_buffer=true
```
