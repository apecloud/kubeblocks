---
title: Simulate HTTP faults
description: Simulate HTTP faults
sidebar_position: 6
sidebar_label: Simulate HTTP faults
---

# Simulate HTTP faults

HTTP chaos experiments simulate the fault scenarios during the HTTP request and response processing. Currently, HTTPChaos supports simulating the following fault types:

* abort: interrupts the connection
* delay: injects latency into the request or response
* replace: replaces part of content in HTTP request or response messages
* patch: adds additional content to HTTP request or response messages

HTTP faults support combinations of different fault types. If you have configured multiple HTTP fault types at the same time when creating HTTPChaos experiments, the order set to inject the faults when the experiments start running is abort -> delay -> replace -> patch. When the abort fault cause short circuits, the connection will be directly interrupted.

## Notes

Before injecting the faults supported by HTTPChaos, note the followings:

* There is no control manager of Chaos Mesh running on the target Pod.
* The rules will affect both of clients and servers in the Pod, if you want to affect only one side, please refer to the specify side section.
* HTTPS accesses should be disabled, because injecting HTTPS connections is not supported currently.
* For HTTPChaos injection to take effect, the client should avoid reusing TCP socket. This is because HTTPChaos does not affect the HTTP requests that are sent via TCP socket before the fault injection.
* Use non-idempotent requests (such as most of the POST requests) with caution in production environments. If such requests are used, the target service may not return to normal status by repeating requests after the fault injection.

## Simulate fault injections by kbcli

This table below describes the general flags for network faults.

📎 Table 1. kbcli fault network http flags description

| Option                   | Description               |
| :----------------------- | :------------------------ |
| `--port` | It specifies the inject port and the default one is 80. |
| `--method` | It specifies the inject method and the default method is `GET`. |
| `--target` | It specifies whether the target of fault injuection is Request or Response. The target-related fields should be configured at the same time. |
| `--path` | The URI path of the target request. Supports [Matching wildcards](https://www.wikiwand.com/en/Matching_wildcards). |
| `--code` | Specifies the status code responded by target. It is effective only when `target=response`. |

### abort

The command below injects 1-minute abort chaos to the specified Pod for 1 minute.

```bash
kbcli fault network http abort --duration=1m
```

### delay

The command below injects latency chaos to the specified Pod for 15 seconds.

```bash
kbcli fault network http delay --delay=15s
```

### replace

The command below replace part of content in HTTP request or response messages for 1 minute.

```bash
kbcli fault network http replace --replace-method=PUT --duration=1m
```

### patch

The command below adds additional content to HTTP request or response messages.

```bash
kbcli fault network http patch --body='{"key":""}' --type=JSON --duration=30s
```

## Simulate fault injections by YAML file
