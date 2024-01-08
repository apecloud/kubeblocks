---
title: 简介
description: 故障注入简介
sidebar_position: 1
sidebar_label: 简介
---

# 简介

故障注入是验证数据产品的稳定性和可靠性的重要方式。Kubernetes 生态中有许多成熟的故障注入产品，如 Chaos Mesh 和 Litmus。KubeBlocks 直接将 [Chaos Mesh](https://chaos-mesh.org/) 集成为插件，通过 CLI 和 YAML 文件执行故障注入。

KubeBlocks 提供了两种类型的故障注入。

* 基本资源类型故障：
  * [PodChaos](./pod-faults.md)：模拟 Pod 故障，如 Pod 节点重启、Pod 持续不可用和容器重启等。
  * [NetworkChaos](./network-faults.md)：模拟网络故障，如网络延迟、网络丢包、包乱序和各种网络分区。
  * [DNSChaos](./dns-faults.md)：模拟 DNS 故障，如随机 DNS 域名和返回错误 IP 地址。
  * [HTTPChaos](./http-fault.md)：模拟 HTTP 通信故障，如 HTTP 通信延迟。
  * [StressChaos](./stress-fault.md)：模拟 CPU 或内存抢占。
  * [IOChaos](./io-faults.md)：模拟特定应用程序的文件 I/O 故障，如 I/O 延迟、读写失败。
  * [TimeChaos](./time-fault.md)：模拟时间跳动异常。

* 平台类型故障：
  * [AWSChaos](./aws-fault.md)：模拟 AWS 平台故障，如 AWS 实例停止、重启和卷卸载。
  * [GCPChaos](./gcp-fault.md)：模拟 GCP 平台故障，如 GCP 实例停止、重启和卷卸载。