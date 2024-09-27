# Qdrant

Qdrant is an open source (Apache-2.0 licensed), vector similarity search engine and vector database.

## Prerequisites

This example assumes that you have a Kubernetes cluster installed and running, and that you have installed the kubectl command line tool and helm somewhere in your path. Please see the [getting started](https://kubernetes.io/docs/setup/)  and [Installing Helm](https://helm.sh/docs/intro/install/) for installation instructions for your platform.

Also, this example requires kubeblocks installed and running. Here is the steps to install kubeblocks, please replace "`$kb_version`" with the version you want to use.
```bash
# Add Helm repo 
helm repo add kubeblocks https://apecloud.github.io/helm-charts
# If github is not accessible or very slow for you, please use following repo instead
helm repo add kubeblocks https://jihulab.com/api/v4/projects/85949/packages/helm/stable

# Update helm repo
helm repo update

# Get the versions of KubeBlocks and select the one you want to use
helm search repo kubeblocks/kubeblocks --versions
# If you want to obtain the development versions of KubeBlocks, Please add the '--devel' parameter as the following command
helm search repo kubeblocks/kubeblocks --versions --devel

# Create dependent CRDs
kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/v$kb_version/kubeblocks_crds.yaml
# If github is not accessible or very slow for you, please use following command instead
kubectl create -f https://jihulab.com/api/v4/projects/98723/packages/generic/kubeblocks/v$kb_version/kubeblocks_crds.yaml

# Install KubeBlocks
helm install kubeblocks kubeblocks/kubeblocks --namespace kb-system --create-namespace --version="$kb_version"
```
Enable qdrant
```bash
# Add Helm repo 
helm repo add kubeblocks-addons https://apecloud.github.io/helm-charts
# If github is not accessible or very slow for you, please use following repo instead
helm repo add kubeblocks-addons https://jihulab.com/api/v4/projects/150246/packages/helm/stable
# Update helm repo
helm repo update

# Enable qdrant 
helm upgrade -i kb-addon-qdrant kubeblocks-addons/qdrant -n kb-system  --version $kb_version
``` 

## Examples

### [Create](cluster.yaml) 
Create a qdrant cluster with specified cluster definition 
```bash
kubectl apply -f examples/qdrant/cluster.yaml
```
Starting from kubeblocks 0.9.0, we introduced a more flexible cluster creation method based on components, allowing customization of cluster topology, functionalities and scale according to specific requirements.
```bash
kubectl apply -f examples/qdrant/cluster-cmpd.yaml
```
### [Horizontal scaling](horizontalscale.yaml)
Horizontal scaling out or in specified components replicas in the cluster
```bash
kubectl apply -f examples/qdrant/horizontalscale.yaml
```

### [Vertical scaling](verticalscale.yaml)
Vertical scaling up or down specified components requests and limits cpu or memory resource in the cluster
```bash
kubectl apply -f examples/qdrant/verticalscale.yaml
```

### [Expand volume](volumeexpand.yaml)
Increase size of volume storage with the specified components in the cluster
```bash
kubectl apply -f examples/qdrant/volumeexpand.yaml
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

### [BackupRepo](backuprepo.yaml)
BackupRepo is the storage repository for backup data, using the full backup and restore function of KubeBlocks relies on BackupRepo
```bash
# Create a secret to save the access key
kubectl create secret generic <storage-provider>-credential-for-backuprepo\
  --from-literal=accessKeyId=<ACCESS KEY> \
  --from-literal=secretAccessKey=<SECRET KEY> \
  -n kb-system 
  
kubectl apply -f examples/qdrant/backuprepo.yaml
```

### [Backup](backup.yaml)
Create a backup for the cluster
```bash
kubectl apply -f examples/qdrant/backup.yaml
```

### [Restore](restore.yaml)
Restore a new cluster from backup
```bash
# Get backup connection password
kubectl get backup qdrant-cluster-backup -ojsonpath='{.metadata.annotations.dataprotection\.kubeblocks\.io\/connection-password}' -n default

kubectl apply -f examples/qdrant/restore.yaml
```

### Delete
If you want to delete the cluster and all its resource, you can modify the termination policy and then delete the cluster
```bash
kubectl patch cluster qdrant-cluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete cluster qdrant-cluster
```
