---
title: How to use ApeCloud MySQL Proxy Cluster
description: ApeCloud MySQL Proxy Cluster tutorial
keywords: [apecloud mysql proxy, proxy]
sidebar_position: 2
sidebar_label: ApeCloud MySQL Proxy Cluster
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# ApeCloud MySQL Proxy

## Before you start

1. [Install KubeBlocks](./../../installation/install-kubeblocks.md).
2. Prepare an ApeCloud MySQL RaftGroup named `mycluster` for demonstrating how to enable the proxy function for an existing cluster. Refer to [Create a MySQL cluster](./../cluster-management/create-and-connect-a-mysql-cluster.md) for details.

## Create a Proxy Cluster

1. Add the KubeBlocks repository.

   ```bash
   helm repo add kubeblocks https://apecloud.github.io/helm-charts
   ```

2. View the repository list to verify whether the KubeBlocks repository is added successfully.

   ```bash
   helm repo list
   ```

3. Run the update command to make sure you have added the latest version.

   ```bash
   helm repo update
   ```

4. View all versions of ApeCloud MySQL Proxy.

   ```bash
   helm search repo kubeblocks/apecloud-mysql --devel --versions
   ```

5. (Optional) If you disable the `apecloud-mysql` addon when installing KuebBlocks, run the command below to specify a version and install the cluster definition of ApeCloud MySQL. Skip this step if you install KubeBlocks with the default settings.

   ```bash
   helm install myproxy kubeblocks/apecloud-mysql --version=v0.9.0
   ```

6. Create an ApeCloud MySQL Proxy Cluster.

   ```bash
   helm install myproxy kubeblocks/apecloud-mysql-cluster --version=v0.9.0 --set mode=raftGroup,proxyEnabled=true 
   ```

:::note

If you only have one node for deploying a RaftGroup Cluster, set the `extra.availability-policy` as `none` when creating a RaftGroup Cluster.

```bash
helm install myproxy kubeblocks/apecloud-mysql-cluster --version=v0.9.0 --set mode=raftGroup,proxyEnabled=true --set extra.availabilityPolicy=none
```

:::

## Enable/Disable Proxy dynamically

As its name suggests, ApeCloud MySQL Proxy in nature is a database proxy. An ApeCloud MySQL RaftGroup Cluster that already exists can be switched to an ApeCloud MySQL Proxy Cluster by setting `proxyEnabled=true`.

```bash
helm upgrade mycluster kubeblocks/apecloud-mysql-cluster --set mode=raftGroup,proxyEnabled=true
```

If you want to disable proxy, run the command below.

```bash
helm upgrade mycluster kubeblocks/apecloud-mysql-cluster --set mode=raftGroup
```

## Connect Proxy Cluster

ApeCloud MySQL Proxy is routed through the `vtgate` component, and the way the MySQL Server accesses `vtgate` is similar to the way of accessing `mysqld`. The external SQL access address provided by ApeCloud MySQL Proxy is the `vtgate` address and port. The `vtgate` address created by KubeBlocks by default is `myproxy-cluster-vtgate-headless`, and the port number is `15306`. You can visit ApeCloud MySQL Proxy through the MySQL Server in any pod under the same namespace as ApeCloud MySQL Proxy.

### Connect Proxy Cluster by VTGate

1. Expose the port of VTGate to the localhost so that the localhost can access the Proxy.

   ```bash
   kubectl port-forward svc/vt-vtgate-headless 15306:15306
   ```

2. Connect to the cluster.

   ```bash
   mysql -h 127.0.0.1 -P 15306
   ```

### Connect Proxy Cluster by MySQL Server

1. Expose the port of the MySQL Server to the localhost so that the localhost can access the MySQL Server.

   ```bash
   kubectl port-forward svc/vt-mysql 3306:3306
   ```

2. Connect to the cluster.

   ```bash
   mysql -h 127.0.0.1 -P 3306
   ```

:::note

If you need to test the failover of MySQL, you need to delete the Pod first and continue to port-forward the port, and you can also write a shell script. Here are examples.

For VTGate,

```bash
while true; do date; kubectl port-forward svc/vt-vtgate-headless 15306:15306; sleep 0.5; done
```

For the MySQL Server,

```bash
while true; do date; kubectl port-forward svc/vt-mysql 3306:3306; sleep 0.5; done
```

:::

## Configure Proxy Cluster parameters

VTGate, VTConsensus, and VTTablet support parameter configuration. You can configure the proxy cluster by editing the configuration file or by performing an OpsRequest.

<Tabs>

<TabItem value="Edit config file" label="Edit config file" default>

1. Get the configuration file of this cluster.

   ```bash
   kubectl edit configurations.apps.kubeblocks.io myproxy-vtgate
   ```

2. Configure parameters according to your needs. The example below adds the `spec.configFileParams` part to configure `max_connections`.

   ```yaml
   spec:
     clusterRef: myproxy
     componentName: vtgate
     configItemDetails:
     - configFileParams:
         vtgate.cnf:
           parameters:
             healthcheck_timeout: "5s"
       configSpec:
         constraintRef: mysql-scale-vtgate-config-constraints
         name: vtgate-config
         namespace: kb-system
         templateRef: vtgate-config-template
         volumeName: mysql-scale-config
       name: vtgate-config
       payload: {}
   ```

3. Connect to this cluster to verify whether the configuration takes effect.

   1. Expose the port of the MySQL Server to the localhost so that the localhost can access the MySQL Server.

      ```bash
      kubectl port-forward svc/vt-vtgate-headless 15306:15306
      ```

   2. Connect to this cluster and verify whether the parameters are configured as expected.

      ```bash
      mysql -h 127.0.0.1 -P 3306

      >
      mysql> show variables like 'healthcheck_timeout';
      +---------------------+-------+
      | Variable_name       | Value |
      +---------------------+-------+
      | healthcheck_timeout |  5s   |
      +---------------------+-------+
      1 row in set (0.00 sec)
      ```

</TabItem>

<TabItem value="OpsRequest" label="OpsRequest">

Apply an OpsRequest to the specified cluster. Configure the parameters according to your needs.

* An example for configuring VTTablet

    ```yaml
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: OpsRequest
    metadata:
      name: acmysql-vttablet-reconfiguring
      namespace: default
    spec:
      clusterName: acmysql-cluster
      force: false
      reconfigure:
        componentName: mysql
        configurations:
          - keys:
              - key: vttablet.cnf
                parameters:
                  - key: health_check_interval
                    value: 4s
            name: vttablet-config
      preConditionDeadlineSeconds: 0
      type: Reconfiguring
    ```

* An example for configuring VTGate

    ```yaml
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: OpsRequest
    metadata:
      name: acmysql-vtgate-reconfiguring
      namespace: default
    spec:
      clusterName: acmysql-cluster
      force: false
      reconfigure:
        componentName: vtgate
        configurations:
        - keys:
          - key: vtgate.cnf
            parameters:
            - key: healthcheck_timeout
              value: 2s
          name: vtgate-config
      preConditionDeadlineSeconds: 0
      type: Reconfiguring
    ```

</TabItem>

</Tabs>

## Log

You can view the log files of components, Pods, and containers.

View the log of VTGate.

```bash
kubectl logs myproxy-cluster-vtgate-8659d5db95-4dzt5
```

View the log of VTTablet and `-c` is required.

```bash
kubectl logs myproxy-cluster-mysql-0 -c vttablet
```

Enter the container and view more logs of VTGate.

```bash
kubectl exec -it myproxy-cluster-vtgate-8659d5db95-4dzt5 -- bash
ls /vtdataroot
```

Enter the container and view more logs of VTTable.

```bash
kubectl exec -it myproxy-cluster-mysql-0  -c vttablet -- bash
ls /vtdataroot
```

## Monitoring

You can monitor the performance of the proxy cluster.

1. Enable the monitoring addons.

   For the testing/demo environment, run the commands below to enable the monitoring addons provided by KubeBlocks.

   ```bash
   helm install prometheus kubeblocks/prometheus --namespace kb-system --create-namespace
   helm install prometheus kubeblocks/prometheus --namespace kb-system --create-namespace
   helm install prometheus kubeblocks/prometheus --namespace kb-system --create-namespace
   ```

   For the production environment, you can integrate the monitoring components. For details, you can refer to the relevant docs provided by the monitoring tools.

2. Check whether the monitoring function of this proxy cluster is enabled.

   ```bash
   kubectl get cluster myproxy -o yaml
   ```

   If the output YAML file shows `disableExporter: false`, the monitoring function of this proxy cluster is enabled.

   If the monitoring function is not enabled, run the command below to enable it first.

   ```bash
   kubectl patch cluster mycluster -n demo --type "json" -p '[{"op":"add","path":"/spec/componentSpecs/0/disableExporter","value":false}]'
   ```

3. View the dashboard.

   For the testing/demo environment, run the commands below to view the Grafana dashboard.

   ```bash
   # 1. Get the username and password 
   kubectl get secret grafana -n kb-system -o jsonpath='{.data.admin-user}' |base64 -d

   kubectl get secret grafana -n kb-system -o jsonpath='{.data.admin-password}' |base64 -d

   # 2. Connect to the Grafana dashboard
   kubectl port-forward svc/grafana -n kb-system 3000:8

   # 3. Open the web browser and enter the address 127.0.0.1:3000 to visit the dashboard.

   # 4. Enter the username and password obtained from step 1.
   ```

   For the production environment, you can view the dashboard of the corresponding cluster via Grafana Web Console. For more detailed information, see [the Grafana dashboard documentation](https://grafana.com/docs/grafana/latest/dashboards/).

:::note

1. If there is no data in the dashboard, you can check whether the job is `kubeblocks-service`. Enter `kubeblocks-service` in the job field and press the enter button.

   ![Monitoring dashboard](./../../../img/api-monitoring.png)

2. For more details on the monitoring function, you can refer to [Monitoring](./../../observability/monitor-database.md).

:::

## Read-write splitting

You can enable the read-write splitting function.

1. Get the configuration file of this cluster.

   ```bash
   kubectl edit configurations.apps.kubeblocks.io myproxy-vtgate
   ```

2. Configure `read_write_splitting_policy` as `random`. 

   ```yaml
   spec:
     clusterRef: myproxy
     componentName: vtgate
     configItemDetails:
     - configFileParams:
         vtgate.cnf:
           parameters:
             read_write_splitting_policy: "random"
       configSpec:
         constraintRef: mysql-scale-vtgate-config-constraints
         name: vtgate-config
         namespace: kb-system
         templateRef: vtgate-config-template
         volumeName: mysql-scale-config
       name: vtgate-config
       payload: {}
   ```

You can also set the ratio for read-write splitting and here is an example of directing 70% flow to the read-only node.

```yaml
spec:
   clusterRef: myproxy
   componentName: vtgate
   configItemDetails:
   - configFileParams:
      vtgate.cnf:
         parameters:
            read_write_splitting_rati: "70"
      configSpec:
      constraintRef: mysql-scale-vtgate-config-constraints
      name: vtgate-config
      namespace: kb-system
      templateRef: vtgate-config-template
      volumeName: mysql-scale-config
      name: vtgate-config
      payload: {}
```

## Transparent failover

Run the command below to perform transparent failover.

1. Get the configuration file of this cluster.

   ```bash
   kubectl edit configurations.apps.kubeblocks.io myproxy-vtgate
   ```

2. Configure `enable_buffer` as `true`.

   ```yaml
   spec:
     clusterRef: myproxy
     componentName: vtgate
     configItemDetails:
     - configFileParams:
         vtgate.cnf:
           parameters:
             enable_buffer: "true"
       configSpec:
         constraintRef: mysql-scale-vtgate-config-constraints
         name: vtgate-config
         namespace: kb-system
         templateRef: vtgate-config-template
         volumeName: mysql-scale-config
       name: vtgate-config
       payload: {}
   ```
