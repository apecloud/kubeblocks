---
title: Simulate pod faults
description: Simulate pod faults
sidebar_position: 3
sidebar_label: Simulate pod faults
---

# Simulate pod faults

Pod faults supports pod failure, pod kill, and container kill.

* Pod failure: injects fault into a specified Pod to make the Pod unavailable for a period of time.
* Pod kill: kills a specified Pod. To ensure that the Pod can be successfully restarted, you need to configure ReplicaSet or similar mechanisms.
* Container kill: kills the specified container in the target Pod.

## Usage restrictions

Chaos Mesh can inject PodChaos into any Pod, no matter whether the Pod is bound to Deployment, StatefulSet, DaemonSet, or other controllers. However, when you inject PodChaos into an independent Pod, some different situations might occur. For example, when you inject "pod-kill" chaos into an independent Pod, Chaos Mesh cannot guarantee that the application recovers from its failure.

## Before you start

* Make sure there is no Control Manager of Chaos Mesh running on the target Pod.
* If the fault type is Pod Kill, replicaSet or a similar mechanism is configured to ensure that Pod can restart automatically.

## Simulate fault injections by kbcli

### Pod-kill

Run the command below to start a pod-kill experiment to kill all pods in the default namespace.

```bash
kbcli fault pod kill
```

You can also add other flags to specifiy the pod-kill configuration.

📎 Table 1. kbcli fault pod kill flags description

| Option                  | Description              |
| :-----------------------| :------------------------|
| Pod name  | Add a pod name to make this pod in the default namespace unavailable. For example, <br /> `kbcli fault pod kill mysql-cluster-mysql-0` |
| `--ns-fault` | Specify a namespace to make all pods in this namespace unavailable. For example, <br /> `kbcli fault pod kill --ns-fault=kb-system` |
| `--node`   | Specify a node to make all pods on this node unavailable. For example, <br /> `kbcli fault pod kill --node=minikube-m02` |
| `--label`  | Specify a label to make the pod with this label in the default namespace unavailable. For example, <br /> `kbcli fault pod kill --label=app.kubernetes.io/component=mysql` |
| `--node-label` | Specify a node lable to make all pods on the node with this node lable unavailable. For example, <br /> `kbcli fault pod kill --node-label=kubernetes.io/arch=arm64` |
| `--mode` and `--value` | Combine these two flags to specify the range and pod amount for fault injection. For example, <br /> Make 50% of all pods unavailable. <br /> `kbcli fault pod kill --mode=fixed-percent --value=50` |

### Pod-failure

Run the command below to start a pod-failure experiment to make all pods in the default namespace unavailable for 10 seconds.

```bash
kbcli fault pod failure --duration=10s
```

You can also add other flags to specifiy the pod-kill configuration.

📎 Table 2. kbcli fault pod failure flags description

| Option                  | Description              |
| :-----------------------| :------------------------|
| Pod name  | 
| `--ns-fault` | Specify a namespace to make all pods in this  unavailable. For example, <br /> `kbcli fault pod failure --ns-fault=kb-system` |
| `--node`   | Specify a node to kill all pods on this node. For example, <br /> `kbcli fault pod failure --node=minikube-m02` |
| `--label`  | Specify a label to kill the pod with this label in the default namespace. For example, <br /> `kbcli fault pod failure --label=app.kubernetes.io/component=mysql` |
| `--node-label` | Specify a node lable to kill all pods on the node with this node lable. For example, <br /> `kbcli fault pod failure --node-label=kubernetes.io/arch=arm64` |
| `--mode` and `--value` | Combine these two flags to specify the pod amount for fault injection. For example, <br /> Kill 50% of all pods. <br /> `kbcli fault pod failure --mode=fixed-percent --value=50` |

### Container-kill

Run the command below to start a container-kill experiment to kill the container of all pods in the default namespace once. `--container` is required.

```bash
kbcli fault pod kill-container --container=mysql
```

You can also add multiple containers. For example, run the command below to kill the mysql and config-manager containers in the default namespace.

```bash
kbcli fault pod kill-container --container=mysql --container=config-manager
```

📎 Table 3. kbcli fault pod kill-container flags description

| Option                   | Description               |
| :----------------------- | :------------------------ |
| Pod name | Specify a pod name to kill the container of this pod in the default namespace. For example, <br /> `kbcli fault pod kill-container mysql-cluster-mysql-0 --container=mysql` |
| `--ns-fault` | Kill the container in the specifies namespace. For example, <br /> `kbcli fault pod kill-container --ns-fault=kb-system --container=mysql` |
| `--label`  | Kill the container with the specified label in the default namespace. For example, <br /> `kbcli fault pod kill-container --container=mysql --label=statefulset.kubernetes.io/pod-name=mysql-cluster-mysql-0` |

## Simulate fault injections by YAML

This section introduces the YAML configuration file examples. You can also refer to the [Chaos Mesh official docs](https://chaos-mesh.org/docs/next/simulate-pod-chaos-on-kubernetes/#create-experiments-using-yaml-configuration-files) for details.

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

This table describes the fileds in the YAML file.

| Parameter | Type  | Description | Default value | Required | Example |
| :---      | :---  | :---        | :---          | :---     | :---    |
| action | string | It specifies the fault type to inject. The supported types include `pod-failure`, `pod-kill`, and `container-kill`. | None | Yes | `pod-kill` |
| duration | string | It specifies the duration of the experiment. | None | Yes | 10s |
| mode | string | It specifies the mode of the experiment. The mode options include `one` (selecting a random Pod), `all` (selecting all eligible Pods), `fixed` (selecting a specified number of eligible Pods), `fixed-percent` (selecting a specified percentage of Pods from the eligible Pods), and `random-max-percent` (selecting the maximum percentage of Pods from the eligible Pods). | None | Yes | `fixed-percent` |
| value | string | It provides parameters for the `mode` configuration, depending on `mode`. For example, when `mode` is set to `fixed-percent`, `value` specifies the percentage of Pods. | None | No | 50 |
| selector | struct | It specifies the target Pod by defining node and labels.| None | Yes. <br /> If not specified, the system kills all pods under the default namespece. |  |
| containerNames | string | When you configure `action` to `container-kill`, this configuration is mandatory to specify the target container name for injecting faults. | None | No | mysql |
| gracePeriod | int64 | When you configure `action` to `pod-kill`, this configuration is mandatory to specify the duration before deleting Pod. | 0 | No | 0 |
| duration | string | Specifies the duration of the experiment. | None | Yes | 30s |
