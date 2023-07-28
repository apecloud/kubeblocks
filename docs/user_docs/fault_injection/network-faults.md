---
title: Simulate network faults
description: Simulate network faults
sidebar_position: 4
sidebar_label: Simulate network faults
---

# Simulate network faults

Network faults supports partition, net emulation (including loss, delay, duplicate, and corrupt), and bandwidth.

* Partition: network disconnection and partition.
* Net Emulation: poor network conditions, such as high delays, high packet loss rate, packet reordering, and so on.
* Bandwidth: limit the communication bandwidth between nodes.

## Before you start

* During the network injection process, make sure that the connection between Controller Manager and Chaos Daemon works, otherwise the NetworkChaos cannot be restored anymore.
* If you want to simulate Net Emulation fault, make sure the NET_SCH_NETEM module is installed in the Linux kernel. If you are using CentOS, you can install the module through the kernel-modules-extra package. Most other Linux distributions have installed the module already by default.

## Simulate fault injections by kbcli

### Network partition

Run the command below to make the Pod `mycluster-mysql-1` partitioned from both the outside network and the internal network of Kubernetes.

```bash
kbcli fault network partition mycluster-mysql-1
```

You can also add other flags to specifiy the pod-kill configuration.

📎 Table 1. kbcli fault network partition flags description

| Option                   | Description               |
| :----------------------- | :------------------------ |
| Pod name  | Add a pod name to make this pod in the default namespace unavailable. For example, <br /> `kbcli fault pod kill mysql-cluster-mysql-0` |
| `--target-mode` | It specifies the mode of the target. If a target is specified, the `target-mode` mode should be specified together. |
| `--target-label` | It specifies the label of the target. |
| `--duration` | It defines how long the partition lasts. |
| `-e`, `--external-target` | It specifies the network target outside Kubernetes. It can be IPv4 address or domain name. |
| `--target-ns-fault` | It specifies the namesapce of the target. |
| `--mode` | It specifies the experimental mode, that is, which Pods to experiment with. |
| `--target-mode` | It specifies the experimental mode, that is, which Pods to experiment with. |
| `--label` | It specifies the label of the Pod. |

//TODO: mode和target-mode的区别

### Network emulation

Net Emulation includes poor network conditions, such as high delays, high packet loss rate, packet reordering, and so on.

#### Loss

The command below injects a loss fault into a Pod and the packet loss rate is 50%.

```bash
kbcli fault network loss mycluster-mysql-1 -e=kubeblocks.io --loss=50
```

📎 Table 2. kbcli fault network loss flags description

| Option                   | Description               |
| :----------------------- | :------------------------ |
| Pod name  | Add a pod name to make this pod in the default namespace unavailable. For example, <br /> `kbcli fault pod kill mysql-cluster-mysql-0` |
| `--correlation` | It indicates the correlation between the probability of a packet error occurring and whether it occurred the previous time. Value range: [0, 100]. |
| `--target-label` | It specifies the label of the target. |
| `--target-mode` | It specifies the experimental mode, that is, which Pods to experiment with. |

#### Delay

The command below injects 15-second delay chaos to the network connection of the specified Pod.

```bash
kbcli fault network delay mycluster-mysql-1 --latency=15s -c=100 --jitter=0ms
```

📎 Table 3. kbcli fault network delay flags description

| Option                   | Description               |
| :----------------------- | :------------------------ |
| Pod name  | Add a pod name to make this pod in the default namespace unavailable. For example, <br /> `kbcli fault pod kill mysql-cluster-mysql-0` |
| `--latency` | It specifies the delay period. |
| `--correlation` | It indicates the correlation between the probability of a packet error occurring and whether it occurred the previous time. Value range: [0, 100]. |
| `--jitter` | It specifies the latency change range. |
| `--target-mode` | It specifies the experimental mode, that is, which Pods to experiment with. |
| `--target-label` | It specifies the label of the target. |

#### Duplicate

The command below injects duplicate chaos into the specified Pod and this experiment lasts for 1 minute and the duplicate rate is 50%.

```bash
kbcli fault network duplicate mysql-cluster-mysql-1 --duplicate=50
```

#### Corrupt

The command below injects corrupt chaos into the specifies Pod and this experiment lasts for 1 minute and the packet corrupt rate is 50%.

```bash
kbcli fault network corrupt mycluster-mysql-1 --corrupt=50 -correlation=100 --duration=1m
```

### Bandwidth

The command below sets the bandwidth between the specified Pod and the outside as 1 Kbps and this experiment lasts for 1 minute.

```bash
kbcli fault network bandwidth mycluster-mysql-1 --rate=1kbps --duration=1m
```

📎 Table 4. kbcli fault network bandwidth flags description

| Option                   | Description               |
| :----------------------- | :------------------------ |
| Pod name  | Add a pod name to make this pod in the default namespace unavailable. For example, <br /> `kbcli fault pod kill mysql-cluster-mysql-0` |
| `--rate` | It indicates the rate of bandwidth limit. |
| `--limit` | It indicates the number of bytes waiting in queue. |
| `--buffer` | It indicates the maximum number of bytes that can be sent instantaneously. |
| `--prakrate` | It indicates the maximum consumption of `bucket` (usually not set). |
| `--minburst` | It indicates the size of `peakrate bucket` (usually not set). |

## Simulate fault injections by YAML file

This section introduces the YAML configuration file examples. You can also refer to the [Chaos Mesh official docs](https://chaos-mesh.org/docs/next/simulate-network-chaos-on-kubernetes/#create-experiments-using-the-yaml-files) for details.

