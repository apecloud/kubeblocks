# RabbitMQ

RabbitMQ is an open-source and lightweight message broker which supports multiple messaging protocols.

## Features In KubeBlocks

### Lifecycle Management

|   Topology       | Horizontal<br/>scaling | Vertical <br/>scaling | Expand<br/>volume | Restart   | Stop/Start | Configure | Expose | Switchover |
|------------------|------------------------|-----------------------|-------------------|-----------|------------|-----------|--------|------------|
| cluster     | Yes                    | Yes                   | Yes              | Yes       | Yes        | No       | Yes    | N/A     |

### Versions

| Major Versions | Description |
|---------------|-------------|
| 3.8 | 3.8.14|
| 3.9 | 3.9.29|
| 3.10 | 3.10.25|
| 3.11 | 3.11.28|
| 3.12 | 3.12.14|
| 3.13 | 3.13.2, 3.13.7|

## Prerequisites

- Kubernetes cluster >= v1.21
- `kubectl` installed, refer to [K8s Install Tools](https://kubernetes.io/docs/tasks/tools/)
- Helm, refer to [Installing Helm](https://helm.sh/docs/intro/install/)
- KubeBlocks installed and running, refer to [Install Kubeblocks](../docs/prerequisites.md)
- RabbitMQ Addon Enabled, refer to [Install Addons](../docs/install-addon.md)
- Create K8s Namespace `demo`, to keep resources created in this tutorial isolated:

  ```bash
  kubectl create ns demo
  ```

## Examples

### [Create](cluster.yaml)

Create a rabbitmq cluster with 3 replicas:

```bash
kubectl apply -f examples/rabbitmq/cluster.yaml
```

> [!Important]
> Unlike others, on creating the cluster, this example creates a ServiceAccount, Role, and RoleBinding for the RabbitMQ cluster.
> RabbitMQ needs `peer discovery` role to create events and get endpoints. This is essential for discovering other RabbitMQ nodes and forming a cluster.
> When `PulicyRule` API is ready, rules defined in the `Role` can be defined in the `ComponentDefintion.Spec.PolicyRule`. Such that KubeBlocks will automatically create and manage the `Role` and `RoleBinding` for the component.

### Horizontal scaling

> [!Important]
> RabbitMQ quorum queue are designed based on the **Raft consensus algorithm**.
> Make sure to have an odd number of replicas, such as 3, 5, 7, to avoid split-brain scenarios, after scaling out/in the cluster.

#### [Scale-out](scale-out.yaml)

Horizontal scaling out cluster by adding ONE more  replica:

```bash
kubectl apply -f examples/rabbitmq/scale-out.yaml
```

#### [Scale-in](scale-in.yaml)

Horizontal scaling in cluster by deleting ONE replica:

```bash
kubectl apply -f examples/rabbitmq/scale-in.yaml
```

On scale-in, the replica with the highest number (if not specified in particular) will be stopped, removed and be `forget_cluster_node` from the cluster.

#### Scale-in/out using Cluster API

Alternatively, you can update the `replicas` field in the `spec.componentSpecs.replicas` section to your desired non-zero number.

```yaml
spec:
  componentSpecs:
    - name: rabbitmq
      componentDef: rabbitmq
      replicas: 3 # Update `replicas` to your desired number
```

### [Vertical scaling](verticalscale.yaml)

Vertical scaling up or down specified components requests and limits cpu or memory resource in the cluster

```bash
kubectl apply -f examples/rabbitmq/verticalscale.yaml
```

#### Scale-up/down using Cluster API

Alternatively, you may update `spec.componentSpecs.resources` field to the desired resources for vertical scale.

```yaml
spec:
  componentSpecs:
    - name: rabbitmq
      componentDef: rabbitmq
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
kubectl apply -f examples/rabbitmq/volumeexpand.yaml
```

#### Volume expansion using Cluster API

Alternatively, you may update the `spec.componentSpecs.volumeClaimTemplates.spec.resources.requests.storage` field to the desired size.

```yaml
spec:
  componentSpecs:
    - name: rabbitmq
      componentDef: rabbitmq
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

Restart the specified components in the cluster:

```bash
kubectl apply -f examples/rabbitmq/restart.yaml
```

### [Stop](stop.yaml)

Stop the cluster and release all the pods of the cluster, but the storage will be reserved

```bash
kubectl apply -f examples/rabbitmq/stop.yaml
```

### [Start](start.yaml)

Start the stopped cluster

```bash
kubectl apply -f examples/rabbitmq/start.yaml
```

### Expose

#### [Enable](expose-enable.yaml)

```bash
kubectl apply -f examples/rabbitmq/expose-enable.yaml
```

#### [Disable](expose-disable.yaml)

```bash
kubectl apply -f examples/rabbitmq/expose-disable.yaml
```

#### Expose SVC using Cluster API

Alternatively, you may expose service by updating `spec.services`

```yaml
spec:
  services:
    # add annotation for cloud loadbalancer if
    # services.spec.type is LoadBalancer
    # here we use annotation for alibaba cloud for example
  - annotations:
      # aws annotations
      service.beta.kubernetes.io/aws-load-balancer-type: nlb  # Use Network Load Balancer
      service.beta.kubernetes.io/aws-load-balancer-internal: "true"  # or "false" for internet
    componentSelector: rabbitmq
    name: rabbitmq-vpc
    serviceName: rabbitmq-vpc
    spec:  # defines the behavior of a K8s service.
      ipFamilyPolicy: PreferDualStack
      ports:
      - name: tcp-rabbitmq
        # port to expose
        port: 15672 # port 15672 for rabbitmq management console
        protocol: TCP
        targetPort: management
      type: LoadBalancer
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

Please consult your cloud provider for more accurate and update-to-date information.

### [Reconfigure](reconfigure.yaml)

A database reconfiguration is the process of modifying database parameters, settings, or configurations to improve performance, security, or availability. The reconfiguration can be either:

- Dynamic: Applied without restart
- Static: Requires database restart

Reconfigure parameters with the specified components in the cluster

```bash
kubectl apply -f examples/rabbitmq/reconfigure.yaml
```

This example will change the `channel_max` to `2000`.

> In RabbitMQ, the `channel_max` parameter is used to set the maximum number of channels that a client can open on a single connection. It is a static parameter, so the change will take effect after restarting the database.

To verify the change, you may login to any replica and run the following command:

```bash
rabbitmq-diagnostics environment
```

### Observability

There are various ways to monitor the cluster. Here we use Prometheus and Grafana to demonstrate how to monitor the cluster.

#### Installing the Prometheus Operator

You may skip this step if you have already installed the Prometheus Operator.
Or you can follow the steps in [How to install the Prometheus Operator](../docs/install-prometheus.md) to install the Prometheus Operator.

##### Step 1. Create PodMonitor

Apply the `PodMonitor` file to monitor the cluster:

```bash
kubectl apply -f examples/rabbitmq/pod-monitor.yaml
```

It sets path to `/metrics` and port to `prometheus` (for container port `15692`).

```yaml
    - path: /metrics
      port: prometheus
      scheme: http
```

##### Step 2. Access the Grafana Dashboard

Login to the Grafana dashboard and import the dashboard.
You can import the dashboard from [Grafana RabbitMQ-Overview](https://grafana.com/grafana/dashboards/10991-rabbitmq-overview/).

> [!NOTE]
> Make sure the labels are set correctly in the `PodMonitor` file to match the dashboard.

### Delete

If you want to delete the cluster and all its resource, you can modify the termination policy and then delete the cluster

```bash
kubectl patch cluster -n demo rabbitmq-cluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete -f examples/rabbitmq/cluster.yaml
```

## Appendix

### How to access RabbitMQ Management Console

To access the RabbitMQ Management console (at port `15672`), you can:

- Option 1. Expose the RabbitMQ cluster service:

```bash
kubectl apply -f examples/rabbitmq/expose-enable.yaml
```

- Option 2. Use port-forwarding:

```bash
kubectl port-forward svc/rabbitmq-cluster-rabbitmq 15672:15672
```

Then log in to the RabbitMQ Management console at `http://<localhost>:<port>/` with the user and password.

The user and password can be found in the cluster secrets named after `<clusterName>-<cmpName>-account-<accountName>`. In this case, the secret name is `rabbitmq-cluster-rabbitmq-account-root`.

```bash
# get user name
kubectl get secrets -n demo rabbitmq-cluster-rabbitmq-account-root -o jsonpath='{.data.username}' | base64 -d
# get password
kubectl get secrets -n demo rabbitmq-cluster-rabbitmq-account-root -o jsonpath='{.data.password}' | base64 -d
```
