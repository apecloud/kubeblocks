# Redis

Redis is an open source (BSD licensed), in-memory data structure store, used as a database, cache and message broker. This example shows how it can be managed in Kubernetes with KubeBlocks.

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
Create a redis cluster with specified cluster definition 
```bash
kubectl apply -f examples/redis/cluster.yaml
```
Starting from kubeblocks 0.9.0, we introduced a more flexible cluster creation method based on components, allowing customization of cluster topology, functionalities and scale according to specific requirements.

Create a redis cluster with ComponentSpecs 
```bash
kubectl apply -f examples/redis/cluster-cmpd.yaml
```
Create a redis cluster with ShardingSpecs
```bash
kubectl apply -f examples/redis/cluster-shard.yaml
```

### [Horizontal scaling](horizontalscale.yaml)
Horizontal scaling out or in specified components replicas in the cluster
```bash
kubectl apply -f examples/redis/horizontalscale.yaml
```

### [Vertical scaling](verticalscale.yaml)
Vertical scaling up or down specified components requests and limits cpu or memory resource in the cluster
```bash
kubectl apply -f examples/redis/verticalscale.yaml
```

### [Expand volume](volumeexpand.yaml)
Increase size of volume storage with the specified components in the cluster
```bash
kubectl apply -f examples/redis/volumeexpand.yaml
```

### [Restart](restart.yaml)
Restart the specified components in the cluster
```bash
kubectl apply -f examples/redis/restart.yaml
```

### [Stop](stop.yaml)
Stop the cluster and release all the pods of the cluster, but the storage will be reserved
```bash
kubectl apply -f examples/redis/stop.yaml
```

### [Start](start.yaml)
Start the stopped cluster
```bash
kubectl apply -f examples/redis/start.yaml
```

### [Configure](configure.yaml)
Configure parameters with the specified components in the cluster
```bash
kubectl apply -f examples/redis/configure.yaml
```

### [BackupRepo](backuprepo.yaml)
BackupRepo is the storage repository for backup data, using the full backup and restore function of KubeBlocks relies on BackupRepo
```bash
# Create a secret to save the access key
kubectl create secret generic <storage-provider>-credential-for-backuprepo\
  --from-literal=accessKeyId=<ACCESS KEY> \
  --from-literal=secretAccessKey=<SECRET KEY> \
  -n kb-system 
  
kubectl apply -f examples/redis/backuprepo.yaml
```

### [Backup](backup.yaml)
Create a backup for the cluster
```bash
kubectl apply -f examples/redis/backup.yaml
```

### [Restore](restore.yaml)
Restore a new cluster from backup
```bash
# Get backup connection password
kubectl get backup redis-cluster-backup -ojsonpath='{.metadata.annotations.dataprotection\.kubeblocks\.io\/connection-password}' -n default

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

### Delete
If you want to delete the cluster and all its resource, you can modify the termination policy and then delete the cluster
```bash
kubectl patch cluster redis-cluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete cluster redis-cluster
```
