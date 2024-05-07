# Mysql

MySQL is a widely used, open-source relational database management system (RDBMS)

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

Enable mysql
```bash
# Add Helm repo 
helm repo add kubeblocks-addons https://apecloud.github.io/helm-charts
# If github is not accessible or very slow for you, please use following repo instead
helm repo add kubeblocks-addons https://jihulab.com/api/v4/projects/150246/packages/helm/stable

# Update helm repo
helm repo update

# Enable mysql 
helm upgrade -i kb-addon-mysql kubeblocks-addons/mysql --version 0.9.1 -n kb-system
```

## Examples

### [Create](cluster.yaml) 
Create an mysql cluster with specified cluster definition 
```bash
kubectl apply -f examples/mysql/cluster.yaml
```

### [Horizontal scaling](horizontalscale.yaml)
Horizontal scaling out or in specified components replicas in the cluster
```bash
kubectl apply -f examples/mysql/horizontalscale.yaml
```

### [Vertical scaling](verticalscale.yaml)
Vertical scaling up or down specified components requests and limits cpu or memory resource in the cluster
```bash
kubectl apply -f examples/mysql/verticalscale.yaml
```

### [Expand volume](volumeexpand.yaml)
Increase size of volume storage with the specified components in the cluster
```bash
kubectl apply -f examples/mysql/volumeexpand.yaml
```

### [Restart](restart.yaml)
Restart the specified components in the cluster
```bash
kubectl apply -f examples/mysql/restart.yaml
```

### [Stop](stop.yaml)
Stop the cluster and release all the pods of the cluster, but the storage will be reserved
```bash
kubectl apply -f examples/mysql/stop.yaml
```

### [Start](start.yaml)
Start the stopped cluster
```bash
kubectl apply -f examples/mysql/start.yaml
```

### [Switchover](switchover.yaml)
Switchover a non-primary or non-leader instance as the new primary or leader of the cluster
```bash
kubectl apply -f examples/mysql/switchover.yaml
```

### [Switchover-specified-instance](switchover-specified-instance.yaml)
Switchover a specified instance as the new primary or leader of the cluster
```bash
kubectl apply -f examples/mysql/switchover-specified-instance.yaml
```

### [Configure](configure.yaml)
Configure parameters with the specified components in the cluster
```bash
kubectl apply -f examples/mysql/configure.yaml
```

### [BackupRepo](backuprepo.yaml)
BackupRepo is the storage repository for backup data, using the full backup and restore function of KubeBlocks relies on BackupRepo
```bash
# Create a secret to save the access key
kubectl create secret generic <storage-provider>-credential-for-backuprepo\
  --from-literal=accessKeyId=<ACCESS KEY> \
  --from-literal=secretAccessKey=<SECRET KEY> \
  -n kb-system 
  
kubectl apply -f examples/mysql/backuprepo.yaml
```

### [Backup](backup.yaml)
Create a backup for the cluster
```bash
kubectl apply -f examples/mysql/backup.yaml
```

### [Restore](restore.yaml)
Restore a new cluster from backup
```bash
# Get backup connection password
kubectl get backup mysql-cluster-backup -ojsonpath='{.metadata.annotations.dataprotection\.kubeblocks\.io\/connection-password}' -n default

kubectl apply -f examples/mysql/restore.yaml
```

### Expose
Expose a cluster with a new endpoint
#### [Enable](expose-enable.yaml)
```bash
kubectl apply -f examples/mysql/expose-enable.yaml
```
#### [Disable](expose-disable.yaml)
```bash
kubectl apply -f examples/mysql/expose-disable.yaml
```

### Delete
If you want to delete the cluster and all its resource, you can modify the termination policy and then delete the cluster
```bash
kubectl patch cluster mysql-cluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete cluster mysql-cluster
```
