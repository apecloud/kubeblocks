---
title: Expand volume
description: How to expand the volume of a MySQL cluster
sidebar_position: 3
sidebar_label: Expand volume
---

# Expand volume

You can expand the storage volume size of each pod.

:::note

Volume expansion triggers pod restart, all pods restart in the order of learner -> follower -> leader and the leader pod may change after the operation.

:::

## Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kubectl get cluster mycluster
>
NAME        CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS    AGE
mycluster   mysql                mysql-8.0.33   Delete               Running   4d18h
```

## Steps

1. Change configuration. There are 3 ways to apply volume expansion.

    <Tabs>

    <TabItem value="OpsRequest" label="OpsRequest" default>

    Change the value of storage according to your need and run the command below to expand the volume of a cluster.

    ```bash
    kubectl apply -f - <<EOF
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: OpsRequest
    metadata:
      name: ops-volume-expansion
    spec:
      clusterRef: mysql-cluster
      type: VolumeExpansion
      volumeExpansion:
      - componentName: mysql
        volumeClaimTemplates:
        - name: data
          storage: "2Gi"
    EOF
    ```

    </TabItem>

    <TabItem value="Cluster YAML File" label="Cluster YAML File">

    Change the value of `spec.componentSpecs.volumeClaimTemplates.spec.resources` in the cluster YAML file.

    `spec.componentSpecs.volumeClaimTemplates.spec.resources` is the storage resource information of the pod and changing this value triggers the volume expansion of a cluster.

    ```yaml
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: Cluster
    metadata:
      name: mysql-cluster
      namespace: default
    spec:
      clusterDefinitionRef: apecloud-mysql
      clusterVersionRef: ac-mysql-8.0.30
      componentSpecs:
      - name: mysql
        componentDefRef: mysql
        replicas: 1
        volumeClaimTemplates:
        - name: data
          spec:
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 1Gi # Change the volume storage size.
      terminationPolicy: Halt
    ```

    </TabItem>

    </Tabs>

2. Validate the volume expansion operation.

   ```bash
   kubectl get cluster mycluster
   >
   NAME        CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS    AGE
   mycluster   mysql                mysql-8.0.33   Delete               Running   4d19h
   ```

   * STATUS=Updating: it means the volume expansion is in progress.
   * STATUS=Running: it means the volume expansion operation has been applied.

3. Check whether the corresponding resources change.

   ```bash
   kubectl describe cluster mycluster
   ```
