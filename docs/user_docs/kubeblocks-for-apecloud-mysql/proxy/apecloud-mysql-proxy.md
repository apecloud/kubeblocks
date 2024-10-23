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

1. [Install kbcli](./../../installation/install-with-kbcli/install-kbcli.md).
2. [Install Helm](https://helm.sh/docs/intro/install/).
3. Install KubeBlocks.

   You can run `kbcli playground init` to install a k3d cluster and KubeBlocks. For details, refer to [Try KubeBlocks on your laptop](./../../try-out-on-playground/try-kubeblocks-on-your-laptop.md) or [Try KubeBlocks on cloud](./../../try-out-on-playground/try-kubeblocks-on-cloud.md).

   ```bash
   kbcli playground init

   # Use --version to specify a version
   kbcli playground init --version='x.x.x'
   ```

   Or if you already have a Kubernetes cluster, you can install KubeBlocks [by kbcli](./../../installation/install-with-kbcli/install-kubeblocks-with-kbcli.md) or [by Helm](./../../installation/install-with-helm/install-kubeblocks.md) directly.
4. Prepare an ApeCloud MySQL RaftGroup named `mycluster` for demonstrating how to enable the proxy function for an existing cluster. Refer to [Create a MySQL cluster](./../cluster-management/create-and-connect-an-apecloud-mysql-cluster.md) for details.

## Create a Proxy Cluster

It is recommended to use kbcli to create an ApeCloud MySQL Proxy Cluster.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

1. Enable the etcd Addon and create an etcd cluster.

   1. Install and enable the etcd Addon. You need to install the etcd Addon first since the etcd Addon is not installed by default. Refer to [Addons installation tutorial](./../../installation/install-with-kbcli/install-addons.md) for details.

       ```bash
       # 1. Check whether the KubeBlocks Addon index is added
       kbcli addon index list

       # If the list is empty, add the index
       kbcli addon index add kubeblocks https://github.com/apecloud/block-index.git

       # 2. Search the etcd Addon
       kbcli addon search etcd

       # 3. Install the etcd Addon
       kbcli addon install etcd --index kubeblocks --version 0.9.0

       # 4. Enable the etcd Addon
       kbcli addon enable etcd

       # 5. Check whether the etcd Addon is enabled.
       kbcli addon list
       ```

   2. Create an etcd cluster.

       ```bash
       kbcli cluster create myetcd --cluster-definition etcd
       ```

   3. Check the status of the etcd service

       ```bash
       kbcli cluster list myetcd
       ```

2. View the etcd service address or the etcd pod address.

   ```bash
   kubectl get service
   >
   NAME                             TYPE        CLUSTER-IP       EXTERNAL-IP   PORT(S)                                                  AGE
   kubernetes                       ClusterIP   10.96.0.1        <none>        443/TCP                                                  85d
   myetcd-etcd                      ClusterIP   10.101.227.143   <none>        2379/TCP                                                 111s
   myetcd-etcd-headless             ClusterIP   None             <none>        2379/TCP,2380/TCP,3501/TCP,50001/TCP                     111s
   ```

3. Create an ApeCloud MySQL Proxy cluster.

    ```bash
    helm repo add kubeblocks https://apecloud.github.io/helm-charts

    helm install myproxy kubeblocks/apecloud-mysql-cluster --set mode=raftGroup,proxyEnabled=true,etcd.serviceReference.endpoint="etcd-cluster-etcd.default.svc.cluster.local:2379"
    ```

4. Check the status of the clusters.

   ```bash
   kbcli get cluster

   kbcli get pods
   ```

   You can also enter the etcd container or wesql-scale container to view the configuration of wesql-scale or to check the availability of the etcd service.

   ```bash
   etcdctl --endpoints=http://etcd-cluster-etcd.default.svc.cluster.local:2379 get /vitess --prefix --keys-only
   ```

</TabItem>

<TabItem value="kubectl" label="kubectl">

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

4. Install etcd to create the external service reference.

   1. View all versions of etcd.

       ```bash
       helm search repo kubeblocks/etcd --devel --versions
       ```

   2. Install the etcd Addon.

       ```bash
       helm install etcd kubeblocks/etcd --version=v0.6.5
       ```

   3. Install the etcd cluster.

       ```bash
       helm install etcd-cluster kubeblocks/etcd-cluster 
       ```

   4. view the status of the etcd cluster and make sure it is running.

       ```bash
       kubectl get cluster
       >
       NAME           CLUSTER-DEFINITION   VERSION       TERMINATION-POLICY   STATUS      AGE
       etcd-cluster   etcd                 etcd-v3.5.6   Halt                 Running     10s
       ```

   5. View the service address of this etcd clsuter.

       ```bash
       kubectl get service
       >
       NAME                             TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)                                                  AGE
       etcd-cluster-etcd                ClusterIP   10.110.23.89   <none>        2379/TCP                                                 55s
       etcd-cluster-etcd-headless       ClusterIP   None           <none>        2379/TCP,2380/TCP,3501/TCP,50001/TCP                     55s
       kubernetes                       ClusterIP   10.96.0.1      <none>        443/TCP                                                  13m
       ```

       You can combine the service address to get the endpoint or you can use the IP of the service address as the access address. 

       Here is an example of combining the service address.

       ```bash
       etcd-cluster-etcd.default.svc.cluster.local:2379
       ```

5. Create an ApeCloud MySQL Proxy Cluster.
   1. View all versions of ApeCloud MySQL Proxy.

       ```bash
       helm search repo kubeblocks/apecloud-mysql --devel --versions
       ```

   2. (Optional) If you disable the `apecloud-mysql` Addon when installing KuebBlocks, run the command below to specify a version and install the cluster definition of ApeCloud MySQL. Skip this step if you install KubeBlocks with the default settings.

       ```bash
       helm install myproxy kubeblocks/apecloud-mysql --version=v0.9.0
       ```

   3. Create an ApeCloud MySQL Proxy Cluster.

       ```bash
       helm install myproxy kubeblocks/apecloud-mysql-cluster --version=v0.9.0 --set mode=raftGroup,proxyEnabled=true,etcd.serviceReference.endpoint="etcd-cluster-etcd.default.svc.cluster.local:2379"
       ```

:::note

If you only have one node for deploying a RaftGroup Cluster, set the `extra.availability-policy` as `none` when creating a RaftGroup Cluster.

```bash
helm install myproxy kubeblocks/apecloud-mysql-cluster --version=v0.9.0 --set mode=raftGroup,proxyEnabled=true,etcd.serviceReference.endpoint="etcd-cluster-etcd.default.svc.cluster.local:2379" --set extra.availabilityPolicy=none
```

:::

6. Check the status of the clusters.

   ```bash
   kubectl get cluster

   kubectl get pods
   ```

   You can also enter the etcd container or wesql-scale container to view the configuration of wesql-scale or to check the availability of the etcd service.

   ```bash
   etcdctl --endpoints=http://etcd-cluster-etcd.default.svc.cluster.local:2379 get /vitess --prefix --keys-only
   ```

</TabItem>

</Tabs>

## Enable/Disable Proxy dynamically

As its name suggests, ApeCloud MySQL Proxy in nature is a database proxy. An ApeCloud MySQL RaftGroup Cluster that already exists can be switched to an ApeCloud MySQL Proxy Cluster by setting `proxyEnabled=true`.

```bash
helm upgrade mycluster kubeblocks/apecloud-mysql-cluster --set mode=raftGroup,proxyEnabled=true,etcd.serviceReference.endpoint="etcd-cluster-etcd.default.svc.cluster.local:2379"
```

If you want to disable proxy, run the command below.

```bash
helm upgrade mycluster kubeblocks/apecloud-mysql-cluster --set mode=raftGroup
```

## Connect Proxy Cluster

ApeCloud MySQL Proxy is routed through the `vtgate` component, and the way the MySQL Server accesses `vtgate` is similar to the way of accessing `mysqld`. The external SQL access address provided by ApeCloud MySQL Proxy is the `vtgate` address and port. The `vtgate` address created by KubeBlocks by default is `myproxy-cluster-vtgate-headless`, and the port number is `15306`. You can visit ApeCloud MySQL Proxy through the MySQL Server in any pod under the same namespace as ApeCloud MySQL Proxy.

### Connect Proxy Cluster by VTGate

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

Run the command below to connect to the Proxy Cluster.

```bash
kbcli cluster connect myproxy --components vtgate
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

1. Expose the port of VTGate to the localhost so that the localhost can access the Proxy.

   ```bash
   kubectl port-forward svc/vt-vtgate-headless 15306:15306
   ```

2. Connect to the cluster.

   ```bash
   mysql -h 127.0.0.1 -P 15306
   ```

</TabItem>

</Tabs>

### Connect Proxy Cluster by MySQL Server

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

Run the command below to connect to the MySQL Server.

```bash
kbcli cluster connect myproxy
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

1. Expose the port of the MySQL Server to the localhost so that the localhost can access the MySQL Server.

   ```bash
   kubectl port-forward svc/vt-mysql 3306:3306
   ```

2. Connect to the cluster.

   ```bash
   mysql -h 127.0.0.1 -P 3306
   ```

</TabItem>

</Tabs>

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

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

VTGate, VTConsensus, and VTTablet support parameter configuration. You can configure VTGate and VTConsensus by using `--components` to specify a component and configure VTTablet by using `--components=mysql --config-specs=vttablet-config` to specify both a component and a configuration file template since VTTablet is the sidecar of the MySQL component.

### View parameter details

* View the details of the current configuration file.

   ```bash
   # vtgate
   kbcli cluster describe-config myproxy --components vtgate --show-detail
   
   # vtcontroller
   kbcli cluster describe-config myproxy --components vtcontroller --show-detail
   
   # vttablet
   kbcli cluster describe-config myproxy --components mysql --show-detail --config-specs vttablet-config
   ```

* View the parameter descriptions.

   ```bash
   # vtgate
   kbcli cluster explain-config myproxy --components vtgate

   # vttablet
   kbcli cluster explain-config myproxy --components mysql --config-specs=vttablet-config
   ```

* View the definition of a specified parameter.

   ```bash
   kbcli cluster explain-config myproxy --components vtgate --param=healthcheck_timeout
   ```

### Reconfigure parameters

1. View the current values in the MySQL Server.

   ```bash
   kbcli cluster connect myproxy --components=vtgate
   ```

   ```bash
   mysql> show variables like '%healthcheck_timeout%';
   ```

   ```bash
   mysql> show variables like '%health_check_interval%';
   ```

2. Configure the `healthcheck_timeout` for VTGate and the `health_check_interval` for VTTablet.

   You can use `--set` flag or edit the parameter configuration file to edit values.

   * By using `--set` flag

      ```bash
      # vtgate
      kbcli cluster configure myproxy --components vtgate --set=healthcheck_timeout=2s

      # vttablet
      kbcli cluster configure myproxy --set=health_check_interval=4s --components=mysql --config-spec=vttablet-config
      ```

   * By editing the parameter configuration file

      ```bash
      kbcli cluster edit-config myproxy --components vtgate
      ```

    :::note

    After the `vtgate` parameter values configuration command is executed, a new vtgate Pod is started and the old vtgate Pod is terminated. You can run the command below to monitor whether the old Pod is terminated.

    ```bash
    kubectl get pod <vtgate-pod-name> -w
    ```

    :::

3. Use the output command to view the configuration status. For example,

   ```bash
   kbcli cluster describe-ops myproxy -reconfiguring-lth8d -n default
   ```

   :::note

   For more information about parameter configuration, refer to [Configuration](./../configuration/configuration.md).

   :::

</TabItem>

<TabItem value="kubectl" label="kubectl">

VTGate, VTConsensus, and VTTablet support parameter configuration. You can configure the proxy cluster by editing the configuration file or by performing an OpsRequest.

### Option 1. Edit the config file

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

### Option 2. Apply an OpsRequest

Apply an OpsRequest to the specified cluster. Configure the parameters according to your needs.

* An example of configuring VTTablet

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

* An example of configuring VTGate

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

You can view the log files of components, Pods, and containers by both kbcli and kubectl.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

View the log of different components.

```bash
kbcli cluster list-logs myproxy
kbcli cluster list-logs myproxy --components vtgate
kbcli cluster list-logs myproxy --components vtcontroller
kbcli cluster list-logs myproxy --components mysql
```

View the log of a Pod.

```bash
kbcli cluster logs myproxy --instance myproxy-vtgate-85bdcf99df-wbmnl
```

View the log of a container in a Pod.

```bash
kbcli cluster logs myproxy --instance myproxy-mysql-0 -c vttablet
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

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

Enter the container and view more logs of VTTablet.

```bash
kubectl exec -it myproxy-cluster-mysql-0  -c vttablet -- bash
ls /vtdataroot
```

</TabItem>

</Tabs>

## Monitoring

:::note

In the production environment, all monitoring Addons are disabled by default when installing KubeBlocks. You can enable these Addons but it is highly recommended to build your monitoring system or purchase a third-party monitoring service. For details, refer to [Monitoring](./../../observability/monitor-database.md).

:::

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

1. Enable the monitoring function.

   ```bash
   kbcli cluster update myproxy --disable-exporter=false
   ```

2. View the Addon list and enable the Grafana Addon.

   ```bash
   kbcli addon list 
   
   kbcli addon enable grafana
   ```

3. View the dashboard list.

   ```bash
   kbcli dashboard list
   ```

4. Open the Grafana dashboard.

   ```bash
   kbcli dashboard open kubeblocks-grafana
   ```

</TabItem>

<TabItem value="kubectl" label="kubectl">

1. Enable the monitoring Addons.

   For the testing/demo environment, run the commands below to enable the monitoring Addons provided by KubeBlocks.

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

</TabItem>

</Tabs>

## Read-write splitting

You can enable the read-write splitting function.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli cluster configure myproxy --components vtgate --set=read_write_splitting_policy=random
```

You can also set the ratio for read-write splitting and here is an example of directing 70% flow to the read-only node.

```bash
kbcli cluster configure myproxy --components vtgate --set=read_write_splitting_ratio=70
```

Moreover, you can [use Grafana](#monitoring) or run `show workload` to view the flow distribution.

```bash
show workload;
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

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

</TabItem>

</Tabs>

## Transparent failover

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

Run the command below to implement transparent failover.

```bash
kbcli cluster configure myproxy --components vtgate --set=enable_buffer=true
```

</TabItem>

<TabItem value="kubectl" label="kubectl">

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

</TabItem>

</Tabs>
