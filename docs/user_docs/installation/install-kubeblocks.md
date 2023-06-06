---
title: Install KubeBlocks
description: Install KubeBlocks on the existing Kubernetes clusters
keywords: [taints, affinity, tolerance, install, kbcli, KubeBlocks]
sidebar_position: 3
sidebar_label: Install KubeBlocks
---

# Install KubeBlocks

The quickest way to try out KubeBlocks is to create a new Kubernetes cluster and install KubeBlocks using the playground. However, production environments are more complex, with applications running in different namespaces and with resource or permission limitations. This document explains how to deploy KubeBlocks on an existing Kubernetes cluster.

## Environment preparation

<table>
	<tr>
	    <th colspan="3">Resource Requirements</th>
	</tr >
	<tr>
	    <td >Control Plane</td>
	    <td colspan="2">It is recommended to create 1 node with 4 cores, 4GB memory and 50GB storage. </td>
	</tr >
	<tr >
	    <td rowspan="4">Data Plane</td>
	    <td> MySQL </td>
	    <td>It is recommended to create at least 3 nodes with 2 cores, 4GB memory and 50GB storage. </td>
	</tr>
	<tr>
	    <td> PostgreSQL </td>
        <td>It is recommended to create at least 2 nodes with 2 cores, 4GB memory and 50GB storage.  </td>
	</tr>
	<tr>
	    <td> Redis </td>
        <td>It is recommended to create at least 2 nodes with 2 cores, 4GB memory and 50GB storage. </td>
	</tr>
	<tr>
	    <td> MongoDB </td>
	    <td>It is recommended to create at least 3 nodes with 2 cores, 4GB memory and 50GB storage. </td>
	</tr>
</table>

## Installation steps

The command `kbcli kubeblocks install` installs KubeBlocks in the `kb-system` namespace on nodes without taints by default. If you want to install KubeBlocks in other namespaces, use the `--namespace` flag to specify one. KubeBlocks can also be installed on the specified nodes by setting taints and tolerations. Choose from the following options.

### Install KubeBlocks with default tolerations

KubeBlocks defines two default tolerations: `kb-controller:NoSchedule` for control plane nodes and `kb-data:NoSchedule` for data plane nodes. You can apply matching taints to nodes so KubeBlocks is installed only on nodes with the default tolerations.

1. Get Kubernetes nodes.

    ```bash
    kubectl get node
    ```

2. Place taints on the selected nodes.

    ```bash
    # set control plane taint
    kubectl taint nodes <nodename> kb-controller=true:NoSchedule
   
    # set data plane taint
    kubectl taint nodes <nodename> kb-data=true:NoSchedule
    ```

3. Install KubeBlocks.

    ```bash
    kbcli kubeblocks install
    ```

### Install KubeBlocks with customized tolerations

If your nodes have existing taints or you want custom taints, specify control plane and data plane tolerations when installing KubeBlocks.

1. Get Kubernetes nodes.

    ```bash
    kubectl get node
    ```

2. Place customized taints on the selected nodes. If you already have taints for KubeBlocks, skip this step.

    ```bash
    # set control plane taint
    kubectl taint nodes <nodename> <control-plane-taint>=true:NoSchedule
     
    # set data plane taint
    kubectl taint nodes <nodename> <data-plane-taint>=true:NoSchedule
    ```

3. Install KubeBlocks with control plane and data plane tolerations.

    ```bash
    kbcli kubeblocks install --create-namespace  --namespace <name> --set-json 'tolerations=[ { "key": "control-plane-taint", "operator": "Equal", "effect": "NoSchedule", "value": "true" } ]' --set-json 'dataPlane.tolerations=[{ "key": "data-plane-taint", "operator": "Equal", "effect": "NoSchedule", "value": "true" } ]'
    ```

:::note

When executing the `kbcli kubeblocks install` command, the `preflight` checks run automatically to check the environment. If the current cluster meets the installation requirements, the installation continues. If it does not, the current process is terminated, and an error message is displayed. To skip the `preflight` checks, you can add the `--force` flag after the `kbcli kubeblocks install` command.

:::

## Verify KubeBlocks

Run the following command to check whether KubeBlocks is installed successfully.

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
```
