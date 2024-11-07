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

   如果已经有 Kubernetes 集群，可以通过 [kbcli](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md)  或 [Helm](./../../installation/install-with-helm/install-kubeblocks.md) 安装 KubeBlocks。
4. 准备一个名为 mycluster 的 ApeCloud MySQL 集群版集群，用于演示如何为现有集群启用代理功能。详情请参考[创建并连接到 MySQL 集群](./../cluster-management/create-and-connect-a-mysql-cluster.md)。

## 创建代理集群

建议使用 `kbcli` 创建 ApeCloud MySQL 代理集群。

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

1. 启用 etcd 引擎，创建 etcd 集群。

   1. 安装并启用 etcd 引擎。etcd 引擎默认未安装，需安装后在进行后续操作。可参考[使用引擎](./../../installation/install-with-kbcli/install-addons.md)文档。

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

</TabItem>

<TabItem value="kubectl" label="kubectl">

1. 添加 KubeBlocks 仓库。

   ```bash
   helm repo add kubeblocks https://apecloud.github.io/helm-charts
   ```

2. 查看仓库列表，确认 KubeBlocks 仓库是否已添加。

   ```bash
   helm repo list
   ```

3. 执行更新命令，确保您已安装最新版本。

   ```bash
   helm repo update
   ```

4. 安装 etcd，用于创建外部服务引用。

   1. 查看 etcd 的所有版本。

       ```bash
       helm search repo kubeblocks/etcd --devel --versions
       ```

   2. 安装 etcd 引擎。

       ```bash
       helm install etcd kubeblocks/etcd --version=v0.6.5
       ```

   3. 安装 etcd 集群。

       ```bash
       helm install etcd-cluster kubeblocks/etcd-cluster 
       ```

   4. 查看 etcd 集群状态，确保其处于 `Running` 状态。

       ```bash
       kubectl get cluster
       >
       NAME           CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS      AGE
       etcd-cluster   etcd                 etcd-v3.5.6   Halt                 Running     10s
       ```

   5. 查看 etcd 集群的服务地址。

       ```bash
       kubectl get service
       >
       NAME                             TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)                                                  AGE
       etcd-cluster-etcd                ClusterIP   10.110.23.89   <none>        2379/TCP                                                 55s
       etcd-cluster-etcd-headless       ClusterIP   None           <none>        2379/TCP,2380/TCP,3501/TCP,50001/TCP                     55s
       kubernetes                       ClusterIP   10.96.0.1      <none>        443/TCP                                                  13m
       ```

       您可以将服务地址组合起来，获取 endpoint。或者您可使用服务地址的 IP 作为连接地址。

       以下为服务地址组合的示例。

       ```bash
       etcd-cluster-etcd.default.svc.cluster.local:2379
       ```

5. 创建 ApeCloud MySQL 代理集群。
   1. 查看 ApeCloud MySQL 代理集群可用版本。

       ```bash
       helm search repo kubeblocks/apecloud-mysql --devel --versions
       ```

   2. （可选）如果您在安装 KubeBlocks 时停用了 `apecloud-mysql` 引擎，可执行以下命令并指定版本，安装 ApeCloud MySQL 的集群定义（cluster definition）。如果您安装 KubeBlocks 时选用了默认设置，可跳过本步骤。

       ```bash
       helm install myproxy kubeblocks/apecloud-mysql --version=v0.9.0
       ```

   3. 创建 ApeCloud MySQL 代理集群。

       ```bash
       helm install myproxy kubeblocks/apecloud-mysql-cluster --version=v0.9.0 --set mode=raftGroup,proxyEnabled=true,etcd.serviceReference.endpoint="etcd-cluster-etcd.default.svc.cluster.local:2379"
       ```

:::note

如果您只有一个节点可用于部署集群版，可在创建集群时将 `extra.availabilityPolicy` 设置为 `none`。

```bash
helm install myproxy kubeblocks/apecloud-mysql-cluster --version=v0.9.0 --set mode=raftGroup,proxyEnabled=true,etcd.serviceReference.endpoint="etcd-cluster-etcd.default.svc.cluster.local:2379" --set extra.availabilityPolicy=none
```

:::

6. 查看集群状态。

   ```bash
   kubectl get cluster

   kubectl get pods
   ```

   您也可以进入 etcd 容器或 wesql-scale 容器内部，查看 wesql-scale 的配置或检查 etcd 服务的可用性。

   ```bash
   etcdctl --endpoints=http://etcd-cluster-etcd.default.svc.cluster.local:2379 get /vitess --prefix --keys-only
   ```

</TabItem>

</Tabs>

## 动态启用代理

ApeCloud MySQL 代理本质是一个数据库代理。通过设置 `proxyEnabled=true`，可以将 ApeCloud MySQL 集群版切换为 ApeCloud MySQL 代理集群。

```bash
helm upgrade mycluster kubeblocks/apecloud-mysql-cluster --set mode=raftGroup,proxyEnabled=true,etcd.serviceReference.endpoint="etcd-cluster-etcd.default.svc.cluster.local:2379"
```

如需关闭代理，可执行以下命令。

```bash
helm upgrade mycluster kubeblocks/apecloud-mysql-cluster --set mode=raftGroup
```

## 连接代理集群

ApeCloud MySQL 代理通过 `vtgate` 组件进行路由，MySQL 服务器访问 `vtgate` 的方式和访问 `mysqld` 很像。代理提供的外部 SQL 访问地址是 `vtgate` 的地址和端口，而 KubeBlocks 默认创建的 `vtgate` 地址是 `myproxy-cluster-vtgate-headless`，端口号为 `15306`。你可以通过与代理处于相同命名空间的任意 Pod 中的 MySQL 服务器访问 ApeCloud MySQL 代理。

### 通过 VTGate 连接代理集群

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

执行以下命令连接到代理集群。

```bash
kbcli cluster connect myproxy --components vtgate
```

### 通过 MySQL 服务器连接代理集群

执行以下命令连接到代理集群。

```bash
kbcli cluster connect myproxy --components vtgate
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

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

执行以下命令，连接至 MySQL 服务器。

```bash
kbcli cluster connect myproxy
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

1. 将 MySQL 服务器的端口暴露到本地主机，使本地主机可以访问 MySQL 服务器。

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

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

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

</TabItem>

<TabItem value="编辑配置文件" label="编辑配置文件">

1. 查看当前配置文件的详细信息。

   ```bash
   kubectl edit configurations.apps.kubeblocks.io myproxy-vtgate
   ```

2. 按需配置参数。如下示例添加 `spec.configFileParams` 部分，用以配置 `max_connections`。

   ```yaml
   spec:
     clusterRef: myproxy
     componentName: vtgate
     configItemDetails:
     - configFileParams:
         vtgate.cnf:
           parameters:
             healthcheck_timeout: "5s"
       configSpec:
         constraintRef: mysql-scale-vtgate-config-constraints
         name: vtgate-config
         namespace: kb-system
         templateRef: vtgate-config-template
         volumeName: mysql-scale-config
       name: vtgate-config
       payload: {}
   ```

3. 连接至该集群，确认配置是否生效。

   1. 将 MySQL 服务器的端口暴露到本地主机，使本地主机可以访问 MySQL 服务器。

      ```bash
      kubectl port-forward svc/vt-vtgate-headless 15306:15306
      ```

   2. 连接集群，确认参数是否已修改。

      ```bash
      mysql -h 127.0.0.1 -P 3306

      >
      mysql> show variables like 'healthcheck_timeout';
      +---------------------+-------+
      | Variable_name       | Value |
      +---------------------+-------+
      | healthcheck_timeout |  5s   |
      +---------------------+-------+
      1 row in set (0.00 sec)
      ```

</TabItem>

<TabItem value="OpsRequest" label="OpsRequest">

将 OpsRequest 应用于指定集群，根据您的需要配置参数。

* 配置 VTTablet 示例

    ```yaml
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: OpsRequest
    metadata:
      name: acmysql-vttablet-reconfiguring
      namespace: default
    spec:
      clusterName: acmysql-cluster
      force: false
      reconfigure:
        componentName: mysql
        configurations:
          - keys:
              - key: vttablet.cnf
                parameters:
                  - key: health_check_interval
                    value: 4s
            name: vttablet-config
      preConditionDeadlineSeconds: 0
      type: Reconfiguring
    ```

* 配置 VTGate 示例

    ```yaml
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: OpsRequest
    metadata:
      name: acmysql-vtgate-reconfiguring
      namespace: default
    spec:
      clusterName: acmysql-cluster
      force: false
      reconfigure:
        componentName: vtgate
        configurations:
        - keys:
          - key: vtgate.cnf
            parameters:
            - key: healthcheck_timeout
              value: 2s
          name: vtgate-config
      preConditionDeadlineSeconds: 0
      type: Reconfiguring
    ```

</TabItem>

</Tabs>

## 日志

您可以使用 kbcli 和 kubectl 来查看组件、Pod 和容器的日志文件。

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

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

</TabItem>

<TabItem value="kubectl" label="kubectl">

查看 VTGate 日志。

```bash
kubectl logs myproxy-cluster-vtgate-8659d5db95-4dzt5
```

查看 VTTablet 日志，`-c` 为必选项。

```bash
kubectl logs myproxy-cluster-mysql-0 -c vttablet
```

进入容器，查看更多 VTGate 日志。

```bash
kubectl exec -it myproxy-cluster-vtgate-8659d5db95-4dzt5 -- bash
ls /vtdataroot
```

进入容器，查看更多 VTTablet 日志。

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

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

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

</TabItem>

<TabItem value="kubectl" label="kubectl">

1. 启用监控引擎。

   对于测试或演示环境，可执行以下命令，启用 KubeBlocks 提供的监控引擎。您可将 `prometheus` 替换为其他需要开启的监控引擎名称。

   ```bash
   helm install prometheus kubeblocks/prometheus --namespace kb-system --create-namespace
   helm install grafana kubeblocks/grafana --namespace kb-system --create-namespace
   ```

   对于生产环境，您可以集成监控组件。可参考监控工具提供的相关文档查看集成细节。

2. 检查当前代理集群的监控功能是否启用。

   ```bash
   kubectl get cluster myproxy -o yaml
   ```

   如果输出的 YAML 文件中显示 `disableExporter: false`，则表示该集群的监控功能已开启。

   如果监控功能未开启，可执行以下命令启用监控功能。

   ```bash
   kubectl patch cluster mycluster -n demo --type "json" -p '[{"op":"add","path":"/spec/componentSpecs/0/disableExporter","value":false}]'
   ```

3. 查看监控大盘。

   对于测试或演示环境，可执行以下命令查看 Grafana 大盘。

   ```bash
   # 1. 获取用户名及密码 
   kubectl get secret grafana -n kb-system -o jsonpath='{.data.admin-user}' |base64 -d

   kubectl get secret grafana -n kb-system -o jsonpath='{.data.admin-password}' |base64 -d

   # 2. 连接至 Grafana 大盘
   kubectl port-forward svc/grafana -n kb-system 3000:8

   # 3. 打开浏览器，输入地址 127.0.0.1:3000，查看大盘

   # 4. 输入步骤 1 中获取的用户名及密码
   ```

   对于生产环境，您可以通过 Grafana Web 控制台查看对应集群的大盘。可查看 [Grafana 大盘文档](https://grafana.com/docs/grafana/latest/dashboards/) 查看配置细节。

:::note

1. 如果大盘中无数据，您可检查 job 是否为 `kubeblocks-service`。如果不是，可在 job 字段中输入 `kubeblocks-service` 并按回车键跳转。

   ![Monitoring dashboard](./../../../img/api-monitoring.png)

2. 更多关于监控功能的细节，可查看 [监控](./../../observability/monitor-database.md) 文档。

:::

</TabItem>

</Tabs>

## 读写分离

你可以启用读写分离功能。

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

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

</TabItem>

<TabItem value="kubectl" label="kubectl">

1. 获取当前集群的配置文件。

   ```bash
   kubectl edit configurations.apps.kubeblocks.io myproxy-vtgate
   ```

2. 将 `read_write_splitting_policy` 配置为 `random`。

   ```yaml
   spec:
     clusterRef: myproxy
     componentName: vtgate
     configItemDetails:
     - configFileParams:
         vtgate.cnf:
           parameters:
             read_write_splitting_policy: "random"
       configSpec:
         constraintRef: mysql-scale-vtgate-config-constraints
         name: vtgate-config
         namespace: kb-system
         templateRef: vtgate-config-template
         volumeName: mysql-scale-config
       name: vtgate-config
       payload: {}
   ```

您也可以设置读写分离的比例，以下示例将 70% 的流量导向只读节点。

```yaml
spec:
   clusterRef: myproxy
   componentName: vtgate
   configItemDetails:
   - configFileParams:
      vtgate.cnf:
         parameters:
            read_write_splitting_rati: "70"
      configSpec:
      constraintRef: mysql-scale-vtgate-config-constraints
      name: vtgate-config
      namespace: kb-system
      templateRef: vtgate-config-template
      volumeName: mysql-scale-config
      name: vtgate-config
      payload: {}
```

</TabItem>

</Tabs>

## 透明故障切换

执行以下命令，实现透明故障切换。

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster configure myproxy --components vtgate --set=enable_buffer=true
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

1. 获取集群的配置文件。

   ```bash
   kubectl edit configurations.apps.kubeblocks.io myproxy-vtgate
   ```

2. 将 `enable_buffer` 设置为 `true`。

   ```yaml
   spec:
     clusterRef: myproxy
     componentName: vtgate
     configItemDetails:
     - configFileParams:
         vtgate.cnf:
           parameters:
             enable_buffer: "true"
       configSpec:
         constraintRef: mysql-scale-vtgate-config-constraints
         name: vtgate-config
         namespace: kb-system
         templateRef: vtgate-config-template
         volumeName: mysql-scale-config
       name: vtgate-config
       payload: {}
   ```

</TabItem>

</Tabs>
