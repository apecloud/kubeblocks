# Solr

Solr is the popular, blazing-fast, open source enterprise search platform built on Apache Lucene.

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
Enable solr
```bash
# Add Helm repo 
helm repo add kubeblocks-addons https://apecloud.github.io/helm-charts
# If github is not accessible or very slow for you, please use following repo instead
helm repo add kubeblocks-addons https://jihulab.com/api/v4/projects/150246/packages/helm/stable
# Update helm repo
helm repo update

# Enable solr 
helm upgrade -i solr kubeblocks-addons/solr --version 0.8.0 -n kb-system  
# Enable zookeeper
helm upgrade -i kb-addon-zookeeper kubeblocks-addons/zookeeper -n kb-system --version 0.9.0 
``` 

## Examples

### [Create](cluster.yaml)
Create a solr cluster with specified cluster definition 
```bash
kubectl apply -f examples/solr/cluster.yaml
```

### [Horizontal scaling](horizontalscale.yaml)
Horizontal scaling out or in specified components replicas in the cluster
```bash
kubectl apply -f examples/solr/horizontalscale.yaml
```

### [Vertical scaling](verticalscale.yaml)
Vertical scaling up or down specified components requests and limits cpu or memory resource in the cluster
```bash
kubectl apply -f examples/solr/verticalscale.yaml
```

### [Restart](restart.yaml)
Restart the specified components in the cluster
```bash
kubectl apply -f examples/solr/restart.yaml
```

### [Stop](stop.yaml)
Stop the cluster and release all the pods of the cluster, but the storage will be reserved
```bash
kubectl apply -f examples/solr/stop.yaml
```

### [Start](start.yaml)
Start the stopped cluster
```bash
kubectl apply -f examples/solr/start.yaml
```

### Delete
If you want to delete the cluster and all its resource, you can modify the termination policy and then delete the cluster
```bash
kubectl patch cluster solr-cluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete cluster solr-cluster

kubectl delete cluster zookeepers-cluster
```