# KubeBlocks Lifecycle API

This guide describes the details of KubeBlocks lifecycle API. KubeBlocks API is declarative and enables providers to describe the database cluster typology and lifecycle by YAML files, thus dynamically generating a management and control task flow to provide users with a consistent database operation experience. KubeBlocks has three APIs, namely `ClusterDefinition`, `AppVersion][`, and `Cluster`. `ClusterDefinition` and `AppVersion` are designed for providers and `Cluster` is for end users.

## ClusterDefinition (for providers)

`ClusterDefinition` enables providers to describe the cluster typology and the dependencies among roles in operation tasks. `ClusterDefinition` has `TypeMeta`, `ObjectMeta`, and `Spec` sections. 

### ClusterDefinition `spec`

#### spec.type

`spec.type` is required. You can fill it as the following examples do: state.redis, mq.mqtt, mq.kafkal, state.mysql-8, state.mysql-7.

#### spec.componentType

`spec.componentType` stands for the component type. KubeBlocks supports `stateless`, `stateful`, and `consensus`. `stateless` is set as default.

#### spec.consensusSpec

When the `spec.componentType` is set as `consensus`, `spec.consensusSpec` is required.

- `leader`

    `leader` stands for the leader node and provides write capability. 

    - `name`
      
        `name` stands for the role name and comes from the result of `roleObserveQuery`.
    
    - `accessMode`
        
        `accessMode` stands for the service capability. There are three types available, namely `readWrite`, `readonly`, and `none`. `readWrite` provides read and write services. `readonly` provides write service. `none` does not provide any service.
  
- `followers`
    
    `followers` participates in the election. Its name and access mode are defined by default.

- `learner`
    
    `learner` does not participate in the election. Its name and access mode are defined by default. Its `replicas` stands for the pod amount and it is non-overridable in the cluster.

- `updateStrategy`
    
    `updateStrategy` stands for the updating strategy. `serial`, `bestEffortParallel` and `parallel` are selectable. `serial` is set as the default.
    
    - `serial` stands for the serial executor. For example, when MySQL three-node cluster is upgrading, this process will be executed following this order, `learner1 -> learner2 -> logger -> follower -> leader`.
    - `bestEffortParallel` means the controller tries to execute in parallel. Under the same scene in `serial`, the process will be executed following this order, `learner1, learner2, logger in parallel way -> follower -> leader`. The majority with election rights will be kept online during the operation process.
    - `parallel` will force a parallel executor.

#### spec.defaultTerminationPolicy

`spec.defaultTerminatingPolicy` can be set as `DoNotTerminate`, `Halt`, `Delete`, and `WipeOut`.

#### spec.connectionCredential

`spec.connectionCredential` is used to create a connection secret.

### Example

```
apiVersion: dbaas.infracreate.com/v1alpha1
kind: ClusterDefinition
metadata:
  name: wesql
spec:
  type: state.mysql
  components:
  - typeName: mysql-a
    minAvailable: 3
    maxAvailable: 3
    defaultReplicas: 3
    componentType: consensus
    consensusSpec:
      leader:
        name: "leader"
        accessMode: readWrite
      followers:
      - name: "follower"
        accessMode: readonly
    service:
      ports:
      - protocol: TCP
        port: 3306
        targetPort: 3306
      type: LoadBalancer
    readonlyService:
      ports:
      - protocol: TCP
        port: 3306
        targetPort: 3306
      type: LoadBalancer
    podSpec:
      containers:
      - name: infracreate/wesql-server-8.0
        imagePullPolicy: IfNotPresent
        volumeMounts:
        - mountPath: /data
          name: data
      ports:
      - containerPort: 3306
        protocol: TCP
        name: mysql
      - containerPort: 13306
        protocol: TCP
        name: paxos
      env:
      - name: MYSQL_ROOT_HOST
        value: "%"
      - name: MYSQL_ROOT_USER
        value: "root"
      - name: MYSQL_ROOT_PASSWORD
        value:
      - name: MYSQL_ALLOW_EMPTY_PASSWORD
        value: "yes"
      - name: MYSQL_DATABASE
        value: "mydb"
      - name: MYSQL_USER
        value: "u1"
      - name: MYSQL_PASSWORD
        value: "u1"
      - name: CLUSTER_ID
        value: 1
      - name: CLUSTER_START_INDEX
        value: 1
      - name: REPLICATIONUSER
        value: "replicator"
      - name: REPLICATION_PASSWORD
        value:
      - name: MYSQL_TEMPLATE_CONFIG
        values:
      - name: MYSQL_CUSTOM_CONFIG
        values:
      - name: MYSQL_DYNAMIC_CONFIG
        values:
      command: [ "/bin/bash", "-c" ]
      args:
      - >
        cluster_info=""; 
        for (( i=0; i< $OPENDBAAS_REPLICASETS_PRIMARY_N; i++ )); do 
        if [ $i -ne 0 ]; then 
        cluster_info="$cluster_info;"; 
        fi; 
        host=$(eval echo \$OPENDBAAS_REPLICASETS_PRIMARY_"$i"_HOSTNAME) 
        cluster_info="$cluster_info$host:13306"; 
        done; 
        idx=0; 
        while IFS='-' read -ra ADDR; do
        for i in "${ADDR[@]}"; do
        idx=$i;
        done;
        done <<< "$OPENDBAAS_MY_POD_NAME"; 
        echo $idx; 
        cluster_info="$cluster_info@$(($idx+1))"; 
        echo $cluster_info; echo {{ .Values.cluster.replicaSetCount }}; 
        docker-entrypoint.sh mysqld --cluster-start-index=$CLUSTER_START_INDEX --cluster-info="$cluster_info" --cluster-id=$CLUSTER_ID
```

## AppVersion (for providers)

`AppVersion` enables providers to describe the image versions and condition variables of the corresponding database versions.

### AppVersion `spec`

#### spec.clusterDefinitionRef

`spec.clusterDefinitionRef` refers to `ClusterDefiniton` and its value should be the same as `ClusterDefinition`.

#### spec.component

`type` should be the same component name as `ClusterDefinition`.

### AppVersion `status`

You can check `phase` and `message` to view the executing status and result.

### Example

```
apiVersion: dbaas.kubeblocks.io/v1alpha1
kind:       AppVersion
metadata:
  name:     wesql-8.0.30
spec:
  clusterDefinitionRef: apecloud-wesql
  components:
    - type: replicasets
      podSpec:
        containers:
          - name: mysql
            image: apecloud/wesql-server:8.0.30-4.alpha1.20221031.g1aa54a3
            imagePullPolicy: IfNotPresent
```

## Cluster (for end users)

`Cluster` enables end users to describe the database cluster they want to create.

### Cluster `spec`

#### spec.clusterDefinitionRef

`spec.clusterDefinitionRef` refers to `ClusterDefiniton` and its value should be the same as `ClusterDefinition`.

#### spec.appVersionRef

It refers to AppVersion and its value should be the same as `AppVersion`.

#### spec.components

`type` points to the component name in ClusterDefinition.

`replicas`: If you edit `replicas`, horizontal scaling will be triggered. If the amount of `replicas` does not meet the limits of `definition`, an error occurs.

`resources`: If you edit the `requets` and `limits` of `resources`, vertical scaling will be triggered.

#### spec.volumeClaimTemplates

`spec.volumeClaimTemplates` points to `statefulset.spec.volumeClaimTemplates`.

### Cluster `status`

`status` describes the current state and progress of the `Cluster`. 

#### cluster.phase

`cluster.phase` includes `Running`, `Failed`, `Creating`, `Upgrading`, `Scaling`, `Deleting`, and `Abnormal`. You can observe the executing status by `phase` changes.

### Example

The following are examples for ApeCloud MySQL three nodes.

- Standard version:

  ```
  apiVersion: dbaas.infracrate.com/v1alpha1
  kind: Cluster
  metadata:
    name: mysql-a-series-standard
  spec:
      clusterDefinitionRef: wesql
      appVersionRef: wesql-8.0.18
      components:
        - name: "mysql-a-1"
          type: mysql-a
  ```

- Enterprise version:

  ```
  apiVersion: dbaas.infracrate.com/v1alpha1
  kind: Cluster
  metadata:
      name: mysql-a-series-enterprise
  spec:
      clusterDefinitionRef: wesql
      appVersionRef: wesql-8.0.18
      components:
        - name: "mysql-a-2"
          type: mysql-a
          replicas: 3
          resoures:
              requests:
                  cpu: 32
                  memory: 128Gi
              limits:
                  cpu: 32 
                  memory: 128Gi
  ```