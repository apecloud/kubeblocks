---
title: How to use ApeCloud MySQL Proxy Cluster
description: ApeCloud MySQL Proxy Cluster tutorial
keywords: [apecloud mysql proxy, proxy]
sidebar_position: 2
sidebar_label: ApeCloud MySQL Proxy Cluster
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# ApeCloud MySQL Proxy

## Before you start

1. [Install kbcli](./../../installation/install-with-kbcli/install-kbcli.md).
2. [Install Helm](https://helm.sh/docs/intro/install/).
3. Install KubeBlocks.

   You can run `kbcli playground init` to install a k3d cluster and KubeBlocks. For details, refer to [Try KubeBlocks on your laptop](./../../try-out-on-playground/try-kubeblocks-on-your-laptop.md) or [Try KubeBlocks on cloud](./../../try-out-on-playground/try-kubeblocks-on-cloud.md).

   ```bash
   kbcli playground init

   # Use --version to specify a version
   kbcli playground init --version='x.x.x'
   ```

   Or if you already have a Kubernetes cluster, you can install [KubeBlocks](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md) directly.
4. Prepare an ApeCloud MySQL RaftGroup named `mycluster` for demonstrating how to enable the proxy function for an existing cluster. Refer to [Create a MySQL cluster](./../cluster-management/create-and-connect-an-apecloud-mysql-cluster.md) for details.

## Create a Proxy Cluster

It is recommended to use kbcli to create an ApeCloud MySQL Proxy Cluster.

1. Enable the etcd addon and create an etcd cluster.

   1. Install and enable the etcd addon. You need to install the etcd addon first since the etcd addon is not installed by default. Refer to [User addons](./../../overview/supported-addons.md#use-addons) for details.

       ```bash
       # 1. Check whether the KubeBlocks addon index is added
       kbcli addon index list

       # If the list is empty, add the index
       kbcli addon index add kubeblocks https://github.com/apecloud/block-index.git

       # 2. Search the etcd addon
       kbcli addon search etcd

       # 3. Install the etcd addon
       kbcli addon install etcd --index kubeblocks --version 0.9.0

       # 4. Enable the etcd addon
       kbcli addon enable etcd

       # 5. Check whether the etcd addon is enabled.
       kbcli addon list
       ```

   2. Create an etcd cluster.

       ```bash
       kbcli cluster create myetcd --cluster-definition etcd
       ```

   3. Check the status of the etcd service

       ```bash
       kbcli cluster list myetcd
       ```

2. View the etcd service address or the etcd pod address.

   ```bash
   kubectl get service
   >
   NAME                             TYPE        CLUSTER-IP       EXTERNAL-IP   PORT(S)                                                  AGE
   kubernetes                       ClusterIP   10.96.0.1        <none>        443/TCP                                                  85d
   myetcd-etcd                      ClusterIP   10.101.227.143   <none>        2379/TCP                                                 111s
   myetcd-etcd-headless             ClusterIP   None             <none>        2379/TCP,2380/TCP,3501/TCP,50001/TCP                     111s
   ```

3. Create an ApeCloud MySQL Proxy cluster.

    ```bash
    helm repo add kubeblocks https://apecloud.github.io/helm-charts

    helm install myproxy kubeblocks/apecloud-mysql-cluster --set mode=raftGroup,proxyEnabled=true,etcd.serviceReference.endpoint="etcd-cluster-etcd.default.svc.cluster.local:2379"
    ```

4. Check the status of the clusters.

   ```bash
   kbcli get cluster

   kbcli get pods
   ```

   You can also enter the etcd container or wesql-scale container to view the configuration of wesql-scale or to check the availability of the etcd service.

   ```bash
   etcdctl --endpoints=http://etcd-cluster-etcd.default.svc.cluster.local:2379 get /vitess --prefix --keys-only
   ```

## Enable Proxy dynamically

As its name suggests, ApeCloud MySQL Proxy in nature is a database proxy. An ApeCloud MySQL RaftGroup Cluster can be switched to an ApeCloud MySQL Proxy Cluster by setting `proxyEnabled=true`.

<Tabs>
<TabItem value="kbcli" label="kbcli" default>

Coming soon...

</TabItem>

<TabItem value="kubectl" label="kubectl">

```bash
helm upgrade mycluster kubeblocks/apecloud-mysql-cluster --set mode=raftGroup,proxyEnabled=true
```

</TabItem>

</Tabs>

## Connect Proxy Cluster

ApeCloud MySQL Proxy is routed through the `vtgate` component, and the way the MySQL Server accesses `vtgate` is similar to the way of accessing `mysqld`. The external SQL access address provided by ApeCloud MySQL Proxy is the `vtgate` address and port. The `vtgate` address created by KubeBlocks by default is `myproxy-cluster-vtgate-headless`, and the port number is `15306`. You can visit ApeCloud MySQL Proxy through the MySQL Server in any pod under the same namespace as ApeCloud MySQL Proxy.

### Connect Proxy Cluster by VTGate

Run the command below to connect to the Proxy Cluster.

```bash
kbcli cluster connect myproxy --components vtgate
```

### Connect Proxy Cluster by MySQL Server

Run the command below to connect to the MySQL Server.

```bash
kbcli cluster connect myproxy
```

:::note

If you need to test the failover of MySQL, you need to delete the Pod first and continue to port-forward the port, and you can also write a shell script. Here are examples.

For VTGate,

```bash
while true; do date; kubectl port-forward svc/vt-vtgate-headless 15306:15306; sleep 0.5; done
```

For the MySQL Server,

```bash
while true; do date; kubectl port-forward svc/vt-mysql 3306:3306; sleep 0.5; done
```

:::

## Configure Proxy Cluster parameters

VTGate, VTConsensus, and VTTablet support parameter configuration. You can configure VTGate and VTConsensus by using `--components` to specify a component and configure VTTablet by using `--components=mysql --config-specs=vttablet-config` to specify both a component and a configuration file template since VTTablet is the sidecar of the MySQL component.

### View parameter details

* View the details of the current configuration file.

   ```bash
   # vtgate
   kbcli cluster describe-config myproxy --components vtgate --show-detai
   
   # vtcontroller
   kbcli cluster describe-config myproxy --components vtcontroller --show-detail
   
   # vttablet
   kbcli cluster describe-config myproxy --components mysql --show-detail --config-specs vttablet-config
   ```

* View the parameter descriptions.

   ```bash
   # vtgate
   kbcli cluster explain-config myproxy --components vtgate

   # vttablet
   kbcli cluster explain-config myproxy --components mysql --config-specs=vttablet-config
   ```

* View the definition of a specified parameter.

   ```bash
   kbcli cluster explain-config myproxy --components vtgate --param=healthcheck_timeout
   ```

### Reconfigure parameters

1. View the current values in the MySQL Server.

   ```bash
   kbcli cluster connect myproxy --components=vtgate
   ```

   ```bash
   mysql> show variables like '%healthcheck_timeout%';
   ```

   ```bash
   mysql> show variables like '%health_check_interval%';
   ```

2. Configure the `healthcheck_timeout` for VTGate and the `health_check_interval` for VTTablet.

   You can use `--set` flag or edit the parameter configuration file to edit values.

   * By using `--set` flag

      ```bash
      # vtgate
      kbcli cluster configure myproxy --components vtgate --set=healthcheck_timeout=2s

      # vttablet
      kbcli cluster configure myproxy --set=health_check_interval=4s --components=mysql --config-spec=vttablet-config
      ```

   * By editing the parameter configuration file

      ```bash
      kbcli cluster edit-config myproxy --components vtgate
      ```

    :::note

    After the `vtgate` parameter values configuration command is executed, a new vtgate Pod is started and the old vtgate Pod is terminated. You can run the command below to monitor whether the old Pod is terminated.

    ```bash
    kubectl get pod <vtgate-pod-name> -w
    ```

    :::

3. Use the output command to view the configuration status. For example,

   ```bash
   kbcli cluster describe-ops myproxy -reconfiguring-lth8d -n default
   ```

   :::note

   For more information about parameter configuration, refer to [Configuration](./../configuration/configuration.md).

   :::

## Log

You can view the log files of components, Pods, and containers by both kbcli and kubectl.

View the log of different components.

```bash
kbcli cluster list-logs myproxy
kbcli cluster list-logs myproxy --components vtgate
kbcli cluster list-logs myproxy --components vtcontroller
kbcli cluster list-logs myproxy --components mysql
```

View the log of a Pod.

```bash
kbcli cluster logs myproxy --instance myproxy-vtgate-85bdcf99df-wbmnl
```

View the log of a container in a Pod.

```bash
kbcli cluster logs myproxy --instance myproxy-mysql-0 -c vttablet
```

## Monitoring

:::note

In the production environment, all monitoring addons are disabled by default when installing KubeBlocks. You can enable these addons but it is highly recommended to build your monitoring system or purchase a third-party monitoring service. For details, refer to [Monitoring](./../../observability/monitor-database.md).

:::

1. Enable the monitoring function.

   ```bash
   kbcli cluster update myproxy --monitor=true
   ```

2. View the addon list and enable the Grafana addon.

   ```bash
   kbcli addon list 
   
   kbcli addon enable grafana
   ```

3. View the dashboard list.

   ```bash
   kbcli dashboard list
   ```

4. Open the Grafana dashboard.

   ```bash
   kbcli dashboard open kubeblocks-grafana
   ```

## Read-write splitting

You can enable the read-write splitting function.

```bash
kbcli cluster configure myproxy --components vtgate --set=read_write_splitting_policy=random
```

You can also set the ratio for read-write splitting and here is an example of directing 70% flow to the read-only node.

```bash
kbcli cluster configure myproxy --components vtgate --set=read_write_splitting_ratio=70
```

Moreover, you can [use Grafana](#monitoring) or run `show workload` to view the flow distribution.

```bash
show workload;
```

## Transparent failover

Run the command below to implement transparent failover.

```bash
kbcli cluster configure myproxy --components vtgate --set=enable_buffer=true
```
