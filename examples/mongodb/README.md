# MongoDB

MongoDB is a document database designed for ease of application development and scaling

## Features In KubeBlocks

### Lifecycle Management

| Topology | Horizontal<br/>scaling | Vertical <br/>scaling | Expand<br/>volume | Restart   | Stop/Start | Configure | Expose | Switchover |
|----------|------------------------|-----------------------|-------------------|-----------|------------|-----------|--------|------------|
| replica set | Yes                    | Yes                   | Yes              | Yes       | Yes        | Yes       | Yes    | Yes      |

### Backup and Restore

| Feature     | Method | Description |
|-------------|--------|------------|
| Full Backup | dump   | uses `mongodump`, a MongoDB utility used to create a binary export of the contents of a database  |
| Full Backup | datafile | backup the data files of the database |

### Versions

| Major Versions | Description |
|---------------|--------------|
| 4.0 | 4.0.28,4.2.24,4.4.29 |
| 5.0 | 5.0.28 |
| 6.0 | 6.0.16 |
| 7.0 | 7.0.12 |

## Prerequisites

- Kubernetes cluster >= v1.21
- `kubectl` installed, refer to [K8s Install Tools](https://kubernetes.io/docs/tasks/tools/)
- Helm, refer to [Installing Helm](https://helm.sh/docs/intro/install/)
- KubeBlocks installed and running, refer to [Install Kubeblocks](../docs/prerequisites.md)
- MongoDB Addon Enabled, refer to [Install Addons](../docs/install-addon.md)
- Create K8s Namespace `demo`, to keep resources created in this tutorial isolated:

  ```bash
  kubectl create ns demo
  ```

## Examples

### [Create](cluster.yaml)

Create a MongoDB replicaset cluster with 1 primary replica and 2 secondary replicas:

```bash
kubectl apply -f examples/mongodb/cluster.yaml
```

To check the roles of the pods, you can use following command:

```bash
# replace `mongo-cluster` with your cluster name
kubectl get po -n demo -l  app.kubernetes.io/instance=mongo-cluster -L kubeblocks.io/role
```

If you want to create a cluster of specified version, set the `spec.componentSpecs.serviceVersion` field in the yaml file before applying it:

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  componentSpecs:
    - name: mongodb
      # ServiceVersion specifies the version of the Service expected to be
      # provisioned by this Component.
      # Valid options are: [4.0.28,4.2.24,4.4.29,5.0.28,6.0.16,7.0.1]
      serviceVersion: "7.0.1"
```

The list of supported versions can be found by following command:

```bash
kubectl get cmpv mongodb
```

#### What is MongoDB Replica Set?

A MongoDB replica set[^1] is a group of MongoDB servers that maintain the same dataset, providing high availability and data redundancy. Replica sets are the foundation of MongoDB's fault tolerance and data reliability. By replicating data across multiple nodes, MongoDB ensures that if one server fails, another can take over seamlessly without affecting the application's availability.

In a replica set, there are typically three types of nodes:

- Primary Node: Handles all write operations and serves read requests by default.
- Secondary Nodes: Maintain copies of the primary's data and can optionally serve read requests.
- Arbiter Node: Participates in elections but does not store data. It is used to maintain an odd number of voting members in the replica set.

And it is recommended to create a cluster with at least three nodes to ensure high availability, one primary and two secondary nodes.

### Horizontal scaling

#### [Scale-out](scale-out.yaml)

Horizontal scaling out cluster by adding ONE more replica:

```bash
kubectl apply -f examples/mongodb/scale-out.yaml
```

After applying the operation, you will see a new pod created and the cluster status goes from `Updating` to `Running`, and the newly created pod has a new role `secondary`.

And you can check the progress of the scaling operation with following command:

```bash
kubectl describe -n demo ops mongo-scale-out
```

#### [Scale-in](scale-in.yaml)

Horizontal scaling in cluster by deleting ONE replica:

```bash
kubectl apply -f examples/mongodb/scale-in.yaml
```

On horizontal scaling in/out, member list of the replica set will be updated to make sure the cluster is healthy.

You may verify the full list of members in the replica set by connecting to any pod, and  running the following command:

```bash
mongo-cluster-mongodb > rs.status();
```

#### [Set Specified Replicas Offline](scale-in-specified-instance.yaml)

There are cases where you want to set a specified replica offline, when it is problematic or you want to do some maintenance work on it. You can use the `onlineInstancesToOffline` field in the `spec.horizontalScaling.scaleIn` section to specify the instance names that need to be taken offline.

```yaml
# snippet of opsrequest
apiVersion: operations.kubeblocks.io/v1alpha1
kind: OpsRequest
spec:
  clusterName: mongo-cluster
  horizontalScaling:
  - componentName: mongodb
    # Specifies the replica changes for scaling out components
    scaleIn:
      onlineInstancesToOffline:
        - 'mongo-cluster-mongodb-1'  # Specifies the instance names that need to be taken offline
```

#### Scale-in/out using Cluster API

Alternatively, you can update the `replicas` field in the `spec.componentSpecs.replicas` section to your desired non-zero number.

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  componentSpecs:
    - name: mongodb
      replicas: 3 # set desired number of replicas
      # Optional. Specifies the names of instances to be transitioned to offline status.
      # If no specified, KubeBlocks will select the instances in descending ordinal number order.
      offlineInstances:
      - mongo-cluster-mongodb-1
```

### [Vertical scaling](verticalscale.yaml)

Vertical scaling involves increasing or decreasing resources to an existing database cluster.
Resources that can be scaled include:, CPU cores/processing power and Memory (RAM).

To vertical scaling up or down specified component, you can apply the following yaml file:

```bash
kubectl apply -f examples/mongodb/verticalscale.yaml
```

You will observe that the `secondary` pods are recreated first, followed by the `primary` pod, to ensure the availability of the cluster.

#### Scale-up/down using Cluster API

Alternatively, you may update `spec.componentSpecs.resources` field to the desired resources for vertical scale.

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  componentSpecs:
    - name: mongodb
      replicas: 3
      resources:
        requests:
          cpu: "1"       # Update the resources to your need.
          memory: "2Gi"  # Update the resources to your need.
        limits:
          cpu: "2"       # Update the resources to your need.
          memory: "4Gi"  # Update the resources to your need.
```

### [Expand volume](volumeexpand.yaml)

Volume expansion is the ability to increase the size of a Persistent Volume Claim (PVC) after it's created. It is introduced in Kubernetes v1.11 and goes GA in Kubernetes v1.24. It allows Kubernetes users to simply edit their PersistentVolumeClaim objects without requiring any downtime at all if possible.

> [!NOTE]
> Make sure the storage class you use supports volume expansion.

Check the storage class with following command:

```bash
kubectl get storageclass
```

If the `ALLOWVOLUMEEXPANSION` column is `true`, the storage class supports volume expansion.

To increase size of volume storage with the specified components in the cluster

```bash
kubectl apply -f examples/mongodb/volumeexpand.yaml
```

After the operation, you will see the volume size of the specified component is increased to `30Gi` in this case. Once you've done the change, check the `status.conditions` field of the PVC to see if the resize has completed.

```bash
kubectl get pvc -l app.kubernetes.io/instance=mongo-cluster -n demo
```

#### Volume expansion using Cluster API

Alternatively, you may update the `spec.componentSpecs.volumeClaimTemplates.spec.resources.requests.storage` field to the desired size.

```yaml
apiVersion: apps.kubeblocks.io/v1
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  componentSpecs:
    - name: mongodb
      volumeClaimTemplates:
        - name: data
          spec:
            storageClassName: "<you-preferred-sc>"
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 30Gi  # specify new size, and make sure it is larger than the current size
```

### [Restart](restart.yaml)

Restart the specified components in the cluster

```bash
kubectl apply -f examples/mongodb/restart.yaml
```

### [Stop](stop.yaml)

Stop the cluster will release all the pods of the cluster, but the storage will be retained. It is useful when you want to save the cost of the cluster.

```bash
kubectl apply -f examples/mongodb/stop.yaml
```

#### Stop using Cluster API

Alternatively, you may stop the cluster by setting the `spec.componentSpecs.stop` field to `true`.

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  componentSpecs:
    - name: mongodb
      stop: true  # set stop `true` to stop the component
      replicas: 3
```

### [Start](start.yaml)

Start the stopped cluster

```bash
kubectl apply -f examples/mongodb/start.yaml
```

#### Start using Cluster API

Alternatively, you may start the cluster by setting the `spec.componentSpecs.stop` field to `false`.

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  componentSpecs:
    - name: mongodb
      stop: false  # set to `false` (or remove this field) to start the component
      replicas: 3
```

### [Switchover](switchover.yaml)

A switchover in database clusters is a planned operation that transfers the primary (leader) role from one database instance to another. The goal of a switchover is to ensure that the database cluster remains available and operational during the transition.

To promote a non-primary or non-leader instance as the new primary or leader of the cluster:

```bash
kubectl apply -f examples/mongodb/switchover.yaml
```

### [Switchover-specified-instance](switchover-specified-instance.yaml)

Switchover a specified instance as the new primary or leader of the cluster

```bash
kubectl apply -f examples/mongodb/switchover-specified-instance.yaml
```

### [Reconfigure](configure.yaml)

A database reconfiguration is the process of modifying database parameters, settings, or configurations to improve performance, security, or availability. The reconfiguration can be either:

- Dynamic: Applied without restart
- Static: Requires database restart

Reconfigure parameters with the specified components in the cluster

```bash
kubectl apply -f examples/mongodb/configure.yaml
```

This example sets `systemLog.verbosity` to `1` and `systemLog.quiet`to `true`.
You may log into the instance to verify the change:

1. Connect to MongoDB using mongosh.
2. Execute an admin command to retrieve values.

```bash
const config = db.adminCommand({ getCmdLineOpts: 1 });
print("systemLog.quiet:", config.parsed.systemLog.quiet);
print("systemLog.verbosity:", config.parsed.systemLog.verbosity);
```

### [Backup](backup.yaml)

> [!IMPORTANT]
> Before you start, please create a `BackupRepo` to store the backup data. Refer to [BackupRepo](../docs/create-backuprepo.md) for more details.

You may find the supported backup methods in the `BackupPolicy` of the cluster, and find how these methods will be scheduled in the `BackupSchedule` of the cluster.

The list of supported backup methods can be found by following command:

```bash
# mongo-cluster-mongodb-backup-policy is the backup policy name
kubectl get backuppolicy -n demo mongo-cluster-mongodb-backup-policy -oyaml | yq '.spec.backupMethods[].name'
```

TO create a backup for the the cluster using `datafile` method:

```bash
kubectl apply -f examples/mongodb/backup.yaml
```

Information, such as `path`, `timeRange` about the backup will be recorded into the `Backup` resource.

Alternatively, you can update the `BackupSchedule` to enable the method `xtrabackup` to schedule base backup periodically.

### [Restore](restore.yaml)

Restore a new cluster from a backup

1. Get the list of accounts and their passwords from the backup:

```bash
kubectl get backup -n demo mongo-cluster-backup -ojsonpath='{.metadata.annotations.kubeblocks\.io/encrypted-system-accounts}'
```

1. Update `examples/mongodb/restore.yaml` and set placeholder `<ENCRYPTED-SYSTEM-ACCOUNTS>` with your own settings and apply it.

```bash
kubectl apply -f examples/mongodb/restore.yaml
```

### Expose

Expose a cluster with a new endpoint

#### [Enable](expose-enable.yaml)

```bash
kubectl apply -f examples/mongodb/expose-enable.yaml
```

#### [Disable](expose-disable.yaml)

```bash
kubectl apply -f examples/mongodb/expose-disable.yaml
```

#### Expose SVC using Cluster API

Alternatively, you may expose service by updating `spec.services`

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  # append service to the list
  services:
    # add annotation for cloud loadbalancer if
    # services.spec.type is LoadBalancer
    # here we use annotation for alibaba cloud for example
  - annotations:
      service.beta.kubernetes.io/alibaba-cloud-loadbalancer-address-type: internet
    componentSelector: mongodb
    name: mongodb-vpc
    serviceName: mongodb-vpc
    # optional. it specify defined role as selector for the service.
    # onece specified, service will select and route traffic to Pods with the label
    # "kubeblocks.io/role=<specified-role-name>".
    # valid options are: [primary, secondary]
    roleSelector: primary
    spec:  # defines the behavior of a K8s service.
      ipFamilyPolicy: PreferDualStack
      ports:
      - name: tcp-mongodb
        # port to expose
        port: 27017
        protocol: TCP
        targetPort: mongodb
      # Determines how the Service is exposed. Defaults to 'ClusterIP'.
      # Valid options are [`ClusterIP`, `NodePort`, and `LoadBalancer`]
      type: LoadBalancer
  componentSpecs:
    - name: mongodb
      replicas: 2
      ...
```

If the service is of type `LoadBalancer`, please add annotations for cloud loadbalancer depending on the cloud provider you are using. Here list annotations for some cloud providers:

```yaml
# alibaba cloud
service.beta.kubernetes.io/alibaba-cloud-loadbalancer-address-type: "internet"  # or "intranet"

# aws
service.beta.kubernetes.io/aws-load-balancer-type: nlb  # Use Network Load Balancer
service.beta.kubernetes.io/aws-load-balancer-internal: "true"  # or "false" for internet

# azure
service.beta.kubernetes.io/azure-load-balancer-internal: "true" # or "false" for internet

# gcp
networking.gke.io/load-balancer-type: "Internal" # for internal access
cloud.google.com/l4-rbs: "enabled" # for internet
```

### Delete

If you want to delete the cluster and all its resource, you can modify the termination policy and then delete the cluster

```bash
kubectl patch cluster -n demo mongo-cluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete cluster -n demo mongo-cluster
```

## References

[^1]: MongoDB Replica Set, <https://www.mongodb.com/docs/manual/replication/>
