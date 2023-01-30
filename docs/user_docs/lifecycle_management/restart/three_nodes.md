# Restart a three-node cluster

This guide shows how to use KubeBlocks to restart a three-node cluster.

## Before you start

- [Install KubeBlocks](../../installation/install_kubeblocks.md). 
- Run the commands below to check whether KubeBlocks is installed successfully and the cluster-related `CR` (custom resources) are created.
  - Run the commands to check whether KubeBlocks is installed successfully.
  ```
  $ kubectl get pod
    NAME                         READY   STATUS    RESTARTS   AGE
    kubeblocks-7644c4854-rkbfj   1/1     Running   0          6m57s
  $ kubectl get svc
    NAME         TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)    AGE
    kubeblocks   ClusterIP   10.111.120.68   <none>        9443/TCP   7m3s
    kubernetes   ClusterIP   10.96.0.1       <none>        443/TCP    3d22h
  ```
  - Run the commands below to check whether the cluster-related CR is installed successfully.
  ```
  $ kubectl get cd 
    NAME             STATUS       AGE
    apecloud-mysql   Available   7m13s
  $ kubectl get clusterversion
    NAME           STATUS       AGE
    ac-mysql-8.0.30   Available   7m23s
  $ kubectl get cm
    NAME                  DATA   AGE
    mysql-3node-tpl-8.0   1      7m28s
  ```
- Learn the following `KubeBlocks`concepts 
  - [KubeBlocks OpsRequest](../configure_ops_request.md) 
  - [Restarting overview](Overview.md) 

## Step 1. Create a three-node cluster for a demo

_Steps_:

1. Prepare a YAML file for a three-node cluster. Below is the YAML file of the single-node cluster. 

```
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: wesql-3nodes
spec:
  clusterVersionRef: ac-mysql-8.0.30
  clusterDefinitionRef: apecloud-mysql
  terminationPolicy: WipeOut
  components:
    - name: wesql-demo
      type: wesql
      monitor: false
      replicas: 3
      volumeClaimTemplates:
        - name: data
          spec:
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 1Gi
            volumeMode: Filesystem
```

2. Run the command line below to create a three-node cluster.

```
kubectl apply -f cluster_three_nodes.yaml
cluster.dbaas.kubeblocks.io/wesql-3nodes created
```
### Result

Wait a few seconds and when the cluster phase changes to  `Running`, the cluster is created successfully.

```
$ kubectl get cluster
NAME                   VERSION    STATUS     AGE
wesql-3nodes           ac-mysql-8.0.30   Running   20s
```

3. Check the operations this cluster supports:

```
$ kubectl describe cluster wesql-3nodes
....
Status:
  Cluster Def Generation:  2
  Components:
    Wesql - Demo:
      Consensus Set Status:
        Followers:
          Access Mode:  Readonly
          Name:         follower
          Pod:          wesql-3nodes-wesql-demo-1
          Access Mode:  Readonly
          Name:         follower
          Pod:          wesql-3nodes-wesql-demo-2
        Leader:
          Access Mode:  ReadWrite
          Name:         leader
          Pod:          wesql-3nodes-wesql-demo-0
      Phase:            Running
```
### Result

When the `status.phase` is `Running`, you can run `OpsRequest` to restart this cluster.

## Step 2. Restart a three-node cluster

_Steps_:
1. Prepare a YAML file for restarting a three-node cluster. Below is the YAML file of the `OpsRequest` CR. 

```
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: ops-restart-threenodes-demo
spec:
  clusterRef: wesql-3nodes
  type: Restart 
  componentOps:
  - componentNames: [wesql-demo]
```

2. Run the command line to apply `OpsRequest`:

```
$ kubectl apply -f restart_three_nodes.yaml
opsrequest.dbaas.kubeblocks.io/ops-restart-threenodes-demo created
```

3. View the `OpsRequest` phase and cluster phase:

```
$ kubectl get ops
NAME                   STATUS     AGE
ops-restart            Running   45s
```

```
$ kubectl get cluster
NAME                   VERSION    STATUS      AGE
wesql-3nodes           ac-mysql-8.0.30   Updating   16m 
```

### Results

When the phase changes to `Succeed`, the `OpsRequest` is applied successfully.

```
$ kubectl get ops
NAME                              STATUS     AGE
ops-restart-threenodes            Succeed   96s
```

And the cluster also changes:

```
$ kubectl get cluster
NAME                   VERSION    STATUS      AGE
wesql-3nodes           ac-mysql-8.0.30   Running    17m
```

4. View the details of `OpsRequest`.

```
$ kubectl describe ops ops-restart-threenodes-demo 
Name:         ops-restart-threenodes-demo
Namespace:    default
Labels:       cluster.kubeblocks.io/name=wesql-3nodes
Annotations:  <none>
API Version:  dbaas.kubeblocks.io/v1alpha1
Kind:         OpsRequest
Metadata:
  Creation Timestamp:  2022-11-17T07:09:53Z
  Finalizers:
    opsrequest.kubeblocks.io/finalizer
  Generation:  1
  Managed Fields:
    API Version:  dbaas.kubeblocks.io/v1alpha1
    Fields Type:  FieldsV1
    fieldsV1:
      f:metadata:
        f:annotations:
          .:
          f:kubectl.kubernetes.io/last-applied-configuration:
      f:spec:
        .:
        f:clusterRef:
        f:componentOps:
        f:type:
    Manager:      kubectl-client-side-apply
    Operation:    Update
    Time:         2022-11-17T07:09:53Z
    API Version:  dbaas.kubeblocks.io/v1alpha1
    Fields Type:  FieldsV1
    fieldsV1:
      f:metadata:
        f:finalizers:
          .:
          v:"opsrequest.kubeblocks.io/finalizer":
        f:labels:
          .:
          f:cluster.kubeblocks.io/name:
        f:ownerReferences:
          .:
          k:{"uid":"fef1ca58-2529-4058-864c-5dcaf9938ed3"}:
    Manager:      manager
    Operation:    Update
    Time:         2022-11-17T07:09:53Z
    API Version:  dbaas.kubeblocks.io/v1alpha1
    Fields Type:  FieldsV1
    fieldsV1:
      f:status:
        .:
        f:StartTimestamp:
        f:completionTimestamp:
        f:components:
          .:
          f:wesql-demo:
            .:
            f:phase:
        f:conditions:
        f:observedGeneration:
        f:phase:
    Manager:      manager
    Operation:    Update
    Subresource:  status
    Time:         2022-11-17T07:11:24Z
  Owner References:
    API Version:     dbaas.kubeblocks.io/v1alpha1
    Kind:            Cluster
    Name:            wesql-3nodes
    UID:             fef1ca58-2529-4058-864c-5dcaf9938ed3
  Resource Version:  375518
  UID:               34240728-3e12-4833-8186-6d9f3ed88c14
Spec:
  Cluster Ref:  wesql-3nodes
  Component Ops:
    Component Names:
      wesql-demo
  Type:  Restart
Status:
  Start Timestamp:       2022-11-17T07:09:53Z
  Completion Timestamp:  2022-11-17T07:11:24Z
  Components:
    Wesql - Demo:
      Phase:  Running
  Conditions:
    Last Transition Time:  2022-11-17T07:09:53Z
    Message:               Controller has started to progress the OpsRequest: ops-restart-threenodes-demo in Cluster: wesql-3nodes
    Reason:                OpsRequestProgressingStarted
    Status:                True
    Type:                  Progressing
    Last Transition Time:  2022-11-17T07:09:53Z
    Message:               OpsRequest: ops-restart-threenodes-demo is validated
    Reason:                ValidateOpsRequestPassed
    Status:                True
    Type:                  Validated
    Last Transition Time:  2022-11-17T07:09:53Z
    Message:               start restarting database in Cluster: wesql-3nodes
    Reason:                RestartingStarted
    Status:                True
    Type:                  Restarting
    Last Transition Time:  2022-11-17T07:11:24Z
    Message:               Controller has successfully processed the OpsRequest: ops-restart-threenodes-demo in Cluster: wesql-3nodes
    Reason:                OpsRequestProcessedSuccessfully
    Status:                True
    Type:                  Succeed
  Observed Generation:     1
  Phase:                   Succeed
Events:
  Type    Reason                           Age    From                    Message
  ----    ------                           ----   ----                    -------
  Normal  OpsRequestProgressingStarted     2m20s  ops-request-controller  Controller has started to progress the OpsRequest: ops-restart-threenodes-demo in Cluster: wesql-3nodes
  Normal  ValidateOpsRequestPassed         2m20s  ops-request-controller  OpsRequest: ops-restart-threenodes-demo is validated
  Normal  RestartingStarted                2m20s  ops-request-controller  start restarting database in Cluster: wesql-3nodes
  Normal  Starting                         2m20s  ops-request-controller  Restart component: wesql-demo in Cluster: wesql-3nodes
  Normal  Successful                       49s    ops-request-controller  Successfully Restart component: wesql-demo in Cluster: wesql-3nodes
  Normal  OpsRequestProcessedSuccessfully  49s    ops-request-controller  Controller has successfully processed the OpsRequest: ops-restart-threenodes-demo in Cluster: wesql-3nodes
```

## Step 3. (Optional) Destroy resources

Run the following commands to destroy the resources created by this guide:

```
kubectl delete cluster <name>
kubectl delete ops <name>
```
