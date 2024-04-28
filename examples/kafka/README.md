# Kafka

Apache Kafka is a distributed streaming platform designed to build real-time pipelines and can be used as a message broker or as a replacement for a log aggregation solution for big data applications.

## Prerequisites

This example assumes that you have a Kubernetes cluster installed and running, and that you have installed the kubectl command line tool and helm somewhere in your path. Please see the [getting started](https://kubernetes.io/docs/setup/)  and [Installing Helm](https://helm.sh/docs/intro/install/) for installation instructions for your platform.

Also, this example requires kubeblocks installed and running. Here is the steps to install kubeblocks, please replace "0.9.0" with the version you want to use.
```bash
# Create dependent CRDs
kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/v0.9.0/kubeblocks_crds.yaml
# If github is not accessible or very slow for you, please use following command instead
kubectl create -f https://jihulab.com/api/v4/projects/98723/packages/generic/kubeblocks/v0.9.0/kubeblocks_crds.yaml

# Add Helm repo 
helm repo add kubeblocks https://apecloud.github.io/helm-charts
# If github is not accessible or very slow for you, please use following repo instead
helm repo add kubeblocks https://jihulab.com/api/v4/projects/85949/packages/helm/stable

# Update helm repo
helm repo update

# Install KubeBlocks
helm install kubeblocks kubeblocks/kubeblocks --namespace kb-system --create-namespace --version="0.9.0"
```
 

## Examples

### [Create](cluster.yaml) 
Create a kafka cluster with combined controller and broker components 
```bash
kubectl apply -f examples/kafka/cluster.yaml
```
Create a kafka cluster with separated controller and broker components
```bash
kubectl apply -f examples/kafka/cluster-separated.yaml
```

### [Horizontal scaling](horizontalscale.yaml)
Horizontal scaling out or in specified components replicas in the cluster
```bash
kubectl apply -f examples/kafka/horizontalscale.yaml
```

### [Vertical scaling](verticalscale.yaml)
Vertical scaling up or down specified components requests and limits cpu or memory resource in the cluster
```bash
kubectl apply -f examples/kafka/verticalscale.yaml
```

### [Expand volume](volumeexpand.yaml)
Increase size of volume storage with the specified components in the cluster
```bash
kubectl apply -f examples/kafka/volumeexpand.yaml
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

### [Start](start.yaml)
Start the stopped cluster
```bash
kubectl apply -f examples/kafka/start.yaml
```

### [Configure](configure.yaml)
Configure parameters with the specified components in the cluster
```bash
kubectl apply -f examples/kafka/configure.yaml
```

### Delete
If you want to delete the cluster and all its resource, you can modify the termination policy and then delete the cluster
```bash
kubectl patch cluster kafka-cluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete cluster kafka-cluster
```
