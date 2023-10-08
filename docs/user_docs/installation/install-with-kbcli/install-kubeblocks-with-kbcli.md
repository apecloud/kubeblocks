---
title: Install KubeBlocks with kbcli
description: Install KubeBlocks on the existing Kubernetes clusters
keywords: [taints, affinity, tolerance, install, kbcli, KubeBlocks]
sidebar_position: 3
sidebar_label: Install KubeBlocks
---

# Install KubeBlocks with kbcli

The quickest way to try out KubeBlocks is to create a new Kubernetes cluster and install KubeBlocks using the playground. However, production environments are more complex, with applications running in different namespaces and with resource or permission limitations. This document explains how to deploy KubeBlocks on an existing Kubernetes cluster.

## Environment preparation

Prepare an accessible Kubernetes cluster with the version 1.22 or above, and this cluster should meet the following requirements.

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

```bash
kbcli kubeblocks install
```


## Verify KubeBlocks installation

Run the following command to check whether KubeBlocks is installed successfully.

```bash
kubectl get pod -n kb-system
```

***Result***

If the following pods are all `Running`, KubeBlocks has been installed successfully.

```bash
NAME                                                     READY   STATUS      RESTARTS   AGE
kb-addon-alertmanager-webhook-adaptor-5549f94599-fsnmc   2/2     Running     0          84s
kb-addon-grafana-5ddcd7758f-x4t5g                        3/3     Running     0          84s
kb-addon-prometheus-alertmanager-0                       2/2     Running     0          84s
kb-addon-prometheus-server-0                             2/2     Running     0          84s
kubeblocks-846b8878d9-q8g2w                              1/1     Running     0          98s
```
