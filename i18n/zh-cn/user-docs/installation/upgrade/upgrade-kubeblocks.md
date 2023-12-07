---
title: 升级到 KubeBlocks v0.6
description: 升级 KubeBlocks, 操作, 注意事项
keywords: [upgrade, 0.6]
sidebar_position: 1
sidebar_label: 升级到 KubeBlocks v0.6
---

# 升级到 KubeBlocks v0.6

本章将介绍如何升级到 KubeBlocks 0.6 版本，并提供相关注意事项。

## 升级前

### 拉取镜像 
与 0.5 版本相比，KubeBlocks 0.6 版本有许多镜像变化。如果集群中存在许多数据库实例，在升级时同时拉取镜像会导致实例长时间不可用。因此，建议在升级之前拉取所需的 0.6 版本镜像。

***步骤***

1. 将以下内容写入一个 `yaml` 文件中。

    ```bash
    apiVersion: apps/v1
        kind: DaemonSet
        metadata:
          name: kubeblocks-image-prepuller
        spec:
          selector:
            matchLabels:
              name: kubeblocks-image-prepuller
          template:
            metadata:
              labels:
                name: kubeblocks-image-prepuller
            spec:
              volumes:
                - name: shared-volume
                  emptyDir: {}
              tolerations:
                - key: "kb-data"
                  operator: "Exists"
                  effect: "NoSchedule"
                - key: "kb-controller"
                  operator: "Exists"
                  effect: "NoSchedule"    
              initContainers:
                - name: pull-kb-tools
                  image: registry.cn-hangzhou.aliyuncs.com/apecloud/kubeblocks-tools:0.6.0
                  imagePullPolicy: IfNotPresent
                  command: ["cp", "-r", "/bin/kbcli", "/kb-tools/kbcli"]
                  volumeMounts:
                    - name: shared-volume
                      mountPath: /kb-tools
                - name: pull-1
                  image: apecloud/kubeblocks-csi-driver:0.1.0
                  imagePullPolicy: IfNotPresent
                  command: ["/kb-tools/kbcli"]
                  volumeMounts:
                    - name: shared-volume
                      mountPath: /kb-tools
                - name: pull-2
                  image: registry.cn-hangzhou.aliyuncs.com/apecloud/apecloud-mysql-scale:0.1.1
                  imagePullPolicy: IfNotPresent
                  command: ["/kb-tools/kbcli"]
                  volumeMounts:
                    - name: shared-volume
                      mountPath: /kb-tools
                - name: pull-4
                  image: registry.cn-hangzhou.aliyuncs.com/apecloud/apecloud-mysql-server:8.0.30-5.alpha8.20230523.g3e93ae7.8
                  imagePullPolicy: IfNotPresent
                  command: ["/kb-tools/kbcli"]
                  volumeMounts:
                    - name: shared-volume
                      mountPath: /kb-tools
                - name: pull-5
                  image: registry.cn-hangzhou.aliyuncs.com/apecloud/apecloud-mysql-server:8.0.30-5.beta1.20230802.g5b589f1.12
                  imagePullPolicy: IfNotPresent
                  command: ["/kb-tools/kbcli"]
                  volumeMounts:
                    - name: shared-volume
                      mountPath: /kb-tools
                - name: pull-6
                  image: registry.cn-hangzhou.aliyuncs.com/apecloud/kubeblocks-charts:0.6.0
                  imagePullPolicy: IfNotPresent
                  command: ["/kb-tools/kbcli"]
                  volumeMounts:
                    - name: shared-volume
                      mountPath: /kb-tools
                - name: pull-7
                  image: registry.cn-hangzhou.aliyuncs.com/apecloud/kubeblocks-datascript:0.6.0
                  imagePullPolicy: IfNotPresent
                  command: ["/kb-tools/kbcli"]
                  volumeMounts:
                    - name: shared-volume
                      mountPath: /kb-tools
                - name: pull-8
                  image: registry.cn-hangzhou.aliyuncs.com/apecloud/kubeblocks-tools:0.6.0
                  imagePullPolicy: IfNotPresent
                  command: ["/kb-tools/kbcli"]
                  volumeMounts:
                    - name: shared-volume
                      mountPath: /kb-tools
                - name: pull-9
                  image: registry.cn-hangzhou.aliyuncs.com/apecloud/kubeblocks:0.6.0
                  imagePullPolicy: IfNotPresent
                  command: ["/kb-tools/kbcli"]
                  volumeMounts:
                    - name: shared-volume
                      mountPath: /kb-tools
                - name: pull-10
                  image: registry.cn-hangzhou.aliyuncs.com/apecloud/mongo:5.0.14
                  imagePullPolicy: IfNotPresent
                  command: ["/kb-tools/kbcli"]
                  volumeMounts:
                    - name: shared-volume
                      mountPath: /kb-tools
                - name: pull-11
                  image: registry.cn-hangzhou.aliyuncs.com/apecloud/mysqld-exporter:0.14.1
                  imagePullPolicy: IfNotPresent
                  command: ["/kb-tools/kbcli"]
                  volumeMounts:
                    - name: shared-volume
                      mountPath: /kb-tools
                - name: pull-12
                  image: registry.cn-hangzhou.aliyuncs.com/apecloud/pgbouncer:1.19.0
                  imagePullPolicy: IfNotPresent
                  command: ["/kb-tools/kbcli"]
                  volumeMounts:
                    - name: shared-volume
                      mountPath: /kb-tools
                - name: pull-13
                  image: registry.cn-hangzhou.aliyuncs.com/apecloud/redis-stack-server:7.0.6-RC8
                  imagePullPolicy: IfNotPresent
                  command: ["/kb-tools/kbcli"]
                  volumeMounts:
                    - name: shared-volume
                      mountPath: /kb-tools
                - name: pull-14
                  image: registry.cn-hangzhou.aliyuncs.com/apecloud/spilo:12.14.0
                  imagePullPolicy: IfNotPresent
                  command: ["/kb-tools/kbcli"]
                  volumeMounts:
                    - name: shared-volume
                      mountPath: /kb-tools
                - name: pull-15
                  image: registry.cn-hangzhou.aliyuncs.com/apecloud/spilo:12.14.1
                  imagePullPolicy: IfNotPresent
                  command: ["/kb-tools/kbcli"]
                  volumeMounts:
                    - name: shared-volume
                      mountPath: /kb-tools
                - name: pull-16
                  image: registry.cn-hangzhou.aliyuncs.com/apecloud/spilo:12.15.0
                  imagePullPolicy: IfNotPresent
                  command: ["/kb-tools/kbcli"]
                  volumeMounts:
                    - name: shared-volume
                      mountPath: /kb-tools
                - name: pull-17
                  image: registry.cn-hangzhou.aliyuncs.com/apecloud/spilo:14.7.2
                  imagePullPolicy: IfNotPresent
                  command: ["/kb-tools/kbcli"]
                  volumeMounts:
                    - name: shared-volume
                      mountPath: /kb-tools
                - name: pull-18
                  image: registry.cn-hangzhou.aliyuncs.com/apecloud/spilo:14.8.0
                  imagePullPolicy: IfNotPresent
                  command: ["/kb-tools/kbcli"]
                  volumeMounts:
                    - name: shared-volume
                      mountPath: /kb-tools
                - name: pull-19
                  image: registry.cn-hangzhou.aliyuncs.com/apecloud/wal-g:mongo-latest
                  imagePullPolicy: IfNotPresent
                  command: ["/kb-tools/kbcli"]
                  volumeMounts:
                    - name: shared-volume
                      mountPath: /kb-tools
                - name: pull-20
                  image: registry.cn-hangzhou.aliyuncs.com/apecloud/wal-g:mysql-latest
                  imagePullPolicy: IfNotPresent
                  command: ["/kb-tools/kbcli"]
                  volumeMounts:
                    - name: shared-volume
                      mountPath: /kb-tools
                - name: pull-21
                  image: registry.cn-hangzhou.aliyuncs.com/apecloud/agamotto:0.1.2-beta.1
                  imagePullPolicy: IfNotPresent
                  command: ["/kb-tools/kbcli"]
                  volumeMounts:
                    - name: shared-volume
                      mountPath: /kb-tools
              containers:
                - name: pause
                  image: k8s.gcr.io/pause:3.2    
    ```     

2. 将该 `yaml` 文件应用于升级前预先拉取的镜像。

    ```bash
    kubectl apply -f prepull.yaml
    >
    daemonset.apps/kubeblocks-image-prepuller created
    ```

3. 检查拉取镜像的状态。

    ```bash
    kubectl get pod
    >
    NAME                               READY   STATUS    RESTARTS      AGE
    kubeblocks-image-prepuller-6l5xr   1/1     Running   0             11m
    kubeblocks-image-prepuller-7t8t2   1/1     Running   0             11m
    kubeblocks-image-prepuller-pxbnp   1/1     Running   0             11m
    ```
4. 删除为拉取镜像创建的 Pod。

    ```bash
    kubectl delete daemonsets.apps kubeblocks-image-prepuller
    ```

## 升级

由于 KubeBlocks 数据库集群的 0.6 版本有许多镜像变化，在升级过程中 Pod 将逐步重启，导致集群会在短时间内不可用。其持续时间由网络条件和集群大小决定。KubeBlocks 控制器的升级时间约为 2 分钟，单个数据库集群的不可用时间约为 20 秒至 200 秒，其完全恢复时间约为 20 秒至 300 秒。如果你在升级之前拉取镜像的话（可参考上一部分），单个数据库集群的不可用时间约为 20 秒至 100 秒，其完全恢复时间约为 20 秒至 150 秒。

***步骤***

1. 安装新版本的 `kbcli`。

    ```bash
    curl -fsSL https://kubeblocks.io/installer/install_cli.sh |bash -s 0.6.0
    ``` 

2. 升级 KubeBlocks。

    ```bash
    kbcli kubeblocks upgrade --version 0.6.0
    ```

## 需要注意的功能变更

### RBAC
在 0.6 版本中，KubeBlocks 将自动管理集群所需的 RBAC。主要新增了以下两个集群角色：
- 用于 Pod 的 ClusterRole/kubeblocks-cluster-pod-role；
- 用于全盘锁定的 ClusterRole/kubeblocks-volume-protection-pod-role。

当触发集群协调时，KubeBlocks 会创建 rolebinding 和 serviceAccount 来绑定 ClusterRole/kubeblocks-cluster-pod-role。

你需要创建一个 clusterrolebinding 来绑定 ClusterRole/kubeblocks-volume-protection-pod-role。

### 全盘锁定
KubeBlocks 0.6 版本新增了 MySQL、PostGreSQL 和 MongoDB 数据库的全盘锁定功能 disk full lock feature。你可以通过修改集群定义来启用该功能。由于每个数据库的实现方法略有不同，请注意以下说明：
- 对于 MySQL 数据库，在磁盘使用达到 `highwatermark` 时，读写账户无法继续写入磁盘，但超级用户仍然可以写入。
- 对于 PostGreSQL 和 MongoDB 数据库，在磁盘使用达到 `highwatermark` 时，无论是读写用户还是超级用户都无法写入。
- 组件的 `highwatermark` 默认设置为 `90`，表示磁盘使用率达到 90% 时将触发阈值。而卷的默认设置为 85，覆盖了组件的阈值设置。

在集群定义中添加以下内容，就可以启用全盘锁定功能。你可以根据实际情况设置相应的值。

```bash
volumeProtectionSpec:
  highWatermark: 90
  volumes:
  - highWatermark: 85
    name: data
```
:::note

`highWatermark` 建议设置为 90。

:::

### 备份恢复
KubeBlocks 0.6 版本对备份恢复功能进行了大幅度调整和升级。把 0.5 版本创建的数据库集群升级到 0.6 版本后，你需要进行手动调整，否则将无法使用 0.6 版本的新功能。

数据库 | v0.5                                                                                                          | v0.6                                                                                                                                 |
|----------|---------------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------|
MySQL    | 快照备份和恢复 全量备份和恢复                                                           | 快照备份和恢复<br />全量备份和恢复 日志文件备份 <br />基于快照的 PITR <br />基于全量数据的 PITR |
PG       | 快照备份和恢复  <br />全量备份和恢复 <br />日志文件备份 <br />基于快照的 PITR | 快照备份和恢复 <br /> 全量备份和恢复 <br />日志文件备份 <br />基于快照的 PITR                          |
Redis    |  快照备份和恢复                                                                                   |  快照备份和恢复 <br />全量备份和恢复                                                                            |
Mongo    |  全量备份和恢复                                                                                    |  快照备份和恢复<br /> 全量备份和恢复 日志文件备份<br />基于快照的 PITR <br />基于全量数据的 PITR |

### 密码验证
0.6 版本取消了 0.5 版本中无密码登录 Postgres 集群的功能，在创建 Postgres 时，你需要使用帐户密码。在你升级到 0.6 版本后，如果备份恢复时集群仍处于 `creating` 状态，你可以检查集群的 Pod 日志中是否存在密码验证失败，并通过更新密码解决此问题。我们将在下一个版本中修复该问题。

检查是否存在密码验证失败。

```bash
kubectl logs <pod-name> kb-checkrole
...
server error (FATAL: password authentication failed for user \"postgres\" (SQLSTATE 28P01))"
...
```

更新密码。
```bash
kubectl exec -it <primary_pod_name> -- bash
psql -c "ALTER USER postgres WITH PASSWORD '${PGPASSWORD}';"
```
