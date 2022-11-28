# OpsRequest

## What is OpsRequest

OpsRequest is a Kubernetes Custom Resource Definitions (CRD). You can initiate an operation request via `OpsRequest` to operate database clusters. Currently, the following operation tasks are supported: database restarting, database version upgrading, vertical scaling, horizontal scaling, and volume expansion.

## OpsRequest CRD Specifications

`OpsRequest` has `TypeMeta`, `ObjectMeta`, `Spec` and `Status` sections. 

The following is sample `OpsRequest` CRs for different operations:

### Sample `OpsRequest` for restarting database

```
apiVersion: dbaas.kubeblocks.io/v1alpha1
metadata:
    name: mysql-restart
    namespace: default
spec:
    clusterRef: mysql-cluster-01
    ttlSecondsAfterSucceed: 3600
    type: Restart
    componentOps:
    - componentNames: [replicasets]
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

### Sample `OpsRequest` for vertical scaling

```
# API Scope: cluster
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: ops-verticalScaling
spec:
  # cluster ref
  clusterRef: myMongoscluster
  type: VerticalScaling 
  componentOps:
  - componentNames: [shard1]
    verticalScaling:
      requests:
        memory: "150Mi"
        cpu: "0.1"
      limits:
        memory: "250Mi"
        cpu: "0.2"
```

### Sample `OpsRequest` for horizontal scaling

```
apiVersion: dbaas.kubeblocks.io/v1alpha1
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


### Sample `OpsRequest` for upgrading database

```
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: ops-xxxx
spec:
  # cluster ref
  clusterRef: myMongoscluster
  type: Upgrade
  clusterOps:
    upgrade:
      # Upgrade the specified appversion
      appVersionRef: 5.0.1
```

### Sample `OpsRequest` for volume expansion

```
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: ops-xxxx
spec:
  # cluster ref
  clusterRef: myMongoscluster
  type: VolumeExpansion
  componentOps:
  - componentNames: [shard1]
    volumeExpansion:
    - name: data
      storage: "2Gi"
```

## OpsRequest `spec`

An `OpsRequest` object has the follwoing fields in the `spec` section.

### spec.clusterRef 

`spec.clusterRef` is a required field and points to the cluster to which the current OpsRequest is applied. Its value should be filled as `cluster.metadata.name`

### spec.type 

`spec.type` is a required field. It points to which operation OpsRequest uses and decides which operation OpsRequest performs now.

The following types of operations are allowed in `OpsRequest`.

- `Upgrage`
- `VerticalScaling`
- `VolumeExpansion`
- `HorizontalScaling`
- `Restart`

### spec.clusterOps

It indicates the cluster-level operation. Its attribute is as follows:

- Upgrade

    It specifies the information for upgrading appversion and `spec.type` should be `Upgrade` to make it effective. Its attribute is as follows:
    - `appVersion` specifies the appVersion object that will be used in the current upgrading operation.
        Value: `AppVersion.metadata.name`

### spec.componentOps

It indicates the component-level operation. Its attribute is as follows:

- componentNames

    It is a required field and specifies the component to which the operation is applied and is a `cluster.component.name` array.

    - verticalScaling

        `verticalScaling` scales up and down the computing resources of the component. Its value is an object of k8s containeer resources. For example, the mongos component of MongoDB:

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

        `horizontalScaling` is used to add or delete the replicas of a component or roleGroup. `spec.type` shoule be `HorizontalScaling` to make it effective. Its value includes `componentName.replicas` and `componentName.roleGroups`. For example:

        ```
        horizontalScaling:
            replicas: 3
            # roleGroups is an array and its data element structure is: 
            roleGroups:
            - name: secondary
              replicas: 2
        ```

## OpsRequest `status`

`status` describes the current state amd progress of the `OpsRequest` opration. It has the followung fileds:

### status.observedGenration

It corresponds to `matadata.generation`.

### status.phase

OpsRequest task is one-time and is deleted after the operation succeeds.

`status.phase` indicates the overall phase of the operation for this OpsRequest. It can have the following four values:

| **Phase**              | **Meaning**                                                |
|:---                    | :---                                                       | 
| Succeed                | OpsRequest is performed successfully and cannot be edited. |
| Running                | OpsRequest is running and cannot be edited.                |
| Pending                | OpsRequest is waiting for processing.                      |
| Failed                 | OpsRequest failed.                                         |

### status.condition

`conditions` is the general data structure provided by k8s, indicating the resource state. It can provide more detailed information (such as state switch time and upgrading time) than `status` does and functions as an extension mechanism.

`condition.type` indicates the type of the last OpsRequest status. 

| **Type**              | **Meaning**                                    |
|:---                   | :---                                           | 
| Progressing           | OpsRequest is under controller procesing.      |
| Validated             | OpsRequest is validated.                       |
| Restarting            | Start Restarting.                              |
| VerticalScaling       | Start VerticalScaling.                         |
| HorizontalScaling     | Start HorizontalScaling.                       |
|VolumeExpanding        | Start VolumeExpanding.                         |
| Upgrading             | Start Upgrading.                               |
| Succeed               | OpsRequest is proceeded successfully.          |

`condition.status` indicates whether this condition is appliable. The results of `condition.status` include `True`, `False`, and `Unkown`, respectively standing for success, failure, and unkown error. 

`condition.Reason` indicates why the current condition changes. Each reason is only one word and exclusive.

| **Reason**                         | **Meanings**                                    |
| :---                               | :---                                            |
| OpsRequestProgressingStarted       | Controller is processing OpsRequest.            |
| ValidateOpsRequestPassed           | OpsRequest is validated.                        |              
| OpsRequestProcessedSuccessfully    | OpsRequest is processed.                        |
| RestartingStarted                  | Restarting started.                             |
| VerticalScalingStarted             | VerticalScaling started.                        |
| HorizontalScalingStarted           | HorizontalScaling started.                      |
| VolumeExpandingStarted             | VolumeExpanding started.                        |
| UpgradingStarted                   | Upgrade started.                                |
| ClusterPhaseMisMatch               | The cluster status mismatches.                  |
| OpsTypeNotSupported                | The cluster does not support this operation.    |
| ClusterExistOtherOperation         | Another operation is running.                   |
| ClusterNotFound                    | The specified cluster is not found.             |
| VolumeExpansionValidateError       | Volume expansion validation failed.             |
