# Create an ApeCloud MySQL cluster on AWS

This guide introduces how to use KubeBlocks to create an ApeCloud MySQL cluster based on the Paxos consensus protocol within 5 minutes under the EKS environment.

## Before you start

1. EKS environment is required and it includes at least three replicas.
2. Install MySQL client on your local host because KubeBlocks communicates with the deployed MySQL clusters by calling MySQL client. Refer to [Installing MySQL Shell on macOS](https://dev.mysql.com/doc/mysql-shell/8.0/en/mysql-shell-install-macos-quick.html) for details.
3. `kubectl` is required and can connect to the EKS cluster. For installing `kubectl`, refer to [Install and Set Up kubectl on macOS](https://kubernetes.io/docs/tasks/tools/install-kubectl-macos/) for details.

### Step 1. Install `kbcli`

1. Run the command below to install `kbcli`. `kbcli` can run on macOS, Linux, and Windows.
   ```
   curl -fsSL http://161.189.136.182:8000/apecloud/kubeblocks/install_cli.sh |bash
   ```
   > Note
   > Please try again if a time-out exception occurs during installation. It may relate to your network condition.
2. Run the command below to check the version and verify whether `kbcli` is installed successfully.
   ```
   kbcli version
   ```
3. Run the command below to uninstall kbcli  if you want to delete kbcli after your trial.
   ```
   sudo rm /usr/local/bin/kbcli
   ```

### Step 2. Install KubeBlocks

1. Run the command below to install KubeBlock.
   ```
   kbcli kubeblocks install --set loadbalancer.enabled=true
   ```

   If you want the node outside the Kubernetes cluster (the node should be within the same VPC) to visit the database cluster created by KubeBlocks, use `--set loadbalancer.enabled=true` option as the above command does. For more details on installation, refer to [Install/Uninstall kbcli and KubeBlocks](../install_kbcli_kubeblocks/install_and_unistall_kbcli_and_kubeblocks.md).

   ***Result***
   This command installs the latest version in your Kubernetes environment since your kubectl can connect to your Kubernetes clusters.

2. Run the command below to verify whether KubeBlocks is installed successfully.
   ```
   kubectl get pod
   ```

   ***Result***
   Four pods starting with `kubeblocks` are displayed. For example,
   ```
   NAME                                                  READY   STATUS    RESTARTS   AGE
   kubeblocks-7d4c6fd684-9hjh7                           1/1     Running   0          3m33s
   kubeblocks-grafana-b765d544f-wj6c6                    3/3     Running   0          3m33s
   kubeblocks-prometheus-alertmanager-7c558865f5-hsfn5   2/2     Running   0          3m33s
   kubeblocks-prometheus-server-5c89c8bc89-mwrx7         2/2     Running   0          3m33s

3. Run the command below to uninstall KubeBlocks if you want to delete KubeBlocks after your trial.
   ```
   kbcli kubeblocks uninstall
   ```

### Step 3. Create a YAML file for ApeCloud MySQL Paxos group

Create a YAML file named `mysql-component.yaml` and copy the configurations below to this YAML file.
```
- name: replicasets
  type: replicasets
  replicas: 3
  monitor: false
  resources:
    limits:
      memory: "1Gi"
      cpu: 2
  volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 10Gi
```

> Caution
> * Configure the parameter of `resources.limit` based on the node storage and CPU kernel amount of your EKS cluster. If there are not three repicas that exceed the values in `resources.limits`, creating a MySQL cluster may fail.
> * KubeBlocks applies for a new EBS volume according to the value of `volumeClaimTemplates.spec.requests.storage`, which causes extra fees. Delete this EBS volume after your trial.

### Step 4. Create an ApeCloud MySQL cluster

1. Run the command below to create an ApeCloud MySQL cluster. The cluster name can be customized and `mysql-example` below is an example.
   ```
   kbcli cluster create mysql-example --cluster-version=ac-mysql-8.0.30 --set=mysql-component.yaml --cluster-definition=apecloud-mysql --termination-policy=WipeOut
   ```

   * `--cluster-version` specifies the ApeCloud MySQL version. The latest version is installed by default when you install KubeBlocks. You can run the command below to view the current MySQL version and use `NAME` as the value of `--cluster-version`.
     ```
     kubectl get clusterversion

     NAME              STATUS      AGE
     ac-mysql-8.0.30   Available   28h
     ```
   * `--set` point to the YAML file created in [Step 3](#step-3-create-a-yaml-file-for-apecloud-mysql-paxos-group).
   * `--termination-policy` specifies the policy of how a cluster is deleted. The available policies are as follows:
     | **Termination Policy**  | **Result**   |
     | :--                     | :--          |
     | DoNotTerminate          | DoNotTerminate blocks delete operation. |
     | Halt                    | Halt deletes workload resources such as statefulset, deployment workloads but keeps PVCs. |
     | Delete                  | Delete is based on Halt and deletes PVCs. |
     | WipeOut                 | WipeOut is based on Delete and wipes out all volume snapshots and snapshot data from backup storage location. |
   * Other parameters in the command can adopt the default value.

2. Run the command below to view the created cluster.
   ```
   kbcli cluster list
   ```

### Step 5. Connect to the ApeCloud MySQL cluster

1. It takes several minutes to create a cluster. Run `kbcli cluster list` to check the cluster status and when the cluster status is `Running`, the cluster has been created. 
2. Run the command below to connect to the leader pod of ApeCloud MySQL. (The leader pod is called leaseholder in other databases.)
   ```
   kbcli cluster connect mysql-example
   ```

After connecting to the cluster, you can operate the created MySQL cluster as you do in the MySQL client.

If you want to connect to the MySQL cluster using MySQL client or your stress test tool, 
1. Run the command below to get the IP and port of this cluster first. 
   ```
   kbcli cluster describe NAME
   ```
2. Find the Endpoints information in the result.
   ```
   Endpoints:
   COMPONENT          MODE             INTERNAL                  EXTERNAL        
   replicasets        ReadWrite        10.100.197.10:3306        <none>
   ```

The ApeCloud MySQL cluster provides high availability to ensure RPO=0. When a failure occurs to the MySQL leader pod, other MySQL pods can be elected as the succeeding leader based on the Paxos protocol. The connection address does not change even though the leader pod changes.

### Step 6. Delete the ApeCloud MySQL cluster
Run the command below to delete the ApeCloud MySQL cluster.
```
kbcli cluster delete mysql-example
```