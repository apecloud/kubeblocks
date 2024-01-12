---
title: Simulate I/O faults
description: Simulate I/O faults
sidebar_position: 7
sidebar_label: Simulate I/O faults
---

# Simulate I/O faults

IOChaos experiment can simulate file system faults. I/O fault injection currently supports latency, fault, attribute override, and mistake.

* Latency: delays file system calls.
* Fault: returns an error for filesystem calls.
* AttrOverride: modifies file properties.
* Mistake: makes the file read or write a wrong value.

## Before you start

* The I/O faults injection can be performed only on Linux.
* The experiment result can be seen inside the container and the volume mount path should be specified.
* It is recommended to perform I/O fault injection to write and read operations.

## Simulate fault injections by kbcli

This table below describes the general flags for I/O faults.

📎 Table 1. kbcli fault io flags description

| Option                   | Description               | Default value | Required |
| :----------------------- | :------------------------ | :------------ | :------- |
| `--volume-path` | It specifies the mount point of volume in the target container. It must be the root directory of the mount. | None | Yes |
| `--path` | It specifies the valid range of fault injections. It can be either a wildcard or a single file. | * | No |
| `--percent` | It specifies the probability of failure per operation and its unit is %. | 100 | No |
| `--container`, `-c` | It specifies the name of the container into which the fault is injected. | None | No |
| `--method` | It specifies the I/O operation. `read` and `write` are supported. | * | No |

### Latency

The command below injects latency chaos into the directory `/data` to delay 10 seconds to display the file content, that is, delay the read operation.

`--delay` specifies the delay time and it is required.

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

The command below injects a file fault into the directory `/data`, which gives a 100% probability of failure in all file system operations under this directory and returns error code 22 (invalid argument).

`--errno` specifies the error number that the system returns and it is required.

```bash
kbcli fault io errno --volume-path=/data --errno=22
```

### Attribute override

The command below injects an attrOverride fault into the `/data` directory, giving a 100% probability that all file system operations in this directory will change the target file permissions to 72 (110 in octal), which will allow files to be executed only by the owner and their group and not authorized to perform other actions.

```bash
kbcli fault io attribute --volume-path=/data --perm=72
```

You can use the following flags to modify attributes.

📎 Table 2. kbcli fault io attribute flags description

| Option                   | Description               | Default value | Required |
| :----------------------- | :------------------------ | :------------ | :------- |
| `--blocks` | Number of blocks that a file uses. | None | No |
| `--ino` | ino number. | None | No |
| `--nlink` | Number of hard links. | None | No |
| `--perm` | File permissions in decimal. | None | No |
| `--size` | File size. | None | No |
| `--uid` | User ID of the owner. | None | No |
| `--gid` | Group ID of the owner. | None | No |

### Mistake

The command below injects read and write faults into the directory `/data`, which gives a 10% probability of failure in the read and write operations under this directory. During this process, one random position with a maximum length of 10 bytes will be replaced with 0 bytes.

```bash
kbcli fault io mistake --volume-path=/data --filling=zero --max-occurrences=10 --max-length=1
```

📎 Table 3. kbcli fault io mistake flags description

| Option                   | Description               | Default value | Required |
| :----------------------- | :------------------------ | :------------ | :------- |
| `--filling` | The wrong data to be filled. Only zero (fill 0) or random (fill random bytes) are supported. | None | Yes |
| `max-occurrences` | Maximum number of errors in each operation. | None | Yes |
| `--max-length` | Maximum length of each error (in bytes). | None | Yes |

:::warning

It is suggested that you only use mistake on READ and WRITE file system calls. Using mistake on other file system calls may lead to unexpected consequences, including but not limited to file system damage and program crashes.

:::

## Simulate fault injections by YAML file

This section introduces the YAML configuration file examples. You can view the YAML file by adding `--dry-run` at the end of the above kbcli commands. Meanwhile, you can also refer to the [Chaos Mesh official docs](https://chaos-mesh.org/docs/next/simulate-io-chaos-on-kubernetes/#create-experiments-using-the-yaml-files) for details.

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
