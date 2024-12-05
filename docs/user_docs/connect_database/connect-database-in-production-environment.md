---
title: Connect database in production environment
description: How to connect to a database in production environment
keywords: [connect to a database, production environment]
sidebar_position: 3
sidebar_label: Production environment
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Connect database in production environment

In the production environment, it is normal to connect a database with CLI and SDK clients. There are three scenarios.

- Scenario 1: Client1 and the database are in the same Kubernetes cluster. To connect client1 and the database, see [Use ClusterIP](#scenario-1-connect-database-in-the-same-kubernetes-cluster).
- Scenario 2: Client2 is outside the Kubernetes cluster, but it is in the same VPC as the database. To connect client2 and the database, see [Expose VPC Private Address](#scenario-2-client-outside-the-kubernetes-cluster-but-in-the-same-vpc-as-the-kubernetes-cluster).
- Scenario 3: Client3 and the database are in different VPCs, such as other VPCs or the public network. To connect client3 and the database, see [Expose VPC Public Address](#scenario-3-connect-database-with-clients-in-other-vpcs-or-public-networks).

See the figure below to get a clear image of the network location.

![Example](./../../img/connect_database_in_a_production_environment.png)

## Scenario 1. Connect database in the same Kubernetes cluster

You can connect with the database ClusterIP or domain name.

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

To check the database endpoint, use `kubectl get service <cluster-name>-<component-name>`.

```bash
kubectl get service mycluster-mysql
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

To check the database endpoint, use `kbcli cluster describe ${cluster-name}`.

```bash
kbcli cluster describe x
>
Name: x         Created Time: Mar 01,2023 11:45 UTC+0800
NAMESPACE   CLUSTER-DEFINITION   VERSION           STATUS    TERMINATION-POLICY
default     apecloud-mysql       ac-mysql-8.0.30   Running   Delete

Endpoints:
COMPONENT   MODE        INTERNAL                                 EXTERNAL
x           ReadWrite   x-mysql.default.svc.cluster.local:3306   <none>

Topology:
COMPONENT   INSTANCE    ROLE     STATUS    AZ                NODE                                                       CREATED-TIME
mysql       x-mysql-0   leader   Running   cn-northwest-1b   ip-10-0-2-184.cn-northwest-1.compute.internal/10.0.2.184   Mar 01,2023 11:45 UTC+0800

Resources Allocation:
COMPONENT   DEDICATED   CPU(REQUEST/LIMIT)   MEMORY(REQUEST/LIMIT)   STORAGE-SIZE   STORAGE-CLASS
mysql       false       1 / 1                1Gi / 1Gi               data:10Gi      <none>

Images:
COMPONENT   TYPE    IMAGE
mysql       mysql   registry.cn-hangzhou.aliyuncs.com/apecloud/apecloud-mysql-server:8.0.30-5.alpha2.20230105.gd6b8719.2

Events(last 5 warnings, see more:kbcli cluster list-events -n default x):
TIME   TYPE   REASON   OBJECT   MESSAGE
```

</TabItem>

</Tabs>

## Scenario 2. Client outside the Kubernetes cluster but in the same VPC as the Kubernetes cluster

A stable domain name for long-term connections is required. An Internal LoadBalancer provided by the cloud vendor can be used for this purpose.

:::note

The following command creates a LoadBalancer instance for the database instance, which may incur expenses from your cloud vendor.

:::

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

This example uses a MySQL cluster to demonstrate how to expose a VPC address on Alibaba Cloud.

```yaml
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: ops-expose-enable
  namespace: demo
spec:
  clusterName: mycluster
  expose:
  - componentName: mysql
    services:
    - name: vpc
      roleSelector: leader
      serviceType: LoadBalancer
    switch: Enable
  preConditionDeadlineSeconds: 0
  type: Expose
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster expose ${cluster-name} --type vpc --enable=true
```

</TabItem>

</Tabs>

To disable the LoadBalancer instance, execute the following command.

:::note

Once disabled, the instance is not accessible.

:::

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

```yaml
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: ops-expose-disable
  namespace: demo
spec:
  clusterName: mycluster
  expose:
  - componentName: mysql
    services:
    - name: vpc
      roleSelector: leader
      serviceType: LoadBalancer
    switch: Disable
  preConditionDeadlineSeconds: 0
  type: Expose
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster expose ${cluster-name} --type vpc --enable=false
```

</TabItem>

</Tabs>

## Scenario 3. Connect database with clients in other VPCs or public networks

If the client is in a different VPC and requires public access, you can enable the cloud vendor's External LoadBalancer.

:::note

The following command creates a LoadBalancer instance for the database instance, which may incur expenses from your cloud vendor.

:::

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

The example uses MySQL to demonstrate how to expose the public address on Alibaba Cloud.

```yaml
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: ops-expose-enable
  namespace: demo
spec:
  clusterName: mycluster
  expose:
  - componentName: mysql
    services:
    - name: internet
      roleSelector: leader
      serviceType: LoadBalancer
    switch: Enable
  preConditionDeadlineSeconds: 0
  type: Expose
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster expose ${cluster-name} --type internet --enable=true
```

</TabItem>

</Tabs>

To disable the LoadBalancer instance, execute the following command.

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

```yaml
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: ops-expose-disable
  namespace: demo
spec:
  clusterName: mycluster
  expose:
  - componentName: mysql
    services:
    - name: internet
      roleSelector: leader
      serviceType: LoadBalancer
    switch: Disable
  preConditionDeadlineSeconds: 0
  type: Expose
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli cluster expose ${cluster-name} --type internet --enable=false
```

</TabItem>

</Tabs>

:::note

The instance is inaccessible after you disable the LoadBalancer instance.

:::
