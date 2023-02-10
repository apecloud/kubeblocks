# Scale for ApeCloud MySQL
You can scale ApeCloud MySQL DB instances in two ways, horizontal scaling and vertical scaling. 

## Vertical scaling
You can vertically scale a cluster by changing resource requirements and limits (CPU and storage). For example, if you need to change the resource demand from 1C2G to 2C4G, vertical scaling is what you need.

***Note:***
During the vertical scaling process, all pods restart in the order of learner -> follower -> leader and the leader pod may change after the restarting.
***Before you start***
Run the command below to check whether the cluster STATUS is Running. Otherwise, the following operations may fail.
```
kbcli cluster list NAME
```
***Example***
```
$ kbcli cluster list mysql-01
NAME            NAMESPACE        CLUSTER-DEFINITION        VERSION                TERMINATION-POLICY        STATUS         CREATED-TIME
mysql-01        default          apecloud-mysql            ac-mysql-8.0.30        Delete                    Running        Jan 29,2023 14:29 UTC+0800
```
***Steps:***
1. Change configuration.
There are 3 ways to apply vertical scaling.
**Option 1**. (Recommanded) Use kbcli
Configure the parameters `component-names`, `requests`, and `limits` and run the command.
***Example***

```
$ kbcli cluster vertical-scale NAME \
--component-names="mysql" \
--requests.memory="2Gi" --requests.cpu="1" \
--limits.memory="4Gi" --limits.cpu="2"```
```
- `component-names` describes the component name ready for vertical scaling.
- `requests` describes the minimum amount of computing resources required. If `requests` is omitted for a container, it uses the `limits` value if `limits` is explicitly specified, otherwise uses an implementation-defined value. For more details, see [Resource Management for Pods and Containers](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/).
- `--limits` describes the maximum amount of computing resources allowed. For more details, see [Resource Management for Pods and Containers](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/)

**Option 2.** Create an OpsRequest
Run the command below to apply an OpsRequest to the specified cluster. Configure the parameters according to your needs.
```
$ kubectl apply -f - <<EOF
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: ops-vertical-scaling
spec:
  clusterRef: mysql
  type: VerticalScaling 
  verticalScaling:
  - componentName: mysql
    requests:
      memory: "2Gi"
      cpu: "1000m"
    limits:
      memory: "4Gi"
      cpu: "2000m"
EOF
```
**Option 3.** Change the YAML file of the cluster
Change the configuration of `spec.components.resources` in the YAML file. `spec.components.resources` controls the requirement and limit of resources and changing them triggers a vertical scaling. 
***Example***
```
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: mysql-01
  namespace: default
spec:
  clusterDefinitionRef: apecloud-mysql
  clusterVersionRef: ac-mysql-8.0.30
  components:
  - name: mysql
    type: mysql
    replicas: 1
    resources: # Change the values of resources.
      requests:
        memory: "2Gi"
        cpu: "1000m"
      limits:
        memory: "4Gi"
        cpu: "2000m"
    volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
  terminationPolicy: Halt
  ```
  2. Validate the vertical scaling.
  Run the command below to check the cluster status to identify the vertical scaling status.
  ```
  kbcli cluster list NAME
  ```
  ***Example***
  ```
    $ kbcli cluster list mysql-01
  NAME            NAMESPACE        CLUSTER-DEFINITION        VERSION                TERMINATION-POLICY        STATUS          CREATED-TIME
  mysql-01        default          apecloud-mysql            ac-mysql-8.0.30        Delete                    Updating        Jan 29,2023 14:29 UTC+0800
  ```
  - STATUS=Running: means the vertical scaling operation is applied.
  - STATUS=Updating: means the vertical scaling is in progress.
  - STATUS=Abnormal: means the vertical scaling is abnormal. The reason may be the normal instances number is less than the total instance number or the leader instance is running properly while others are abnormal. 
  >>To solve the problem, you can check manually to see whether resource is sufficient. If AutoScaling is supported, the system recovers when there is enough resource, otherwise, create enough resource and check wth `kubectl describe` command.

## Horizontal scaling
Horizontal scaling changes the amount of pods. For example, you can apply a horizontal scaling to scale up from three pods to five pods. The scaling process includes the backup and restore of data.
The process of the horizontal scaling:
1. Create a snapshot for the leader pod.
2. Create the PVC (PersistentVolumeClaim) required by the new pod by the snapshot.
3. Mount the new pod on PVC.
4. Clean up the resources, such as backup and snapshot, created during the scaling process.
The existing pods do not restart during the horizontal scaling process.
The scale-down operation immediately delete the pods that are no longer needed and delete their corresponding PVC after 30 minutes. If another scale-up operation is triggered within the waiting 30 minutes, the PVC that has not been deleted is used for storage.
***Before you start***
Run the command below to check whether the cluster STATUS is `Running`. Otherwise, the following operations may fail.

  ```
  kbcli cluster list NAME
  ```

 
***Example***
```$ kbcli cluster list mysql-01
NAME            NAMESPACE        CLUSTER-DEFINITION        VERSION                TERMINATION-POLICY        STATUS         CREATED-TIME
mysql-01        default          apecloud-mysql            ac-mysql-8.0.30        Delete                    Running        Jan 29,2023 14:29 UTC+0800
```

***Steps:***
1. Change configuration.
There are 3 ways to apply horizontal scaling.
**Option 1**. (Recommanded) Use kbcli
Configure the parameters `component-names` and `replicas`, and run the command.
***Example***

```
kbcli cluster horizontal-scale NAME \
 --component-names="mysql" --replicas=3
```
- `component-names` describes the component name ready for vertical scaling.
- `replicas` describe the replicas with the specified components.


**Option 2.** Create an OpsRequest
Run the command below to apply an OpsRequest to the specified cluster. Configure the parameters according to your needs.
```
$ kubectl apply -f - <<EOF
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: ops-horizontal-scaling
spec:
  clusterRef: mysql-01
  type: HorizontalScaling
  horizontalScaling:
  - componentName: mysql
    replicas: 3
EOF
```
**Option 3.** Change the YAML file of the cluster
Change the configuration of `spec.components.replicas` in the YAML file. `spec.components.replicas` tand for the pod amount and changing this value triggers a horizontal scaling of a cluster. 
***Example***
```
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
 apiVersion: dbaas.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: mysql-01
  namespace: default
spec:
  clusterDefinitionRef: apecloud-mysql
  clusterVersionRef: ac-mysql-8.0.30
  components:
  - name: mysql
    type: mysql
    replicas: 1 # Change the pod amount.
    volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
  terminationPolicy: Halt
  ```
2. Validate the horizontal scaling operation
Run the command below to check the cluster STATUS to identify the horizontal scaling status.
```
kbcli cluster list NAME
```

- STATUS=Updating: means horizontal scaling is being applied.
- STATUS=Running: means horizontal scaling is applied.