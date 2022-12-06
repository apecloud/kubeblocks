# Restart a single-node cluster

This guide introduces how to use KubeBlocks to restart a single-node cluster.

## Before you start

- [Install KubeBlocks](../../installation/deploy_kubeblocks.md). 
- Run the commands below to check whether the KubeBlocks is installed successfully and the cluster-related `CR` (custom resources) are created.
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
  - Run the commands below to check whether the cluster-related `CR` is installed successfully.
  ```
  $ kubectl get cd 
    NAME             PHASE       AGE
    apecloud-wesql   Available   7m13s
  $ kubectl get appversion
    NAME           PHASE       AGE
    wesql-8.0.18   Available   7m23s
  $ kubectl get cm
    NAME                  DATA   AGE
    mysql-3node-tpl-8.0   1      7m28s
  ```
- Learn the following KubeBlocks concepts 
  - [KubeBlocks OpsRequest](../configure_ops_request.md) 
  - [Restarting overview](Overview.md) 

## Create a single-node cluster for a demo

_Steps_:
1. Prepare a YAML file for a single-node cluster. Below is the YAML file of the single-node cluster. You can find [this demo file, `cluster.yaml`](kubeblocks/examples/../../../../../../examples/dbaas/cluster.yaml), in [`kubeblocks/examples/dbaas`](https://github.com/apecloud/kubeblocks/tree/main/examples/dbaas).

```
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: wesql
spec:
  appVersionRef: wesql-8.0.18
  clusterDefinitionRef: apecloud-wesql
  terminationPolicy: WipeOut
  components:
    - name: wesql-demo
      type: replicasets
      monitor: false
      replicas: 1
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

2. Run the command line to create a single-node cluster.

```
$ kubectl apply -f cluster.yaml
cluster.dbaas.kubeblocks.io/wesql created
```

### Result

Wait a few seconds and when the cluster phase changes to  `Running`, the cluster is created successfully.

```
$ kubectl get cluster
NAME            APP-VERSION    PHASE     AGE
wesql           wesql-8.0.18   Running   22s
```

1. Check the operations this cluster supports:

```
$ kubectl describe cluster wesql
....
Status:
  Cluster Def Generation:  2
  Components:
    Wesql - Demo:
      Consensus Set Status:
        Leader:
          Access Mode:  ReadWrite
          Name:         leader
          Pod:          wesql-wesql-demo-0
      Phase:            Running
  Observed Generation:  1
  Operations:
    Horizontal Scalable:
      Name:  wesql-demo
    Restartable:
      wesql-demo
    Vertical Scalable:
      wesql-demo
  Phase:  Running
```

### Result
When the `status.phase` is `Running`, you can run `OpsRequest` to restart this cluster.

## Restart a single-node cluster

_Steps_:

1. Prepare a YAML file for restarting a single-node cluster. Below is the YAML file of the `OpsRequest` CR. You can find [this demo file, `restart.yaml`](../../../../examples/dbaas/restart.yaml), in [`kubeblocks/examples/dbaas`](https://github.com/apecloud/kubeblocks/tree/main/examples/dbaas).

```
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: ops-restart-demo
spec:
  clusterRef: wesql
  type: Restart 
  componentOps:
  - componentNames: [wesql-demo]
```

2. Run the command line to apply `OpsRequest`:

```
$ kubectl apply -f restart.yaml
opsrequest.dbaas.kubeblocks.io/ops-restart created
```

3. View the `OpsRequest` phase and cluster phase:

```
$ kubectl get ops
NAME                        PHASE     AGE
ops-restart-demo            Running   12s
```

```
$ kubectl get cluster
NAME            APP-VERSION    PHASE      AGE
wesql           wesql-8.0.18   Updating   11m46s 
```

### Results
When the phase changes to `Succeed`, the `OpsRequest` is applied successfully.

```
$ kubectl get ops
NAME                        PHASE     AGE
ops-restart-demo            Succeed   52s
```

And the cluster also changes:

```
$ kubectl get cluster
NAME            APP-VERSION    PHASE      AGE
wesql           wesql-8.0.18   Running    12m26s
```

4. (Optional) View the details of `OpsRequest`.

```
$ kubectl describe ops ops-restart-demo
Name:         ops-restart-demo
Namespace:    default
Labels:       cluster.kubeblocks.io/name=wesql
Annotations:  <none>
API Version:  dbaas.kubeblocks.io/v1alpha1
Kind:         OpsRequest
Metadata:
  Creation Timestamp:  2022-11-17T06:26:33Z
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
    Time:         2022-11-17T06:26:33Z
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
          k:{"uid":"a0293dcd-0e78-44f0-89b7-cf2c1768fb66"}:
    Manager:      manager
    Operation:    Update
    Time:         2022-11-17T06:26:34Z
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
    Time:         2022-11-17T06:27:27Z
  Owner References:
    API Version:     dbaas.kubeblocks.io/v1alpha1
    Kind:            Cluster
    Name:            wesql
    UID:             a0293dcd-0e78-44f0-89b7-cf2c1768fb66
  Resource Version:  370242
  UID:               35d0395f-a20e-4237-91e5-5be67038bc17
Spec:
  Cluster Ref:  wesql
  Component Ops:
    Component Names:
      wesql-demo
  Type:  Restart
Status:
  Start Timestamp:       2022-11-17T06:26:34Z
  Completion Timestamp:  2022-11-17T06:27:27Z
  Components:
    Wesql - Demo:
      Phase:  Running
  Conditions:
    Last Transition Time:  2022-11-17T06:26:34Z
    Message:               Controller has started to progress the OpsRequest: ops-restart-demo in Cluster: wesql
    Reason:                OpsRequestProgressingStarted
    Status:                True
    Type:                  Progressing
    Last Transition Time:  2022-11-17T06:26:34Z
    Message:               OpsRequest: ops-restart-demo is validated
    Reason:                ValidateOpsRequestPassed
    Status:                True
    Type:                  Validated
    Last Transition Time:  2022-11-17T06:26:34Z
    Message:               start restarting database in Cluster: wesql
    Reason:                RestartingStarted
    Status:                True
    Type:                  Restarting
    Last Transition Time:  2022-11-17T06:27:27Z
    Message:               Controller has successfully processed the OpsRequest: ops-restart-demo in Cluster: wesql
    Reason:                OpsRequestProcessedSuccessfully
    Status:                True
    Type:                  Succeed
  Observed Generation:     1
  Phase:                   Succeed
Events:
  Type    Reason                           Age   From                    Message
  ----    ------                           ----  ----                    -------
  Normal  OpsRequestProgressingStarted     103s  ops-request-controller  Controller has started to progress the OpsRequest: ops-restart-demo in Cluster: wesql
  Normal  ValidateOpsRequestPassed         103s  ops-request-controller  OpsRequest: ops-restart-demo is validated
  Normal  RestartingStarted                103s  ops-request-controller  start restarting database in Cluster: wesql
  Normal  Starting                         103s  ops-request-controller  Restart component: wesql-demo in Cluster: wesql
  Normal  Successful                       50s   ops-request-controller  Successfully Restart component: wesql-demo in Cluster: wesql
  Normal  OpsRequestProcessedSuccessfully  50s   ops-request-controller  Controller has successfully processed the OpsRequest: ops-restart-demo in Cluster: wesql
```

## (Optional) Destroy

Run the following commands to destroy the resources created by this guide:

```
kubectl delete cluster <name>
kubectl delete ops <name>
```