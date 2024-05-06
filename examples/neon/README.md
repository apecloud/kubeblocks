# Neon

Neon is a serverless open-source alternative to AWS Aurora Postgres. It separates storage and compute and substitutes the PostgreSQL storage layer by redistributing data across a cluster of nodes.

## Prerequisites

Neon Autoscaling requires virtualization technology, requiring the host to have virtualization turned on.

NeonVM and Autoscaling are not expected to work outside Linux x86.

NeonVM deployment relies on cert-manager.

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

Enable neon

```bash
# Add Helm repo 
helm repo add kubeblocks-addons https://apecloud.github.io/helm-charts
# If github is not accessible or very slow for you, please use following repo instead
helm repo add kubeblocks-addons https://jihulab.com/api/v4/projects/150246/packages/helm/stable
# Update helm repo
helm repo update

# Enable neon 
helm upgrade -i kb-addon-neon kubeblocks-addons/neon --version 0.8.0 -n kb-system  

# Install cert-manager
helm upgrade -i cert-manager kubeblocks-addons/cert-manager --version 0.7.5 -n cert-manager
``` 

## Examples

### [Create](cluster.yaml) 
Create a neon cluster with specified cluster definition.
```bash
kubectl apply -f examples/neon/cluster.yaml
```


### Vertical scaling NeonVM
Vertical scaling up or down NeonVM specified cpu or memory.

View NeonVM CPU/MEMORY information.
```bash
kubectl get neonvm -n default
NAME              CPUS   MEMORY   POD                     EXTRAIP   STATUS    AGE
vm-compute-node   1      1Gi      vm-compute-node-g8wsb             Running   5m22s
```

Vertical scaling NeonVM CPU
```bash

kubectl patch neonvm -n default vm-compute-node --type='json' -p='[{"op": "replace", "path": "/spec/guest/cpus/use", "value":2}]'
```
View NeonVM CPU information after Vertical scaling.
```bash
kubectl get neonvm -n default
NAME              CPUS   MEMORY   POD                     EXTRAIP   STATUS    AGE
vm-compute-node   2      1Gi      vm-compute-node-g8wsb             Running   5m45s
```

Vertical scaling NeonVM MEMORY 
```bash
kubectl patch neonvm vm-compute-node --type='json' -p='[{"op": "replace", "path": "/spec/guest/memorySlots/use", "value":4}]'
```

View NeonVM MEMORY information after Vertical scaling.
```bash
kubectl get neonvm -n default
NAME              CPUS   MEMORY   POD                     EXTRAIP   STATUS    AGE
vm-compute-node   2      4Gi      vm-compute-node-g8wsb             Running   10m
```


### Delete
If you want to delete the cluster and all its resource, you can modify the termination policy and then delete the cluster
```bash
kubectl patch cluster neon-cluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete cluster neon-cluster
```
