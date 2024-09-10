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
   kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/vx.x.x/kubeblocks_crds.yaml
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
      helm install kubeblocks kubeblocks/kubeblocks --namespace kb-system --create-namespace --version="x.x.x"
      ```

     :::note

     By default, the latest release version is installed.

     :::

</TabItem>

<TabItem value="kubectl" label="Install with kubectl">

KubeBlocks can be installed like any other resource in Kubernetes, through a YAML manifest applied via `kubectl`.

Run the following command to install the latest operator manifest for this minor release:

 ```bash
 kubectl create -f \address.yaml
 ```

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
NAME                                                     READY   STATUS       AGE
kb-addon-snapshot-controller-7b447684d4-q86zf            1/1     Running      33d
kb-addon-csi-hostpath-driver-0                           8/8     Running      33d
kb-addon-grafana-54b9cbf65d-k8522                        3/3     Running      33d
kb-addon-apecloud-otel-collector-j4thb                   1/1     Running      33d
kubeblocks-5b5648bfd9-8jpvv                              1/1     Running      33d
kubeblocks-dataprotection-f54c9486c-2nfmr                1/1     Running      33d
kb-addon-alertmanager-webhook-adaptor-76b87f9df8-xb74g   2/2     Running      33d
kb-addon-prometheus-server-0                             2/2     Running      33d
kb-addon-prometheus-alertmanager-0                       2/2     Running      33d
```
