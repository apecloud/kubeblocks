---
title: 在生产环境中连接数据库
description: 如何在生产环境中连接数据库
keywords: [连接数据库, 生产环境]
sidebar_position: 3
sidebar_label: 生产环境
---

# 在生产环境中连接数据库

在生产环境中，使用 CLI 和 SDK 客户端连接数据库都很常见。主要有以下三种场景：

在生产环境中，使用 CLI 和 SDK 客户端连接数据库是很常见的。主要有以下三种场景：

- 场景 1：Client1 和数据库位于同一个 Kubernetes 集群中。如果要连接 Client1 和数据库，请参考[方案 3](#procedure-3-connect-database-in-the-same-kubernetes-cluster)。
- 场景 2：Client2 在 Kubernetes 集群之外，但与数据库位于同一个 VPC 中。如果要连接 Client2 和数据库，请参考[方案 5](#procedure-5-client-outside-the-kubernetes-cluster-but-in-the-same-vpc-as-the-kubernetes-cluster)。
- 场景 3：Client3 和数据库位于不同的 VPC，例如其他 VPC 或公共网络。如果要连接 Client3 和数据库，请参考[方案 4](#procedure-4-connect-database-with-clients-in-other-vpcs-or-public-networks)。

可参考如下网络位置关系图。

![Example](./../../img/connect-to-database-in-production-env-network-locations.jpg)

## 方案 3. 连接在同一个 Kubernetes 集群中的客户端

您可以使用数据库的 ClusterIP 或域名进行连接。使用 `kbcli cluster describe ${cluster-name}` 检查数据库的端口。

```bash
kubectl get service mycluster-mysql
```

## 方案 4. 连接在其他 VPC 或公共网络中的客户端

您可以启用云厂商提供的外部负载均衡器。

:::note

以下命令能够为数据库创建负载均衡器实例，并可能会产生相关云服务费用。

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

如果要禁用负载均衡器实例，请执行以下命令。

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

禁用负载均衡器实例后，实例将无法访问。

:::

## 方案 5. 连接在 Kubernetes 集群之外但与 Kubernetes 集群位于同一 VPC 中的客户端

使用一个稳定的域名以实现长期连接。您可以使用云厂商提供的内部负载均衡器来实现这一目的。

:::note

以下命令会为数据库实例创建负载均衡器实例，并可能会产生相关云服务费用。

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

如果要禁用负载均衡器实例，请执行以下命令。

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

一旦禁用，实例将无法访问。

:::
