---
title: How to use a MySQL Proxy Cluster
description: MySQL Proxy Cluster tutorial
keywords: [mysql proxy, proxy]
sidebar_position: 2
sidebar_label: MySQL Proxy Cluster
---

# MySQL Proxy

## Before you start

1. Install KubeBlocks by [Helm](./../../installation/install-with-helm/install-kubeblocks-with-helm).
2. [Prepare a MySQL RaftGroup](./../cluster-management/create-and-connect-a-mysql-cluster.md#create-a-mysql-cluster) named `mycluster` to demonstrate how to enable the proxy function for an existing cluster.

## Create a Proxy Cluster

1. View the repository list to verify whether the KubeBlocks repository has been added successfully.

   ```bash
   helm repo list
   ```

2. Run the update command to make sure you have added the latest version.

   ```bash
   helm repo update
   ```

3. View all versions of MySQL Proxy.

   ```bash
   helm search repo kubeblocks/mysql-cluster --devel --versions
   ```

4. Specify a version and install the cluster definition of MySQL.

   ```bash
   helm install myproxy kubeblocks/mysql --version=v0.9.0
   ```

5. Create a MySQL Proxy Cluster.

   ```bash
   helm install myproxy kubeblocks/mysql-cluster --version=v0.9.0 --set mode=raftGroup,proxyEnabled=true 
   ```

:::note

If you only have one node for deploying a RaftGroup, set the `availability-policy` as `none` when creating a RaftGroup.

```bash
helm install myproxy kubeblocks/mysql-cluster --version=v0.9.0 --set mode=raftGroup,proxyEnabled=true --set extra.availabilityPolicy=none
```

:::

## Enable Proxy dynamically

As its name suggests, MySQL Proxy in nature is a database proxy. A MySQL RaftGroup Cluster can be switched to a MySQL Proxy Cluster by setting `proxyEnabled=true`.

```bash
helm upgrade mycluster kubeblocks/mysql-cluster --set mode=raftGroup,proxyEnabled=true
```

## Connect Proxy Cluster

MySQL Proxy is routed through the `vtgate` component, and the way the MySQL Server accesses `vtgate` is similar to the way of accessing `mysqld`. The external SQL access address provided by MySQL Proxy is the `vtgate` address and port. The `vtgate` address created by KubeBlocks by default is `myproxy-cluster-vtgate-headless`, and the port number is `15306`. You can visit MySQL Proxy through the MySQL Server in any pod under the same namespace as MySQL Proxy.

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

## Log

You can view the log files of components, Pods, and containers by both kbcli and kubectl.

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

## Monitoring

:::note

In the production environment, all monitoring add-ons are disabled by default when installing KubeBlocks. You can enable these add-ons but it is highly recommended to build your monitoring system or purchase a third-party monitoring service. For details, refer to [Monitoring](./../../observability/monitor-database.md).

:::

## Read-write splitting

You can enable the read-write splitting function.

## Transparent failover

Run the command below to implement transparent failover.
