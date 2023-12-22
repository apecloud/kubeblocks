---
title: Simulate pod faults
description: Simulate pod faults
sidebar_position: 3
sidebar_label: Simulate pod faults
---

# Simulate pod faults

Pod faults support pod failure, pod kill, and container kill.

* Pod failure: injects fault into a specified Pod to make the Pod unavailable for a while.
* Pod kill: kills a specified Pod. To ensure that the Pod can be successfully restarted, you need to configure ReplicaSet or similar mechanisms.
* Container kill: kills the specified container in the target Pod.

## Usage restrictions

Chaos Mesh can inject PodChaos into any Pod, no matter whether the Pod is bound to Deployment, StatefulSet, DaemonSet, or other controllers. However, when you inject PodChaos into an independent Pod, some different situations might occur. For example, when you inject `pod-kill` chaos into an independent Pod, Chaos Mesh cannot guarantee that the application recovers from its failure.

## Before you start

* Make sure there is no Control Manager of Chaos Mesh running on the target Pod.
* If the fault type is `pod-kill`, ReplicaSet or a similar mechanism is configured to ensure that Pod can restart automatically.

## Simulate fault injections by kbcli

Common flags for all types of Pod faults.

📎 Table 1. Pod faults flags description

| Option                  | Description              | Default value | Required |
| :-----------------------| :------------------------| :------------ | :------- |
| `pod name`  | Specify the name of the Pod to inject the fault. For example, add the Pod name `mysql-cluster-mysql-0` to the command, and the complete command would be `kubectl fault pod kill mysql-cluster-mysql-0`. | Default | No |
| `--namespace` | It specifies the namespace where the Chaos is created. | Current namespace | No |
| `--ns-fault` | It specifies a namespace to make all Pods in this namespace unavailable. For example, <br /> `kbcli fault pod kill --ns-fault=kb-system` | Default | No |
| `--node`   | It specifies a node to make all Pods on this node unavailable. For example, <br /> `kbcli fault pod kill --node=minikube-m02` | None | No |
| `--label`  | It specifies a label to make the Pod with this label in the default namespace unavailable. For example, <br /> `kbcli fault pod kill --label=app.kubernetes.io/component=mysql` | None | No |
| `--node-label` | It specifies a node label to make all Pods on the node with this node label unavailable. For example, <br /> `kbcli fault pod kill --node-label=kubernetes.io/arch=arm64` | None | No |
| `--mode` | It specifies the mode of the experiment. The mode options include `one` (selecting a random Pod), `all` (selecting all eligible Pods), `fixed` (selecting a specified number of eligible Pods), `fixed-percent` (selecting a specified percentage of Pods from the eligible Pods), and `random-max-percent` (selecting the maximum percentage of Pods from the eligible Pods). | `all` | No |
| `--value` | It provides parameters for the `mode` configuration, depending on `mode`. For example, when `mode` is set to `fixed-percent`, `--value` specifies the percentage of Pods. For example,<br /> `kbcli fault pod kill --mode=fixed-percent --value=50` | None | No |
| `--duration` | It specifies how long the experiment lasts. | 10 seconds | No |

### Pod kill

Run the command below to inject `pod-kill` into all Pods in the default namespace and make the Pods unavailable for 30 seconds.

```bash
kbcli fault pod kill
```

### Pod failure

Run the command below to inject `pod-failure` into all Pods in the default namespace and make the Pods unavailable for 10 seconds.

```bash
kbcli fault pod failure --duration=10s
```

### Container kill

Run the command below to inject `container-kill` into the containers of all Pods in the default namespace once and make the containers unavailable for 10 seconds. `--container` is required.

```bash
kbcli fault pod kill-container --container=mysql
```

You can also add multiple containers. For example, run the command below to kill the `mysql` and `config-manager` containers in the default namespace.

```bash
kbcli fault pod kill-container --container=mysql --container=config-manager
```

## Simulate fault injections by YAML

This section introduces the YAML configuration file examples. You can view the YAML file by adding `--dry-run` at the end of the above kbcli commands. Meanwhile, you can also refer to the [Chaos Mesh official docs](https://chaos-mesh.org/docs/next/simulate-pod-chaos-on-kubernetes/#create-experiments-using-yaml-configuration-files) for details.

### Pod-kill example

1. Write the experiment configuration to the `pod-kill.yaml` file.

    In the following example, Chaos Mesh injects `pod-kill` into the specified Pod and kills the Pod once.

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: PodChaos
    metadata:
      creationTimestamp: null
      generateName: pod-chaos-
      namespace: default
    spec:
      action: pod-kill
      duration: 10s
      mode: fixed-percent
      selector:
        namespaces:
        - default
        labelSelectors:
        'app.kubernetes.io/component': 'mysql'
      value: "50"
    ```

2. Run `kubectl` to start an experiment.

   ```bash
   kubectl apply -f ./pod-kill.yaml
   ```

### Pod-failure example

1. Write the experiment configuration to the `pod-kill.yaml` file.

    In the following example, Chaos Mesh injects `pod-kill` into the specified Pod and kills the Pod once.

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: PodChaos
    metadata:
      creationTimestamp: null
      generateName: pod-chaos-
      namespace: default
    spec:
      action: pod-kill
      duration: 10s
      mode: fixed-percent
      selector:
        namespaces:
        - default
        labelSelectors:
        'app.kubernetes.io/component': 'mysql'
      value: "50"
    ```

2. Run `kubectl` to start an experiment.

   ```bash
   kubectl apply -f ./pod-kill.yaml
   ```

### Container-kill example

1. Write the experiment configuration to the `pod-kill.yaml` file.

    In the following example, Chaos Mesh injects `pod-kill` into the specified Pod and kills the Pod once.

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: PodChaos
    metadata:
      creationTimestamp: null
      generateName: pod-chaos-
      namespace: default
    spec:
      action: pod-kill
      duration: 10s
      mode: fixed-percent
      selector:
        namespaces:
        - default
        labelSelectors:
        'app.kubernetes.io/component': 'mysql'
      value: "50"
    ```

2. Run `kubectl` to start an experiment.

   ```bash
   kubectl apply -f ./pod-kill.yaml
   ```

### Field description

This table describes the fields in the YAML file.

| Parameter | Type  | Description | Default value | Required | Example |
| :---      | :---  | :---        | :---          | :---     | :---    |
| action | string | It specifies the fault type to inject. The supported types include `pod-failure`, `pod-kill`, and `container-kill`. | None | Yes | `pod-kill` |
| duration | string | It specifies the duration of the experiment. | None | Yes | 10s |
| mode | string | It specifies the mode of the experiment. The mode options include `one` (selecting a random Pod), `all` (selecting all eligible Pods), `fixed` (selecting a specified number of eligible Pods), `fixed-percent` (selecting a specified percentage of Pods from the eligible Pods), and `random-max-percent` (selecting the maximum percentage of Pods from the eligible Pods). | None | Yes | `fixed-percent` |
| value | string | It provides parameters for the `mode` configuration, depending on `mode`. For example, when `mode` is set to `fixed-percent`, `value` specifies the percentage of Pods. | None | No | 50 |
| selector | struct | It specifies the target Pod by defining node and labels.| None | Yes. <br /> If not specified, the system kills all pods under the default namespece. |  |
| containerNames | string | When you configure `action` to `container-kill`, this configuration is mandatory to specify the target container name for injecting faults. | None | No | mysql |
| gracePeriod | int64 | When you configure `action` to `pod-kill`, this configuration is mandatory to specify the duration before deleting Pod. | 0 | No | 0 |
| duration | string | It specifies the duration of the experiment. | None | Yes | 30s |
