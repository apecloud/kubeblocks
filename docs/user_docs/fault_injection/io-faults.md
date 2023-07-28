---
title: Simulate IO faults
description: Simulate IO faults
sidebar_position: 7
sidebar_label: Simulate IO faults
---

# Simulate I/O faults

I/O fault injection currently supports latency, fault, attribure override, and mistake.

* latency: delays file system calls
* fault: returns an error for filesystem calls
* attrOverride: modifies file properties
* mistake: makes the file read or write a wrong value

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
| `--method` | Type of the file system call that requires injecting fault. |

### Latency

The command below injects latency chaos to delay 10 seconds to display the file content, that is, delay the read operation.

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

```bash
kbcli fault io errno --volume-path=/data --errno=22
```

### Attribute override

```bash
kbcli fault io attribute --volume-path=/data --perm=72
```

### Mistake

```bash
kbcli fault io mistake --volume-path=/data --filling=zero --max-occurrences=10 --max-length=1
```
