---
title: Simulate network faults
description: Simulate network faults
sidebar_position: 4
sidebar_label: Simulate network faults
---

# Simulate network faults

Network faults support partition, net emulation (including loss, delay, duplicate, and corrupt), and bandwidth.

* Partition: injects network disconnection and partition.
* Net emulation: simulates poor network conditions, such as high delays, high packet loss rate, packet reordering, and so on.
* Bandwidth: limits the communication bandwidth between nodes.

## Before you start

* During the network injection process, make sure that the connection between Controller Manager and Chaos Daemon works. Otherwise, the NetworkChaos cannot be restored anymore.
* If you want to simulate Net emulation fault, make sure the `NET_SCH_NETEM` module is installed in the Linux kernel. If you are using CentOS, you can install the module through the kernel-modules-extra package. Most other Linux distributions have installed the module already by default.

## Simulate fault injections by kbcli

Common flags for all types of network faults.

📎 Table 1. kbcli fault network partition flags description

| Option                   | Description               | Default value | Required |
| :----------------------- | :------------------------ | :------------ | :------- |
| `pod name`  | Specify the name of the Pod to inject the fault. For example, add the Pod name `mysql-cluster-mysql-0` to the command, and the complete command would be `kubectl fault pod kill mysql-cluster-mysql-0`.  | Default | No |
| `--direction` | It indicates the direction of target packets. Available values include `from` (the packets from target), `to` (the packets to target), and `both` ( the packets from or to target). | `to` | No |
| `-e`,`--external-target` | It indicates the network targets outside Kubernetes, which can be IPv4 addresses or domain names. This parameter only works with `direction: to`. | None | No |
| `--target-mode` | It specifies the mode of the target. If a target is specified, the `target-mode` mode should be specified together. `one` (selecting a random Pod), `all` (selecting all eligible Pods), `fixed` (selecting a specified number of eligible Pods), `fixed-percent` (selecting a specified percentage of Pods from the eligible Pods), and `random-max-percent` (selecting the maximum percentage of Pods from the eligible Pods) are selectable. | None | No |
| `--target-value` | It specifies the value of the target. | None | No |
| `--target-label` | It specifies the label of the target. | None | No |
| `--duration` | It defines how long the partition lasts. | None | No |
| `--target-ns-fault` | It specifies the namespace of the target. | None | No |

### Network partition

Run the command below to inject `network-partition` into the Pod and to make the Pod `mycluster-mysql-1` partitioned from both the outside network and the internal network of Kubernetes.

```bash
kbcli fault network partition mycluster-mysql-1
```

### Network emulation

Net Emulation includes poor network conditions, such as high delays, high packet loss rate, packet reordering, and so on.

#### Loss

The command below injects `network-loss` into the Pod `mycluster-mysql-1` and the packet loss rate is 50%.

```bash
kbcli fault network loss mycluster-mysql-1 -e=kubeblocks.io --loss=50
```

📎 Table 2. kbcli fault network loss flags description

| Option                   | Description               | Default value | Required |
| :----------------------- | :------------------------ | :------------ | :------- |
| `--loss` | It specifies the rate of packet loss. | None | Yes |
| `-c`, `--correlation` | It indicates the correlation between the probability of a packet error occurring and whether it occurred the previous time. Value range: [0, 100]. | None | No |

#### Delay

The command below injects `network-delay` into the Pod `mycluster-mysql-1` and causes a 15-second delay to the network connection of the specified Pod.

```bash
kbcli fault network delay mycluster-mysql-1 --latency=15s -c=100 --jitter=0ms
```

📎 Table 3. kbcli fault network delay flags description

| Option                   | Description               | Default value | Required |
| :----------------------- | :------------------------ | :------------ | :------- |
| `--latency` | It specifies the delay period.         | None          | Yes      |
| `--jitter` | It specifies the latency change range.  | 0 ms          | No       |
| `-c`, `--correlation` | It indicates the correlation between the probability of a packet error occurring and whether it occurred the previous time. Value range: [0, 100]. | None | No |

#### Duplicate

The command below injects duplicate chaos into the specified Pod and this experiment lasts for 1 minute and the duplicate rate is 50%.

`--duplicate` specifies the rate of duplicate packets and the value range is [0,100].

```bash
kbcli fault network duplicate mysql-cluster-mysql-1 --duplicate=50
```

📎 Table 4. kbcli fault network duplicate flags description

| Option                   | Description               | Default value | Required |
| :----------------------- | :------------------------ | :------------ | :------- |
| `--duplicate`         | It indicates the probability of a packet being duplicated. Value range: [0, 100]. | None | Yes |
| `-c`, `--correlation` | It indicates the correlation between the probability of a packet error occurring and whether it occurred the previous time. Value range: [0, 100]. | None | No |

#### Corrupt

The command below injects corrupt chaos into the specified Pod and this experiment lasts for 1 minute and the packet corrupt rate is 50%.

```bash
kbcli fault network corrupt mycluster-mysql-1 --corrupt=50 --correlation=100 --duration=1m
```

### Bandwidth

The command below sets the bandwidth between the specified Pod and the outside environment as 1 Kbps and this experiment lasts for 1 minute.

```bash
kbcli fault network bandwidth mycluster-mysql-1 --rate=1kbps --duration=1m
```

📎 Table 4. kbcli fault network bandwidth flags description

| Option                   | Description               | Default value | Required |
| :----------------------- | :------------------------ | :------------ | :------- |
| `--rate` | It indicates the rate of bandwidth limit. | None | Yes |
| `--limit` | It indicates the number of bytes waiting in the queue. | 1 | No |
| `--buffer` | It indicates the maximum number of bytes that can be sent instantaneously. | 1 | No |
| `--prakrate` | It indicates the maximum consumption rate of `bucket`. | 0 | No |
| `--minburst` | It indicates the size of `peakrate bucket`. | 0 | No |

## Simulate fault injections by YAML file

This section introduces the YAML configuration file examples. You can view the YAML file by adding `--dry-run` at the end of the above kbcli commands. Meanwhile, you can also refer to the [Chaos Mesh official docs](https://chaos-mesh.org/docs/next/simulate-network-chaos-on-kubernetes/#create-experiments-using-the-yaml-files) for details.

### Network-partition example

1. Write the experiment configuration to the `network-partition.yaml` file.

    In the following example, Chaos Mesh injects `network-partition` into the Pod and to make the Pod `mycluster-mysql-1` partitioned from both the outside network and the internal network of Kubernetes.

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: NetworkChaos
    metadata:
      creationTimestamp: null
      generateName: network-chaos-
      namespace: default
    spec:
      action: partition
      direction: to
      duration: 10s
      mode: all
      selector:
        namespaces:
        - default
        pods:
          default:
          - mycluster-mysql-1
    ```

2. Run `kubectl` to start an experiment.

   ```bash
   kubectl apply -f ./network-partition.yaml
   ```

### Network-emulation example

#### Network-loss example

1. Write the experiment configuration to the `network-loss.yaml` file.

    In the following example, Chaos Mesh injects `network-loss` into the Pod `mycluster-mysql-1` and the packet loss rate is 50%.

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: NetworkChaos
    metadata:
      creationTimestamp: null
      generateName: network-chaos-
      namespace: default
    spec:
      action: loss
      direction: to
      duration: 10s
      externalTargets:
      - kubeblocks.io
      loss:
        loss: "50"
      mode: all
      selector:
        namespaces:
        - default
        pods:
          default:
          - mycluster-mysql-1
    ```

2. Run `kubectl` to start an experiment.

   ```bash
   kubectl apply -f ./network-loss.yaml
   ```

#### Network-delay example

1. Write the experiment configuration to the `network-delay.yaml` file.

    In the following example, Chaos Mesh injects `network-delay` into the Pod `mycluster-mysql-1` and causes a 15-second delay to the network connection of the specified Pod.

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: NetworkChaos
    metadata:
      creationTimestamp: null
      generateName: network-chaos-
      namespace: default
    spec:
      action: delay
      delay:
        correlation: "100"
        jitter: 0ms
        latency: 15s
      direction: to
      duration: 10s
      mode: all
      selector:
        namespaces:
        - default
        pods:
          default:
          - mycluster-mysql-1
    ```

2. Run `kubectl` to start an experiment.

   ```bash
   kubectl apply -f ./network-delay.yaml
   ```

#### Network-duplicate example

1. Write the experiment configuration to the `network-duplicate.yaml` file.

    In the following example, Chaos Mesh injects duplicate chaos into the specified Pod and this experiment lasts for 1 minute and the duplicate rate is 50%.

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: NetworkChaos
    metadata:
      creationTimestamp: null
      generateName: network-chaos-
      namespace: default
    spec:
      action: duplicate
      direction: to
      duplicate:
        duplicate: "50"
      duration: 10s
      mode: all
      selector:
        namespaces:
        - default
        pods:
          default:
          - mysql-cluster-mysql-1
    ```

2. Run `kubectl` to start an experiment.

   ```bash
   kubectl apply -f ./network-duplicate.yaml
   ```

#### Network-corrupt example

1. Write the experiment configuration to the `network-corrupt.yaml` file.

    In the following example, Chaos Mesh injects corrupt chaos into the specified Pod and this experiment lasts for 1 minute and the packet corrupt rate is 50%.


    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: NetworkChaos
    metadata:
      creationTimestamp: null
      generateName: network-chaos-
      namespace: default
    spec:
      action: corrupt
      corrupt:
        correlation: "100"
        corrupt: "50"
      direction: to
      duration: 1m
      mode: all
      selector:
        namespaces:
        - default
        pods:
        default:
        - mycluster-mysql-1
    ```

2. Run `kubectl` to start an experiment.

   ```bash
   kubectl apply -f ./network-corrupt.yaml
   ```

### Bandwidth example

1. Write the experiment configuration to the `network-bandwidth.yaml` file.

    In the following example, Chaos Mesh sets the bandwidth between the specified Pod and the outside as 1 Kbps and this experiment lasts for 1 minute.

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: NetworkChaos
    metadata:
      creationTimestamp: null
      generateName: network-chaos-
      namespace: default
    spec:
      action: bandwidth
      bandwidth:
        buffer: 1
        limit: 1
        rate: 1kbps
      direction: to
      duration: 1m
      mode: all
      selector:
        namespaces:
        - default
        pods:
          default:
          - mycluster-mysql-1
    ```

2. Run `kubectl` to start an experiment.

   ```bash
   kubectl apply -f ./network-bandwidth.yaml
   ```

### Field description

This table describes the fields in the YAML file.

| Parameter | Type  | Description | Default value | Required | Example |
| :---      | :---  | :---        | :---          | :---     | :---    |
| action | string | It specifies the fault type to inject. The supported types include `partition`、`loss`、`delay`、`duplicate`、`corrupt` and `bandwidth`。| None | Yes | `bandwidth` |
| duration | string | It specifies the duration of the experiment. | None | Yes | 10s |
| mode | string | It specifies the mode of the experiment. The mode options include `one` (selecting a random Pod), `all` (selecting all eligible Pods), `fixed` (selecting a specified number of eligible Pods), `fixed-percent` (selecting a specified percentage of Pods from the eligible Pods), and `random-max-percent` (selecting the maximum percentage of Pods from the eligible Pods). | None | Yes | `fixed-percent` |
| value | string | It provides parameters for the `mode` configuration, depending on `mode`. For example, when `mode` is set to `fixed-percent`, `value` specifies the percentage of Pods. | None | No | 50 |
| selector | struct | It specifies the target Pod by defining node and labels.| None | Yes. <br /> If not specified, the system kills all pods under the default namespece. |
| duration | string | It specifies the duration of the experiment. | None | Yes | 30s |
