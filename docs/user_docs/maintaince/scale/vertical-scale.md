---
title: Vertical Scale
description: How to vertically scale a cluster
keywords: [vertical scale, vertical scaling]
sidebar_position: 1
sidebar_label: Vertical Scale
---

# Vertical Scale

You can change the resource requirements and limits (CPU and storage) by performing the vertical scale. For example, you can change the resource class from 1C2G to 2C4G.

This tutorial takes MySQL as an example to illustrate how to vertically scale a cluster.

:::note

- During the vertical scaling process, all pods restart and the pod role may change after the restarting.
- From v0.9.0, for MySQL and PostgreSQL, after vertical scaling is performed, KubeBlocks automatically matches the appropriate configuration template based on the new specification. This is the KubeBlocks dynamic configuration feature. This feature simplifies the process of configuring parameters, saves time and effort and reduces performance issues caused by incorrect configuration. For detailed instructions, refer to [Configuration](./../configuration/configuration.md).

:::

## Before you start

Check whether the cluster status is `Running`. Otherwise, the following operations may fail.

```bash
kbcli cluster list mycluster
>
NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS    CREATED-TIME
mycluster   default     mysql                mysql-8.0.33   Delete               Running   Jul 05,2024 19:06 UTC+0800
```

## Steps

1. Change configuration.

    Configure the parameters `--components`, `--memory`, and `--cpu` and run the command.

    ```bash
    kbcli cluster vscale mycluster \
    --components="mysql" \
    --memory="4Gi" --cpu="2" \
    ```

    - `--components` describes the component name ready for vertical scaling.
    - `--memory` describes the requested and limited size of the component memory.
    - `--cpu` describes the requested and limited size of the component CPU.

2. Check the cluster status to validate the vertical scaling.

    ```bash
    kbcli cluster list mycluster
    >
    NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION        TERMINATION-POLICY   STATUS     CREATED-TIME
    mycluster   default     mysql                mysql-8.0.33   Delete               Updating   Jul 05,2024 19:11 UTC+0800
    ```

    - STATUS=Updating: it means the vertical scaling is in progress.
    - STATUS=Running: it means the vertical scaling operation has been applied.
    - STATUS=Abnormal: it means the vertical scaling is abnormal. The reason may be that the number of the normal instances is less than that of the total instance or the leader instance is running properly while others are abnormal.

       To solve the problem, you can manually check whether this error is caused by insufficient resources. Then if AutoScaling is supported by the Kubernetes cluster, the system recovers when there are enough resources. Otherwise, you can create enough resources and troubleshoot with `kubectl describe` command.

3. Check whether the corresponding resources change.

    ```bash
    kbcli cluster describe mycluster
    ```
