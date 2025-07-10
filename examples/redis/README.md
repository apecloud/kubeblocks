# Redis

Redis is an open source (BSD licensed), in-memory data structure store, used as a database, cache and message broker. This example shows how it can be managed in Kubernetes with KubeBlocks.

## Features In KubeBlocks

### Lifecycle Management

|   Topology       | Horizontal<br/>scaling | Vertical <br/>scaling | Expand<br/>volume | Restart   | Stop/Start | Configure | Expose | Switchover |
|------------------|------------------------|-----------------------|-------------------|-----------|------------|-----------|--------|------------|
| replication     | Yes                    | Yes                   | Yes              | Yes       | Yes        | Yes       | Yes    | No      |
| standalone      | Yes                    | Yes                   | Yes              | Yes       | Yes        | Yes       | Yes    | N/A      |
| sharding      | Yes                    | Yes                   | Yes              | Yes       | Yes        | Yes       | Yes    | No      |

### Backup and Restore

| Feature     | Method | Description |
|-------------|--------|------------|
| Full Backup | datafile  | uses `redis-cli BGSAVE` command to backup data |
| Continuous Backup | aof | continuously perform incremental backups by archiving Append-Only Files (AOF) |

### Versions

| Major Versions | Description |
|---------------|-------------|
| 7.0           | 7.0.6 |
| 7.2           | 7.2.4 |

## Prerequisites

- Kubernetes cluster >= v1.21
- `kubectl` installed, refer to [K8s Install Tools](https://kubernetes.io/docs/tasks/tools/)
- Helm, refer to [Installing Helm](https://helm.sh/docs/intro/install/)
- KubeBlocks installed and running, refer to [Install Kubeblocks](../docs/prerequisites.md)
- Redis Addon Enabled, refer to [Install Addons](../docs/install-addon.md)
- Create K8s Namespace `demo`, to keep resources created in this tutorial isolated:

  ```bash
  kubectl create ns demo
  ```

## Examples

### [Create](cluster.yaml)

Create a Redis replication cluster with two components, one for Redis, and one for Redis Sentinel[^1].

For optimal reliability, you should run at least three Redis Sentinel replicas. Having three or more Sentinels ensures a quorum can be reached during failover decisions, maintaining the high availability of your Redis deployment.

```bash
kubectl apply -f examples/redis/cluster.yaml
```

A cluster named `redis-replication` will be created with 1 primary pod, 1 secondary pod, and 3 sentinel pods.

```bash
kubectl get -n demo cluster redis-replication # get cluster info
kubectl get pod -n demo -l app.kubernetes.io/instance=redis-replication # get all pods of the cluster
```

> [!NOTE]
> If all Pods are running, but the cluster status is still `Creating`, you may need to wait for a while until all Redis Pods are ready with corresponding Roles.

To check the role of each Redis pod, you can use the following command:

```bash
# replace `redis-replication` with your cluster name
kubectl get po -n demo -l app.kubernetes.io/instance=redis-replication,apps.kubeblocks.io/component-name=redis -L kubeblocks.io/role
```

#### Why Redis Sentinel starts first?

Redis Sentinel is a high availability solution for Redis. It provides monitoring, notifications, and automatic failover for Redis instances.

Each Redis replica, from the Redis component, upon startup, will connect to the Redis Sentinel instances to get the current leader and follower information. It needs to determine:

- Whether it should act as the primary (master) node.
- If not, which node is the current primary to replicate from.

In more detail, each Redis replica will:

1. Check for Existing Primary Node
    - Queries Redis Sentinel to find out if a primary node is already elected.
    - Retrieve the primary's address and port.
1. Initialize as Primary if Necessary
    - If no primary is found (e.g., during initial cluster setup), it configures the current Redis instance to become the primary.
    - Updates Redis configuration to disable replication.
1. Configure as Replica if Primary Exists
    - If a primary is found, it sets up the current Redis instance as a replica.
    - Updates the Redis configuration with the `replicaof` directive pointing to the primary's address and port.
    - Initiates replication to synchronize data from the primary.

KubeBlocks ensures that Redis Sentinel starts first to provide the necessary information for the Redis replicas to initialize correctly. Such dependency is well-expressed in the KubeBlocks CRD `ClusterDefinition` ensuring the correct startup order.

More details on how components for the `replication` topology are started, upgraded can be found in:

```bash
kubectl get cd redis -oyaml | yq '.spec.topologies[] | select(.name=="replication") | .orders'
```

### Horizontal scaling

#### [Scale-out](scale-out.yaml)

Horizontal scaling out Redis component by adding ONE more replica:

```bash
kubectl apply -f examples/redis/scale-out.yaml
```

After applying the operation, you will see a new pod created and the cluster status goes from `Updating` to `Running`, and the newly created pod has a new role `secondary`.

And you can check the progress of the scaling operation with following command:

```bash
kubectl describe -n demo ops redis-scale-out
```

#### [Scale-in](scale-in.yaml)

Horizontal scaling in Redis component by removing ONE replica:

```bash
kubectl apply -f examples/redis/scale-in.yaml
```

#### Scale-in/out using Cluster API

Alternatively, you can update the `replicas` field in the `spec.componentSpecs.replicas` section to your desired non-zero number.

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  componentSpecs:
    - name: redis
      serviceVersion: "7.2.4"
      replicas: 2 # decrease `replicas` for scaling in, and increase for scaling out
      disableExporter: false
```

### [Vertical scaling](verticalscale.yaml)

Vertical scaling involves increasing or decreasing resources to an existing database cluster.
Resources that can be scaled include:, CPU cores/processing power and Memory (RAM).

Vertical scaling up or down requests and limits cpu or memory resource for Redis component:

```bash
kubectl apply -f examples/redis/verticalscale.yaml
```

You will observe that the `follower` pods are recreated first, followed by the `leader` pod, to ensure the availability of the cluster.

#### Scale-up/down using Cluster API

Alternatively, you may update `spec.componentSpecs.resources` field to the desired resources for vertical scale.

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  componentSpecs:
    - name: redis
      serviceVersion: "7.2.4"
      replicas: 2 # decrease `replicas` for scaling in, and increase for scaling out
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

If you don't have a storage class that supports volume expansion, you can create a new one. Please refer to [CSI drivers](https://kubernetes-csi.github.io/docs/drivers.html) for more information.

To increase size of volume storage with the Redis component in the cluster:

```bash
kubectl apply -f examples/redis/volumeexpand.yaml
```

Once you've done the change, check the `status.conditions` field of the PVC to see if the resize has completed.

#### Volume expansion using Cluster API

Alternatively, you may update the `spec.componentSpecs.volumeClaimTemplates.spec.resources.requests.storage` field to the desired size.

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  componentSpecs:
    - name: redis
      serviceVersion: "7.2.4"
      replicas: 2
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
kubectl apply -f examples/redis/restart.yaml
```

### [Stop](stop.yaml)

Stop the cluster will release all the pods of the cluster, but the storage will be retained. It is useful when you want to save the cost of the cluster.

To stop the all component (without specifying the component name) in the cluster:

```bash
kubectl apply -f examples/redis/stop.yaml
```

#### Stop using Cluster API

Alternatively, you may stop the cluster by setting all `spec.componentSpecs.stop` field to `true`.

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  componentSpecs:
    - name: redis
      serviceVersion: "7.2.4"
      stop: true  # set stop `true` to stop the component
      replicas: 2
      ...
    - name: redis-sentinel
      stop: true  # set stop `true` to stop the component
      replicas: 3
      ....
```

### [Start](start.yaml)

Start the stopped cluster

```bash
kubectl apply -f examples/redis/start.yaml
```

#### Start using Cluster API

Alternatively, you may start the cluster by setting all `spec.componentSpecs.stop` field to `false`.

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  componentSpecs:
    - name: redis
      serviceVersion: "7.2.4"
      stop: false  # set to `false` (or remove this field) to start the component
      replicas: 2
    - name: redis-sentinel
      stop: false  # set stop `true` to stop the component
      replicas: 3
      ....
```

### [Reconfigure](configure.yaml)

A database reconfiguration is the process of modifying database parameters, settings, or configurations to improve performance, security, or availability. The reconfiguration can be either:

- Dynamic: Applied without restart
- Static: Requires database restart

Reconfigure parameters with the specified components in the cluster:

```bash
kubectl apply -f examples/redis/configure.yaml
```

This example will change the `maxclients` to `10001` for the Redis component.
`maxclients` in Redis specifies the maximum number of simultaneous client connections the server will accept. Once this limit is reached, Redis will start rejecting new connections until existing clients disconnect.

> [!CAUTION]
> It is defined as a static parameter, which means the Redis component will be restarted after the reconfiguration.

To verify the reconfiguration, you can connect to the Redis pod and check the configuration with the following command:

```bash
reids> config get maxclients
```

And the output should be:

```bash
1) "maxclients"
2) "10001"     # where 10001 is the new value set in the reconfiguration
```

### [Backup](backup.yaml)

> [!IMPORTANT]
> Before you start, please create a `BackupRepo` to store the backup data. Refer to [BackupRepo](../docs/create-backuprepo.md) for more details.

You may find the supported backup methods in the `BackupPolicy` of the cluster, e.g. `redis-replication-redis-backup-policy` in this case, and find how these methods will be scheduled in the `BackupSchedule` of the cluster, e.g.. `redis-replication-redis-backup-schedule` in this case.

To the the list of backup policies and schedules, you can use the following command:

```bash
kubectl get backuppolicy -n demo redis-replication-redis-backup-policy -oyaml | yq '.spec.backupMethods[].name'
```

And the output should be like:

```yaml
datafile  # for base backup
aof       # for pitr
volume-snapshot # for snapshot backup, make sure the storage class supports volume snapshot
```

#### [Full Backup](backup.yaml)

To create a backup for the reids component in the cluster:

```bash
kubectl apply -f examples/redis/backup.yaml
```

It will trigger a backup operation for the Redis component using `redis-cli BGSAVE` command against one secondary pod.
After the operation, you will see a `Backup` is created

```bash
kubectl get backup -n demo -l app.kubernetes.io/instance=redis-replication
```

Information, such as `path`, `timeRange` about the backup will be recorded into the `Backup` resource.

#### Continuous Backup

Redis Append Only Files(AOFs) record every write operation received by the server, in the order they were processed, which allows Redis to reconstruct the dataset by replaying these commands.
KubeBlocks supports continuous backup for the Redis component by archiving Append-Only Files (AOF). It will process incremental AOF files, update base AOF file, purge expired files and save backup status (records metadata about the backup process, such as total size and timestamps, to the `Backup` resource).

To create a continuous backup for the reids component, you should follow the steps below:

1. set variable `aof-timestamp-enabled` to `yes`

```bash
kubectl apply -f examples/redis/reconfigure-aof.yaml
```

> [!IMPORTANT]
> Once `aof-timestamp-enabled` is on, Redis will include timestamp in the AOF file.
> It may have following side effects: storage overhead, performance overhead (write latency).
> It is not recommended to enable this feature when you have high write throughput, or you have limited storage space.

1. enable continuous backup

update the `BackupSchedule` to enable the `aof` method.

```yaml
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: BackupSchedule
metadata:
  name: redis-replication-redis-backup-policy
  namespace: demo
spec:
  backupPolicyName: redis-replication-redis-backup-policy
  schedules:
  - backupMethod: datafile
    # ┌───────────── minute (0-59)
    # │ ┌───────────── hour (0-23)
    # │ │ ┌───────────── day of month (1-31)
    # │ │ │ ┌───────────── month (1-12)
    # │ │ │ │ ┌───────────── day of week (0-6) (Sunday=0)
    # │ │ │ │ │
    # 0 18 * * *
    # schedule this job every day at 6:00 PM (18:00).
    cronExpression: 0 18 * * * # update the cronExpression to your need
    enabled: false # set to `true` to schedule base backup periodically
    retentionPeriod: 7d # set the retention period to your need
  - backupMethod: aof
    cronExpression: 0 18 * * 0
    enabled: true     # set this to `true` to enable continous backup
    retentionPeriod: 7d
  - backupMethod: volume-snapshot
    cronExpression: 0 18 * * 0
    enabled: false   # set to `true` to schedule base backup using volume snapshot periodically
    retentionPeriod: 7d
```

#### Backup using Cluster API

Alternatively, you may update `spec.backup` field to the desired backup method.

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  # Specifies the backup configuration of the Cluster.
  backup:
    cronExpression: 0 18 * * *
    enabled: true     #  Specifies whether automated backup is enabled for the Cluster.
    method: datafile  #  Specifies the backup method to use, as defined in backupPolicy
    pitrEnabled: true #  Specifies whether to enable point-in-time recovery
    retentionPeriod: 7d # set the retention period to your need
    # Specifies the name of the BackupRepo to use for storing backups
    # If not specified, the default BackupRepo will be used.
    # `default` BackupRepo is the one annotated with `dataprotection.kubeblocks.io/is-default-repo=true`
    repoName: kb-oss
  componentSpecs:
    - name: redis
      serviceVersion: "7.2.4"
      ...
```

### [Restore](restore.yaml)

To restore a new cluster from a Backup:

1. Get the list of accounts and their passwords from the backup:

```bash
kubectl get backup -n demo acmysql-cluster-backup -ojsonpath='{.metadata.annotations.kubeblocks\.io/encrypted-system-accounts}'
```

1. Update `examples/redis/restore.yaml` and set placeholder `<ENCRYPTED-SYSTEM-ACCOUNTS>` with your own settings and apply it.

```bash
kubectl apply -f examples/redis/restore.yaml
```

### Expose

Expose a cluster with a new endpoint

#### [Enable](expose-enable.yaml)

```bash
kubectl apply -f examples/redis/expose-enable.yaml
```

#### [Disable](expose-disable.yaml)

```bash
kubectl apply -f examples/redis/expose-disable.yaml
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
    componentSelector: redis
    name: redis-vpc
    serviceName: redis-vpc
    # optional. it specify defined role as selector for the service.
    # onece specified, service will select and route traffic to Pods with the label
    # "kubeblocks.io/role=<specified-role-name>".
    # valid options are: [primary, secondary] for MySQL
    roleSelector: primary
    spec:  # defines the behavior of a K8s service.
      ipFamilyPolicy: PreferDualStack
      ports:
      - name: tcp-redis
        # port to expose
        port: 6379
        protocol: TCP
        targetPort: redis
      # Determines how the Service is exposed. Defaults to 'ClusterIP'.
      # Valid options are [`ClusterIP`, `NodePort`, and `LoadBalancer`]
      type: LoadBalancer
  componentSpecs:
    - name: redis
      serviceVersion: "7.2.4"
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

### Observability

There are various ways to monitor the cluster. Here we use Prometheus and Grafana to demonstrate how to monitor the cluster.

#### Installing the Prometheus Operator

You may skip this step if you have already installed the Prometheus Operator.
Or you can follow the steps in [How to install the Prometheus Operator](../docs/install-prometheus.md) to install the Prometheus Operator.

#### [Create a Cluster](cluster.yaml)

Create a new cluster with following command:

> [!NOTE]
> Make sure `spec.componentSpecs.disableExporter` is set to `false` when creating cluster.

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  componentSpecs:
    - name: redis
      serviceVersion: "7.2.4"
      disableExporter: false # set to `false` to enable exporter
```

```bash
kubectl apply -f examples/redis/cluster.yaml
```

When the cluster is running, each POD should have a sidecar container, named `metrics` running the postgres-exporter.

#### Create PodMonitor

##### Step 1. Query ScrapePath and ScrapePort

You can retrieve the `scrapePath` and `scrapePort` from pod's exporter container.

```bash
get po redis-replication-redis-0 -oyaml | yq '.spec.containers[] | select(.name=="metrics") | .ports'
```

And the expected output is like:

```text
- containerPort: 9121
  name: http-metrics
  protocol: TCP
```

Or you may check the `scrapePath` and `scrapePort` from the headless service of the Redis component.

And the expected output is like:

```text
monitor.kubeblocks.io/path: /metrics
monitor.kubeblocks.io/port: "9121"
monitor.kubeblocks.io/scheme: http
```

Or you may check the `scrapePath` and `scrapePort` from the `ComponentDefinition` of the Redis component.

1. check which container is used for the exporter

```bash
kubectl get cmpd <redis-cmpd-name> -oyaml | yq '.spec.exporter'
```

And the expected output is like:

```text
containerName: metrics  # which contaiiner for the exporter
scrapePath: /metrics    # scrape path
scrapePort: http-metrics # scrapte port
```

##### Step 2. Create PodMonitor

Apply the `PodMonitor` file to monitor the cluster:

```bash
kubectl apply -f examples/redis/pod-monitor.yaml
```

##### Step 3. Accessing the Grafana Dashboard

Login to the Grafana dashboard and import the dashboard.

There is a pre-configured dashboard for PostgreSQL under the `Redis` folder in the Grafana dashboard. And more dashboards can be found in the Grafana dashboard store[^5].

> [!NOTE]
> Make sure the labels are set correctly in the `PodMonitor` file to match the dashboard.

Sometimes the default dashboard may not work as expected, you may need to adjust the dashboard to match the labels the metrics are scraped with, in particular, the `job` label. In our case, the `job` variable should be set to `monitoring/redis-replication-pod-monitor` in the dashboard.

### Delete

If you want to delete the cluster and all its resource, you can modify the termination policy and then delete the cluster:

```bash
kubectl patch cluster -n demo redis-replication -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete cluster -n demo redis-replication
```

### More Examples to Create a Redis

Redis has various deployment topology, such as standalone, replication, sharding, etc. Here are some examples to create different Redis clusters.

#### Create Redis Replication with NodePort

Redis and Sentinel's advertise port is mainly used for correct node discovery and communication in distributed deployment environments. The advertise port is the port that a Redis or Sentinel node announces to other nodes or clients as its externally accessible service port. This is especially important in containerized, NAT, or port-mapped environments where the actual listening port inside the container may differ from the port exposed to the outside world.

- For Redis, the `cluster-announce-port` configuration allows the node to advertise a different port than the one it listens on. This ensures that other nodes or clients can connect to the correct external port.
- For Sentinel, the `sentinel announce-port` configuration serves a similar purpose, allowing Sentinel nodes to announce the correct external port for inter-node communication and monitoring.

This mechanism solves the problem where the internal and external ports are inconsistent, ensuring reliable communication and failover in Redis and Sentinel clusters.

To access redis replication from the outside of K8s cluster, you should create a redis replication cluster (with an official Redis Sentinel HA) with `NodePort` service type to advertise addresses.

```bash
kubectl apply -f examples/redis/cluster-with-nodeport.yaml
```

This example shows how to create a redis replication cluster and override the service type of the `redis-advertised` service to `NodePort`.
```yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  componentSpecs:
  - name: redis
    services:
    - name: redis-advertised  # override service named `redis-advertised` defined in ComponentDefinition
      serviceType: NodePort
      podService: true
# irrelevant lines commited
  - name: redis-sentinel
    services:
    - name: sentinel-advertised  # override service named `sentinel-advertised` defined in ComponentDefinition
      serviceType: NodePort
      podService: true
# irrelevant lines commited
```
Service `redis-advertised` and `redis-sentinel` are defined in `ComponentDefinition` name `redis-7` and `redis-sentinel-7`.  They are used to to parse the advertised endpoints of the Redis pods and Sentinel Pods.


#### Create Redis with Proxy

To create a redis with a proxy (Twemproxy) in front of it:

```bash
kubectl apply -f examples/redis/cluster-twemproxy.yaml
```

A cluster named `redis-twemproxy` will be created with three components, one for Redis (2 replicas), one for Sentinel (3 replicas), and one for twemproxy (3 replicas).

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  terminationPolicy: Delete
  clusterDef: redis
  topology: replication-twemproxy  # set topology to standalone
  componentSpecs:
  - name: redis
  - name: redis-sentinel
  - name: redis-twemproxy       # add one componet on provisioniing: twemproxy
    replicas: 3                 # set the desired number of replicas for twemproxy
    resources:
```

#### Create Redis with Multiple Shards

To create a redis sharding cluster (An official distributed Redis)  with 3 shards and 2 replica for each shard:

```bash
kubectl apply -f examples/redis/cluster-sharding.yaml
```

You may change the number of shards and replicas in the yaml file.

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  shardings:
  - name: shard
    shards: 3  # set the desired number of shards.
    template:
      name: redis
      componentDef: redis-cluster-7
      replicas: 2 # set the desired number of replicas for each shard.
      serviceVersion: 7.2.4
      # Component-level services override services defined in
      # referenced ComponentDefinition and expose
      # endpoints that can be accessed by clients
      # This example explicitly override the svc `redis-advertised` to use the NodePort
      services:
      - name: redis-advertised # This is a per-pod svc, and will be used to parse advertised endpoints
        podService: true
        #  - NodePort
        #  - LoadBalancer
        serviceType: NodePort
  ...
```

In this example we demonstrate how to create a Redis cluster with multiple shards, and how to override the service type of the `redis-advertised` service to `NodePort`.

The service `redis-advertised` is defined in `ComponentDefinition` and will be used to parse the advertised endpoints of the Redis pods.

By default, the service type is `NodePort`. If you want to expose the service to the outside of the cluster, you can override the service type to `NodePort` or `LoadBalancer` depending on your need.

Similarly to add or remove shards, you can update the `shardings` field in the `Cluster` resource.

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  shardings:
  - name: shard
    shards: 3 # increase or decrease the number of shards.
    template:
      name: redis
      componentDef: redis-cluster-7
      replicas: 2 # set the desired number of replicas for each shard.
      serviceVersion: 7.2.4
      stop: false # set to `true` to stop all components
```

## Reference

[^1]: Redis Sentinel: <https://redis.io/docs/latest/operate/oss_and_stack/management/sentinel/>
[^5]: Grafana Dashboard Store: <https://grafana.com/grafana/dashboards/>
