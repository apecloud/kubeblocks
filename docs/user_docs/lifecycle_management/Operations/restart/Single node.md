# Restart a single-node cluster

This guide introduces how to use `KubeBlocks` to restart a single-node cluster.

## Before you begin

- Install KubeBlocks following the instruction here #links to be completed
- Learn the following `KubeBlocks`concepts 
  - [KubeBlocks OpsRequest](../configure_ops_request.md) 
  - [Restarting overview](Overview.md) 

## Apply restarting on standalone

In this guide, we will deploy a single-node `WeSQL` cluster and then restart it.

### Deploy a single-node cluster

First, we will deploy a single-node cluster. Below is the YAML of the cluster we are going to create:

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

Run the command line to create a standalone cluster.

```
$ kubectl apply -f cluster.yaml
cluster.dbaas.infracreate.com/wesql created
```

Wait for a few seconds, we can see the cluster is running, which means the cluster is deployed successfully.

```
$ kubectl get cluster
NAME            APP-VERSION    PHASE     AGE
wesql           wesql-8.0.18   Running   22s
```

Then, let us check which operations this cluster supports:

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

Now, We are ready to run `OpsRequest` to restart this cluster.

### Restart a single-node cluster

Below is the YAML of the `OpsRequest` CR we are going to create:

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

Then run the command line to apply `OpsRequest`:

```
$ kubectl apply -f restart.yaml
opsrequest.dbaas.kubeblocks.io/ops-restart created
```

View `OpsRequest` status:

```
$ kubectl get ops
NAME                        PHASE     AGE
ops-restart-demo            Running   12s
```

At the same time, you can view the cluster status:

```
$ kubectl get cluster
NAME            APP-VERSION    PHASE      AGE
wesql           wesql-8.0.18   Updating   11m46s 
```

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

You can also view the details of `OpsRequest`.

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

## Destroy

Run the following commands to destroy the resources created by this guide:

```
kubectl delete cluster <name>
kubectl delete ops <name>
```