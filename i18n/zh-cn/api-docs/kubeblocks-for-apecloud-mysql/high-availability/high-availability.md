---
title: 故障模拟与自动恢复
description: 集群自动恢复
keywords: [mysql, 高可用, 故障模拟, 自动恢复]
sidebar_position: 1
---

# 故障模拟与自动恢复

作为一个开源数据管理平台，Kubeblocks目前支持三十多种数据库引擎，并且持续扩展中。由于这些数据库本身的高可用能力是参差不齐的，因此 KubeBlocks 设计实现了一套高可用系统用于保障数据库实例的高可用能力。KubeBlocks 高可用系统采用了统一的 HA 框架设计，实现对数据库的高可用支持，使得不同的数据库在 KubeBlocks 上可以获得类似的高可用能力和体验。

下面以 ApeCloud MySQL为例，演示它的故障模拟和恢复能力。

## 故障恢复

:::note

下面通过删除 Pod 来模拟故障。在资源充足的情况下，也可以通过机器宕机或删除容器来模拟故障，其自动恢复过程与本文描述的相同。

:::

### 开始之前

* [安装 KubeBlocks](./../../installation/install-kubeblocks.md)。
* [创建 ApeCloud MySQL 集群版](./../cluster-management/create-and-connect-an-apecloud-mysql-cluster.md)。
* 执行 `kubectl get cd apecloud-mysql -o yaml` 检查 ApeCloud MySQL 集群版是否已启用 _rolechangedprobe_（默认情况下是启用的）。如果出现以下配置信息，则表明已启用：

  ```bash
  probes:
    roleProbe:
      failureThreshold: 2
      periodSeconds: 1
      timeoutSeconds: 1
  ```

### Leader 节点异常

***步骤：***

1. 查看 ApeCloud MySQL 集群版 pod 角色。在本示例中，Leader 节点为 `mycluster-1`。

    ```bash
    kubectl get pods --show-labels -n demo | grep role
    ```

    ![describe_pod](./../../../img/api-ha-grep-role.png)
2. 删除 Leader 节点 `mycluster-mysql-1`，模拟节点故障。

    ```bash
    kubectl delete pod mycluster-mysql-1 -n demo
    ```

    ![delete_pod](./../../../img/api-ha-delete-leader-pod.png)
3. 检查 pod 状态和集群连接。

    此处示例显示 pod 角色发生变化。原 Leader 节点删除后，系统选出新的 Leader 为 `mycluster-mysql-0`。

    ```bash
    kubectl get pods --show-labels -n demo | grep role
    ```

    ![describe_cluster_after](./../../../img/api-ha-delete-leader-pod-after.png)

    连接到该集群，检查 pod 角色和状态。该集群可在几秒内连接成功。

    ```bash
    kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
    >
    root

    kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
    >
    pt2mmdlp4

    kubectl exec -ti -n demo mycluster-mysql-0 -- bash

    mysql -uroot -pt2mmdlp4
    ```

    ![connect_cluster_after](./../../../img/api-ha-leader-pod-connect-check.png)

   ***自动恢复机制***

   Leader 节点删除后，ApeCloud MySQL 集群版会自行选主。上述示例中，选出新的 Leader 为 `mycluster-mysql-0`，KubeBlocks 探测到 Leader 角色发生变化，会发出通知，更新访问链路。原先异常节点会自动重建，恢复正常集群版状态。从异常开始到恢复完成，整体耗时正常在 30s 之内。

### 单个 follower 节点异常

***步骤：***

1. 查看 pod 角色，如下示例中 follower 节点为 `mycluster-mysql-1` and `mycluster-mysql-2`。

    ```bash
    kubectl get pods --show-labels -n demo | grep role
    ```

    ![describe_cluster](./../../../img/api-ha-grep-role-single-follower-pod.png)
2. 删除 follower 节点 `mycluster-mysql-1`。

    ```bash
    kubectl delete pod mycluster-mysql-1 -n demo
    ```

    ![delete_follower_pod](./../../../img/api-ha-single-follower-pod-delete.png)
3. 打开一个新的终端窗口，查看 pod 状态。可以发现 follower 节点 `mycluster-mysql-1` 处于 `Terminating` 状态。

    ```bash
    kubectl get pod -n demo
    ```

    ![view_cluster_follower_status](./../../../img/api-delete-single-follower-pod-status.png)

    再次查看 pod 角色。

    ![describe_cluster_follower](./../../../img/api-ha-single-follower-pod-grep-role-after.png)

4. 连接集群，发现单个 Follower 节点异常不影响集群的读写操作。

    ```bash
    kubectl exec -ti -n demo mycluster-mysql-0 -- bash

    mysql -uroot -pt2mmdlp4
    ```

    ![connect_cluster_follower](./../../../img/api-ha-connect-single-follower-pod.png)

   ***自动恢复机制***

   单个 Follower 节点异常不会触发角色重新选主，也不会切换访问链路，所以集群读写不受影响，Follower 节点异常后会自动触发重建，恢复正常，整体耗时正常 30s 之内。

### 两个节点异常

集群可用一般要求满足多数节点状态正常，当多数节点异常时，原 Leader 节点会自动降级为 Follower 节点，因此任意两个节点异常都会导致仅存一个 Follower 节点。所以一个 Leader 节点一个 Follower 节点异常和两个 Follower 节点异常，故障表现和自动恢复情况是一样的。

***步骤：***

1. 查看 Pod 状态。如下示例中，follower 节点为 `mycluster-mysql-1` 和 `mycluster-mysql-2`。

    ```bash
    kubectl get pods --show-labels -n demo | grep role
    ```

    ![describe_cluster](./../../../img/api-ha-two-pods-grep-role.png)
2. 删除两个 follower 节点。

    ```bash
    kubectl delete pod mycluster-mysql-1 mycluster-mysql-2 -n demo
    ```

    ![delete_two_pods](./../../../img/api-ha-two-pod-get-status.png)
3. 打开一个新的终端窗口，查看 pod 状态，发现两个 follower 节点 `mycluster-mysql-1` 和 `mycluster-mysql-2` 都处于 `Terminating` 状态。

    ```bash
    kubectl get pod -n demo
    ```

    ![view_cluster_follower_status](./../../../img/api-ha-two-pod-get-status.png)

    查看节点角色，发现已选举产生新的 leader。

    ```bash
    kubectl get pods --show-labels -n demo | grep role
    ```

    ![describe_cluster_follower](./../../../img/api-ha-two-pods-grep-role-after.png)

4. 稍等几秒后，连接集群，发现集群中的 pod 在此正常运行。

    ```bash
    kubectl exec -ti -n demo mycluster-mysql-0 -- bash

    mysql -uroot -pt2mmdlp4
    ```

    ![connect_two_pods](./../../../img/api-ha-two-pods-connect-after.png)

   ***自动恢复机制***

   当 ApeCloud MySQL 集群版中两个节点异常时，会满足数节点不可用，导致 Leader 会自动降级为 Follower，此时集群不可读写。待 Pod 自动重建完成后，集群重新选出 Leader 并恢复到可 Read Write 状态。整体耗时正常 30s 之内。

### 所有节点异常

***步骤：***

1. 查看节点角色。

    ```bash
    kubectl get pods --show-labels -n demo | grep role
    ```

    ![describe_cluster](./../../../img/api-ha-all-pods-grep-role.png)
2. 删除所有节点。

    ```bash
    kubectl delete pod mycluster-mysql-1 mycluster-mysql-0 mycluster-mysql-2 -n demo
    ```

    ![delete_three_pods](./../../../img/api-ha-all-pods-delete.png)
3. 打开一个新的终端窗口，查看 pod 状态，发现所有 pod 均处于 `Terminating` 状态。

    ```bash
    kubectl get pod -n demo
    ```

    ![describe_three_clusters](./../../../img/api-ha-all-pods-get-status.png)
4. 再次查看节点角色，发现已选举产生新的 leader。

    ```bash
    kubectl get pods --show-labels -n demo | grep role
    ```

    ![describe_cluster_follower](./../../../img/api-ha-all-pods-grep-role-after.png)
5. 稍等几秒后，连接集群，发现集群中的 pod 已恢复正常运行。

    ```bash
    kubectl exec -ti -n demo mycluster-mysql-0 -- bash

    mysql -uroot -pt2mmdlp4
    ```

    ![connect_three_clusters](./../../../img/api-ha-all-pods-connect-after.png)

   ***自动恢复机制***

   节点删除后，都会自动触发重建。随后，ApeCloud MySQL 会自动完成集群恢复及选主。选主完成后，KubeBlocks 会探测新 Leader，并更新访问链路，恢复可用。整体耗时正常 30s 之内。
