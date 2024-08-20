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

1. [安装 KubeBlocks](./../../installation/install-kubeblocks.md)。
2. 准备一个名为 mycluster 的 ApeCloud MySQL 三节点集群，用于演示如何为现有集群启用代理功能。详情请参考 [创建并连接到 MySQL 集群](./../cluster-management/create-and-connect-an-apecloud-mysql-cluster.md)。

## 创建代理集群

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

如果您只有一个节点可用于部署集群版，可在创建集群时将 `extra.availability-policy` 设置为 `none`。

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

## 动态启用/关闭代理

ApeCloud MySQL 代理本质上是数据库代理。已创建的 ApeClud MySQL 集群版可通过设置 `proxyEnabled=true`，切换为 ApeCloud MySQL 代理集群。

```bash
helm upgrade mycluster kubeblocks/apecloud-mysql-cluster --set mode=raftGroup,proxyEnabled=true,etcd.serviceReference.endpoint="etcd-cluster-etcd.default.svc.cluster.local:2379"
```

如果您想要关闭代理，可执行以下命令。

```bash
helm upgrade mycluster kubeblocks/apecloud-mysql-cluster --set mode=raftGroup
```

## 连接代理集群

ApeCloud MySQL 代理集群是通过 `vtgate` 组件进行路由，MySQL 客户端访问 `vtgate` 方式与访问 `mysqld` 方式类似。ApeCloud MySQL 代理提供的对外 SQL 访问地址即是 `vtgate` 地址和端口号。KubeBlocks 默认创建的 `vtgate` 地址为 `myproxy-cluster-vtgate-headless`，端口号为 `15306`。可以在 ApeCloud MySQL 同一个 namespace 下的任意 pod 中通过 MySQL 客户端访客 ApeCloud MySQL 集群。

### 通过 VTGate 连接代理集群

1. 将 VTGate 的端口映射到本地主机，使本地主机可以访问代理。

   ```bash
   kubectl port-forward svc/vt-vtgate-headless 15306:15306
   ```

2. 连接到集群。

   ```bash
   mysql -h 127.0.0.1 -P 15306
   ```

### 通过 MySQL 服务器连接代理集群

1. 将 MySQL 服务器的端口暴露到本地主机，使本地主机可以访问 MySQL 服务器。

   ```bash
   kubectl port-forward svc/vt-mysql 3306:3306
   ```

2. 连接到集群。

   ```bash
   mysql -h 127.0.0.1 -P 3306
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

<Tabs>

<TabItem value="编辑配置文件" label="编辑配置文件" default>

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

您可查看组件、Pod 和容器的日志文件。

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

## 监控

您可监控代理集群的性能。

1. 启用监控引擎。

   对于测试或演示环境，可执行以下命令，启用 KubeBlocks 提供的监控引擎。

   ```bash
   helm install prometheus kubeblocks/prometheus --namespace kb-system --create-namespace
   helm install prometheus kubeblocks/prometheus --namespace kb-system --create-namespace
   helm install prometheus kubeblocks/prometheus --namespace kb-system --create-namespace
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
   # 1. Get the username and password 
   kubectl get secret grafana -n kb-system -o jsonpath='{.data.admin-user}' |base64 -d

   kubectl get secret grafana -n kb-system -o jsonpath='{.data.admin-password}' |base64 -d

   # 2. Connect to the Grafana dashboard
   kubectl port-forward svc/grafana -n kb-system 3000:8

   # 3. Open the web browser and enter the address 127.0.0.1:3000 to visit the dashboard.

   # 4. Enter the username and password obtained from step 1.
   ```

   对于生产环境，您可以通过 Grafana Web 控制台查看对应集群的大盘。可查看 [Grafana 大盘文档](https://grafana.com/docs/grafana/latest/dashboards/) 查看配置细节。

:::note

1. 如果大盘中无数据，您可检查 job 是否为 `kubeblocks-service`。如果不是，可在 job 字段中输入 `kubeblocks-service` 并按回车键跳转。

   ![Monitoring dashboard](./../../../img/api-monitoring.png)

2. 更多关于监控功能的细节，可查看 [监控](./../../observability/monitor-database.md) 文档。

:::

## 读写分离

您可启用读写分离功能。

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

## 透明故障切换

执行以下命令，实现透明的故障切换。

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
