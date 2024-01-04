---
title: 模拟 HTTP 故障
description: 模拟 HTTP 故障
sidebar_position: 6
sidebar_label: 模拟 HTTP 故障
---

# 模拟 HTTP 故障

HTTPChaos 实验用于模拟在 HTTP 请求和响应过程中发生故障的场景。目前，HTTPChaos 支持以下几种故障类型：

* Abort：中断请求和响应；
* Delay：为请求或响应过程注入延迟；
* Replace：替换 HTTP 请求或响应报文中的部分内容；
* Patch：在 HTTP 请求或响应报文中添加额外内容。

HTTPChaos 支持多种故障类型的组合。在创建 HTTPChaos 实验时，如果同时配置了多种 HTTP 故障类型，实验运行时注入故障的优先级（顺序）固定为 abort -> delay -> replace -> patch。其中 abort 故障会导致短路，直接中断此次连接。

## 开始之前

在注入 HTTPChaos 相关故障之前，请注意以下事项：

* 确保目标 Pod 上没有运行 Chaos Mesh 的 Control Manager。
* 默认情况下，相关命令将同时作用于 Pod 中的客户端和服务器。如果你不需要这种设置，请参考[官方文档](https://chaos-mesh.org/docs/simulate-http-chaos-on-kubernetes/#specify-side)。
* 确保目标服务已禁用 HTTPS 访问，因为 HTTPChaos 暂不支持注入 HTTPS 连接。
* 为使 HTTPChaos 故障注入生效，尽量避免复用客户端的 TCP socket，在注入故障前建立的 TCP socket 上进行 HTTP 请求不受 HTTPChaos 影响。
* 请在生产环境谨慎使用非幂等语义请求（例如大多数 POST 请求）。若使用了这类请求，故障注入后可能无法通过重复请求使目标服务恢复正常状态。

## 使用 kbcli 模拟故障注入

下表介绍所有 HTTP 故障类型的常见字段。

📎 Table 1. kbcli HTTP 故障参数说明

| 参数                   | 说明               | 默认值 | 是否必填 |
| :----------------------- | :------------------------ | :------------ | :------- |
| `--target` | 指定故障注入的目标过程为 `Request` 或 `Response`，需要同时配置与 target 相关的字段。 | `Request` | 否 |
| `--port` | 指定目标服务监听的 TCP 端口。| 80 | 否 |
| `--path` | 指定目标请求的 URL 路径，支持[通配符](https://www.wikiwand.com/en/Matching_wildcards)。 | * | 否 |
| `--method` | 指定目标请求的 HTTP method。 | `GET` | 否 |
| `--code` | 指定目标响应的状态码，仅当 `target=response` 时生效。 | 0 | 否 |

### Abort

执行以下命令，向指定的 Pod 中注入 abort 故障 1 分钟。

```bash
kbcli fault network http abort --duration=1m
```

### Delay

执行以下命令，向指定的 Pod 中注入 delay 故障 15 秒。

```bash
kbcli fault network http delay --delay=15s
```

### Replace

执行以下命令，替换 HTTP 请求或响应报文中的部分内容，持续 1 分钟。

```bash
kbcli fault network http replace --replace-method=PUT --duration=1m
```

### Patch

执行以下命令，在 HTTP 请求或响应报文中添加额外的内容。

```bash
kbcli fault network http patch --body='{"key":""}' --type=JSON --duration=30s
```

## 使用 YAML 文件模拟故障注入

本节介绍如何使用 YAML 文件模拟故障注入。你可以在上述 kbcli 命令的末尾添加 `--dry-run` 命令来查看 YAML 文件，还可以参考 [Chaos Mesh 官方文档](https://chaos-mesh.org/zh/docs/next/simulate-http-chaos-on-kubernetes/#使用-yaml-文件创建实验)获取更详细的信息。

### HTTP abort 示例

1. 将实验配置写入到 `http-abort.yaml` 文件中。

    在下例中，Chaos Mesh 将向指定的 Pod 中注入 abort 故障 1 分钟。

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

2. 使用 `kubectl` 创建实验。

   ```bash
   kubectl apply -f ./http-abort.yaml
   ```

### HTTP delay 示例

1. 将实验配置写入到 `http-delay.yaml` 文件中。

    在下例中，Chaos Mesh 将向指定的 Pod 中注入 delay 故障 15 秒。

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

2. 使用 `kubectl` 创建实验。

   ```bash
   kubectl apply -f ./http-delay.yaml
   ```

### HTTP replace 示例

1. 将实验配置写入到 `http-replace.yaml` 文件中。

    在下例中，Chaos Mesh 将替换 HTTP 请求或响应报文中的部分内容，持续 1 分钟。

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

2. 使用 `kubectl` 创建实验。

   ```bash
   kubectl apply -f ./http-replace.yaml
   ```

### HTTP patch 示例

1. 将实验配置写入到 `http-patch.yaml` 文件中。

    在下例中，Chaos Mesh 将在 HTTP 请求或响应报文中添加额外的内容。

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

2. 使用 `kubectl` 创建实验。

   ```bash
   kubectl apply -f ./http-patch.yaml
   ```