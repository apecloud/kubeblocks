---
title: Connect database in production environment
description: How to connect to a database in production environment
keywords: [connect to a database, production environment]
sidebar_position: 3
sidebar_label: Production environment
---

# Connect database in production environment

In the production environment, it is normal to connect a database with CLI and SDK clients. There are three scenarios.

- Scenario 1: Client1 and the database are in the same Kubernetes cluster. To connect client1 and the database, see [Procedure 3](#procedure-3-connect-database-in-the-same-kubernetes-cluster).
- Scenario 2: Client2 is outside the Kubernetes cluster, but it is in the same VPC as the database. To connect client2 and the database, see [Procedure 5](#procedure-5-client-outside-the-kubernetes-cluster-but-in-the-same-vpc-as-the-kubernetes-cluster).
- Scenario 3: Client3 and the database are in different VPCs, such as other VPCs or the public network. To connect client3 and the database, see [Procedure 4](#procedure-4-connect-database-with-clients-in-other-vpcs-or-public-networks).

See the figure below to get a clear image of the network location.

![Example](./../../img/connect_database_in_a_production_environment.png)

## Procedure 3. Connect database in the same Kubernetes cluster

You can connect with the database ClusterIP or domain name. To check the database endpoint, use `kubectl get service <cluster-name>-<component-name>`.

```bash
kubectl get service mycluster-mysql
```

## Procedure 4. Connect database with clients in other VPCs or public networks

You can enable the External LoadBalancer of the cloud vendor.

:::note

The following command creates a LoadBalancer instance for the database instance, which may incur expenses from your cloud vendor.

:::

```yaml
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: ops-expose
spec:
  clusterRef: mycluster
  expose:
  - componentName: mysql
    services:
    - annotations:
        service.beta.kubernetes.io/alibaba-cloud-loadbalancer-address-type: intranet
      ipFamilyPolicy: PreferDualStack
      name: vpc
      serviceType: LoadBalancer
    switch: Enable
  ttlSecondsBeforeAbort: 0
  type: Expose
EOF
```

To disable the LoadBalancer instance, execute the following command.

```yaml
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: ops-expose
spec:
  clusterRef: mycluster
  expose:
  - componentName: mysql
    services:
    - annotations:
        service.beta.kubernetes.io/alibaba-cloud-loadbalancer-address-type: intranet
      ipFamilyPolicy: PreferDualStack
      name: vpc
      serviceType: LoadBalancer
    switch: Disable
  ttlSecondsBeforeAbort: 0
  type: Expose
EOF
```

:::note

The instance is inaccessible after you disable the LoadBalancer instance.

:::

## Procedure 5. Client outside the Kubernetes cluster but in the same VPC as the Kubernetes cluster

A stable domain name for long-term connections is required. An Internal LoadBalancer provided by the cloud vendor can be used for this purpose.

:::note

The following command creates a LoadBalancer instance for the database instance, which may incur expenses from your cloud vendor.

:::

```yaml
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: ops-expose
spec:
  clusterRef: mycluster
  expose:
  - componentName: mysql
    services:
    - annotations:
        service.beta.kubernetes.io/alibaba-cloud-loadbalancer-address-type: internet
      ipFamilyPolicy: PreferDualStack
      name: vpc
      serviceType: LoadBalancer
    switch: Enable
  ttlSecondsBeforeAbort: 0
  type: Expose
EOF
```

To disable the LoadBalancer instance, execute the following command.

```yaml
kubectl apply -f - <<EOF
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: ops-expose
spec:
  clusterRef: mycluster
  expose:
  - componentName: mysql
    services:
    - annotations:
        service.beta.kubernetes.io/alibaba-cloud-loadbalancer-address-type: internet
      ipFamilyPolicy: PreferDualStack
      name: vpc
      serviceType: LoadBalancer
    switch: Disable
  ttlSecondsBeforeAbort: 0
  type: Expose
EOF
```

:::note

Once disabled, the instance is not accessible.

:::
