---
title: OpsRequest
description: The API of KubeBlocks OpsRequest
sidebar_position: 2
sidebar_label: OpsRequest
---

# OpsRequest

## What is OpsRequest

`OpsRequest` is a Kubernetes Custom Resource Definitions (CRD). You can initiate an operation request via `OpsRequest` to operate database clusters. KubeBlocks supports the following operation tasks: database restarting, database version upgrading, vertical scaling, horizontal scaling, and volume expansion.

## OpsRequest CRD Specifications

The following are examples of `OpsRequest` CRs for different operations:

### Example for restarting a KubeBlocks cluster

```
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: mysql-restart
  namespace: default
spec:
  clusterRef: mysql-cluster-01
  ttlSecondsAfterSucceed: 3600
  type: Restart
  restart: 
  - componentName: replicasets
status:
    StartTimestamp: "2022-09-27T06:01:31Z"
    completionTimestamp: "2022-09-27T06:02:30Z"
    components:
        replicasets:
            phase: Running
    conditions:
    - lastTransitionTime: "2022-09-27T06:01:31Z"
      message: 'Controller has started to progress the OpsRequest: mysql-restart in
      Cluster: mysql-cluster-01'
      reason: OpsRequestProgressingStarted
      status: "True"
      type: Progressing
    - lastTransitionTime: "2022-09-27T06:01:31Z"
      message: 'OpsRequest: mysql-restart is validated'
      reason: ValidateOpsRequestPassed
      status: "True"
      type: Validated
    - lastTransitionTime: "2022-09-27T06:01:31Z"
      message: 'start restarting database in Cluster: mysql-cluster-01'
      reason: RestartingStarted
      status: "True"
      type: Restarting
    - lastTransitionTime: "2022-09-27T06:02:30Z"
      message: 'Controller has successfully processed the OpsRequest: mysql-restart
      in Cluster: mysql-cluster-01'
      reason: OpsRequestProcessedSuccessfully
      status: "True"
      type: Succeed
  observedGeneration: 1
  phase: Succeed
```

### Example for vertical scaling

```
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  generate-name: verticalscaling-
spec:
  # cluster ref
  clusterRef: myMongoscluster
  type: VerticalScaling 
  verticalScaling:
  - componentName: shard1
    requests:
      memory: "150Mi"
      cpu: "0.1"
    limits:
      memory: "250Mi"
      cpu: "0.2"
```

### Example for horizontal scaling

```
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: ops-xxxx
spec:
  # cluster ref
  clusterRef: myMongoscluster
  type: HorizontalScaling
  componentOps:
  - componentNames: [shard1]
    horizontalScaling:
      replicas: 3
```

### Example for upgrading a KubeBlocks cluster

```
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: ops-xxxx
spec:
  # cluster ref
  clusterRef: myMongoscluster
  type: Upgrade
  upgrade:
    # Upgrade to the specidief clusterversion
    clusterVersionRef: 5.0.1
```

### Example for volume expansion

```
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: ops-xxxx
spec:
  # cluster ref
  clusterRef: myMongoscluster
  type: VolumeExpansion
  volumeExpansion:
  - componentName: shard1
    volumeClaimTemplates:
    - name: data
      storage: "2Gi" 
```

## OpsRequest spec

An `OpsRequest` object has the following fields in the `spec` section.

### spec.clusterRef 

`spec.clusterRef` is a required field and points to the cluster to which the current OpsRequest is applied. Its value should be filled as `cluster.metadata.name`

### spec.type 

`spec.type` is a required field. It points to the operation OpsRequest uses and decides the operation OpsRequest performs.

The following types of operations are allowed in `OpsRequest`.

- `Upgrade`
- `VerticalScaling`
- `VolumeExpansion`
- `HorizontalScaling`
- `Restart`

### spec.clusterOps

It indicates the cluster-level operation. Its attribute is as follows:

- Upgrade

    It specifies the information for upgrading clusterversion and `spec.type` should be `Upgrade` to make it effective. Its attribute is as follows:
    - `clusterVersion` specifies the clusterVersion object used in the current upgrading operation.
        Value: `ClusterVersion.metadata.name`

### spec.componentOps

It indicates the component-level operation and is an array that supports operations of different parameters. Its attribute is as follows:

- componentNames

  It is a required field and specifies the component to which the operation is applied and is a `cluster.component.name` array.

- verticalScaling

  `verticalScaling` scales up and down the computing resources of a component. Its value is an object of Kubernetes container resources. For example,

    ```
    verticalScaling:
      requests:
        memory: "200Mi"
        cpu: "0.1"
      limits:
        memory: "300Mi"
        cpu: "0.2"
    ```

- volumeExpansion

  `volumeExpansion`, a volume array, indicates the storage resources that apply to each component database engine. `spec.type` should be `VolumeExpansion` to make it effective. Its attributes are as follows:

    - storage: the storage space
    - name: the name of volumeClaimTemplates.

- horizontalScaling

  `horizontalScaling` is the replicas amount of the current component. `spec.type` should be `HorizontalScaling` to make it effective. Its value includes `componentName.replicas`. For example:

    ```
    horizontalScaling:
      replicas: 3
    ```

## OpsRequest status

`status` describes the current state and progress of the `OpsRequest` operation. It has the following fields:

### status.observedGeneration

It corresponds to `metadata.generation`.

### status.phase

`OpsRequest` task is one-time and is deleted after the operation succeeds.

`status.phase` indicates the overall phase of the operation for this OpsRequest. It can have the following four values:

| **Phase**              | **Meaning**                                                |
|:---                    | :---                                                       | 
| Succeed                | OpsRequest is performed successfully and cannot be edited. |
| Running                | OpsRequest is running and cannot be edited.                |
| Pending                | OpsRequest is waiting for processing.                      |
| Failed                 | OpsRequest failed.                                         |

### status.conditions

`status.conditions` is the general data structure provided by Kubernetes, indicating the resource state. It can provide more detailed information (such as state switch time and upgrading time) than `status` does and functions as an extension mechanism.

`condition.type` indicates the type of the last OpsRequest status. 

| **Type**              | **Meaning**                                    |
|:---                   | :---                                           | 
| Progressing           | OpsRequest is under controller processing.      |
| Validated             | OpsRequest is validated.                       |
| Restarting            | Start processing restart ops.                  |
| VerticalScaling       | Start scaling resources vertically.            |
| HorizontalScaling     | Start scaling nodes horizontally.              |
| VolumeExpanding       | Start process volume expansion.                |
| Upgrading             | Start upgrading.                               |
| Succeed               | Operation is proceeded successfully.           |
| Failed                | Operation failed.                              |

`condition.status` indicates whether this condition is applicable. The results of `condition.status` include `True`, `False`, and `Unknown`, respectively standing for success, failure, and unknown error. 

`condition.Reason` indicates why the current condition changes. Each reason is only one word and exclusive.

| **Reason**                         | **Meanings**                                    |
| :---                               | :---                                            |
| OpsRequestProgressingStarted       | Controller is processing operations.            |
| Starting                           | Controller starts running operations.           |
| ValidateOpsRequestPassed           | OpsRequest is validated.                        |              
| OpsRequestProcessedSuccessfully    | OpsRequest is processed.                        |
| RestartingStarted                  | Restarting started.                             |
| VerticalScalingStarted             | VerticalScaling started.                        |
| HorizontalScalingStarted           | HorizontalScaling started.                      |
| VolumeExpandingStarted             | VolumeExpanding started.                        |
| UpgradingStarted                   | Upgrade started.                                |
| ClusterPhaseMisMatch               | The cluster status mismatches.                  |
| OpsTypeNotSupported                | The cluster does not support this operation.    |
| ClusterExistOtherOperation         | Another mutually exclusive operation is running.|
| ClusterNotFound                    | The specified cluster is not found.             |
| VolumeExpansionValidateError       | Volume expansion validation failed.             |
