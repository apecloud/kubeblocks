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

The command `kbcli kubeblocks install` installs KubeBlocks in the `kb-system` namespace, or you can use the `--namespace` flag to specify one.

You can also isolate the KubeBlocks control plane and data plane resources by setting taints and tolerations. Choose from the following options.

### Install KubeBlocks with default tolerations

By default, KubeBlocks tolerates two taints: `kb-controller:NoSchedule` for the control plane and `kb-data:NoSchedule` for the data plane. You can add these taints to nodes so that KubeBlocks and database clusters are scheduled to the appropriate nodes.

1. Get Kubernetes nodes.

    ```bash
    kubectl get node
    ```

2. Add taints to the selected nodes.

    ```bash
    # add control plane taint
    kubectl taint nodes <nodename> kb-controller=true:NoSchedule
   
    # add data plane taint
    kubectl taint nodes <nodename> kb-data=true:NoSchedule
    ```

3. Install KubeBlocks.

    ```bash
    kbcli kubeblocks install
    ```

### Install KubeBlocks with custom tolerations

Another option is to tolerate custom taints, regardless of whether they are already set on the nodes.

1. Get Kubernetes nodes.

    ```bash
    kubectl get node
    ```

2. If the selected nodes do not already have custom taints, add them.

    ```bash
    # set control plane taint
    kubectl taint nodes <nodename> <control-plane-taint>=true:NoSchedule
     
    # set data plane taint
    kubectl taint nodes <nodename> <data-plane-taint>=true:NoSchedule
    ```

3. Install KubeBlocks with control plane and data plane tolerations.

    ```bash
    kbcli kubeblocks install --set-json 'tolerations=[ { "key": "control-plane-taint", "operator": "Equal", "effect": "NoSchedule", "value": "true" } ]' --set-json 'dataPlane.tolerations=[{ "key": "data-plane-taint", "operator": "Equal", "effect": "NoSchedule", "value": "true" } ]'
    ```

**Use Helm to install KubeBlocks**
```bash
helm repo add kubeblocks https://apecloud.github.io/helm-charts
helm repo update
helm install kubeblocks kubeblocks/kubeblocks \
    --namespace kb-system --create-namespaces
````

If you want to install KubeBlocks with custom tolerations, you can use the following command:
```bash
helm install kubeblocks kubeblocks/kubeblocks \
    --namespace kb-system --create-namespaces \
    --set-json 'tolerations=[ { "key": "control-plane-taint", "operator": "Equal", "effect": "NoSchedule", "value": "true" } ]' \
    --set-json 'dataPlane.tolerations=[{ "key": "data-plane-taint", "operator": "Equal", "effect": "NoSchedule", "value": "true" } ]'
```

**Use YAML to install KubeBlocks**

If you prefer to not use Helm, you can generate YAMLs from KubeBlocks chart and deploy using `kubectl`. 

```bash
helm repo add kubeblocks https://apecloud.github.io/helm-charts
helm repo update
helm template kubeblocks kubeblocks/kubeblocks \
    --namespace kb-system --create-namespaces | kubectl apply -f -
```

:::note

When executing the `kbcli kubeblocks install` command, the `preflight` checks will automatically verify the environment. If the cluster satisfies the basic requirements, the installation process will proceed. Otherwise, the process will be terminated, and an error message will be displayed. To skip the `preflight` checks, add the `--force` flag after the `kbcli kubeblocks install` command.

:::

## Verify KubeBlocks

Run the following command to check whether KubeBlocks is installed successfully.

```bash
kubectl get pods --all-namespaces -l "app.kubernetes.io/instance=kubeblocks" -w

NAME                                                     READY   STATUS      RESTARTS   AGE
kubeblocks-846b8878d9-q8g2w                              1/1     Running     0          98s
```

If the operator pods are all `Running`, KubeBlocks has been installed successfully. You can cancel the above command by typing `Ctrl+C`.
