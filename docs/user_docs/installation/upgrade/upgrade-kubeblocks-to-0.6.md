---
title: Upgrade to KubeBlocks v0.6
description: Upgrade to KubeBlocks, operation, tips and notes
keywords: [upgrade, prepull images, feature changes between 0.5 and 0.6]
sidebar_position: 1
sidebar_label: Upgrade to KubeBlocks v0.6
---

# Upgrade to KubeBlocks v0.6

This chapter shows how to upgrade to the KubeBlocks 0.6, including notes and tips.

## Before upgrading

### Pull images 

KubeBlocks version 0.6 has many image changes from version 0.5. During upgrading, if there are many database instances in the cluster, the instance may be unavailable for a long time if images are pulled at the same time. It is recommended to pull the images required for version 0.6 in advance before upgrading.
***Steps***

1. Write the following content to a `yaml` file.

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

2. Apply the `yaml` file to pre-pull images required before upgrading.

    ```bash
    kubectl apply -f prepull.yaml
    >
    daemonset.apps/kubeblocks-image-prepuller created
    ```

3. Check the pulling status.

    ```bash
    kubectl get pod
    >
    NAME                               READY   STATUS    RESTARTS      AGE
    kubeblocks-image-prepuller-6l5xr   1/1     Running   0             11m
    kubeblocks-image-prepuller-7t8t2   1/1     Running   0             11m
    kubeblocks-image-prepuller-pxbnp   1/1     Running   0             11m
    ```

4. Delete the pod created for pulling image.

    ```bash
    kubectl delete daemonsets.apps kubeblocks-image-prepuller 
    ```

## Upgrade

Since version 0.6 of the KubeBlocks DB cluster has many image changes, the Pod will be gradually restarted during the upgrade, resulting in the DB cluster unavailable for a short period of time. The unavailable time of the DB cluster is decided by network conditions and cluster size. The upgrade time of the KubeBlocks controller is about 2 minutes, the unavailable time of a single DB cluster is about 20s to 200s, and the complete recovery time of a single DB cluster is about 20s to 300s. If you pull images before you upgrade, see the section above, the unavailable time of a single DB cluster is about 20s to 100s, and the complete recovery time of a single DB cluster is about 20s to 150s.

***Steps***

1. Install the new version of `kbcli`.

    ```bash
    curl -fsSL https://kubeblocks.io/installer/install_cli.sh |bash -s 0.6.0
    ```

2. Upgrade KubeBlocks.

    ```bash
    kbcli kubeblocks upgrade --version 0.6.0
    ```

## Feature changes require attention

### RABC

In version 0.6, KubeBlocks automatically manages the RBAC required by the cluster. Added the following 2 cluster roles.
- ClusterRole/kubeblocks-cluster-pod-role for pod.
- ClusterRole/kubeblocks-volume-protection-pod-role for full disk lock.

When cluster reconcile is triggered, KubeBlocks creates rolebinding and serviceAccount to bind ClusterRole/kubeblocks-cluster-pod-role.

You need to create a clusterrolebinding to bind ClusterRole/kubeblocks-volume-protection-pod-role.

### Full disk lock

Kubeblocks version 0.6 adds the disk full lock feature of MySQL, PostGreSQL, and MongoDB databases, which needs to be enabled by modifying the cluster definition. Due to the slight differences in the implementation methods of each database, pay attention to the following instructions:

- For MySQL database, readwrite account cannot write to the disk when the disk usage reaches the `highwatermark` value, while superuser can still write.
- FOr PostGreSQL database and MongoDB, both readwrite user and superruser cannot write when disk usage reaches `highwatermark`.
- `90` is the default value setting for highwatermark at the component level which means 90% of disk usage is the threshold, while `85` is used for the volumes setting which will overwrites the component's threshold value.
In the cluster definition, add following content to enable full disk lock function. You can set the value according to the real situation.

```bash
volumeProtectionSpec:
  highWatermark: 90
  volumes:
  - highWatermark: 85
    name: data
```

:::note

The recommended value of `highWatermark` is 90.

:::

### Backup and restore

The backup and restore function of Kubeblocks version 0.6 has been greatly adjusted and upgraded. After the database cluster created in version 0.5 is upgraded to version 0.6, manual adjustment is required, otherwise the new functions of version 0.6 cannot be used.

| Database | v0.5                                                                                                          | v0.6                                                                                                                                 |
|----------|---------------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------|
| MySQL    | Snapshot backup and restore full backup and restore                                                           | Snapshot backup and restore <br />Full backup and restore Log file backup <br />PITR based on snapshot <br />PITR based on full data |
| PG       | Snapshot backup and restore  <br />Full backup and restore <br />Log file backup <br />PITR based on snapshot | Snapshot backup and restore <br />Full backup and restore <br />Log file backup <br />PITR based on snapshot                         |
| Redis    | Snapshot backup and restore                                                                                   | Snapshot backup and restore <br />Full backup and restore                                                                            |
| Mongo    | Full backup and restore                                                                                       | Snapshot backup and restore <br />Full backup and restore Log file backup <br />PITR based on snapshot <br />PITR based on full data |

### Password authentication

The 0.6 version cancels the password-free login capability of the Postgres cluster. Compared with the password-free access of the 0.5 version, the newly created Postgres of 0.6 requires an account password to access. And after you upgrade to 0.6 version, if the cluster remains in` creating` status when restore from a backup, you can check whether there is a password authentication failed error in the pod log of the cluster. This problem can be solved by updating the password. We fix the problem in the next release.
Check whether there is a password authentication error.

```bash
kubectl logs <pod-name> kb-checkrole
...
server error (FATAL: password authentication failed for user \"postgres\" (SQLSTATE 28P01))"
...
```

Update the password.

```bash
kubectl exec -it <primary_pod_name> -- bash
psql -c "ALTER USER postgres WITH PASSWORD '${PGPASSWORD}';"
```
