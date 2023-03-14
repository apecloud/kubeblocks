---
title: Monitor database
description: How to monitor your database
sidebar_position: 1
---

# Observability of KubeBlocks 
With the built-in database observability, you can observe the database health status and track and measure your database in real-time to optimize database performance. This section shows you how database observability works with KubeBlocks and how to use the function.

## Monitor PostgreSQL database
KubeBlocks integrates open-source monitoring components such as Prometheus, AlertManager, and Granfana. KubeBlocks also uses open-source and customized Prometheus Exporter for exporting database indicators. The monitoring function is enabled by default when you install KubeBlocks and no other operation is required.

## Enable database monitor

***Steps:***

1. Install monitor components.
   If you didn't install KubeBlocks, monitoring components (Prometheus/AlertManager/Grafana) are installed by default with the installation of KubeBlocks. The installation command line is as follows, see detailed information in [Install KubeBlocks](./../../installation/install-and-uninstall-kbcli-and-kubeblocks.md):
   ```bash
   kbcli kubeblocks install
   ```
    You can disable the monitoring function when installing KubeBlocks by setting the `--monitor` option. 
   ```bash
   kbcli kubeblocks install --monitor=false
   ```
    > ***Note:*** 
    > 
    > `--monitor` is set as true by default and it is not recommended to disable the monitoring function.

   If you have KubeBlocks installed, you can install monitoring components with `kbcli kubeblocks upgrade`.
   ```bash
   kbcli kubeblocks upgrade --monitor=true
   ```

    > ***Note:*** 
    > 
    > Prometheus uses PersistentVolume to ensure the historical data is not lost in case of failover or reinstallation. When Prometheus is installed, the default setting is to save 3 days of historical data with a PersistentVolume of 8Gi, which can ensure normal startup in most environments. As for production environments, it is recommended to evaluate the data retention days and the size of the PersistentVolume. You can also enable high availability or use `remote write` to write to remote storage. 
    
    To configure saving days, volume size, HA, and remote write, when installing or updating KubeBlocks, enable Prometheus with the following parameters.
    ```bash
    kbcli kubeblocks [install | upgrade] --monitor=true\
    --set "prometheus.server.remoteWrite={url1,url2}" # remoteWrite address. Multiple addresses are supported. It is recommended to enable this option in the production environment for long-term data storage.
    --set prometheus.server.persistentVolume.size=8Gi # PersistentVolume size. The default value is 1Gi. It is recommended to set the value to 8Gi or more in a production environment. You can evaluate this value according to the retention period and the collected database instance amount.
    --set prometheus.server.replicaCount=2 # Set the instance amount of Prometheus. The default value is 1. If there is a demand for high availability,  you can set it to 2 and then deduplication capability is required for remote write to remote storage.
    --set prometheus.server.retention=15d  # Set the data retention period. The default is 15 days.
    ```
2. View the Web Console of the monitoring components.
   Run the command below to view the Web Console list of the monitoring components after the components are installed.
   ```bash
   kbcli dashboard list
   >
   NAME                                      NAMESPACE        PORT        CREATED-TIME
   kubeblocks-grafana                        default          3000        Jan 13,2023 10:53 UTC+0800
   kubeblocks-prometheus-alertmanager        default          9093        Jan 13,2023 10:53 UTC+0800
   kubeblocks-prometheus-server              default          9090        Jan 13,2023 10:53 UTC+0800
   ```
   For the Web Console list returned by the above command, if you want to view the Web Console of a specific monitoring component, run the command below and this command enables the port-forward of your local host and opens the default browser:
   ```bash
   kbcli dashboard open <name>
   ```
3. Enable the database monitoring function.
   The monitoring function is enabled by default when a database is created. The open-source or customized Exporter is injected after the monitoring function is enabled. This Exporter can be found by Prometheus server automatically and scrape monitoring indicators at regular intervals. 
   - For a new cluster, run the command below to create a database cluster.
      ```bash
      kbcli cluster create <name> --cluster-definition='xxx'
      ```

      ***Example***

      ```bash
      kbcli cluster create pg-cluster --cluster-definition='postgresql'
      ```
      > ***Note:*** 
      >
      > The setting of `monitor` is `true` by default and it is not recommended to disable it. For example,
      > ```bash
      > kbcli cluster create mycluster --cluster-definition='postgresql' --monitor=false
      > ```
    You can change the value to `false` to disable the monitor function if required.
   - For the existing cluster, you can update it to enable the monitor function with `update` command.

     ```bash
     kbcli cluster update <name> --monitor=true
     ```

     ***Example***

     ```bash
     kbcli cluster update pg-cluster --monitor=true
     ```

You can view the dashboard of the corresponding cluster via Grafana Web Console. For more detailed information, see [Grafana documentation](https://grafana.com/docs/grafana/latest/dashboards/).