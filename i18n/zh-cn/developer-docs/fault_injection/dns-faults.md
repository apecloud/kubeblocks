---
title: 模拟 DNS 故障
description: 模拟 DNS 故障
sidebar_position: 5
sidebar_label: 模拟 DNS 故障
---

# 模拟 DNS 故障

DNSChaos 用于模拟错误的 DNS 响应。例如，DNSChaos 可以在接收到 DNS 请求时返回错误，或随机的 IP 地址。

## 部署 Chaos DNS server

执行以下命令，检查 DNS 服务的状态是否正常：

```bash
kubectl get pods -n chaos-mesh -l app.kubernetes.io/component=chaos-dns-server
```

请确保 Pod 的状态为 `Running`。

## 注意事项

1. 目前 DNSChaos 只支持 DNS 记录类型 `A` 和 `AAAA`。

2. Chaos DNS 服务运行带有 [k8s_dns_chaos](https://github.com/chaos-mesh/k8s_dns_chaos) 插件的 CoreDNS。如果 Kubernetes 集群本身的 CoreDNS 服务包含一些特殊配置，请执行以下命令编辑 configMap `dns-server-config`，使 Chaos DNS 服务配置与 K8s CoreDNS 服务配置一致：

    ```bash
    kubectl edit configmap dns-server-config -n chaos-mesh
    ```

## 使用 kbcli 模拟故障注入

DNS 故障可分为 `random` 和 `error` 两种类型。你可以选择其中一种进行 DNS 故障注入。

`--pattern` 用于选择与故障匹配的域名模板，是必需的，支持使用占位符 `?` 和通配符 `*`。

### DNS random

执行以下命令，向默认命名空间中的所有 Pod 注入 DNS 故障。当发送 DNS 请求到指定的域名时，将返回一个随机的 IP 地址。

```bash
kbcli fault network dns random --patterns=google.com --duration=1m
```

### DNS error

执行以下命令，向默认命名空间中的所有 Pod 注入 DNS 故障。当发送 DNS 请求到指定的域名时，将返回一个错误。

```bash
kbcli fault network dns error --patterns=google.com --duration=1m
```

## 使用 YAML 文件模拟故障注入

本节介绍如何使用 YAML 文件模拟故障注入。你可以在上述 kbcli 命令的末尾添加 `--dry-run` 命令来查看 YAML 文件，还可以参考 [Chaos Mesh 官方文档](https://chaos-mesh.org/zh/docs/next/simulate-dns-chaos-on-kubernetes/#使用-yaml-方式创建实验)获取更详细的信息。

### DNS random 示例

1. 将实验配置写入到 `dns-random.yaml` 文件中。

    在下例中，Chaos Mesh 向默认命名空间中的所有 Pod 注入 DNS 故障。当发送 DNS 请求到指定的域名时，将返回一个 IP 地址。

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: DNSChaos
    metadata:
      creationTimestamp: null
      generateName: dns-chaos-
      namespace: default
    spec:
      action: random
      duration: 1m
      mode: all
      patterns:
      - google.com
      selector:
        namespaces:
        - default
    ```

2. 使用 `kubectl` 创建实验。

   ```bash
   kubectl apply -f ./dns-random.yaml
   ```

### DNS error 示例

1. 将实验配置写入到 `dns-error.yaml` 文件中。

    在下例中，Chaos Mesh 向默认命名空间中的所有 Pod 注入 DNS 故障。当发送 DNS 请求到指定的域名时，将返回一个错误。

    ```yaml
    apiVersion: chaos-mesh.org/v1alpha1
    kind: DNSChaos
    metadata:
      creationTimestamp: null
      generateName: dns-chaos-
      namespace: default
    spec:
      action: error
      duration: 1m
      mode: all
      patterns:
      - google.com
      selector:
        namespaces:
        - default
    ```

2. 使用 `kubectl` 创建实验。

   ```bash
   kubectl apply -f ./network-partition.yaml
   ```

### 字段说明

| 参数 | 类型 | 说明 | 默认值 | 是否必填 | 示例 |
| :-- | :-- | :-- | :-- | :-- | :-- |
| `action` | string | 定义 DNS 故障的行为，可选值为 `random` 或 `error`。当值为 `random` 时， DNS 服务返回随机的 IP 地址；当值为 `error` 时 DNS 服务返回错误。| 无 | 是 | `random` 或 `error` |
| `patterns` | 选择匹配故障行为的域名模版， 支持占位符 `?` 以及通配符 `*`。 | [] | 否 | `google.com`、`chaos-mesh.org`、`github.com` |
| `mode` | string | 指定实验的运行方式，可选项包括：`one`（表示随机选出一个符合条件的 Pod）、`all`（表示选出所有符合条件的 Pod）、`fixed`（表示选出指定数量且符合条件的 Pod）、`fixed-percent`（表示选出占符合条件的 Pod 中指定百分比的 Pod）和 `random-max-percent`（表示选出占符合条件的 Pod 中不超过指定百分比的 Pod）。 | 无 | 是 | `one` |
| `value` | string | 取决于 `mode` 的配置，为 `mode` 提供对应的参数。例如，当你将 `mode` 配置为 `fixed-percent` 时，`value` 用于指定 Pod 的百分比。 | 无 | 否 | `1` |
| `selector` | struct | 指定目标 Pod。| 无 | 是 |  |
