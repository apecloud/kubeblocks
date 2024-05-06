# Oceanbase

OceanBase Database is an enterprise-level native distributed database independently developed by Ant Group.

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
Enable oceanbase
```bash
# Add Helm repo 
helm repo add kubeblocks-addons https://apecloud.github.io/helm-charts
# If github is not accessible or very slow for you, please use following repo instead
helm repo add kubeblocks-addons https://jihulab.com/api/v4/projects/150246/packages/helm/stable
# Update helm repo
helm repo update

# Enable oceanbase 
helm upgrade -i oceanbase-ce kubeblocks-addons/oceanbase-ce --version 0.9.0 -n kb-system  
``` 

## Examples

### [Create](cluster.yaml) 
Create a distributed oceanbase cluster
```bash
kubectl apply -f examples/oceanbase/cluster.yaml
```
Create a primary and standby oceanbase cluster 
```bash
kubectl apply -f examples/oceanbase/cluster-repl.yaml
```

Please note that not all operations are currently supported for all topologies. We plan to extend this capability soon.

|                                   | cmpd | Horizontal<br/>scaling | Vertical <br/>scaling | Expand<br/>volume | Restart | Stop/Start | Configure | Expose | Switchover | 
|-----------------------------------|------|------------------------|-----------------------|--------------|---------|----------|---------|--------|----------|
| Distributed<br/>container network |  ob-ce    | Supported  | Not support           | Not support  | Not support |Not support |Supported |Not support| N/A      |
| Distributed<br/>host network       |  ob-ce-hostnetwork    | Supported | Supported             | Not support  | Supported | Supported |Supported|Not support | N/A      |
| Primary/Standby<br/>container network |  ob-ce-repl    | Not support | Not support           | Not support  |Not support |Not support |Supported|Supported | Supported |
| Primary/Standby<br/>host network   |   ob-ce-repl-host   | Not support   | Supported          | Not support  |Supported | Supported |Supported|Supported | Supported |


### [Horizontal scaling](horizontalscale.yaml)
Horizontal scaling out or in specified components replicas in the cluster
```bash
kubectl apply -f examples/oceanbase/horizontalscale.yaml
```

### [Restart](restart.yaml)
Restart the specified components in the cluster
```bash
kubectl apply -f examples/oceanbase/restart.yaml
```

### [Stop](stop.yaml)
Stop the cluster and release all the pods of the cluster, but the storage will be reserved
```bash
kubectl apply -f examples/oceanbase/stop.yaml
```

### [Start](start.yaml)
Start the stopped cluster
```bash
kubectl apply -f examples/oceanbase/start.yaml
```

### [Switchover](switchover.yaml)
Switchover a non-primary or non-leader instance as the new primary or leader of the primary and standby cluster
```bash
kubectl apply -f examples/oceanbase/switchover.yaml
```

### [Configure](configure.yaml)
Configure parameters with the specified components in the cluster
```bash
kubectl apply -f examples/oceanbase/configure.yaml
```

### [BackupRepo](backuprepo.yaml)
BackupRepo is the storage repository for backup data, using the full backup and restore function of KubeBlocks relies on BackupRepo
```bash
# Create a secret to save the access key
kubectl create secret generic <storage-provider>-credential-for-backuprepo\
  --from-literal=accessKeyId=<ACCESS KEY> \
  --from-literal=secretAccessKey=<SECRET KEY> \
  -n kb-system 
  
kubectl apply -f examples/oceanbase/backuprepo.yaml
```

### [Backup](backup.yaml)
Create a backup for the cluster
```bash
kubectl apply -f examples/oceanbase/backup.yaml
```

### [Restore](restore.yaml)
Restore a new cluster from backup
```bash
# Get backup connection password
kubectl get backup acmysql-cluster-backup -ojsonpath='{.metadata.annotations.dataprotection\.kubeblocks\.io\/connection-password}' -n default

kubectl apply -f examples/oceanbase/restore.yaml
```

### Expose
Expose a cluster with a new endpoint
#### [Enable](expose-enable.yaml)
```bash
kubectl apply -f examples/oceanbase/expose-enable.yaml
```
#### [Disable](expose-disable.yaml)
```bash
kubectl apply -f examples/oceanbase/expose-disable.yaml
```

### Delete
If you want to delete the cluster and all its resource, you can modify the termination policy and then delete the cluster
```bash
kubectl patch cluster oceanbase-cluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete cluster oceanbase-cluster
```
