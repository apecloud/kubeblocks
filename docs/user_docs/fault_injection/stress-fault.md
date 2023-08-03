---
title: Simulate stress faults
description: Simulate stress faults
sidebar_position: 8
sidebar_label: Simulate stress faults
---

# Simulate stress faults

Chaos Mesh provides StressChaos experiments to simulate stress scenarios inside containers. This document describes how to create StressChaos experiments and how to prepare the corresponding configuration file.

| Option                   | Description               | Default value | Required |
| :----------------------- | :------------------------ | :------------ | :------- |
| `--cpu-worker` | It specifies the number of threads that exert CPU stress. One of `--cpu-worker` and `--memory-worker` must be specified. | None | No |
| `--cpu-load` | It specifies the percentage of CPU occupied. 0 means no extra load added, 100 means full load. The total load is workers * load. | 20 | No |
| `--memory-worker` | It specifies the number of threads that exert memory pressure. One of `--cpu-worker` and `--memory-worker` must be specified. | None | No |
| `--memory-size` | It specifies the size of the allocated memory or the percentage of the total memory, and the sum of the allocated memory is size. | None | No |
| `--container` | It specifies a container name and multiple containers can be specified. If not specified, it defaults to the first container in the Pod. | None | No |

## Simulate fault injections by kbcli

The command below creates a process in the first container of all pods in the default namespace, continuously allocate and read and write in memory, occupying up to 100MB of memory for 10 seconds. During this process, there are 2 threads that exert CPU stress and 1 thread that exerts memory stress.

```bash
kbcli fault stress --cpu-worker=2 --cpu-load=50 --memory-worker=1 --memory-size=100Mi
```

## Simulate fault injections by YAML file

This section introduces the YAML configuration file examples. You can also refer to the [Chaos Mesh official docs](https://chaos-mesh.org/docs/next/simulate-heavy-stress-on-kubernetes/#create-experiments-using-the-yaml-file) for details.

### Stress example

1. Write the experiment configuration to the `stress.yaml` file.

    In the following example, Chaos Mesh creates a process in the first container of all pods in the default namespace, continuously allocate and read and write in memory, occupying up to 100MB of memory for 10 seconds. During this process, there are 2 threads that exert CPU stress and 1 thread that exerts memory stress.

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: StressChaos
    metadata:
      creationTimestamp: null
      generateName: stress-chaos-
      namespace: default
    spec:
      duration: 10s
      mode: all
      selector:
        namespaces:
        - default
      stressors:
        cpu:
          load: 50
          workers: 2
        memory:
          size: 100Mi
          workers: 1
    ```

2. Run `kubectl` to start an experiment.

   ```bash
   kubectl apply -f ./stress.yaml
   ```
