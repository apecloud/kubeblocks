# Kafka

Apache Kafka is a distributed streaming platform designed to build real-time pipelines and can be used as a message broker or as a replacement for a log aggregation solution for big data applications.

- A broker is a Kafka server that stores data and handles requests from producers and consumers. Kafka clusters consist of multiple brokers, each identified by a unique ID. Brokers work together to distribute and replicate data across the cluster.
- KRaft was introduced in Kafka 3.3.1 in October 2022 as an alternative to Zookeeper. A subset of brokers are designated as controllers, and these controllers provide the consensus services that used to be provided by Zookeeper.

## Features In KubeBlocks

### Lifecycle Management

| Topology | Horizontal<br/>scaling | Vertical <br/>scaling | Expand<br/>volume | Restart   | Stop/Start | Configure | Expose | Switchover |
|----------|------------------------|-----------------------|-------------------|-----------|------------|-----------|--------|------------|
| Combined/Separated | Yes          | Yes                   | Yes               | Yes       | Yes        | Yes       | Yes    | N/A   |

- Combine Mode: KRaft (Controller) and Broker components are combined in the same pod.
- Separated Mode: KRaft (Controller) and Broker components are deployed in different pods.

### Backup and Restore

| Feature     | Method | Description |
|-------------|--------|------------|
| N/A | N/A | N/A |

### Versions

| Versions |
|----------|
| 3.3.2 |

## Prerequisites

- Kubernetes cluster >= v1.21
- `kubectl` installed, refer to [K8s Install Tools](https://kubernetes.io/docs/tasks/tools/)
- Helm, refer to [Installing Helm](https://helm.sh/docs/intro/install/)
- KubeBlocks installed and running, refer to [Install Kubeblocks](../docs/prerequisites.md)
- Kafka Addon Enabled, refer to [Install Addons](../docs/install-addon.md)
- Create K8s Namespace `demo`, to keep resources created in this tutorial isolated:

  ```bash
  kubectl create ns demo
  ```

## Examples

### Create

Create a Kafka cluster with combined controller and broker components

```bash
kubectl apply -f examples/kafka/cluster-combined.yaml
```

Create a Kafka cluster with separated controller and broker components:

```bash
kubectl apply -f examples/kafka/cluster-separated.yaml
```

### Horizontal scaling

> [!IMPORTANT]
> As per the Kafka documentation, the number of KRaft replicas should be odd to avoid split-brain scenarios.
> Make sure the number of KRaft replicas, i.e. Controller replicas,  is always odd after Horizontal Scaling, either in Separated or Combined mode.

#### [Scale-out](scale-out.yaml)

Horizontal scaling out `kafka-combine` component in cluster `kafka-combined-cluster` by adding ONE more replica:

```bash
kubectl apply -f examples/kafka/scale-out.yaml
```

After applying the operation, you will see a new pod created. You can check the progress of the scaling operation with following command:

```bash
kubectl describe -n demo ops kafka-combined-scale-out
```

#### [Scale-in](scale-in.yaml)

Horizontal scaling in  `kafka-combine` component in cluster `kafka-combined-cluster` by deleting ONE replica:

```bash
kubectl apply -f examples/kafka/scale-in.yaml
```

#### Scale-in/out using Cluster API

Alternatively, you can update the `replicas` field in the `spec.componentSpecs.replicas` section to your desired non-zero number.

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  componentSpecs:
    - name: kafka-combine
      replicas: 1 # Set the number of replicas to your desired number
```

### [Vertical scaling](verticalscale.yaml)

Vertical scaling up or down specified components requests and limits cpu or memory resource in the cluster:

```bash
kubectl apply -f examples/kafka/verticalscale.yaml
```

#### Scale-up/down using Cluster API

Alternatively, you may update `spec.componentSpecs.resources` field to the desired resources for vertical scale.

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  componentSpecs:
    - name: kafka-combine
      replicas: 1
      resources:
        requests:
          cpu: "1"       # Update the resources to your need.
          memory: "2Gi"  # Update the resources to your need.
        limits:
          cpu: "2"       # Update the resources to your need.
          memory: "4Gi"  # Update the resources to your need.
```

### [Expand volume](volumeexpand.yaml)

Volume expansion is the ability to increase the size of a Persistent Volume Claim (PVC) after it's created. It is introduced in Kubernetes v1.11 and goes GA in Kubernetes v1.24. It allows Kubernetes users to simply edit their PersistentVolumeClaim objects  without requiring any downtime at all if possible.

> [!NOTE]
> Make sure the storage class you use supports volume expansion.

Check the storage class with following command:

```bash
kubectl get storageclass
```

If the `ALLOWVOLUMEEXPANSION` column is `true`, the storage class supports volume expansion.

To increase size of volume storage with the specified components in the cluster:

```bash
kubectl apply -f examples/kafka/volumeexpand.yaml
```

After the operation, you will see the volume size of the specified component is increased to `30Gi` in this case. Once you've done the change, check the `status.conditions` field of the PVC to see if the resize has completed.

```bash
kubectl get pvc -l app.kubernetes.io/instance=kafka-combined-cluster -n demo
```

#### Volume expansion using Cluster API

Alternatively, you may update the `spec.componentSpecs.volumeClaimTemplates.spec.resources.requests.storage` field to the desired size.

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  componentSpecs:
    - name: kafka-combine
      volumeClaimTemplates:
        - name: data
          spec:
            storageClassName: "<you-preferred-sc>"
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 30Gi  # specify new size, and make sure it is larger than the current size
        - name: metadata
          spec:
            storageClassName: "<you-preferred-sc>"
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 10Gi  # specify new size, and make sure it is larger than the current size
```

### [Restart](restart.yaml)

Restart the specified components in the cluster

```bash
kubectl apply -f examples/kafka/restart.yaml
```

### [Stop](stop.yaml)

Stop the cluster and release all the pods of the cluster, but the storage will be reserved

```bash
kubectl apply -f examples/kafka/stop.yaml
```

#### Stop using Cluster API

Alternatively, you may stop the cluster by setting the `spec.componentSpecs.stop` field to `true`.

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  componentSpecs:
    - name: kafka-combine
      stop: true  # set stop `true` to stop the component
      replicas: 1
```

### [Start](start.yaml)

Start the stopped cluster

```bash
kubectl apply -f examples/kafka/start.yaml
```

#### Start using Cluster API

Alternatively, you may start the cluster by setting the `spec.componentSpecs.stop` field to `false`.

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  componentSpecs:
    - name: kafka-combine
      stop: false  # set to `false` (or remove this field) to start the component
      replicas: 1
```

### [Reconfigure](configure.yaml)

Configure parameters with the specified components in the cluster

```bash
kubectl apply -f examples/kafka/configure.yaml
```

This example update `log.flush.interval.ms` parameter of the `kafka-combine` component in the cluster `kafka-combined-cluster` to `1000`.
This parameter is the maximum time in ms that a message in any topic is kept in memory before flushed to disk. If not set, the value in log.flush.scheduler.interval.ms is used.

To verify the configuration change, you may log into the pod and check the configuration file.

```bash
cat  /opt/bitnami/kafka/config/kraft/server.properties | grep 'log.flush.interval.ms'
```

### Delete

If you want to delete the cluster and all its resource, you can modify the termination policy and then delete the cluster

```bash
kubectl patch cluster -n demo kafka-cluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete cluster -n demo kafka-cluster
```

### Observability

#### Installing the Prometheus Operator

You may skip this step if you have already installed the Prometheus Operator.
Or you can follow the steps in [How to install the Prometheus Operator](../docs/install-prometheus.md) to install the Prometheus Operator.

#### Create Cluster

Create a Kafka cluster with separated controller and broker components for instance:

```bash
kubectl apply -f examples/kafka/cluster-separated.yaml
```

#### Create PodMonitor

##### Step 1. Create PodMonitor

Apply the `PodMonitor` file to monitor the cluster.
Please set the labels correctly in the `PodMonitor` file to match the target pods.

```yaml
# cat pod monitor file
  selector:
    matchLabels:
      app.kubernetes.io/instance: kafka-separated-cluster  # cluster name, set it to your cluster name
      apps.kubeblocks.io/component-name: kafka-controller  # component name
```

- Pod Monitor Kafka JVM:

```bash
kubectl apply -f examples/kafka/jvm-pod-monitor.yaml
```

- Pod Monitor for Kafka Exporter:

```bash
kubectl apply -f examples/kafka/exporter-pod-monitor.yaml
```

##### Step 2. Accessing the Grafana Dashboard

Login to the Grafana dashboard and import the dashboard.

KubeBlocks provides a Grafana dashboard for monitoring the Kafka cluster. You can find it at [Kafka Dashboard](https://github.com/apecloud/kubeblocks-addons/tree/main/addons/kafka).

> [!NOTE]
>
> - Make sure the labels are set correctly in the `PodMonitor` file to match the dashboard.
> - set `job` to `kubeblocks` on Grafana dashboard to view the metrics.

### FAQ

#### How to Access Kafka Cluster

##### With Direct Pod Access

To connect to the Kafka cluster, you can use the following command to get the service for connection:

```bash
kubectl get svc -l app.kubernetes.io/instance=kafka-combined-cluster -n demo
```

And the excepted output is like below:

```text
NAME                                                         TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)    AGE
kafka-combined-cluster-kafka-combine-advertised-listener-0   ClusterIP   10.96.221.254   <none>        9092/TCP   28m
```

You can connect to the Kafka cluster using the `CLUSTER-IP` and `PORT`.

##### With NodePort Service

Currently only `nodeport` and `clusterIp` network modes are supported for Kafka
To access the Kafka cluster using the `nodeport` service, you can create Kafka cluster with the following configuration,  refer to [Kafka Network Modes Example](./cluster-combined-nodeport.yaml) for more details.

```yaml
# snippet of cluster.yaml
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
spec:
  componentSpecs:
    - name: kafka-combine
      stop: false  # set to `false` (or remove this field) to start the component
      services:
        - name: advertised-listener
          serviceType: NodePort
          podService: true
      replicas: 1
      env:
        - name: KB_KAFKA_BROKER_HEAP
          value: "-XshowSettings:vm -XX:MaxRAMPercentage=100 -Ddepth=64"
        - name: KB_KAFKA_CONTROLLER_HEAP
          value: "-XshowSettings:vm -XX:MaxRAMPercentage=100 -Ddepth=64"
        - name: KB_BROKER_DIRECT_POD_ACCESS # set KB_BROKER_DIRECT_POD_ACCESS to FALSE to disable direct pod access
          value: "false"
```
