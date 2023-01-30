# Vertically scale a single-node cluster

This section shows you how to use KubeBlocks to scale up a cluster.

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
  - Run the commands below to check whether the cluster-related `CR` is installed successfully.
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
- Learn the following `KubeBlocks`concepts.
  - [KubeBlocks OpsRequest](../configure_ops_request.md)
  - [Vertical scaling overview](Overview.md) 

## Step 1. Create a single-node cluster for a demo

_Steps_:

1. Prepare a YAML file for a single-node cluster. Below is the YAML file of the single-node cluster. 


  ```
  apiVersion: dbaas.kubeblocks.io/v1alpha1
  kind: Cluster
  metadata:
    name: wesql
  spec:
    clusterVersionRef: ac-mysql-8.0.30
    clusterDefinitionRef: apecloud-mysql
    terminationPolicy: WipeOut
    components:
      - name: wesql-demo
        type: wesql
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

2. Run the command line below to create a single-node cluster.

  ```
  $ kubectl apply -f cluster.yaml
  cluster.dbaas.kubeblocks.io/wesql created
  ```

### Result 

Wait a few seconds and when the cluster phase changes to  `Running`, the cluster is created successfully.

```
$ kubectl get cluster
NAME            VERSION    STATUS     AGE
wesql           ac-mysql-8.0.30   Running   22s
```

3. Check the operations this cluster supports:

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

## Step 2. Vertically scale a cluster

_Steps_:

1. Prepare a YAML file for vertically scaling a single-node cluster. Below is the YAML file of the `OpsRequest` CR. 

  ```
  apiVersion: dbaas.kubeblocks.io/v1alpha1
  kind: OpsRequest
  metadata:
    name: ops-vertical-scaling-demo
  spec:
    clusterRef: wesql
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
  $ kubectl apply -f vertical_scaling.yaml
  opsrequest.dbaas.kubeblocks.io/ops-vertical-scaling-demo created
  ```

3. View the `OpsRequest` phase and cluster phase:

  ```
  $ kubectl get ops
  NAME                        STATUS     AGE
  ops-vertical-scaling-demo   Running   13s
  ```

  ```
  $ kubectl get cluster
  NAME            VERSION    STATUS      AGE
  wesql           ac-mysql-8.0.30   Updating   8m46s
  ```

### Results
When the `ops` phase changes to `Succeed`, this `OpsRequest` is applied successfully.

  ```
  $ kubectl get ops
  NAME                        STATUS     AGE
  ops-vertical-scaling-demo   Succeed   52s
  ```

And the cluster also changes:

  ```
  $ kubectl get cluster
  NAME            VERSION    STATUS      AGE
  wesql           ac-mysql-8.0.30   Running  9m16s
  ```

4. View the details of `OpsRequest`.

  ```
  $ kubectl describe ops ops-vertical-scaling-demo
  Name:         ops-vertical-scaling-demo
  Namespace:    default
  Labels:       cluster.kubeblocks.io/name=wesql
  Annotations:  <none>
  API Version:  dbaas.kubeblocks.io/v1alpha1
  Kind:         OpsRequest
  Metadata:
    Creation Timestamp:  2022-11-17T06:23:31Z
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
      Time:         2022-11-17T06:23:31Z
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
      Time:         2022-11-17T06:23:31Z
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
      Time:         2022-11-17T06:24:22Z
    Owner References:
      API Version:     dbaas.kubeblocks.io/v1alpha1
      Kind:            Cluster
      Name:            wesql
      UID:             a0293dcd-0e78-44f0-89b7-cf2c1768fb66
    Resource Version:  369833
    UID:               c85703aa-138d-44af-b161-1665fbdf35f3
  Spec:
    Cluster Ref:  wesql
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
    Start Timestamp:       2022-11-17T06:23:31Z
    Completion Timestamp:  2022-11-17T06:24:22Z
    Components:
      Wesql - Demo:
        Phase:  Running
    Conditions:
      Last Transition Time:  2022-11-17T06:23:31Z
      Message:               Controller has started to progress the OpsRequest: ops-vertical-scaling-demo in Cluster: wesql
      Reason:                OpsRequestProgressingStarted
      Status:                True
      Type:                  Progressing
      Last Transition Time:  2022-11-17T06:23:31Z
      Message:               OpsRequest: ops-vertical-scaling-demo is validated
      Reason:                ValidateOpsRequestPassed
      Status:                True
      Type:                  Validated
      Last Transition Time:  2022-11-17T06:23:31Z
      Message:               start vertical scaling in Cluster: wesql
      Reason:                VerticalScalingStarted
      Status:                True
      Type:                  VerticalScaling
      Last Transition Time:  2022-11-17T06:24:22Z
      Message:               Controller has successfully processed the OpsRequest: ops-vertical-scaling-demo in Cluster: wesql
      Reason:                OpsRequestProcessedSuccessfully
      Status:                True
      Type:                  Succeed
    Observed Generation:     1
    Phase:                   Succeed
  Events:
    Type    Reason                           Age   From                    Message
    ----    ------                           ----  ----                    -------
    Normal  OpsRequestProgressingStarted     92s   ops-request-controller  Controller has started to progress the OpsRequest: ops-vertical-scaling-demo in Cluster: wesql
    Normal  ValidateOpsRequestPassed         92s   ops-request-controller  OpsRequest: ops-vertical-scaling-demo is validated
    Normal  VerticalScalingStarted           92s   ops-request-controller  start vertical scaling in Cluster: wesql
    Normal  Starting                         92s   ops-request-controller  VerticalScaling component: wesql-demo in Cluster: wesql
    Normal  Successful                       41s   ops-request-controller  Successfully VerticalScaling component: wesql-demo in Cluster: wesql
    Normal  OpsRequestProcessedSuccessfully  41s   ops-request-controller  Controller has successfully processed the OpsRequest: ops-vertical-scaling-demo in Cluster: wesql
  ```

## Step 3. (Optional) Destroy resources

Run the following commands to destroy the resources created by this guide:

```
kubectl delete cluster <name>
kubectl delete ops <name>
```
