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
````

If you want to install KubeBlocks with custom tolerations, you can use the following command:
```bash
helm install kubeblocks kubeblocks/kubeblocks \
    --namespace kb-system --create-namespace \
    --set-json 'tolerations=[ { "key": "control-plane-taint", "operator": "Equal", "effect": "NoSchedule", "value": "true" } ]' \
    --set-json 'dataPlane.tolerations=[{ "key": "data-plane-taint", "operator": "Equal", "effect": "NoSchedule", "value": "true" } ]'
```

## Verify KubeBlocks installation

Run the following command to check whether KubeBlocks is installed successfully.

```bash
kubectl get pods --all-namespaces -l "app.kubernetes.io/instance=kubeblocks" -w

NAME                                                     READY   STATUS      RESTARTS   AGE
kubeblocks-846b8878d9-q8g2w                              1/1     Running     0          98s
```

If the operator pods are all `Running`, KubeBlocks has been installed successfully. You can cancel the above command by typing `Ctrl+C`.

:::note

Clusters installed through `helm` need to be deleted using `helm` to avoid resource residue.

:::