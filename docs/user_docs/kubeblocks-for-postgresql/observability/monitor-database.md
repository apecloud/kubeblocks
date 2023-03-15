---
title: Monitor database
description: How to monitor your database
sidebar_position: 1
---

# Observability of KubeBlocks 
With the built-in database observability, you can observe the database health status and track and measure your database in real-time to optimize database performance. This section shows you how database observability works with KubeBlocks and how to use the function.

## Monitor PostgreSQL database

KubeBlocks integrates open-source monitoring components such as Prometheus, AlertManager, and Granfana by add-ons in a native way. KubeBlocks also uses open-source and customized Prometheus Exporter for exporting database indicators. The monitoring function is enabled by default when you install KubeBlocks and no other operation is required.

KubeBlocks has three monitoring add-ons.

* prometheus add-on: It includes Prometheus and AlertManager components.
* granfana add-on: It includes the Granfana component.
* alertmanager-webhook-adaptor add-on: It includes instant message notification components and is used as an extension of the capability of AlertManager. Currently, Feishu custom bot, DingTalk custombot, and WeChat Enterprise custom bot are supported.

## Enable database monitor

***Steps:***

1. Install KubeBlocks and the monitoring add-ons are installed by default.
    
    ```bash
    kbcli kubeblocks install
    ```

    If you do not want to enable the monitoring add-ons when installing KubeBlocks, set `--monitor` to cancel the add-on installation. But it is not recommended to disable the monitoring function.

    ```bash
    kbcli kubeblocks install --monitor=false
    ```
    
    If you have installed KubeBlocks without the monitoring add-ons, you can use `kbcli addon` to enable the monitoring add-ons. To ensure the completeness of the monitoring function, it is recommended to enable three monitoring add-ons. 

    ```bash
    # View all add-ons supported
    kbcli addon list

    # Enable prometheus add-on
    kbcli addon enable prometheus

    # Enable granfana add-on
    kbcli addon enable granfana

    # Enable alertmanager-webhook-adaptor add-on
    kbcli addon enable alertmanager-webhook-adaptor
    ```

    :::note

    Refer to [Enable add-ons](./../../installation/enable-add-ons.md) for details.

    :::

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
       # Search the cluster definition
       kbcli clusterdefinition list 

       # Create a cluster
       kbcli cluster create <name> --cluster-definition='xxx'
       ```

        ***Example***

        ```bash
        kbcli cluster create pg-cluster --cluster-definition='postgresql'
        ```

    :::note

    The setting of `monitor` is `true` by default and it is not recommended to disable it. For example,
    ```bash
    kbcli cluster create mycluster --cluster-definition='postgresql' --monitor=false
    ```

    :::

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