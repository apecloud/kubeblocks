---
title: How to use ApeCloud MySQL Proxy Cluster
description: ApeCloud MySQL Proxy Cluster tutorial
keywords: [apecloud mysql proxy, proxy]
sidebar_position: 2
sidebar_label: ApeCloud MySQL Proxy Cluster
---

# ApeCloud MySQL Proxy

## Before you start

1. [Install KubeBlocks](./../../installation/install-kubeblocks.md).
2. Prepare an ApeCloud MySQL RaftGroup named `mycluster` for demonstrating how to enable the proxy function for an existing cluster. Refer to [Create a MySQL cluster](./../cluster-management/create-and-connect-a-mysql-cluster.md) for details.

## Create a Proxy Cluster

1. Add the KubeBlocks repository.

   ```bash
   helm repo add kubeblocks https://apecloud.github.io/helm-charts
   ```

2. View the repository list to verify whether the KubeBlocks repository is added successfully.

   ```bash
   helm repo list
   ```

3. Run the update command to make sure you have added the latest version.

   ```bash
   helm repo update
   ```

4. View all versions of ApeCloud MySQL Proxy.

   ```bash
   helm search repo kubeblocks/apecloud-mysql --devel --versions
   ```

5. (Optional) If you disable the `apecloud-mysql` add-on when installing KuebBlocks, run the command below to specify a version and install the cluster definition of ApeCloud MySQL. Skip this step if you install KubeBlocks with the default settings.

   ```bash
   helm install myproxy kubeblocks/apecloud-mysql --version=v0.6.0
   ```

6. Create an ApeCloud MySQL Proxy Cluster.

   ```bash
   helm install myproxy kubeblocks/apecloud-mysql-cluster --version=v0.6.0 --set mode=raftGroup,proxyEnabled=true 
   ```

:::note

If you only have one node for deploying a RaftGroup, set the `availability-policy` as `none` when creating a RaftGroup.

```bash
helm install myproxy kubeblocks/apecloud-mysql-cluster --version=v0.6.0 --set mode=raftGroup,proxyEnabled=true --set extra.availabilityPolicy=none
```

:::

## Enable Proxy dynamically

As its name suggests, ApeCloud MySQL Proxy in nature is a database proxy. An ApeCloud MySQL RaftGroup Cluster can be switched to an ApeCloud MySQL Proxy Cluster by setting `proxyEnabled=true`.

```bash
helm upgrade mycluster kubeblocks/apecloud-mysql-cluster --set mode=raftGroup,proxyEnabled=true
```

## Connect Proxy Cluster

ApeCloud MySQL Proxy is routed through the `vtgate` component, and the way the MySQL Server accesses `vtgate` is similar to the way of accessing `mysqld`. The external SQL access address provided by ApeCloud MySQL Proxy is the `vtgate` address and port. The `vtgate` address created by KubeBlocks by default is `myproxy-cluster-vtgate-headless`, and the port number is `15306`. You can visit ApeCloud MySQL Proxy through the MySQL Server in any pod under the same namespace as ApeCloud MySQL Proxy.

### Connect Proxy Cluster by VTGate

1. Expose the port of VTGate to the localhost so that the localhost can access the Proxy.

   ```bash
   kubectl port-forward svc/vt-vtgate-headless 15306:15306
   ```

2. Connect to the cluster.

   ```bash
   mysql -h 127.0.0.1 -P 15306
   ```

### Connect Proxy Cluster by MySQL Server

1. Expose the port of the MySQL Server to the localhost so that the localhost can access the MySQL Server.

   ```bash
   kubectl port-forward svc/vt-mysql 3306:3306
   ```

2. Connect to the cluster.

   ```bash
   mysql -h 127.0.0.1 -P 3306
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

VTGate, VTConsensus, and VTTablet support parameter configuration. You can configure VTGate and VTConsensus by using `--component` to specify a component and configure VTTablet by using `--component=mysql --config-specs=vttablet-config` to specify both a component and a configuration file template since VTTablet is the sidecar of the MySQL component.

### View parameter details

* View the details of the current configuration file.

   ```bash
   # vtgate
   kbcli cluster describe-config myproxy --component vtgate --show-detai
   
   # vtcontroller
   kbcli cluster describe-config myproxy --component vtcontroller --show-detail
   
   # vttablet
   kbcli cluster describe-config myproxy --component mysql --show-detail --config-specs vttablet-config
   ```

* View the parameter descriptions.

   ```bash
   # vtgate
   kbcli cluster explain-config myproxy --component vtgate

   # vttablet
   kbcli cluster explain-config myproxy --component mysql --config-specs=vttablet-config
   ```

* View the definition of a specified parameter.

   ```bash
   kbcli cluster explain-config myproxy --component vtgate --param=healthcheck_timeout
   ```

### Reconfigure parameters

1. View the current values in the MySQL Server.

   ```bash
   kbcli cluster connect myproxy --component=vtgate
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
      kbcli cluster configure myproxy --component vtgate --set=healthcheck_timeout=2s

      # vttablet
      kbcli cluster configure myproxy --set=health_check_interval=4s --component=mysql --config-spec=vttablet-config
      ```

   * By editing the parameter configuration file

      ```bash
      kbcli cluster edit-config myproxy --component vtgate
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

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

View the log of different components.

```bash
kbcli cluster list-logs myproxy
kbcli cluster list-logs myproxy --component vtgate
kbcli cluster list-logs myproxy --component vtcontroller
kbcli cluster list-logs myproxy --component mysql
```

View the log of a Pod.

```bash
kbcli cluster logs myproxy --instance myproxy-vtgate-85bdcf99df-wbmnl
```

View the log of a container in a Pod.

```bash
kbcli cluster logs myproxy --instance myproxy-mysql-0 -c vttablet
```

</TabItem>

<TabItem value="kubectl" label="kubectl" default>

View the log of VTGate.

```bash
kubectl logs myproxy-cluster-vtgate-8659d5db95-4dzt5
```

View the log of VTTablet and `-c` is required.

```bash
kubectl logs myproxy-cluster-mysql-0 -c vttablet
```

Enter the container and view more logs of VTGate.

```bash
kubectl exec -it myproxy-cluster-vtgate-8659d5db95-4dzt5 -- bash
ls /vtdataroot
```

Enter the container and view more logs of VTTable.

```bash
kubectl exec -it myproxy-cluster-mysql-0  -c vttablet -- bash
ls /vtdataroot
```

</TabItem>

</Tabs>

## Monitoring

:::note

In the production environment, all monitoring add-ons are disabled by default when installing KubeBlocks. You can enable these add-ons but it is highly recommended to build your monitoring system or purchase a third-party monitoring service. For details, refer to [Monitoring](./../../observability/monitor-database.md).

:::

1. Enable the monitoring function.

   ```bash
   kbcli cluster update myproxy --monitor=true
   ```

2. View the add-on list and enable the Grafana add-on.

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
kbcli cluster configure myproxy --component vtgate --set=read_write_splitting_policy=random
```

You can also set the ratio for read-write splitting and here is an example of directing 70% flow to the read-only node.

```bash
kbcli cluster configure myproxy --component vtgate --set=read_write_splitting_ratio=70
```

Moreover, you can [use Grafana](#monitoring) or run `show workload` to view the flow distribution.

```bash
show workload;
```

## Transparent failover

Run the command below to implement transparent failover.

```bash
kbcli cluster configure myproxy --component vtgate --set=enable_buffer=true
```
