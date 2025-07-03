# PostgreSQL

PostgreSQL (Postgres) is an open source object-relational database known for reliability and data integrity. ACID-compliant, it supports foreign keys, joins, views, triggers and stored procedures.

## Features In KubeBlocks

### Lifecycle Management

|   Topology       | Horizontal<br/>scaling | Vertical <br/>scaling | Expand<br/>volume | Restart   | Stop/Start | Configure | Expose | Switchover |
|------------------|------------------------|-----------------------|-------------------|-----------|------------|-----------|--------|------------|
| replication     | Yes                    | Yes                   | Yes              | Yes       | Yes        | Yes       | Yes    | Yes      |

### Backup and Restore

| Feature     | Method | Description |
|-------------|--------|------------|
| Full Backup | pg-basebackup   | uses `pg_basebackup`,  a PostgreSQL utility to create a base backup   |
| Full Backup | postgres-wal-g   | uses `wal-g` to create a full backup (and don't forget to config wal-g by setting envs ahead) |
| Continuous Backup | postgresql-pitr | uploading PostgreSQL Write-Ahead Logging (WAL) files periodically to the backup repository. |

### Versions

| Major Versions | Description |
|---------------|-------------|
| 12            | 12.14.0,12.14.1,12.15.0,12.22.0|
| 14            | 14.7.2,14.8.0,14.18.0|
| 15            | 15.7.0,15.13.0|
| 16            | 16.4.0,16.9.0|
| 17            | 17.5.0|

## Prerequisites

- Kubernetes cluster >= v1.21
- `kubectl` installed, refer to [K8s Install Tools](https://kubernetes.io/docs/tasks/tools/)
- Helm, refer to [Installing Helm](https://helm.sh/docs/intro/install/)
- KubeBlocks installed and running, refer to [Install Kubeblocks](../docs/prerequisites.md)
- PostgreSQL Addon Enabled, refer to [Install Addons](../docs/install-addon.md)
- Create K8s Namespace `demo`, to keep resources created in this tutorial isolated:

  ```bash
  kubectl create ns demo
  ```

## Examples

### [Create](cluster.yaml)

Create a PostgreSQL cluster with one primary and one secondary instance:

```bash
kubectl apply -f examples/postgresql/cluster.yaml
```

And you will see the PostgreSQL cluster status goes `Running` after a while:

```bash
kubectl get cluster -n demo pg-cluster
```

and two pods are `Running` with roles `primary` and `secondary` separately. To check the roles of the pods, you can use following command:

```bash
# replace `pg-cluster` with your cluster name
kubectl get po -l  app.kubernetes.io/instance=pg-cluster -L kubeblocks.io/role -n demo
# or login to the pod and use `patronictl` to check the roles:
kubectl exec -it pg-cluster-postgresql-0 -n demo -- patronictl list
```

If you want to create a PostgreSQL cluster of specified version, set the `spec.componentSpecs.serviceVersion` field in the yaml file before applying it:

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  terminationPolicy: Delete
  clusterDef: postgresql
  topology: replication
  componentSpecs:
    - name: postgresql
      # ServiceVersion specifies the version of the Service expected to be
      # provisioned by this Component.
      # Valid options are: [12.14.0,12.14.1,12.15.0,12.22.0,14.7.2,14.8.0,14.18.0,15.7.0,15.13.0,16.4.0,16.9.0,17.5.0]
      serviceVersion: "14.7.2"
```

The list of supported versions can be found by following command:

```bash
kubectl get cmpv postgresql
```

And the expected output is like:

```bash
NAME         VERSIONS                                                                                    STATUS      AGE
postgresql   17.5.0,16.9.0,16.4.0,15.13.0,15.7.0,14.18.0,14.8.0,14.7.2,12.22.0,12.15.0,12.14.1,12.14.0   Available   Xd
```

### Horizontal scaling

#### [Scale-out](scale-out.yaml)

Horizontal scaling out PostgreSQL cluster by adding ONE more replica:

```bash
kubectl apply -f examples/postgresql/scale-out.yaml
```

After applying the operation, you will see a new pod created and the PostgreSQL cluster status goes from `Updating` to `Running`, and the newly created pod has a new role `secondary`.

And you can check the progress of the scaling operation with following command:

```bash
kubectl describe -n demo ops pg-scale-out
```

#### [Scale-in](scale-in.yaml)

Horizontal scaling in PostgreSQL cluster by deleting ONE replica:

```bash
kubectl apply -f examples/postgresql/scale-in.yaml
```

#### Scale-in/out using Cluster API

Alternatively, you can update the `replicas` field in the `spec.componentSpecs.replicas` section to your desired non-zero number.

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  componentSpecs:
    - name: postgresql
      serviceVersion: "14.7.2"
      labels:
        apps.kubeblocks.postgres.patroni/scope: pg-cluster-postgresql
      replicas: 2 # Update `replicas` to 1 for scaling in, and to 3 for scaling out
```

### [Vertical scaling](verticalscale.yaml)

Vertical scaling involves increasing or decreasing resources to an existing database cluster.
Resources that can be scaled include:, CPU cores/processing power and Memory (RAM).

To vertical scaling up or down specified component, you can apply the following yaml file:

```bash
kubectl apply -f examples/postgresql/verticalscale.yaml
```

You will observe that the `secondary` pod is recreated first, followed by the `primary` pod, to ensure the availability of the cluster.

#### Scale-up/down using Cluster API

Alternatively, you may update `spec.componentSpecs.resources` field to the desired resources for vertical scale.

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  componentSpecs:
    - name: postgresql
      replicas: 2
      resources:
        requests:
          cpu: "1"       # Update the resources to your need.
          memory: "2Gi"  # Update the resources to your need.
        limits:
          cpu: "2"       # Update the resources to your need.
          memory: "4Gi"  # Update the resources to your need.
```

### [Expand volume](volumeexpand.yaml)

Volume expansion is the ability to increase the size of a Persistent Volume Claim (PVC) after it's created. It is introduced in Kubernetes v1.11 and goes GA in Kubernetes v1.24. It allows Kubernetes users to simply edit their PersistentVolumeClaim objects  without requiring any downtime at all if possible[^4].

> [!NOTE]
> Make sure the storage class you use supports volume expansion.

Check the storage class with following command:

```bash
kubectl get storageclass
```

If the `ALLOWVOLUMEEXPANSION` column is `true`, the storage class supports volume expansion.

To increase size of volume storage with the specified components in the cluster

```bash
kubectl apply -f examples/postgresql/volumeexpand.yaml
```

After the operation, you will see the volume size of the specified component is increased to `30Gi` in this case. Once you've done the change, check the `status.conditions` field of the PVC to see if the resize has completed.

```bash
kubectl get pvc -l app.kubernetes.io/instance=pg-cluster -n demo
```

#### Volume expansion using Cluster API

Alternatively, you may update the `spec.componentSpecs.volumeClaimTemplates.spec.resources.requests.storage` field to the desired size.

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  componentSpecs:
    - name: postgresql
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

Restart the specified components in the cluster, and instances will be recreated on after another to ensure the availability of the cluster

```bash
kubectl apply -f examples/postgresql/restart.yaml
```

### [Stop](stop.yaml)

Stop the cluster will release all the pods of the cluster, but the storage will be retained. It is useful when you want to save the cost of the cluster.

```bash
kubectl apply -f examples/postgresql/stop.yaml
```

#### Stop using Cluster API

Alternatively, you may stop the cluster by setting the `spec.componentSpecs.stop` field to `true`.

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  componentSpecs:
    - name: postgresql
      stop: true  # set stop `true` to stop the component
      replicas: 2
```

### [Start](start.yaml)

Start the stopped cluster

```bash
kubectl apply -f examples/postgresql/start.yaml
```

#### Start using Cluster API

Alternatively, you may start the cluster by setting the `spec.componentSpecs.stop` field to `false`.

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  componentSpecs:
    - name: postgresql
      stop: false  # set to `false` (or remove this field) to start the component
      replicas: 2
```

### Switchover

A switchover in database clusters is a planned operation that transfers the primary (leader) role from one database instance to another. The goal of a switchover is to ensure that the database cluster remains available and operational during the transition.

#### [Switchover without preferred candidates](switchover.yaml)

To perform a switchover without any preferred candidates, you can apply the following yaml file:

```bash
kubectl apply -f examples/postgresql/switchover.yaml
```

<details>
<summary>Details</summary>

By applying this yaml file, KubeBlocks will perform a switchover operation defined in PostgreSQL's component definition, and you can checkout the details in `componentdefinition.spec.lifecycleActions.switchover`.

You may get the switchover operation details with following command:

```bash
kubectl get cluster -n demo pg-cluster -ojson | jq '.spec.componentSpecs[0].componentDef' | xargs kubectl get cmpd -ojson | jq '.spec.lifecycleActions.switchover'
```

</details>

#### [Switchover with candidate specified](switchover-specified-instance.yaml)

Switchover a specified instance as the new primary or leader of the cluster

```bash
kubectl apply -f examples/postgresql/switchover-specified-instance.yaml
```

You may need to update the `opsrequest.spec.switchover.instanceName` field to your desired instance name.

Once this `opsrequest` is completed, you can check the status of the switchover operation and the roles of the pods to verify the switchover operation.

### [Reconfigure](configure.yaml)

A database reconfiguration is the process of modifying database parameters, settings, or configurations to improve performance, security, or availability. The reconfiguration can be either:

- Dynamic: Applied without restart
- Static: Requires database restart

Reconfigure parameters with the specified components in the cluster

```bash
kubectl apply -f examples/postgresql/configure.yaml
```

This example will change the `max_connections` to `200`.
> `max_connections` indicates maximum number of client connections allowed. It is a dynamic parameter, so the change will take effect without restarting the database.

```bash
kbcli cluster explain-config pg-cluster # kbcli is a command line tool to interact with KubeBlocks
```

### [Backup](backup.yaml)

> [!IMPORTANT]
> Before you start, please create a `BackupRepo` to store the backup data. Refer to [BackupRepo](../docs/create-backuprepo.md) for more details.

KubeBlocks supports multiple backup methods for PostgreSQL cluster, such as `pg-basebackup`, `volume-snapshot`, `wal-g`, etc.

You may find the supported backup methods in the `BackupPolicy` of the cluster, e.g. `pg-cluster-postgresql-backup-policy` in this case, and find how these methods will be scheduled in the `BackupSchedule` of the cluster, e.g.. `pg-cluster-postgresql-backup-schedule` in this case.

We will elaborate on the `pg-basebackup` and `wal-g` backup methods in the following sections to demonstrate how to create full backup and continuous backup for the cluster.

#### pg_basebackup

##### Full Backup(backup-pg-basebasekup.yaml)

The method `pg-basebackup` uses `pg_basebackup`,  a PostgreSQL utility to create a base backup[^1]

To create a base backup for the cluster, you can apply the following yaml file:

```bash
kubectl apply -f examples/postgresql/backup-pg-basebasekup.yaml
```

After the operation, you will see a `Backup` is created

```bash
kubectl get backup -n demo -l app.kubernetes.io/instance=pg-cluster
```

and the status of the backup goes from `Running` to `Completed` after a while. And the backup data will be pushed to your specified `BackupRepo`.

Information, such as `path`, `timeRange` about the backup will be recorded into the `Backup` resource.

Alternatively, you can update the `BackupSchedule` to enable the method `pg-basebackup` to schedule base backup periodically, will be elaborated in the following section.

#### Continuous Backup

To enable continuous backup, you need to update `BackupSchedule` and enable the method `archive-wal`.

```yaml
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: BackupSchedule
metadata:
  name: pg-cluster-postgresql-backup-schedule
  namespace: demo
spec:
  backupPolicyName: pg-cluster-postgresql-backup-policy
  schedules:
  - backupMethod: pg-basebackup
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
  - backupMethod: archive-wal
    cronExpression: '*/5 * * * *'
    enabled: true  # set to `true` to enable continuouse backup
    retentionPeriod: 8d # set the retention period to your need
```

Once the `BackupSchedule` is updated, the continuous backup starts to work, and you can check the status of the backup with following command:

```bash
kubectl get backup -n demo -l app.kubernetes.io/instance=pg-cluster
```

And you will find one `Backup` named with suffix 'pg-cluster-postgresql-archive-wal' is created with the method `archive-wal`.

It will run continuously until you disable the method `archive-wal` in the `BackupSchedule`. And the valid time range of the backup will be recorded in the `Backup` resource:

```bash
kubectl get backup -n demo -l app.kubernetes.io/instance=pg-cluster -l dataprotection.kubeblocks.io/backup-type=Continuous  -oyaml | yq '.items[].status.timeRange'
```

#### wal-g

WAL-G is an archival restoration tool for PostgreSQL, MySQL/MariaDB, and MS SQL Server (beta for MongoDB and Redis).[^2]

##### [Full Backup](basebackup-wal-g.yaml)

To create wal-g backup for the cluster, it is a multi-step process.

1. configure WAL-G on all PostgreSQL pods

```bash
kubectl apply -f examples/postgresql/config-wal-g.yaml
```

1. set `archive_command` to `wal-g wal-push %p`

```bash
kubectl apply -f examples/postgresql/backup-wal-g.yaml
```

1. you cannot do wal-g backup for a brand-new cluster, you need to insert some data before backup

1. create a backup

```bash
kubectl apply -f examples/postgresql/backup-wal-g.yaml
```

> [!NOTE]
> if there is horizontal scaling out new pods after step 2, you need to do config-wal-g again

### [Restore](restore.yaml)

To restore a new cluster from a Backup:

1. Get the list of accounts and their passwords from the backup:

```bash
kubectl get backup -n demo pg-cluster-pg-basebackup -ojsonpath='{.metadata.annotations.kubeblocks\.io/encrypted-system-accounts}'
```

1. Update `examples/postgresql/restore.yaml` and set placeholder `<ENCRYPTED-SYSTEM-ACCOUNTS>` with your own settings and apply it.

```bash
kubectl apply -f examples/postgresql/restore.yaml
```

### Expose

Expose a cluster with a new endpoint

#### [Enable](expose-enable.yaml)

```bash
kubectl apply -f examples/postgresql/expose-enable.yaml
```

#### [Disable](expose-disable.yaml)

```bash
kubectl apply -f examples/postgresql/expose-disable.yaml
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
    componentSelector: postgresql
    name: postgresql-vpc
    serviceName: postgresql-vpc
    # optional. it specify defined role as selector for the service.
    # onece specified, service will select and route traffic to Pods with the label
    # "kubeblocks.io/role=<specified-role-name>".
    # valid options are: [primary, secondary] for postgresql
    roleSelector: primary
    spec:  # defines the behavior of a K8s service.
      ipFamilyPolicy: PreferDualStack
      ports:
      - name: tcp-postgresql
        # port to expose
        port: 5432
        protocol: TCP
        targetPort: tcp-postgresql
      type: LoadBalancer
```

If the service is of type `LoadBalancer`, please add annotations for cloud loadbalancer depending on the cloud provider you are using[^3]. Here list annotations for some cloud providers:

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

Please consult your cloud provider for more accurate and update-to-date information.

### [Upgrade](upgrade.yaml)

Upgrade postgresql cluster to another version

```bash
kubectl apply -f examples/postgresql/upgrade.yaml
```

In this example, the cluster will be upgraded to version `14.8.0`.

#### Upgrade using Cluster API

Alternatively, you may modify the `spec.componentSpecs.serviceVersion` field to the desired version to upgrade the cluster.

> [!WARNING]
> Do remember to to check the compatibility of versions before upgrading the cluster.

```bash
kubectl get cmpv postgresql -ojson | jq '.spec.compatibilityRules'
```

Expected output:

```json
[
  {
    "compDefs": [
      "postgresql-12-"
    ],
    "releases": [
      "12.14.0",
      "12.14.1",
      "12.15.0"
    ]
  },
  {
    "compDefs": [
      "postgresql-14-"
    ],
    "releases": [
      "14.7.2",
      "14.8.0"
    ]
  }
]
```

Releases are grouped by component definitions, and each group has a list of compatible releases.
In this example, it shows you can upgrade from version `12.14.0` to `12.14.1` or `12.15.0`, and upgrade from `14.7.2` to `14.8.0`.
But cannot upgrade from `12.14.0` to `14.8.0`.

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  componentSpecs:
    - name: postgresql
      serviceVersion: "14.8.0" # set to the desired version
      replicas: 2
      resources:
```

### Observability

There are various ways to monitor the cluster. Here we use Prometheus and Grafana to demonstrate how to monitor the cluster.

#### Installing the Prometheus Operator

You may skip this step if you have already installed the Prometheus Operator.
Or you can follow the steps in [How to install the Prometheus Operator](../docs/install-prometheus.md) to install the Prometheus Operator.

#### Create a Cluster

Create a new cluster with following command:

> [!NOTE]
> Make sure `spec.componentSpecs.disableExporter` is set to `false` when creating cluster.

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  componentSpecs:
    - name: postgresql
      serviceVersion: "14.7.2"
      disableExporter: false # set to `false` to enable exporter
```

```bash
kubectl apply -f examples/postgresql/cluster.yaml
```

When the cluster is running, each POD should have a sidecar container, named `exporter` running the postgres-exporter.

#### Create PodMonitor

##### Step 1. Query ScrapePath and ScrapePort

You can retrieve the `scrapePath` and `scrapePort` from pod's exporter container.

```bash
kubectl get po -n demo pg-cluster-postgresql-0 -oyaml | yq '.spec.containers[] | select(.name=="exporter") | .ports '
```

And the expected output is like:

```text
- containerPort: 9187
  name: http-metrics
  protocol: TCP
```

##### Step 2. Create PodMonitor

Apply the `PodMonitor` file to monitor the cluster:

```bash
kubectl apply -f examples/postgresql/pod-monitor.yaml
```

##### Step 3. Accessing the Grafana Dashboard

Login to the Grafana dashboard and import the dashboard.

There is a pre-configured dashboard for PostgreSQL under the `APPS / PostgreSQL` folder in the Grafana dashboard. And more dashboards can be found in the Grafana dashboard store[^5].

> [!NOTE]
> Make sure the labels are set correctly in the `PodMonitor` file to match the dashboard.

### Delete

If you want to delete the cluster and all its resource, you can modify the termination policy and then delete the cluster

```bash
kubectl patch cluster -n demo pg-cluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete cluster -n demo pg-cluster
```

## Appendix

### How to Create PostgreSQL Cluster with ETCD or Zookeeper as DCS

```bash
kubectl apply -f examples/postgresql/cluster-with-etcd.yaml
```

This example will create a PostgreSQL cluster with ETCD as DCS. It will set two envs:

- `DCS_ENABLE_KUBERNETES_API` to empty, unset this env if you use zookeeper or etcd, default to empty
- `ETCD3_HOST` to the ETCD cluster's headless service address, or use `ETCD3_HOSTS` to set ETCD hosts and ports.

Similar for Zookeeper, you can set `ZOOKEEPER_HOSTS` to the Zookeeper's address.

More environment variables can be found in [Spilo's ENVIRONMENT](https://github.com/zalando/spilo/blob/master/ENVIRONMENT.rst).

## Reference

[^1]: pg_basebackup, <https://www.postgresql.org/docs/current/app-pgbasebackup.html>
[^2]: wal-g <https://github.com/wal-g/wal-g>
[^3]: Internal load balancer: <https://kubernetes.io/docs/concepts/services-networking/service/#internal-load-balancer>
[^4]: Volume Expansion: <https://kubernetes.io/blog/2022/05/05/volume-expansion-ga/>
[^5]: Grafana Dashboard Store: <https://grafana.com/grafana/dashboards/>
