# Restart a three-node cluster

This guide shows how to use `KubeBlocks` to restart a three-node cluster.

## Before you begin

- Install KubeBlocks following the instruction here #links to be completed
- Learn the following `KubeBlocks`concepts 
  - [KubeBlocks OpsRequest](../configure_ops_request.md) 
  - [Restarting overview](Overview.md) 

## Apply restarting on a three-node cluster

In this guide, we will deploy a three-node `WeSQL` cluster and then restart it.

### Deploy a three-node cluster

First, we will deploy a three-node cluster. Below is the YAML of the cluster we are going to create:

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

Run the command line to create a three-node cluster.

```
kubectl apply -f cluster_three_nodes.yaml
cluster.dbaas.kubeblocks.io/wesql-3nodes created
```

Wait for a few seconds, we can see the cluster is running, which means the cluster is deployed successfully.

```
$ kubectl get cluster
NAME                   APP-VERSION    PHASE     AGE
wesql-3nodes           wesql-8.0.18   Running   20s
```

Then, let us check which operations this cluster supports:

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

Now, We are ready to run `OpsRequest` to restart this cluster.

### Restart a three-node cluster

Below is the YAML of the `OpsRequest` CR we are going to create:

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

Then run the command line to apply `OpsRequest`:

```
$ kubectl apply -f restart_three_nodes.yaml
opsrequest.dbaas.kubeblocks.io/ops-restart-threenodes-demo created
```

View `OpsRequest` status:

```
$ kubectl get ops
NAME                   PHASE     AGE
ops-restart            Running   45s
```

At the same time, you can view the cluster status:

```
$ kubectl get cluster
NAME                   APP-VERSION    PHASE      AGE
wesql-3nodes           wesql-8.0.18   Updating   16m 
```

When the phase changes to `Succeed`, the `OpsRequest` is applied successfully.

```
$ kubectl get ops
NAME                              PHASE     AGE
ops-restart-threenodes            Succeed   96s
```

And the cluster also changes:

```
$ kubectl get cluster
NAME                   APP-VERSION    PHASE      AGE
wesql-3nodes           wesql-8.0.18   Running    17m
```

You can also view the details of `OpsRequest`.

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

## Destroy

Run the following commands to destroy the resources created by this guide:

```
kubectl delete cluster <name>
kubectl delete ops <name>
```