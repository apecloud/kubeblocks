---
title: 环境变量和占位符
description: KubeBlocks 的环境变量和占位符
keywords: [环境变量, 占位符]
sidebar_position: 10
sidebar_label: 环境变量和占位符
---

# 环境变量和占位符

## 环境变量

### Pod 环境变量

以下变量由 KubeBlocks 注入到每个 Pod 中。

| 名称 | 说明 |
| :--- | :---------- |
| `KB_POD_NAME` | K8s Pod 名称 |
| `KB_NAMESPACE` | K8s Pod 命名空间 |
| `KB_SA_NAME` | KubeBlocks 服务账号名称 |
| `KB_NODENAME` | K8s 节点名称 |
| `KB_HOSTIP` | K8s 主机 IP 地址 |
| `KB_PODIP` | K8s Pod IP 地址 |
| `KB_PODIPS` | K8s Pod IP 地址 |
| `KB_POD_UID` | POD UID (`pod.metadata.uid`) |
| `KB_CLUSTER_NAME` | KubeBlocks 集群 API 对象名称 |
| `KB_COMP_NAME` | 运行中 Pod 的 KubeBlocks 集群 API 对象的 `.spec.components.name`。 |
| `KB_CLUSTER_COMP_NAME` | 运行中 Pod 的 KubeBlocks 集群 API 对象的 `<.metadata.name>-<.spec.components.name>`。 |
| `KB_REPLICA_COUNT` | 运行中 Pod 的组件副本数 |
| `KB_CLUSTER_UID` | 运行中 Pod 的 KubeBlocks 集群 API 对象的 `metadata.uid`。 |
| `KB_CLUSTER_UID_POSTFIX_8` | `KB_CLUSTER_UID` 的最后八位数字 |
| `KB_{ordinal}_HOSTNAME` | 运行中 Pod 的主机名，其中 `{ordinal}` 是 Pod 的序号。<br />如果工作负载类型为无状态（Stateless），则不适用。 |
| `KB_POD_FQDN` | 运行中 Pod 的完全限定域名（FQDN）。<br />如果工作负载类型为无状态（Stateless），则不适用。|

## 占位符

### ComponentValueFrom API

| 名称 | 说明 |
| :--- | :---------- |
| `POD_ORDINAL` | Pod 的序号 |
| `POD_FQDN` | Pod 的完全限定域名（FQDN） |
| `POD_NAME` | Pod 的名称 |

### ConnectionCredential API

| 名称 | 说明 |
| :--- | :---------- |
| `UUID` | 生成一个随机的 UUID v4 字符串 |
| `UUID_B64` | 生成一个随机的 UUID v4 BASE64 编码的字符串 |
| `UUID_STR_B64` | 生成一个随机的 UUID v4 字符串，然后进行 BASE64 编码 |
| `UUID_HEX` | 生成一个随机的 UUID v4 的 HEX 表示 |
| `HEADLESS_SVC_FQDN` | 无头服务的 FQDN 占位符。值为 `- $(CLUSTER_NAME)-$(1ST_COMP_NAME)-headless.$(NAMESPACE).svc`，其中 1ST_COMP_NAME 是提供 `ClusterDefinition.spec.componentDefs[].service` 属性的第一个组件。|
| `SVC_FQDN` | 服务的 FQDN 占位符。值为 `- $(CLUSTER_NAME)-$(1ST_COMP_NAME).$(NAMESPACE).svc`，其中 1ST_COMP_NAME 是提供 `ClusterDefinition.spec.componentDefs[].service` 属性的第一个组件。 |
| `SVC_PORT_{PORT_NAME}` | 具有指定端口名称的 ServicePort 的端口值。例如，在一个 servicePort 的 JSON struct：<br /> `{"name": "mysql", "targetPort": "mysqlContainerPort", "port": 3306}` 中，连接凭证值中的 `"$(SVC_PORT_mysql)"` 为 3306。 |
| `RANDOM_PASSWD` | 随机生成的 8 个字符 |
