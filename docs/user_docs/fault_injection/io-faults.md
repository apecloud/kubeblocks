---
title: Simulate I/O faults
description: Simulate I/O faults
sidebar_position: 7
sidebar_label: Simulate I/O faults
---

# Simulate I/O faults

IOChaos experiment can simulate file system faults. I/O fault injection currently supports latency, fault, attribure override, and mistake.

* latency: delays file system calls.
* fault: returns an error for filesystem calls.
* attrOverride: modifies file properties.
* mistake: makes the file read or write a wrong value.

## Before you start

* The I/O faults injection can be performed only on Linux.
* The experiment result can be seen inside the container and the volume mount path should be specified.
* It is recommended to perform I/O fault injection to write and read operations.

## Simulate fault injections by kbcli

This table below describes the general flags for I/O faults.

📎 Table 1. kbcli fault io flags description

| Option                   | Description               |
| :----------------------- | :------------------------ |
| `--volume-path` | The mount point of volume in the target container. Must be the root directory of the mount. |
| `--path` | The valid range of fault injections, either a wildcard or a single file. |
| `--percent` | Probability of failure per operation, in %. |
| `--container`, `-c` | Specifies the name of the container into which the fault is injected. |
| `--method` | Type of the file system call that requires injecting fault. `read` and `write` are supported. |

### Latency

The command below injects latency chaos into the directory `/data` to delay 10 seconds to display the file content, that is, delay the read operation.

```bash
kbcli fault io latency --delay=10s --volume-path=/data
```

### Fault

Common error number:

* 1: Operation not permitted
* 2: No such file or directory
* 5: I/O error
* 6: No such device or address
* 12: Out of memory
* 16: Device or resource busy
* 17: File exists
* 20: Not a directory
* 22: Invalid argument
* 24: Too many open files
* 28: No space left on device

You can find the full error number list [here](https://raw.githubusercontent.com/torvalds/linux/master/include/uapi/asm-generic/errno-base.h).

The command below inject an error in `/data`.

Chaos Mesh injects a file fault into the directory `/data`, which gives a 100% probability of failure in all file system operations under this directory and returns error code 22 (invalid argument).

```bash
kbcli fault io errno --volume-path=/data --errno=22
```

### Attribute override

Chaos Mesh injects an attrOverride fault into the `/data` directory, giving a 100% probability that all file system operations in this directory will change the target file permissions to 72 (110 in octal), which will allow files to be executed only by the owner and their group and not authorized to perform other actions.

```bash
kbcli fault io attribute --volume-path=/data --perm=72
```

You can use the following flags to modify attributes.

📎 Table 2. kbcli fault io attribute flags description

| Option                   | Description               | Default value | Reuiqred |
| :----------------------- | :------------------------ | :------------ | :------- |
| `--blocks` | Number of blcoks that a file uses. | None | No |
| `--ino` | ino number. | None | No |
| `--nlink` | Number of hard links. | None | No |
| `--perm` | File permissions in decimal. | None | No |
| `--size` | File size. | None | No |
| `--uid` | User ID of the owner. | None | No |
| `--gid` | Group ID of the owner. | None | No |

### Mistake

Chaos Mesh injects read and write faults into the directory `/data`, which gives a 10% probability of failure in the read and write operations under this directory. During this process, one random position with a maximum length of 10 bytes will be replaced with 0 bytes.

```bash
kbcli fault io mistake --volume-path=/data --filling=zero --max-occurrences=10 --max-length=1
```

📎 Table 3. kbcli fault io mistake flags description

| Option                   | Description               | Default value | Reuiqred |
| :----------------------- | :------------------------ | :------------ | :------- |
| `--filling` | The wrong data to be filled. Only zero (fill 0) or random (fill random bytes) are supported. | None | Yes |
| `max-occurrences` | Maximum number of errors in each operation. | None | Yes |
| `--max-length` | Maximum length of each error (in bytes). | None | Yes |

:::warning

It is suggested that you only use mistake on READ and WRITE file system calls. Using mistake on other file system calls may lead to unexpected consequences, including but not limited to file system damage and program crashes.

:::

## Simulate fault injections by YAML file

This section introduces the YAML configuration file examples. You can also refer to the [Chaos Mesh official docs](https://chaos-mesh.org/docs/next/simulate-io-chaos-on-kubernetes/#create-experiments-using-the-yaml-files) for details.

### Fault-latency example

1. Write the experiment configuration to the `fault-latency.yaml` file.

    In the following example, Chaos Mesh injects latency chaos into the directory `/data` to delay 10 seconds to display the file content, that is, delay the read operation.

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: IOChaos
    metadata:
      creationTimestamp: null
      generateName: io-chaos-
      namespace: default
    spec:
      action: latency
      delay: 10s
      duration: 10s
      mode: all
      percent: 100
      selector:
        namespaces:
        - default
      volumePath: /data
    ```

2. Run `kubectl` to start an experiment.

   ```bash
   kubectl apply -f ./fault-latency.yaml
   ```

### Fault-fault example

1. Write the experiment configuration to the `fault-fault.yaml` file.

    In the following example, Chaos Mesh injects a file fault into the directory `/data`, which gives a 100% probability of failure in all file system operations under this directory and returns error code 22 (invalid argument).

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: IOChaos
    metadata:
      creationTimestamp: null
      generateName: io-chaos-
      namespace: default
    spec:
      action: fault
      duration: 10s
      errno: 22
      mode: all
      percent: 100
      selector:
        namespaces:
        - default
      volumePath: /data
    ```

2. Run `kubectl` to start an experiment.

   ```bash
   kubectl apply -f ./fault-fault.yaml
   ```

### Fault-attrOverride example

1. Write the experiment configuration to the `fault-attrOverride.yaml` file.

    In the following example, Chaos Mesh injects an attrOverride fault into the `/data` directory, giving a 100% probability that all file system operations in this directory will change the target file permissions to 72 (110 in octal), which will allow files to be executed only by the owner and their group and not authorized to perform other actions.

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: IOChaos
    metadata:
      creationTimestamp: null
      generateName: io-chaos-
      namespace: default
    spec:
      action: attrOverride
      attr:
        perm: 72
      duration: 10s
      mode: all
      percent: 100
      selector:
        namespaces:
        - default
      volumePath: /data
    ```

2. Run `kubectl` to start an experiment.

   ```bash
   kubectl apply -f ./fault-attrOverride.yaml
   ```

### Fault-mistake example

1. Write the experiment configuration to the `fault-mistake.yaml` file.

    In the following example, Chaos Mesh injects read and write faults into the directory `/data`, which gives a 10% probability of failure in the read and write operations under this directory. During this process, one random position with a maximum length of 10 bytes will be replaced with 0 bytes.

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: IOChaos
    metadata:
      creationTimestamp: null
      generateName: io-chaos-
      namespace: default
    spec:
      action: mistake
      duration: 10s
      mistake:
        filling: zero
        maxLength: 1
        maxOccurrences: 10
      mode: all
      percent: 100
      selector:
        namespaces:
        - default
      volumePath: /data
    ```

2. Run `kubectl` to start an experiment.

   ```bash
   kubectl apply -f ./fault-mistake.yaml
   ```
