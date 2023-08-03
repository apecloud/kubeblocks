---
title: Simulate time faults
description: Simulate time faults
sidebar_position: 9
sidebar_label: Simulate time faults
---

# Simulate time faults

You can use this experiment type to simulate a time offset scenario. This document describes how to create a TimeChaos experiment and its associated configuration file.

:::note

TimeChaos only affects the PID `1` process in the PID namespace of the container, and child processes of the PID `1`. For example, the process started by `kubectl exec` does not be affected.

:::

| Option                   | short | Description               | Default value | Required |
| :----------------------- | :------- | :------------------------ | :------------ | :------- |
| `--time-offset` | None | It specifies the length of time offset. | None | No |
| `--clock-id` | None | It specifies the ID of clock that will be offset. See the [clock_gettime documentation](https://man7.org/linux/man-pages/man2/clock_gettime.2.html) for details. | 20 | No |
| `--container` | -c | It specifies a container name for fault injection. | None | No |

## Simulate fault injections by kbcli

This experiment configuration shifts the time of the processes in the specified Pod forward by 5 seconds. Once this time fault is injected into the Pod, a failure occurs to this Pod and this Pod restarts.

```bash
kbcli fault time --time-offset=-5s
```

## Simulate fault injections by YAML file

This section introduces the YAML configuration file examples. You can also refer to the [Chaos Mesh official docs](https://chaos-mesh.org/docs/next/simulate-time-chaos-on-kubernetes/#create-experiments-using-the-yaml-file) for details.

1. Write the experiment configuration to the `time.yaml` file.

    In the following example, Chaos Mesh injects a time fault to shift the time of the processes in the specified Pod forward by 5 seconds. Once this time fault is injected into the Pod, a failure occurs to this Pod and this Pod restarts.

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: TimeChaos
    metadata:
      creationTimestamp: null
      generateName: time-chaos-
      namespace: default
    spec:
      duration: 10s
      mode: all
      selector:
        namespaces:
        - default
      timeOffset: -5s
    ```

2. Run `kubectl` to start an experiment.

   ```bash
   kubectl apply -f ./time.yaml
   ```

### Field description

The fields in the YAML configuration file are described in the following table:

| Parameter | Type | Note | Default value | Required | Example |
| :--- | :--- | :--- | :--- | :--- | :--- |
| timeOffset | string | Specifies the length of time offset. | None | Yes | `-5m` |
| clockIds | []string | Specifies the ID of clock that will be offset. See the [<clock>clock_gettime</clock> documentation](https://man7.org/linux/man-pages/man2/clock_gettime.2.html) for details. | `["CLOCK_REALTIME"]` | No | `["CLOCK_REALTIME", "CLOCK_MONOTONIC"]` |
| mode | string | Specifies the mode of the experiment. The mode options include `one` (selecting a random Pod), `all` (selecting all eligible Pods), `fixed` (selecting a specified number of eligible Pods), `fixed-percent` (selecting a specified percentage of Pods from the eligible Pods), and `random-max-percent` (selecting the maximum percentage of Pods from the eligible Pods). | None | Yes | `one` |
| value | string | Provides parameters for the `mode` configuration, depending on `mode`.For example, when `mode` is set to `fixed-percent`, `value` specifies the percentage of Pods. | None | No | 1 |
| containerNames | []string | Specifies the name of the container into which the fault is injected. | None | No | `["nginx"]` |
| selector | struct | Specifies the target Pod. For details, refer to [Define the experiment scope](./define-chaos-experiment-scope.md). | None | Yes |  |
