---
title: Simulate HTTP faults
description: Simulate HTTP faults
sidebar_position: 6
sidebar_label: Simulate HTTP faults
---

# Simulate HTTP faults

HTTPChaos experiments simulate the fault scenarios during the HTTP request and response processing. Currently, HTTPChaos supports simulating the following fault types:

* Abort: blocks requests and responses.
* Delay: injects latency into the request or response.
* Replace: replaces part of the content in HTTP request or response messages.
* Patch: adds additional content to the HTTP request or response messages.

HTTP faults support combining different fault types. If you have configured multiple HTTP fault types at the same time when creating HTTPChaos experiments, the order of injecting the faults follows abort -> delay -> replace -> patch. When the abort fault causes short circuits, the connection will be directly interrupted.

## Before you start

Before injecting the faults supported by HTTPChaos, make sure the following requirements are met:

* There is no control manager of Chaos Mesh running on the target Pod.
* The rules affect both clients and servers in the Pod. If you want to affect only one of them, refer to the [official specific side](https://chaos-mesh.org/docs/simulate-http-chaos-on-kubernetes/#specify-side) section.
* HTTPS access should be disabled because injecting HTTPS connections is not supported currently.
* To make HTTPChaos injection take effect, the client should avoid reusing TCP socket. This is because HTTPChaos does not affect the HTTP requests that are sent via TCP socket before the fault injection.
* Use non-idempotent requests (such as most of the POST requests) with caution in production environments. If such requests are used, the target service may not return to normal status by repeating requests after the fault injection.

## Simulate fault injections by kbcli

This table below describes the general flags for HTTP faults.

📎 Table 1. kbcli fault network http flags description

| Option                   | Description               | Default value | Required |
| :----------------------- | :------------------------ | :------------ | :------- |
| `--target` | It specifies whether the target of fault injection is `Request` or `Response`. The target-related fields should be configured at the same time. | Request | No |
| `--port` | It specifies the TCP port that the target service listens on. | 80 | No |
| `--path` | The URL path of the target request. Supports [Matching wildcards](https://www.wikiwand.com/en/Matching_wildcards). | * | No |
| `--method` | It specifies the HTTP method that the target requests. | `GET` | No |
| `--code` | It specifies the status code responded by the target. It is effective only when `target=response`. | 0 | No |

### Abort

The command below injects one-minute abort chaos to the specified Pod.

```bash
kbcli fault network http abort --duration=1m
```

### Delay

The command below injects a 15-second latency chaos to the specified Pod.

```bash
kbcli fault network http delay --delay=15s
```

### Replace

The command below replaces part of content in HTTP request or response messages for 1 minute.

```bash
kbcli fault network http replace --replace-method=PUT --duration=1m
```

### Patch

The command below adds additional contents to HTTP request or response messages.

```bash
kbcli fault network http patch --body='{"key":""}' --type=JSON --duration=30s
```

## Simulate fault injections by YAML file

This section introduces the YAML configuration file examples. You can view the YAML file by adding `--dry-run` at the end of the above kbcli commands. Meanwhile, you can also refer to the [Chaos Mesh official docs](https://chaos-mesh.org/docs/next/simulate-http-chaos-on-kubernetes/#create-experiments-using-yaml-files) for details.

### HTTP-abort example

1. Write the experiment configuration to the `http-abort.yaml` file.

    In the following example, Chaos Mesh injects 1-minute abort chaos to the specified Pod for 1 minute.

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: HTTPChaos
    metadata:
      creationTimestamp: null
      generateName: http-chaos-
      namespace: default
    spec:
      abort: true
      duration: 1m
      method: GET
      mode: all
      path: '*'
      port: 80
      selector:
        namespaces:
        - default
      target: Request
    ```

2. Run `kubectl` to start an experiment.

   ```bash
   kubectl apply -f ./http-abort.yaml
   ```

### HTTP-delay example


1. Write the experiment configuration to the `http-delay.yaml` file.

    In the following example, Chaos Mesh injects a 15-second latency chaos to the specified Pod.

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: HTTPChaos
    metadata:
      creationTimestamp: null
      generateName: http-chaos-
      namespace: default
    spec:
      delay: 15s
      duration: 10s
      method: GET
      mode: all
      path: '*'
      port: 80
      selector:
        namespaces:
        - default
      target: Request
    ```

2. Run `kubectl` to start an experiment.

   ```bash
   kubectl apply -f ./http-delay.yaml
   ```

### HTTP-replace example

1. Write the experiment configuration to the `http-replace.yaml` file.

    In the following example, Chaos Mesh replaces part of content in HTTP request or response messages for 1 minute.

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: HTTPChaos
    metadata:
      creationTimestamp: null
      generateName: http-chaos-
      namespace: default
    spec:
      duration: 1m
      method: GET
      mode: all
      path: '*'
      port: 80
      replace:
        method: PUT
      selector:
        namespaces:
        - default
      target: Request
    ```

2. Run `kubectl` to start an experiment.

   ```bash
   kubectl apply -f ./http-replace.yaml
   ```

### HTTP-patch example

1. Write the experiment configuration to the `http-patch.yaml` file.

    In the following example, Chaos Mesh adds additional contents to HTTP request or response messages.

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: HTTPChaos
    metadata:
      creationTimestamp: null
      generateName: http-chaos-
      namespace: default
    spec:
      duration: 30s
      method: GET
      mode: all
      patch:
        body:
          type: JSON
          value: '{"key":""}'
      path: '*'
      port: 80
      selector:
        namespaces:
        - default
      target: Request
    ```

2. Run `kubectl` to start an experiment.

   ```bash
   kubectl apply -f ./http-patch.yaml
   ```
