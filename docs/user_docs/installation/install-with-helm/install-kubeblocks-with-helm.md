---
title: Install KubeBlocks with Helm
description: Install KubeBlocks on the existing Kubernetes clusters with Helm
keywords: [taints, affinity, tolerance, install, kbcli, KubeBlocks]
sidebar_position: 3
sidebar_label: Install KubeBlocks
---

# Install KubeBlocks with Helm

KubeBlocks is kubernetes-native, you can use Helm to install it.
:::note

If you install KubeBlocks with Helm, to uninstall it, you have to use Helm too.
Make sure you have [kubectl](https://kubernetes.io/docs/tasks/tools/) and [Helm](https://helm.sh/docs/intro/install/) installed.
:::


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

**Use Helm to install KubeBlocks**

Run the following command:

```bash
helm repo add kubeblocks https://apecloud.github.io/helm-charts
helm repo update
helm install kubeblocks kubeblocks/kubeblocks \
    --namespace kb-system --create-namespace
```

If you want to install KubeBlocks with custom tolerations, you can use the following command:

```bash
helm install kubeblocks kubeblocks/kubeblocks \
    --namespace kb-system --create-namespace \
    --set-json 'tolerations=[ { "key": "control-plane-taint", "operator": "Equal", "effect": "NoSchedule", "value": "true" } ]' \
    --set-json 'dataPlane.tolerations=[{ "key": "data-plane-taint", "operator": "Equal", "effect": "NoSchedule", "value": "true" } ]'
```

If you want to install KubeBlocks with a specified version, follow the steps below.

1. View the available versions in [KubeBlocks Release](https://github.com/apecloud/kubeblocks/releases/).
2. Specify a version with `--version` and run the command below.

   ```bash
   helm install kubeblocks kubeblocks/kubeblocks \
    --namespace kb-system --create-namespace --version="x.x.x"
   ```

  :::note

  By default, the latest release version is installed.

  :::

## Verify KubeBlocks installation

Run the following command to check whether KubeBlocks is installed successfully.

```bash
kubectl -n kb-system get pods
```

***Result***

If the KubeBlocks Workloads are all ready, KubeBlocks has been installed successfully.

```bash
NAME                                            READY   STATUS    RESTARTS      AGE
kb-addon-snapshot-controller-649f8b9949-2wzzk   1/1     Running   2 (24m ago)   147d
kubeblocks-dataprotection-f6dbdbf7f-5fdr9       1/1     Running   2 (24m ago)   147d
kubeblocks-6497f7947-mc7vc                      1/1     Running   2 (24m ago)   147d
```
