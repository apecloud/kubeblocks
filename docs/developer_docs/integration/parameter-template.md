---
title: Parameter template
description: How to configure parameter templates in KubeBlocks 
keywords: [parameter template]
sidebar_position: 4
sidebar_label: Parameter template
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Parameter template

This tutorial demonstrates how to configure parameter templates in KubeBlocks with Oracle MySQL as an example. You can find [the full PR here](https://github.com/apecloud/learn-kubeblocks-addon/tree/main/tutorial-3-config-and-reconfig/).

## Before you start

1. Grasp basic concepts of Kubernetes, such as Pod and ConfigMap.
2. Finish [Tutorial 1](./how-to-add-an-add-on.md).
3. Know something about Go Template (Optional).

## Introduction

When creating a cluster, developers typically configure parameters according to resource availability, performance needs, environment, etc. Cloud database providers like AWS and Alibaba Cloud have therefore offered various parameter templates (such as high-performance and asynchronous templates for RDS) to facilitate a quick startup for users.

In this tutorial, you will learn how to configure parameters in KubeBlocks, which includes adding parameter templates, configuring parameters, and configuring parameter validation.

Although Kubernetes allows users to mount parameter files as ConfigMap on volumes of the Pod, it only manages ConfigMap updates and synchronizes them to the volume. Therefore, if the database engine (such as MySQL and Postgres) fails to support dynamic loading of configuration files, you can only log in to the database to perform update operations, which can easily lead to configuration drift.

To prevent that, KubeBlocks manages all parameters through ConfigMap with the principle that `ConfigMap is the only source-of-truth`. It means that all parameter configurations are first applied to ConfigMap, and then, depending on different ways the parameters take effect, applied to each Pod in the cluster. A comprehensive guide on how to configure parameters will be provided in the next tutorial.

## ConfigTemplate

KubeBlocks renders parameter templates with ***Go Template***. Apart from common functions, it also includes some frequently-used calculation functions such as `callBufferSizeByResource` and `getContainerCPU`.

With KubeBlocks's enhanced rendering capabilities, you can quickly create an ***Adaptive ConfigTemplate*** and generate appropriate configuration files based on the context, such as memory and CPU.

### Add a parameter template

```yaml
1 apiVersion: v1
2 kind: ConfigMap
3 metadata:
4   name: oracle-mysql-config-template
5   labels:
6     {{- include "oracle-mysql.labels" . | nindent 4 }}
7 data:
8   my.cnf: |-
9     {{`
10      [mysqld]
11      port=3306
12      {{- $phy_memory := getContainerMemory ( index $.podSpec.containers 0 ) }}
13      {{- $pool_buffer_size := ( callBufferSizeByResource ( index $.podSpec.containers 0 ) ) }}
14      {{- if $pool_buffer_size }}
15      innodb_buffer_pool_size={{ $pool_buffer_size }}
16      {{- end }}
17 
18      # If the memory is less than 8Gi, disable performance_schema
19      {{- if lt $phy_memory 8589934592 }}
20      performance_schema=OFF
21      {{- end }}
22 
23      [client]
24      port=3306
25      socket=/var/run/mysqld/mysqld.sock
26      `
27    }}
```

The above example illustrates an adaptive ConfigTemplate for MySQL defined through ConfigMap. It includes several common MySQL parameters, such as `port` and `innodb_buffer_pool_size`.

Based on the memory parameter configured when the container is started, it can:

- Calculate the size of `innodb_buffer_size` (Line 11 to 15);
- Disable `performance_schema` when the memory is less than 8Gi to reduce performance impact (Line 19 to 21).

`callBufferSizeByResource` is a predefined bufferPool calculation rule, primarily for MySQL. You can also customize your calculation formulas by querying memory and CPU:

- `getContainerMemory` retrieves the memory size of a particular container in the Pod.
- `getContainerCPU` retrieves the CPU size of a particular container in the Pod.

:::note

Tailor additional parameter calculation options as you wish:

- Calculate an appropriate `max_connection` value based on memory size.
- Calculate reasonable configurations for other components based on the total memory available.

:::

### Use a parameter template

#### Modify ClusterDefinition

Specify parameter templates through `configSpecs` in `ClusterDefinition` and quote the ConfigMap defined in [Add a parameter template](#add-a-parameter-template).

```yaml
  componentDefs:
    - name: mysql-compdef
      configSpecs:
        - name: mysql-config
          templateRef: oracle-mysql-config-template # Defines the ConfigMap name for the parameter template
          volumeName: configs                       # Name of the mounted volume          
          namespace: {{ .Release.Namespace }}       # Namespace of the ConfigMap
      podSpec:
        containers:
         - name: mysql-container
           volumeMounts:
             - mountPath: /var/lib/mysql
               name: data
             - mountPath: /etc/mysql/conf.d       # Path to the mounted configuration files, engine-related  
               name: configs                      # Corresponds to the volumeName on Line 6    
           ports:
            ...
```

As shown above, you need to modify `ClusterDefinition.yaml` file by adding `configSpecs`. Remember to specify the following:

- templateRef: The name of the ConfigMap where the template is.
- volumeName: The name of the volume mounted to the Pod.
- namespace: The namespace of the template file (ConfigMap is namespace-scoped, usually in the namespace where KubeBlocks is installed).

#### View configuration info

When a new cluster is created, KubeBlocks renders the corresponding ConfigMap based on configuration templates and mounts it to the `configs` volume.

1. Install a Helm chart.

   ```bash
   helm install oracle-mysql path-to-your-helm-char/oracle-mysql
   ```

2. Create a cluster.

   ```bash
   kbcli cluster create mycluster --cluster-definition oracle-mysql --cluster-version oracle-mysql-8.0.32
   ```

3. View configuration.

   kbcli provides the subcommand `describe-config` to view the configuration of a cluster.

   ```bash
   kbcli cluster describe-config mycluster --component mysql-compdef
   >
   ConfigSpecs Meta:
   CONFIG-SPEC-NAME   FILE     ENABLED   TEMPLATE                       CONSTRAINT   RENDERED                               COMPONENT       CLUSTER
   mysql-config       my.cnf   false     oracle-mysql-config-template                mycluster-mysql-compdef-mysql-config   mysql-compdef   mycluster

   History modifications:
   OPS-NAME   CLUSTER   COMPONENT   CONFIG-SPEC-NAME   FILE   STATUS   POLICY   PROGRESS   CREATED-TIME   VALID-UPDATED
   ```

You can view:

- Name of the configuration template: oracle-mysql-config-template
- Rendered ConfigMap: mycluster-mysql-compdef-mysql-config
- Name of the file loaded: my.cnf

## Summary

This tutorial introduces how to render "adaptive" parameters with configuration templates in KubeBlocks.

In Kubernetes, ConfigMap changes are periodically synchronized to Pods, but most database engines (such as MySQL, PostgreSQL, and Redis) do not actively load new configurations. This is because modifying ConfigMap alone does not provide the capability to Reconfig (parameter changes).

## Appendix

### A.1 How to configure multiple parameter templates?

To meet various requirements, developers often need to configure multiple parameter templates in a production environment. For example, Alibaba Cloud provides many high-performance parameter templates and asynchronous templates for customized needs.

In KubeBlocks, developers can use multiple `ClusterVersion` to achieve their goals.

$$Cluster = ClusterDefinition.yaml \Join ClusterVersion.yaml \Join Cluster.yaml
$$

The JoinKey is the Component Name.

As the cluster definition formula indicates, multiple ClusterVersion can be combined with the same ClusterDefinition to set up different configurations.

```yaml
## The first ClusterVersion, uses configurations from ClusterDefinition
apiVersion: apps.kubeblocks.io/v1alpha1
kind: ClusterVersion
metadata:
  name: oracle-mysql
spec:
  clusterDefinitionRef: oracle-mysql
  componentVersions:
  - componentDefRef: mysql-compdef
    versionsContext:
      containers:
        - name: mysql-container
          ...
---
## The second ClusterDefinition, defines its own configSpecs and overrides the configuration of ClusterDefinition
apiVersion: apps.kubeblocks.io/v1alpha1
kind: ClusterVersion
metadata:
  name: oracle-mysql-perf
spec:
  clusterDefinitionRef: oracle-mysql
  componentVersions:
  - componentDefRef: mysql-compdef
    versionsContext:
      containers:
        - name: mysql-container
         ...
    # The name needs to be consistent with that of the ConfigMap defined in ClusterDefinition
    configSpecs:
      - name: mysql-config    
        templateRef: oracle-mysql-perf-config-template
        volumeName: configs
```

As shown above, two ClusterVersion objects are created.

The first one uses the default parameter template (without any configuration), and the second one specifies a new parameter template `oracle-mysql-perf-config-template` through `configSpecs`.

When creating a cluster, you can specify `ClusterVersion` to create clusters with different configurations, such as:

```bash
kbcli cluster create mysqlcuster --cluster-definition oracle-mysql --cluster-version  oracle-mysql-perf
```

:::note

KubeBlocks merges configurations from ClusterVersion and ClusterDefinition via `configSpecs.name`. Therefore, make sure that `configSpecs.name` defined in ClusterVersion matches the name defined in ClusterDefinition.

:::
