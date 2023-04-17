---
title: Monitor database
description: How to monitor your database
keywords: [redis, monitor]
sidebar_position: 1
---

# Observability of KubeBlocks

With the built-in database observability, you can observe the database health status and track and measure your database in real-time to optimize database performance. This section shows you how database observability works with KubeBlocks and how to use this function.

## Enable database monitoring

***Steps:***

1. Install KubeBlocks and the monitoring add-ons are installed by default.

    ```bash
    kbcli kubeblocks install
    ```

    If you do not want to enable the monitoring add-ons when installing KubeBlocks, set `--monitor=false` to cancel the monitoring add-ons installation. But it is not recommended to disable the monitoring function.

    ```bash
    kbcli kubeblocks install --monitor=false
    ```

    If you have installed KubeBlocks without the monitoring add-ons, use `kbcli addon` to enable the monitoring add-ons. To ensure the completeness of the monitoring function, it is recommended to enable three monitoring add-ons.

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

    1. View the Web Console list of the monitoring components after the components are installed.

        ```bash
        kbcli dashboard list
        >
        NAME                                      NAMESPACE        PORT        CREATED-TIME
        kubeblocks-grafana                        default          3000        Jan 13,2023 10:53 UTC+0800
        kubeblocks-prometheus-alertmanager        default          9093        Jan 13,2023 10:53 UTC+0800
        kubeblocks-prometheus-server              default          9090        Jan 13,2023 10:53 UTC+0800
        ```

    2. Open the Web Console of a specific monitoring add-on listed above.

        ```bash
        kbcli dashboard open <name>
        ```

3. Enable the monitoring function for a database cluster.

    The monitoring function is enabled by default when a cluster is created. The open-source or customized Exporter is injected after the monitoring function is enabled. This Exporter can be found by the Prometheus server automatically and scrape monitoring indicators at regular intervals.

    - For a new cluster, run the command below to create a database cluster.

       ```bash
       # Search the cluster definition
       kbcli clusterdefinition list 

       # Create a cluster
       kbcli cluster create <name> --cluster-definition='xxx'
       ```

       ***Example***

       ```bash
       kbcli clusterdefinition list

       kbcli cluster create redis-cluster --cluster-definition='redis'
       ```

    :::note

    The `monitor` function for a database is set as `true` by default. If you do not want to enable the monitoring add-ons when creating a cluster, set `--monitor=false` but it is not recommended to disable it. For example,

    ```bash
    kbcli cluster create redis-cluster --cluster-definition='redis' --monitor=false
    ```

    :::

    - For the existing cluster, you can update it to enable the monitor function with the `update` subcommand.

       ```bash
       kbcli cluster update <name> --monitor=true
       ```

       ***Example***

       ```bash
       kbcli cluster update redis-cluster --monitor=true
       ```

   You can view the dashboard of the corresponding cluster via Grafana Web Console. For more detailed information, see [Grafana documentation](https://grafana.com/docs/grafana/latest/dashboards/).
