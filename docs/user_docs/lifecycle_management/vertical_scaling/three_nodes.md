# Vertically scale a three-node cluster

This guide shows you how to use KubeBlocks to scale up a three-node cluster.

## Before you start

- [Install KubeBlocks](../../installation/deploy_kubeblocks.md). 
- Run the commands below to check whether the KubeBlocks is installed successfully and the cluster-related CR (custom resources) are created.
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
    NAME             PHASE       AGE
    apecloud-wesql   Available   7m13s
  $ kubectl get appversion
    NAME           PHASE       AGE
    wesql-8.0.18   Available   7m23s
  $ kubectl get cm
    NAME                  DATA   AGE
    mysql-3node-tpl-8.0   1      7m28s
  ```
- Learn the following `KubeBlocks`concepts 
  - [KubeBlocks OpsRequest](../configure_ops_request.md)
  - [Vertical scaling overview](Overview.md) 

### Create a three-node cluster for a demo

_Steps_:
1. Prepare a YAML file for a single-node cluster. Below is the YAML file of the three-node cluster. You can find this demo file in `kubeblocks/example/dbaas`.

```
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: wesql-3nodes
spec:
  appVersionRef: wesql-8.0.18
  clusterDefinitionRef: apecloud-wesql
  terminationPolicy: WipeOut
  components:
    - name: wesql-demo
      type: replicasets
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

2. Run the command line to create a three-node cluster.

```
kubectl apply -f cluster_three_nodes.yaml
cluster.dbaas.kubeblocks.io/wesql-3nodes created
```

### Result

Wait a few seconds and when the cluster phase changes to  `Running`, the cluster is created successfully.

```
$ kubectl get cluster
NAME                   APP-VERSION    PHASE     AGE
wesql-3nodes           wesql-8.0.18   Running   20s
```

1. Check the operations this cluster supports:

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

## Vertically scale a cluster

_Steps_:

1. Prepare a YAML file for vertically scaling a three-node cluster. Below is the YAML file of the `OpsRequest` CR:

```
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: ops-vertical-scaling-threenodes-demo
spec:
  clusterRef: wesql-3nodes
  type: VerticalScaling 
  componentOps:
  - componentNames: [wesql-demo]
    verticalScaling:
      requests:
        memory: "500Mi"
        cpu: "0.5"
      limits:
        memory: "500Mi"
        cpu: "0.5"
```

2. Run the command line to apply `OpsRequest`.

```
kubectl apply -f verticalscaling-threenode.yaml
opsrequest.dbaas.kubeblocks.io/ops-vertical-scaling-threenodes-demo created
```

3. View the `OpsRequest` phase and cluster phase:

```
$ kubectl get ops
NAME                              PHASE     AGE
ops-vertical-scaling-threenodes   Running   12s
```

```
$ kubectl get cluster
NAME                   APP-VERSION    PHASE      AGE
wesql-3nodes           wesql-8.0.18   Updating   2m46s
```

### Results

When the phase changes to `Succeed`, the `OpsRequest` is applied successfully.

```
$ kubectl get ops
NAME                              PHASE     AGE
ops-vertical-scaling-threenodes   Succeed   96s
```

And the cluster also changes:

```
$ kubectl get cluster
NAME                   APP-VERSION    PHASE      AGE
wesql-3nodes           wesql-8.0.18   Running  4m25s
```

4. View the details of `OpsRequest`.

```
Name:         ops-vertical-scaling-threenodes-demo
Namespace:    default
Labels:       cluster.kubeblocks.io/name=wesql-3nodes
Annotations:  <none>
API Version:  dbaas.kubeblocks.io/v1alpha1
Kind:         OpsRequest
Metadata:
  Creation Timestamp:  2022-11-17T07:01:39Z
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
    Time:         2022-11-17T07:01:39Z
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
    Time:         2022-11-17T07:01:39Z
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
    Time:         2022-11-17T07:03:09Z
  Owner References:
    API Version:     dbaas.kubeblocks.io/v1alpha1
    Kind:            Cluster
    Name:            wesql-3nodes
    UID:             fef1ca58-2529-4058-864c-5dcaf9938ed3
  Resource Version:  374445
  UID:               0ebf59b9-fd81-4e3b-a36c-4eaa92b17008
Spec:
  Cluster Ref:  wesql-3nodes
  Component Ops:
    Component Names:
      wesql-demo
    Vertical Scaling:
      Limits:
        Cpu:     500m
        Memory:  500Mi
      Requests:
        Cpu:     500m
        Memory:  500Mi
  Type:          VerticalScaling
Status:
  Start Timestamp:       2022-11-17T07:01:39Z
  Completion Timestamp:  2022-11-17T07:03:09Z
  Components:
    Wesql - Demo:
      Phase:  Running
  Conditions:
    Last Transition Time:  2022-11-17T07:01:39Z
    Message:               Controller has started to progress the OpsRequest: ops-vertical-scaling-threenodes-demo in Cluster: wesql-3nodes
    Reason:                OpsRequestProgressingStarted
    Status:                True
    Type:                  Progressing
    Last Transition Time:  2022-11-17T07:01:39Z
    Message:               OpsRequest: ops-vertical-scaling-threenodes-demo is validated
    Reason:                ValidateOpsRequestPassed
    Status:                True
    Type:                  Validated
    Last Transition Time:  2022-11-17T07:01:39Z
    Message:               start vertical scaling in Cluster: wesql-3nodes
    Reason:                VerticalScalingStarted
    Status:                True
    Type:                  VerticalScaling
    Last Transition Time:  2022-11-17T07:03:09Z
    Message:               Controller has successfully processed the OpsRequest: ops-vertical-scaling-threenodes-demo in Cluster: wesql-3nodes
    Reason:                OpsRequestProcessedSuccessfully
    Status:                True
    Type:                  Succeed
  Observed Generation:     1
  Phase:                   Succeed
Events:
  Type    Reason                           Age                    From                    Message
  ----    ------                           ----                   ----                    -------
  Normal  OpsRequestProgressingStarted     3m53s (x2 over 3m53s)  ops-request-controller  Controller has started to progress the OpsRequest: ops-vertical-scaling-threenodes-demo in Cluster: wesql-3nodes
  Normal  ValidateOpsRequestPassed         3m53s                  ops-request-controller  OpsRequest: ops-vertical-scaling-threenodes-demo is validated
  Normal  VerticalScalingStarted           3m53s                  ops-request-controller  start vertical scaling in Cluster: wesql-3nodes
  Normal  Starting                         3m53s                  ops-request-controller  VerticalScaling component: wesql-demo in Cluster: wesql-3nodes
  Normal  Successful                       2m23s                  ops-request-controller  Successfully VerticalScaling component: wesql-demo in Cluster: wesql-3nodes
  Normal  OpsRequestProcessedSuccessfully  2m23s (x2 over 2m23s)  ops-request-controller  Controller has successfully processed the OpsRequest: ops-vertical-scaling-threenodes-demo in Cluster: wesql-3nodes
```

## (Optional) Destroy

Run the following commands to destroy the resources created by this guide:

```
kubectl delete cluster <name>
kubectl delete ops <name>
```