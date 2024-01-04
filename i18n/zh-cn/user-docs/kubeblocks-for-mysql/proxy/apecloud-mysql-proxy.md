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
2. 安装 KubeBlocks。

   可以执行 `kbcli playground init` 安装 k3d 集群和 KubeBlocks。详情请参考[在本地使用 KubeBlocks](./../../try-out-on-playground/try-kubeblocks-on-local-host.md) 或[在云上使用 KubeBlocks](./../../try-out-on-playground/try-kubeblocks-on-cloud.md)。

   ```bash
   kbcli playground init

   # 使用--version 指定版本
   kbcli playground init --version='0.6.0'
   ```

   如果已经有 Kubernetes 集群，可以选择用 [Helm](./../../installation/install-with-helm/install-kubeblocks-with-helm.md) 或 [kbcli](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md) 安装 KubeBlocks。
3. 准备一个名为 mycluster 的 ApeCloud MySQL 三节点集群，用于演示如何为现有集群启用代理功能。详情请参考[创建并连接到 MySQL 集群](./../../kubeblocks-for-mysql/cluster-management/create-and-connect-a-mysql-cluster.md)。

## 创建代理集群

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

建议使用 kbcli 创建 ApeCloud MySQL 代理集群。

```bash
kbcli cluster create mysql myproxy --mode raftGroup --availability-policy none --proxy-enabled true
```

</TabItem>

<TabItem value="Helm" label="Helm">

1. 添加 KubeBlocks 仓库。

   ```bash
   helm repo add kubeblocks https://apecloud.github.io/helm-charts
   ```

2. 查看仓库列表，验证 KubeBlocks 仓库是否添加成功。

   ```bash
   helm repo list
   ```

3. 执行 `update` 命令，确保已添加最新版本。

   ```bash
   helm repo update
   ```

4. 查看各版本的 ApeCloud MySQL 代理。

   ```bash
   helm search repo kubeblocks/apecloud-mysql --devel --versions
   ```

5. （可选）如果安装 KubeBlocks 时禁用了 `apecloud-mysql` 插件，执行以下命令指定版本并安装 ApeCloud MySQL 集群定义。如果使用默认设置安装 KubeBlocks，请跳过此步骤。

   ```bash
   helm install myproxy kubeblocks/apecloud-mysql --version=v0.6.0
   ```

6. 创建 ApeCloud MySQL 代理集群。

   ```bash
   helm install myproxy kubeblocks/apecloud-mysql-cluster --version=v0.6.0 --set mode=raftGroup,proxyEnabled=true 
   ```

:::note

如果只有一个节点用于部署 ApeCloud MySQL 集群版，请在创建时将 `availability-policy` 设置为 `none`。

```bash
helm install myproxy kubeblocks/apecloud-mysql-cluster --version=v0.6.0 --set mode=raftGroup,proxyEnabled=true --set extra.availabilityPolicy=none
```

:::

</TabItem>

</Tabs>

## 动态启用代理

ApeCloud MySQL 代理本质是一个数据库代理。通过设置 `proxyEnabled=true`，可以将 ApeCloud MySQL 集群版切换为 ApeCloud MySQL 代理集群。

<Tabs>
<TabItem value="kbcli" label="kbcli" default>

敬请期待......

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
helm upgrade mycluster kubeblocks/apecloud-mysql-cluster --set mode=raftGroup,proxyEnabled=true
```

</TabItem>

</Tabs>

## 连接代理集群

ApeCloud MySQL 代理通过 `vtgate` 组件进行路由，MySQL 服务器访问 `vtgate` 的方式和访问 `mysqld` 很像。代理提供的外部 SQL 访问地址是 `vtgate` 的地址和端口，而 KubeBlocks 默认创建的 `vtgate` 地址是 `myproxy-cluster-vtgate-headless`，端口号为 `15306`。你可以通过与代理处于相同命名空间的任意 Pod 中的 MySQL 服务器访问 ApeCloud MySQL 代理。

### 通过 VTGate 连接代理集群

<Tabs>
<TabItem value="kbcli" label="kbcli" default>

执行以下命令连接到代理集群。

```bash
kbcli cluster connect myproxy --component vtgate
```

</TabItem>

<TabItem value="port-forward" label="port-forward">

1. 将 VTGate 的端口映射到本地主机，使本地主机可以访问代理。

   ```bash
   kubectl port-forward svc/vt-vtgate-headless 15306:15306
   ```

2. 连接到集群。

   ```bash
   mysql -h 127.0.0.1 -P 15306
   ```

</TabItem>
</Tabs>

### 通过 MySQL 服务器连接代理集群

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

执行以下命令连接到 MySQL 服务器。

```bash
kbcli cluster connect myproxy
```

</TabItem>

<TabItem value="port-forward" label="port-forward">

1. 将 MySQL 服务器的端口映射到本地主机，使本地主机可以访问 MySQL 服务器。

   ```bash
   kubectl port-forward svc/vt-mysql 3306:3306
   ```

2. 连接到集群。

   ```bash
   mysql -h 127.0.0.1 -P 3306
   ```

</TabItem>
</Tabs>

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

VTGate、VTConsensus 和 VTTablet 都支持参数配置。你可以使用 `--component` 参数指定组件来配置 VTGate 和 VTConsensus，使用 `--component=mysql --config-specs=vttablet-config` 同时指定组件和配置文件模板来配置 VTTablet，因为 VTTablet 是 MySQL 组件的附属组件。

### 查看参数详情

* 查看当前配置文件的详细信息。

   ```bash
   # vtgate
   kbcli cluster describe-config myproxy --component vtgate --show-detai
   
   # vtcontroller
   kbcli cluster describe-config myproxy --component vtcontroller --show-detail
   
   # vttablet
   kbcli cluster describe-config myproxy --component mysql --show-detail --config-specs vttablet-config
   ```

* 查看参数描述。

   ```bash
   # vtgate
   kbcli cluster explain-config myproxy --component vtgate

   # vttablet
   kbcli cluster explain-config myproxy --component mysql --config-specs=vttablet-config
   ```

* 查看指定参数的定义。

   ```bash
   kbcli cluster explain-config myproxy --component vtgate --param=healthcheck_timeout
   ```

### 重新配置参数

1. 查看 MySQL 服务器中的当前值。

   ```bash
   kbcli cluster connect myproxy --component=vtgate
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
      kbcli cluster configure myproxy --component vtgate --set=healthcheck_timeout=2s

      # vttablet
      kbcli cluster configure myproxy --set=health_check_interval=4s --component=mysql --config-spec=vttablet-config
      ```

   * 编辑参数配置文件。

      ```bash
      kbcli cluster edit-config myproxy --component vtgate
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

    关于参数配置的更多信息，请参考[配置](./../../kubeblocks-for-postgresql/configuration/configuration.md)。

    :::

## 日志

你可以使用 kbcli 和 kubectl 来查看组件、Pod 和容器的日志文件。

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

查看不同组件的日志。

```bash
kbcli cluster list-logs myproxy
kbcli cluster list-logs myproxy --component vtgate
kbcli cluster list-logs myproxy --component vtcontroller
kbcli cluster list-logs myproxy --component mysql
```

查看 Pod 日志。

```bash
kbcli cluster logs myproxy --instance myproxy-vtgate-85bdcf99df-wbmnl
```

查看 Pod 中容器的日志。

```bash
kbcli cluster logs myproxy --instance myproxy-mysql-0 -c vttablet
```

</TabItem>

<TabItem value="kubectl" label="kubectl" default>

查看 VTGate 的日志。

```bash
kubectl logs myproxy-cluster-vtgate-8659d5db95-4dzt5
```

查看 VTTable 的日志，`-c` 是必需的。

```bash
kubectl logs myproxy-cluster-mysql-0 -c vttablet
```

进入容器并查看更多 VTGate 日志。

```bash
kubectl exec -it myproxy-cluster-vtgate-8659d5db95-4dzt5 -- bash
ls /vtdataroot
```

进入容器并查看更多 VTTablet 日志。

```bash
kubectl exec -it myproxy-cluster-mysql-0  -c vttablet -- bash
ls /vtdataroot
```

</TabItem>

</Tabs>

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
kbcli cluster configure myproxy --component vtgate --set=read_write_splitting_policy=random
```

还可以设置读写分离的比例。如下是设置 70% 的流量导向只读节点的例子。

```bash
kbcli cluster configure myproxy --component vtgate --set=read_write_splitting_ratio=70
```

此外，你还可以[使用 Grafana](#监控) 或执行 `show workload` 来查看流量分布。

```bash
show workload;
```

## 透明的故障切换

执行以下命令，实现透明的故障切换。

```bash
kbcli cluster configure myproxy --component vtgate --set=enable_buffer=true
```
