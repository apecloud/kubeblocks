# Qdrant

Qdrant is an open source (Apache-2.0 licensed), vector similarity search engine and vector database.

## Features In KubeBlocks

### Lifecycle Management

|   Topology       | Horizontal<br/>scaling | Vertical <br/>scaling | Expand<br/>volume | Restart   | Stop/Start | Configure | Expose | Switchover |
|------------------|------------------------|-----------------------|-------------------|-----------|------------|-----------|--------|------------|
| cluster     | Yes                    | Yes                   | Yes              | Yes       | Yes        | No       | Yes    | N/A     |

### Backup and Restore

| Feature     | Method | Description |
|-------------|--------|------------|
| Full Backup | datafile | uses HTTP API `snapshot` to create snapshot for all collections. |

### Versions

| Major Versions | Description |
|---------------|-------------|
| 1.5 | 1.5.0 |
| 1.7 | 1.7.3 |
| 1.8 | 1.8.1,1.8.4 |
| 1.10| 1.10.0 |

## Prerequisites

- Kubernetes cluster >= v1.21
- `kubectl` installed, refer to [K8s Install Tools](https://kubernetes.io/docs/tasks/tools/)
- Helm, refer to [Installing Helm](https://helm.sh/docs/intro/install/)
- KubeBlocks installed and running, refer to [Install Kubeblocks](../docs/prerequisites.md)
- Qdrant Addon Enabled, refer to [Install Addons](../docs/install-addon.md)
- Create K8s Namespace `demo`, to keep resources created in this tutorial isolated:

  ```bash
  kubectl create ns demo
  ```

## Examples

### [Create](cluster.yaml)

Create a Qdrant cluster with specified cluster definition

```bash
kubectl apply -f examples/qdrant/cluster.yaml
```

### Horizontal scaling

> [!Important]
> Qdrant uses the **Raft consensus protocol** to maintain consistency regarding the cluster topology and the collections structure.
> Make sure to have an odd number of replicas, such as 3, 5, 7, to avoid split-brain scenarios, after scaling out/in the cluster.

#### [Scale-out](scale-out.yaml)

Horizontal scaling out cluster by adding ONE more  replica:

```bash
kubectl apply -f examples/qdrant/scale-out.yaml
```

#### [Scale-in](scale-in.yaml)

Horizontal scaling in cluster by deleting ONE replica:

```bash
kubectl apply -f examples/qdrant/scale-in.yaml
```

> [!IMPORTANT]
> On scale-in, data will be redistributed among the remaining replicas. Make sure to have enough capacity to accommodate the data.
> The data redistribution process may take some time depending on the amount of data.
> It is handled by Qdrant `MemberLeave` operation, and Pods won't be deleted until the data redistribution, i.e. the `MemberLeave` actions completed successfully.

#### Scale-in/out using Cluster API

Alternatively, you can update the `replicas` field in the `spec.componentSpecs.replicas` section to your desired non-zero number.

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  componentSpecs:
    - name: qdrant
      replicas: 3 # Update `replicas` to your desired number
```

### [Vertical scaling](verticalscale.yaml)

Vertical scaling up or down specified components requests and limits cpu or memory resource in the cluster

```bash
kubectl apply -f examples/qdrant/verticalscale.yaml
```

#### Scale-up/down using Cluster API

Alternatively, you may update `spec.componentSpecs.resources` field to the desired resources for vertical scale.

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  componentSpecs:
    - name: qdrant
      componentDef: qdrant
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

> [!NOTE]
> Make sure the storage class you use supports volume expansion.

Check the storage class with following command:

```bash
kubectl get storageclass
```

If the `ALLOWVOLUMEEXPANSION` column is `true`, the storage class supports volume expansion.

To increase size of volume storage with the specified components in the cluster

Increase size of volume storage with the specified components in the cluster

```bash
kubectl apply -f examples/qdrant/volumeexpand.yaml
```

#### Volume expansion using Cluster API

Alternatively, you may update the `spec.componentSpecs.volumeClaimTemplates.spec.resources.requests.storage` field to the desired size.

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  componentSpecs:
    - name: qdrant
      componentDef: qdrant
      replicas: 3
      volumeClaimTemplates:
        - name: data
          spec:
            storageClassName: ""
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                # specify new size, and make sure it is larger than the current size
                storage: 30Gi
```

### [Restart](restart.yaml)

Restart the specified components in the cluster

```bash
kubectl apply -f examples/qdrant/restart.yaml
```

### [Stop](stop.yaml)

Stop the cluster and release all the pods of the cluster, but the storage will be reserved

```bash
kubectl apply -f examples/qdrant/stop.yaml
```

### [Start](start.yaml)

Start the stopped cluster

```bash
kubectl apply -f examples/qdrant/start.yaml
```

### [Backup](backup.yaml)

> [!NOTE]
> Before you start, please create a `BackupRepo` to store the backup data. Refer to [BackupRepo](../docs/create-backuprepo.md) for more details.

The backup method uses the `snapshot` API to create a snapshot for all collections. It works as follows:

- Retrieve all Qdrant collections
- For each collection, create a snapshot, validate the snapshot, and push it to the backup repository

To create a backup for the cluster:

```bash
kubectl apply -f examples/qdrant/backup.yaml
```

After the operation, you will see a `Backup` is created

```bash
kubectl get backup -n demo -l app.kubernetes.io/instance=qdrant-cluster
```

and the status of the backup goes from `Running` to `Completed` after a while. And the backup data will be pushed to your specified `BackupRepo`.

### [Restore](restore.yaml)

To restore a new cluster from backup:

```bash
kubectl apply -f examples/qdrant/restore.yaml
```

The restore methods uses Qdrant HTTP API to restore snapshots to the new cluster, when the restore is completed, the new cluster will be ready to use.

### Observability

There are various ways to monitor the cluster. Here we use Prometheus and Grafana to demonstrate how to monitor the cluster.

#### Installing the Prometheus Operator

You may skip this step if you have already installed the Prometheus Operator.
Or you can follow the steps in [How to install the Prometheus Operator](../docs/install-prometheus.md) to install the Prometheus Operator.

#### Create PodMonitor

##### Step 1. Create PodMonitor

Apply the `PodMonitor` file to monitor the cluster:

```bash
kubectl apply -f examples/qdrant/pod-monitor.yaml
```

It sets path to `/metrics` and port to `tcp-qdrant` (for container port `6333`).

```yaml
    - path: /metrics
      port: tcp-qdrant
      scheme: http
```

##### Step 2. Accessing the Grafana Dashboard

Login to the Grafana dashboard and import the dashboard, e.g. using dashboard from [Grafana](https://grafana.com/grafana/dashboards).

> [!NOTE]
> Make sure the labels are set correctly in the `PodMonitor` file to match the dashboard.

### Delete

If you want to delete the cluster and all its resource, you can modify the termination policy and then delete the cluster

```bash
kubectl patch cluster -n demo qdrant-cluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete cluster -n demo qdrant-cluster
```
