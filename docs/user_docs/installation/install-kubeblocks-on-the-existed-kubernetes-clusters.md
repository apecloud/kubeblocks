---
title: Install KubeBlocks on the existed Kubernetes clusters
description: Install KubeBlocks on the existed Kubernetes clusters
keywords: [taints, affinity, tolerance, install, kbcli, KubeBlocks]
sidebar_position: 3
sidebar_label: On the existed Kubernetes clusters
---

# Install KubeBlocks on the existed Kubernetes clusters

In the actual environment, it is normal to install `kbcli` and KubeBlocks on the existed Kubernetes clusters.

## Environment preparation

<table>
	<tr>
	    <th colspan="3">Installation Requirements</th>
	</tr >
	<tr>
	    <td >Control Plane</td>
	    <td colspan="2">It is recommended to create 1 node with at least 4c4g and 50Gi storage. </td>
	</tr >
	<tr >
	    <td rowspan="4">Data Plane</td>
	    <td>For MySQL database </td>
	    <td>It is recommended to create at least 3 nodes with 2c4Gi and 50Gi storage. </td>
	</tr>
	<tr>
	    <td>For PostgreSQL database </td>
        <td>It is recommended to create at least 2 nodes with 2c4Gi and 50Gi storage.  </td>
	</tr>
	<tr>
	    <td>For Redis database</td>
        <td>It is recommended to create at least 2 nodes with 2c4Gi and 50Gi storage. </td>
	</tr>
	<tr>
	    <td>For MongoDB database</td>
	    <td>It is recommended to create at least 3 nodes with 2c4Gi and 50Gi storage. </td>
	</tr>
</table>

## Installation steps

**Before you start**

Make sure you have kbcli installed, for detailed information, check [Install kbcli](#install-kbcli).

**Install KubeBlocks with `kbcli kubeblocks install` command.**

The installation command is `kbcli kubeblocks install`, simply running this command installs KubeBlocks on nodes without taints with default namespace `kb-system`.

But in actual scenarios, you are recommendend to install KubeBlocks on nodes with taints and customized namespace.

1. Get Kubernetes nodes.

    ```bash
    kubectl get node
    ```

2. Place taints on the selected nodes.

    ```bash
    kubectl taint nodes <nodename> <taint1name>=true:NoSchedule
    ```

3. Install KubeBlocks.

    ```bash
    kbcli kubeblocks install --create-namespace  --namespace <name> --set-json 'tolerations=[ { "key": "taint1name", "operator": "Equal", "effect": "NoSchedule", "value": "true" }, { "key": "taint2name", "operator": "Equal", "effect": "NoSchedule", "value": "true" } ]'
    ```

:::note

When executing of the `kbcli kubeblocks install` command, the `preflight` checks are automatically performed to check the environment. If the current cluster meets the installation requirements, the installation continues. If it does not, the current process is terminated, and an error message is displayed. To force skip the preflight checks, you can add the `--force` flag after the "kbcli kubeblocks install" command.

:::

4. Verify whether KubeBlocks is installed successfully.

    ```bash
    kubectl get pod -n kb-system
    ```

    ***Result***

    When the following pods are `Running`, it means KubeBlocks is installed successfully.

    ```bash
    NAME                                                     READY   STATUS      RESTARTS   AGE
    kb-addon-alertmanager-webhook-adaptor-5549f94599-fsnmc   2/2     Running     0          84s
    kb-addon-grafana-5ddcd7758f-x4t5g                        3/3     Running     0          84s
    kb-addon-prometheus-alertmanager-0                       2/2     Running     0          84s
    kb-addon-prometheus-server-0                             2/2     Running     0          84s
    kubeblocks-846b8878d9-q8g2w                              1/1     Running     0          98s