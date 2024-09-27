# Postgresql

PostgreSQL (Postgres) is an open source object-relational database known for reliability and data integrity. ACID-compliant, it supports foreign keys, joins, views, triggers and stored procedures.

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
 

## Examples

### [Create](cluster.yaml) 
Create a postgresql cluster with specified cluster definition 
```bash
kubectl apply -f examples/postgresql/cluster.yaml
```
Starting from kubeblocks 0.9.0, we introduced a more flexible cluster creation method based on components, allowing customization of cluster topology, functionalities and scale according to specific requirements.
```bash
kubectl apply -f examples/postgresql/cluster-cmpd.yaml
```
### [Horizontal scaling](horizontalscale.yaml)
Horizontal scaling out or in specified components replicas in the cluster
```bash
kubectl apply -f examples/postgresql/horizontalscale.yaml
```

### [Vertical scaling](verticalscale.yaml)
Vertical scaling up or down specified components requests and limits cpu or memory resource in the cluster
```bash
kubectl apply -f examples/postgresql/verticalscale.yaml
```

### [Expand volume](volumeexpand.yaml)
Increase size of volume storage with the specified components in the cluster
```bash
kubectl apply -f examples/postgresql/volumeexpand.yaml
```

### [Restart](restart.yaml)
Restart the specified components in the cluster
```bash
kubectl apply -f examples/postgresql/restart.yaml
```

### [Stop](stop.yaml)
Stop the cluster and release all the pods of the cluster, but the storage will be reserved
```bash
kubectl apply -f examples/postgresql/stop.yaml
```

### [Start](start.yaml)
Start the stopped cluster
```bash
kubectl apply -f examples/postgresql/start.yaml
```

### [Switchover](switchover.yaml)
Switchover a non-primary or non-leader instance as the new primary or leader of the cluster
```bash
kubectl apply -f examples/postgresql/switchover.yaml
```

### [Switchover-specified-instance](switchover-specified-instance.yaml)
Switchover a specified instance as the new primary or leader of the cluster
```bash
kubectl apply -f examples/postgresql/switchover-specified-instance.yaml
```

### [Configure](configure.yaml)
Configure parameters with the specified components in the cluster
```bash
kubectl apply -f examples/postgresql/configure.yaml
```

### [BackupRepo](backuprepo.yaml)
BackupRepo is the storage repository for backup data, using the full backup and restore function of KubeBlocks relies on BackupRepo
```bash
# Create a secret to save the access key
kubectl create secret generic <storage-provider>-credential-for-backuprepo\
  --from-literal=accessKeyId=<ACCESS KEY> \
  --from-literal=secretAccessKey=<SECRET KEY> \
  -n kb-system 
  
kubectl apply -f examples/postgresql/backuprepo.yaml
```

### [Backup](backup.yaml)
Create pg-basebackup or volume-snapshot backup for the cluster
```bash
kubectl apply -f examples/postgresql/backup.yaml
```
Create wal-g backup for the cluster
```bash
# Step 1: you cannot do wal-g backup for a brand-new cluster, you need to insert some data before backup
# step 2: config-wal-g backup to put the wal-g binary to postgresql pods and configure the archive_command
# Note: if there is horizontal scaling out new pods after step 2, you need to do config-wal-g again
kubectl apply -f examples/postgresql/config-wal-g.yaml
# Step 3: do wal-g backup
kubectl apply -f examples/postgresql/backup-wal-g.yaml
# Step 4:log in to the cluster, and manually upload wal with following sql statement
select pg_switch_wal();
```

### [Restore](restore.yaml)
Restore a new cluster from backup
```bash
# Get backup connection password
kubectl get backup pg-cluster-backup -ojsonpath='{.metadata.annotations.dataprotection\.kubeblocks\.io\/connection-password}' -n default

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

### [Upgrade](upgrade.yaml)
Upgrade pg cluster to a newer version
```bash
kubectl apply -f examples/postgresql/upgrade.yaml
```

### Delete
If you want to delete the cluster and all its resource, you can modify the termination policy and then delete the cluster
```bash
kubectl patch cluster pg-cluster -p '{"spec":{"terminationPolicy":"WipeOut"}}' --type="merge"

kubectl delete cluster pg-cluster
```
