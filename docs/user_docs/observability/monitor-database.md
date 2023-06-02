---
title: Monitor database
description: How to monitor your database
keywords: [monitor database, monitor a cluster, monitor]
sidebar_position: 1
---

# Observability of KubeBlocks

With the built-in database observability, you can observe the database health status and track and measure your database in real-time to optimize database performance. This section shows you how database observability works with KubeBlocks and how to use the function.

## Enable database monitor

***Steps:***

1. Enable the monitor function for KubeBlocks.

   When installing KubeBlocks, the monitoring add-ons are installed by default.

   ```bash
   kbcli kubeblocks install
   ```

   If it is not `true`, you can set `--monitor` to `true`.

    ```bash
    kbcli kubeblocks install --monitor=true
    ```

   If you have installed KubeBlocks without the monitoring add-ons, you can use `kbcli addon` to enable the monitoring add-ons. To ensure the completeness of the monitoring function, it is recommended to enable three monitoring add-ons.

    ```bash
    # View all add-ons supported
    kbcli addon list
    ...
    grafana                        Helm   Enabled                   true                                                                                    
    alertmanager-webhook-adaptor   Helm   Enabled                   true                                                                                    
    prometheus                     Helm   Enabled    alertmanager   true 
    ...
    # Enable prometheus add-on
    kbcli addon enable prometheus

    # Enable granfana add-on
    kbcli addon enable granfana

    # Enable alertmanager-webhook-adaptor add-on
    kbcli addon enable alertmanager-webhook-adaptor
    ```

:::note

Refer to [Enable add-ons](./../installation/enable-addons.md) for details.

:::

1. Enable the database monitoring function.

    The monitoring function is enabled by default when a database is created. The open-source or customized Exporter is injected after the monitoring function is enabled. This Exporter can be found by Prometheus server automatically and scrape monitoring indicators at regular intervals.

    - For a new cluster, run the command below to create a database cluster.

       ```bash
       # Search the cluster definition
       kbcli clusterdefinition list 

       # Create a cluster
       kbcli cluster create <name> --cluster-definition='xxx'
       ```

       ***Example***

       ```bash
       kbcli cluster create mysql-cluster --cluster-definition='apecloud-mysql'
       ```

    :::note

    The setting of `monitor` is `true` by default and it is not recommended to disable it. In the cluster definition, you can choose any supported database engines, such as PGSQL, MongoDB, Redis.

    ```bash
    kbcli cluster create mycluster --cluster-definition='apecloud-mysql' --monitor=true
    ```

    :::

    - For the existing cluster, you can update it to enable the monitor function with `update` command.

       ```bash
       kbcli cluster update <name> --monitor=true
       ```

       ***Example***

       ```bash
       kbcli cluster update mysql-cluster --monitor=true
       ```

You can view the dashboard of the corresponding cluster via Grafana Web Console. For more detailed information, see [Grafana documentation](https://grafana.com/docs/grafana/latest/dashboards/).

3. View the Web Console of the monitoring components.

    1. View the Web Console list of the monitoring components after the components are installed.

       ```bash
       kbcli dashboard list
       >
       NAME                                      NAMESPACE        PORT        CREATED-TIME
       kubeblocks-grafana                        default          3000        Jan 13,2023 10:53 UTC+0800
       kubeblocks-prometheus-alertmanager        default          9093        Jan 13,2023 10:53 UTC+0800
       kubeblocks-prometheus-server              default          9090        Jan 13,2023 10:53 UTC+0800
       ```

    2. Open the Web Console of a specific monitoring add-on listed above, you can copy it from the above list.

       ```bash
       kbcli dashboard open <name>
       ```

    ***Example***

     ```bash
         kbcli dashboard open kubeblocks-grafana
     ```

    ***Result***

    A monitoring page on Grafana website is loaded automatically after the command is executed.

    3. Click the Dashboard icon on the left bar and two monitoring panels show on the page.
     ![Dashboards](./../../img/quick_start_dashboards.png)

    4. Click **General** -> **MySQL** to monitor the status of the ApeCloud MySQL cluster created by Playground.
     ![MySQL_panel](./../../img/quick_start_mysql_panel.png)

:::note

The Prometheus add-on uses the local ephemeral storage by default, which might cause data loss when its Pod migrates to other pods. To avoid data loss, it is recommended to follow the steps below to enable PersistentVolume to meet the requirement of data persistence.

1. Disable the Prometheus add-on.

   ```bash
   kbcli addon disable prometheus
   ```

   :::caution

   Disabling the Prometheus add-on might cause the loss of local ephemeral storage.

   :::

2. Enable the PersistentVolume.

   PersistentVolumeClaim and PersistentVolume are created after executing this command and these resources require manual cleanup.

   ```bash
   kbcli addon enable prometheus --storage 10Gi
   ```

3. (Optional) If you want to stop using the PersistentVolume, execute the command below.

   ```bash
   kbcli addon enable prometheus --storage 0Gi
   ```

:::
