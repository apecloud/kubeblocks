---
title: Environment variables and placeholders
description: KubeBlocks Environment Variables and Placeholders
keywords: [environment variables, placeholders]
sidebar_position: 10
sidebar_label: Environment variables and placeholders
---

# Environment variables and placeholders

## Environment variables

### Automatic pod's container environment variables

The following variables are injected by KubeBlocks into each pod.

| Name | Description |
| :--- | :---------- |
| KB_POD_NAME | K8s Pod Name |
| KB_NAMESPACE | K8s Pod Namespace |
| KB_SA_NAME | KubeBlocks Service Account Name |
| KB_NODENAME | K8s Node Name |
| KB_HOSTIP | K8s Host IP address |
| KB_PODIP | K8s Pod IP address |
| KB_PODIPS | K8s Pod IP addresses |
| KB_POD_UID | POD UID (`pod.metadata.uid`) |
| KB_CLUSTER_NAME | KubeBlocks Cluster API object name |
| KB_COMP_NAME | Running pod's KubeBlocks Cluster API object's `.spec.components.name` |
| KB_CLUSTER_COMP_NAME | Running pod's KubeBlocks Cluster API object's `<.metadata.name>-<.spec.components.name>` |
| KB_REPLICA_COUNT | Running pod's component's replica |
| KB_CLUSTER_UID | Running pods' KubeBlocks Cluster API object's `metadata.uid` |
| KB_CLUSTER_UID_POSTFIX_8 | Last eight digits of KB_CLUSTER_UID |
| KB_{ordinal}_HOSTNAME | Running pod's hostname, where `{ordinal}` is the ordinal of pod. <br /> N/A if workloadType=Stateless. |
| KB_POD_FQDN | Running pod's fully qualified domain name (FQDN). <br /> N/A if workloadType=Stateless. |

## Built-in Place-holders

### ComponentValueFrom API

| Name | Description |
| :--- | :---------- |
| POD_ORDINAL | Pod ordinal |
| POD_FQDN | Pod FQDN (fully qualified domain name) |
| POD_NAME | Pod Name |

### ConnectionCredential API

| Name | Description |
| :--- | :---------- |
| UUID | Generate a random UUID v4 string. |
| UUID_B64 | Generate a random UUID v4 BASE64 encoded string. |
| UUID_STR_B64 | Generate a random UUID v4 string then BASE64 encoded. |
| UUID_HEX | Generate a random UUID v4 HEX representation. |
| HEADLESS_SVC_FQDN | Headless service FQDN placeholder, value pattern - `$(CLUSTER_NAME)-$(1ST_COMP_NAME)-headless.$(NAMESPACE).svc`, where 1ST_COMP_NAME is the 1st component that provide `ClusterDefinition.spec.componentDefs[].service` attribute; |
| SVC_FQDN | Service FQDN  placeholder, value pattern - `$(CLUSTER_NAME)-$(1ST_COMP_NAME).$(NAMESPACE).svc`, where 1ST_COMP_NAME is the 1st component that provide `ClusterDefinition.spec.componentDefs[].service` attribute; |
| SVC_PORT_{PORT_NAME} | A ServicePort's port value with specified port name, i.e, a servicePort JSON struct: <br /> `{"name": "mysql", "targetPort": "mysqlContainerPort", "port": 3306}`, and "$(SVC_PORT_mysql)" in the connection credential value is 3306. |
| RANDOM_PASSWD | Random 8 characters |
