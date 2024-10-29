---
title: Install KubeBlocks
description: Install KubeBlocks on the existing Kubernetes clusters with Helm
keywords: [taints, affinity, tolerance, install, kbcli, KubeBlocks]
sidebar_position: 1
sidebar_label: Install KubeBlocks
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Install KubeBlocks 

KubeBlocks is Kubernetes-native, you can use Helm or kubectl with a YAML file to install it.

To try out KubeBlocks on your local host, you can use the [Playground](./../../try-out-on-playground/try-kubeblocks-on-your-laptop.md) or [create a local Kubernetes test cluster first](./../prerequisite/prepare-a-local-k8s-cluster.md) and then follow the steps in this tutorial to install KubeBlocks.

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

<Tabs>

<TabItem value="Helm" label="Install with Helm" default>

Use Helm and follow the steps below to install KubeBlocks.

1. Create dependent CRDs. Specify the version you want to install.

   ```bash
   kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/vx.y.z/kubeblocks_crds.yaml
   ```
   You can view all versions of kubeblocks, including alpha and beta releases, on the [kubeblocks releases list](https://github.com/apecloud/kubeblocks/releases).

   To get stable releases, use this command:
   ```bash
   curl -s "https://api.github.com/repos/apecloud/kubeblocks/releases?per_page=100&page=1" | jq -r '.[] | select(.prerelease == false) | .tag_name' | sort -V -r
   ```
   

2. Add the KubeBlocks Helm repo.

   ```bash
   helm repo add kubeblocks https://apecloud.github.io/helm-charts
   helm repo update
   ```

3. Install KubeBlocks.

   ```bash
   helm install kubeblocks kubeblocks/kubeblocks --namespace kb-system --create-namespace
   ```

   If you want to install KubeBlocks with custom tolerations, you can use the following command:

   ```bash
   helm install kubeblocks kubeblocks/kubeblocks --namespace kb-system --create-namespace \
       --set-json 'tolerations=[ { "key": "control-plane-taint", "operator": "Equal", "effect": "NoSchedule", "value": "true" } ]' \
       --set-json 'dataPlane.tolerations=[{ "key": "data-plane-taint", "operator": "Equal", "effect": "NoSchedule", "value": "true"    }]'
   ```

   If you want to install KubeBlocks with a specified version, follow the steps below.

   1. View the available versions in the [KubeBlocks Release](https://github.com/apecloud/kubeblocks/releases/).
   2. Specify a version with `--version` and run the command below.

      ```bash
      helm install kubeblocks kubeblocks/kubeblocks --namespace kb-system --create-namespace --version="x.y.z"
      ```

     :::note

     By default, the latest release version is installed.

     :::

</TabItem>

<TabItem value="kubectl" label="Install with kubectl">

KubeBlocks can be installed like any other resource in Kubernetes, through a YAML manifest applied via `kubectl`.

Run the following command to install the latest operator manifest for this minor release:

 ```bash
 kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/vx.y.x/kubeblocks.yaml
 ```

You can find the YAML file URL from the [KubeBlocks release assets](https://github.com/apecloud/kubeblocks/releases).

</TabItem>

</Tabs>

## Verify KubeBlocks installation

Run the following command to check whether KubeBlocks is installed successfully.

```bash
kubectl -n kb-system get pods
```

***Result***

If the KubeBlocks Workloads are all ready, KubeBlocks has been installed successfully.

```bash
NAME                                             READY   STATUS    RESTARTS       AGE
alertmanager-webhook-adaptor-8dfc4878d-svzrc     2/2     Running   0              3m56s
grafana-77dfd6959-4gnhp                          1/1     Running   0              3m56s
kb-addon-snapshot-controller-546f84b78d-8rjs4    1/1     Running   0              3m56s
kubeblocks-7cf7745685-ddlwk                      1/1     Running   0              4m39s
kubeblocks-dataprotection-95fbc79cc-b544l        1/1     Running   0              4m39s
prometheus-alertmanager-5c9fc88996-qltrk         2/2     Running   0              3m56s
prometheus-kube-state-metrics-5dbbf757f5-db9v6   1/1     Running   0              3m56s
prometheus-node-exporter-r6kvl                   1/1     Running   0              3m56s
prometheus-pushgateway-8555888c7d-xkgfc          1/1     Running   0              3m56s
prometheus-server-5759b94fc8-686xp               2/2     Running   0              3m56s
```
