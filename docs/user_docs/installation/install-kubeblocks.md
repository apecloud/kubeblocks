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

To try out KubeBlocks on your local host, you can use the [Playground](./../try-out-on-playground/try-kubeblocks-on-your-laptop.md) or [create a local Kubernetes test cluster first](./prepare-a-local-k8s-cluster/prepare-a-local-k8s-cluster.md) and then follow the steps in this tutorial to install KubeBlocks.

:::note

- Note that you install and uninstall KubeBlocks with the same tool. For example, if you install KubeBlocks with Helm, to uninstall it, you have to use Helm too.
- Make sure you have [kubectl](https://kubernetes.io/docs/tasks/tools/), [Helm](https://helm.sh/docs/intro/install/), or [kbcli](./install-kbcli.md) installed.
- Make sure you have Snapshot Controller installed before installing KubeBlocks. If you haven't installed it, follow the steps in section [Install Snapshot Controller](#install-snapshot-controller) to install it.

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

## Install Snapshot Controller

The SnapshotController is a Kubernetes component that manages CSI Volume Snapshots, enabling users to create, restore, and delete snapshots of Persistent Volumes (PVs). KubeBlocks DataProtection Controller uses the Snapshot Controller to create and manage snapshots for databases.

If your Kubernetes cluster does NOT have the following CRDs, you likely also do not have a snapshot controller deployed and you need to install one.

```bash
kubectl get crd volumesnapshotclasses.snapshot.storage.k8s.io
kubectl get crd volumesnapshots.snapshot.storage.k8s.io
kubectl get crd volumesnapshotcontents.snapshot.storage.k8s.io
```

:::note

If you are sure you don't need the snapshot backup feature, you can install only the SnapshotController CRD and skip steps 1 and 2.

```bash
# v8.2.0 is the latest version of the external-snapshotter, you can replace it with the version you need.
kubectl create -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v8.2.0/client/config/crd/snapshot.storage.k8s.io_volumesnapshotclasses.yaml
kubectl create -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v8.2.0/client/config/crd/snapshot.storage.k8s.io_volumesnapshots.yaml
kubectl create -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v8.2.0/client/config/crd/snapshot.storage.k8s.io_volumesnapshotcontents.yaml
```

:::

### Step 1. Deploy Snapshot Controller

You can install the Snapshot Controller using Helm or kubectl. Here are the steps to install the Snapshot Controller using Helm.

```bash
helm repo add piraeus-charts https://piraeus.io/helm-charts/
helm repo update
# Update the namespace to an appropriate value for your environment (e.g. kb-system)
helm install snapshot-controller piraeus-charts/snapshot-controller -n kb-system --create-namespace
```

For more information, please refer to [Snapshot Controller Configuration](https://artifacthub.io/packages/helm/piraeus-charts/snapshot-controller#configuration).

### Step 2. Verify Snapshot Controller installation

Check if the snapshot-controller Pod is running:

```bash
kubectl get pods -n kb-system | grep snapshot-controller
```

<details>

<summary>Output</summary>

```bash
snapshot-controller-xxxx-yyyy   1/1   Running   0   30s
```

</details>

If the pod is in a CrashLoopBackOff state, check logs:

```bash
kubectl logs -n kb-system deployment/snapshot-controller
```

## Installation KubeBlocks

<Tabs>

<TabItem value="Helm" label="Install with Helm" default>

Follow these steps to install KubeBlocks using Helm.

1. Get the KubeBlocks version:

   * Option A - Get the latest stable version (e.g., v0.9.2):

      ```bash
      curl -s "https://api.github.com/repos/apecloud/kubeblocks/releases?per_page=100&page=1" | jq -r '.[] | select(.prerelease == false) | .tag_name' | sort -V -r | head -n 1
      ```

   * Option B - View all available versions (including alpha and beta releases):
      * Visit the [KubeBlocks Releases](https://github.com/apecloud/kubeblocks/releases).
      * Or use the command:
        ```bash
        curl -s "https://api.github.com/repos/apecloud/kubeblocks/releases?per_page=100&page=1" | jq -r '.[].tag_name' | sort -V -r
        ```

2. Create the required CRDs using your selected version:

   ```bash
   # Replace <VERSION> with your selected version
   kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/<VERSION>/kubeblocks_crds.yaml

   # Example: If the version is v0.9.2
   kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/v0.9.2/kubeblocks_crds.yaml
   ```

3. Add the KubeBlocks Helm repo:

   ```bash
   helm repo add kubeblocks https://apecloud.github.io/helm-charts
   helm repo update
   ```

4. Install KubeBlocks:

   ```bash
   helm install kubeblocks kubeblocks/kubeblocks --namespace kb-system --create-namespace
   ```

   If you want to install KubeBlocks with custom tolerations, you can use the following command:

   ```bash
   helm install kubeblocks kubeblocks/kubeblocks --namespace kb-system --create-namespace \
       --set-json 'tolerations=[ { "key": "control-plane-taint", "operator": "Equal", "effect": "NoSchedule", "value": "true" } ]' \
       --set-json 'dataPlane.tolerations=[{ "key": "data-plane-taint", "operator": "Equal", "effect": "NoSchedule", "value": "true"    }]'
   ```

   If you want to install KubeBlocks with a specified version, follow the steps below:

   1. View the available versions in the [KubeBlocks Releases](https://github.com/apecloud/kubeblocks/releases/).
   2. Specify a version with `--version` and run the command below:

      ```bash
      helm install kubeblocks kubeblocks/kubeblocks --namespace kb-system --create-namespace --version=<VERSION>
      ```

     :::note

     By default, the latest release version is installed.

     :::

</TabItem>

<TabItem value="kubectl" label="Install with kubectl">

Like any other resource in Kubernetes, KubeBlocks can be installed through a YAML manifest applied via `kubectl`.

1. Get the KubeBlocks version:

   * Option A - Get the latest stable version (e.g., v0.9.2):

      ```bash
      curl -s "https://api.github.com/repos/apecloud/kubeblocks/releases?per_page=100&page=1" | jq -r '.[] | select(.prerelease == false) | .tag_name' | sort -V -r | head -n 1
      ```

   * Option B - View all available versions (including alpha and beta releases):
      * Visit the [KubeBlocks Releases](https://github.com/apecloud/kubeblocks/releases).
      * Or use the command:
        ```bash
        curl -s "https://api.github.com/repos/apecloud/kubeblocks/releases?per_page=100&page=1" | jq -r '.[].tag_name' | sort -V -r
        ```

2. Create the required CRDs using your selected version:

   ```bash
   # Replace <VERSION> with your selected version
   kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/<VERSION>/kubeblocks_crds.yaml

   # Example: If the version is v0.9.2
   kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/v0.9.2/kubeblocks_crds.yaml
   ```

3. Install KubeBlocks:

   ```bash
   # Replace <VERSION> with the same version used in step 2
   kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/<VERSION>/kubeblocks.yaml

   # Example: If the version is v0.9.2
   kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/v0.9.2/kubeblocks.yaml
   ```

   :::note

   Make sure to use the same version for both CRDs and KubeBlocks installation to avoid compatibility issues.

   :::

</TabItem>

<TabItem value="kbcli" label="Install with kbcli">

The command `kbcli kubeblocks install` installs KubeBlocks in the `kb-system` namespace, or you can use the `--namespace` flag to specify one.

```bash
kbcli kubeblocks install
```

If you want to install KubeBlocks with a specified version, follow the steps below.

1. View the available versions:

   ```bash
   kbcli kubeblocks list-versions
   ```

   To include alpha and beta releases, use:

   ```bash
   kbcli kb list-versions --devel --limit=100
   ```

   Or you can view all available versions in [KubeBlocks Releases](https://github.com/apecloud/kubeblocks/releases/).
2. Specify a version with `--version` and run the command below:

   ```bash
   kbcli kubeblocks install --version=<VERSION>
   ```

  :::note

   By default, when installing KubeBlocks, kbcli installs the corresponding version of KubeBlocks. It's important to ensure the major versions of kbcli and KubeBlocks are the same, if you specify a different version explicitly here.

   For example, you can install kbcli v0.9.2 with KubeBlocks v0.9.1, but using mismatched major versions, such as kbcli v0.9.2 with KubeBlocks v1.0.0, may lead to errors.

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
kubeblocks-7cf7745685-ddlwk                      1/1     Running   0              4m39s
kubeblocks-dataprotection-95fbc79cc-b544l        1/1     Running   0              4m39s
```

</TabItem>

<TabItem value="kbcli" label="kbcli">

```bash
kbcli kubeblocks status
```

***Result***

If the KubeBlocks Workloads are all ready, KubeBlocks has been installed successfully.

```bash
KubeBlocks is deployed in namespace: kb-system, version: <VERSION>
>
KubeBlocks Workloads:
NAMESPACE   KIND         NAME                           READY PODS   CPU(CORES)   MEMORY(BYTES)   CREATED-AT
kb-system   Deployment   kubeblocks                     1/1          N/A          N/A             Oct 13,2023 14:26 UTC+0800
kb-system   Deployment   kubeblocks-dataprotection      1/1          N/A          N/A             Oct 13,2023 14:26 UTC+0800
```

</TabItem>

</Tabs>
