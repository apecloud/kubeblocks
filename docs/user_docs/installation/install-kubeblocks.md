---
title: Install KubeBlocks
description: Install KubeBlocks on the existing Kubernetes clusters with Helm
keywords: [taints, affinity, tolerance, install, kbcli, KubeBlocks, helm]
sidebar_position: 3
sidebar_label: Install KubeBlocks
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Install KubeBlocks

The quickest way to try out KubeBlocks is to create a new Kubernetes cluster and install KubeBlocks using the playground. However, production environments are more complex, with applications running in different namespaces and with resource or permission limitations. This document explains how to deploy KubeBlocks on an existing Kubernetes cluster.

KubeBlocks is Kubernetes-native, you can use Helm or kubectl with a YAML file to install it. You can also use kbcli, an intuitive CLI tool, to install KubeBlocks.

To try out KubeBlocks on your local host, you can use the [Playground](./../../try-out-on-playground/try-kubeblocks-on-your-laptop.md) or [create a local Kubernetes test cluster first](./../prepare-a-local-k8s-cluster/prepare-a-local-k8s-cluster.md) and then follow the steps in this tutorial to install KubeBlocks.

:::note

- Note that you install and uninstall KubeBlocks with the same tool. For example, if you install KubeBlocks with Helm, to uninstall it, you have to use Helm too.
- Make sure you have [kubectl](https://kubernetes.io/docs/tasks/tools/), [Helm](https://helm.sh/docs/intro/install/), or [kbcli](./install-kbcli.md) installed.

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

Like any other resource in Kubernetes, KubeBlocks can be installed through a YAML manifest applied via `kubectl`.

1. Create dependent CRDs. Specify the version you want to install.

   ```bash
   kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/vx.y.z/kubeblocks_crds.yaml
   ```

   You can view all versions of kubeblocks, including alpha and beta releases, on the [kubeblocks releases list](https://github.com/apecloud/kubeblocks/releases).

   To get stable releases, use this command:

   ```bash
   curl -s "https://api.github.com/repos/apecloud/kubeblocks/releases?per_page=100&page=1" | jq -r '.[] | select(.prerelease == false) | .tag_name' | sort -V -r
   ```

2. Copy the URL of the `kubeblocks.yaml` file for the version you need from the Assets on the [KubeBlocks Release page](https://github.com/apecloud/kubeblocks/releases).
3. Replace the YAML file URL in the command below and run this command to install KubeBlocks.

     ```bash
     kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/vx.y.x/kubeblocks.yaml
     ```

</TabItem>

<TabItem value="kbcli" label="Install with kbcli">

The command `kbcli kubeblocks install` installs KubeBlocks in the `kb-system` namespace, or you can use the `--namespace` flag to specify one.

```bash
kbcli kubeblocks install
```

If you want to install KubeBlocks with a specified version, follow the steps below.

1. View the available versions.

   ```bash
   kbcli kubeblocks list-versions
   ```

   To include alpha and beta releases, use:

   ```bash
   kbcli kb list-versions --devel --limit=100
   ```

   Or you can view all available versions in [KubeBlocks Release](https://github.com/apecloud/kubeblocks/releases/).
2. Specify a version with `--version` and run the command below.

   ```bash
   kbcli kubeblocks install --version=x.y.z
   ```

  :::note

   By default, when installing KubeBlocks, kbcli installs the corresponding version of KubeBlocks. It's important to ensure the major versions of kbcli and KubeBlocks are the same, if you specify a different version explicitly here.

   For example, you can install kbcli v0.8.3 with KubeBlocks v0.8.1, but using mismatched major versions, such as kbcli v0.8.3 with KubeBlocks v0.9.0, may lead to errors.
  
  :::

</TabItem>

</Tabs>

## Verify KubeBlocks installation

Run the following command to check whether KubeBlocks is installed successfully.

<Tabs>

<TabItem value="kubectl" label="kubectl" default>

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

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli kubeblocks status
```

***Result***

If the KubeBlocks Workloads are all ready, KubeBlocks has been installed successfully.

```bash
KubeBlocks is deployed in namespace: kb-system,version: x.y.z
>
KubeBlocks Workloads:
NAMESPACE   KIND         NAME                           READY PODS   CPU(CORES)   MEMORY(BYTES)   CREATED-AT
kb-system   Deployment   kb-addon-snapshot-controller   1/1          N/A          N/A             Oct 13,2023 14:27 UTC+0800
kb-system   Deployment   kubeblocks                     1/1          N/A          N/A             Oct 13,2023 14:26 UTC+0800
kb-system   Deployment   kubeblocks-dataprotection      1/1          N/A          N/A             Oct 13,2023 14:26 UTC+0800
```

</TabItem>

</Tabs>
