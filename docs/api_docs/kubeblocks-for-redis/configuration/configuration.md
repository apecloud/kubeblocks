---
title: Configure cluster parameters
description: Configure cluster parameters
keywords: [redis, parameter, configuration, reconfiguration]
sidebar_position: 1
sidebar_label: Configuration
---

# Configure cluster parameters

This guide shows how to configure cluster parameters.

KubeBlocks supports dynamic configuration. When the specification of a database instance changes (e.g., a user vertically scales a cluster), KubeBlocks automatically matches the appropriate configuration template based on the new specification. This is because different specifications of a database instance may require different optimal configurations to optimize performance and resource utilization. When you choose a different database instance specification, KubeBlocks automatically detects and determines the best database configuration for the new specification, ensuring optimal performance and configuration of the database under the new specifications.

This feature simplifies the process of configuring parameters, which saves you from manually configuring database parameters as KubeBlocks handles the updates and configurations automatically to adapt to the new specifications. This saves time and effort and reduces performance issues caused by incorrect configuration.

But it's also important to note that the dynamic parameter configuration doesn't apply to all parameters. Some parameters may require manual configuration. Additionally, if you have manually modified database parameters before, KubeBlocks may overwrite your customized configurations when refreshing the database configuration template. Therefore, when using the dynamic configuration feature, it is recommended to back up and record your custom configuration so that you can restore them if needed.

## Before you start

1. [Install KubeBlocks](./../../installation/install-kubeblocks.md).
2. [Create a Redis cluster](./../cluster-management/create-and-connect-a-redis-cluster.md).

## Configure cluster parameters by configuration file

1. Get the configuration file of this cluster. 

   ```bash
   kubectl edit configurations.apps.kubeblocks.io mycluster-redis -n demo
   ```

2. Configure parameters according to your needs. The example below adds the `- configFileParams` part to configure `acllog-max-len`.

    ```yaml
    spec:
      clusterRef: mycluster
      componentName: redis
      configItemDetails:
      - configSpec:
          constraintRef: redis7-config-constraints
          name: redis-replication-config
          namespace: kb-system
          reRenderResourceTypes:
          - vscale
          templateRef: redis7-config-template
          volumeName: redis-config
      - configFileParams:
          redis.conf:
            parameters:
              acllog-max-len: "256"
        name: mycluster-redis-redis-replication-config
    ```

3. Connect to this cluster to verify whether the configuration takes effect.

   1. Get the username and password.

      ```bash
      kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
      >
      default

      kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
      >
      kpz77mcs
      ```

   2. Connect to this cluster and verify whether the parameters are configured as expected.

      ```bash
      kubectl exec -ti -n demo mycluster-redis-0 -- bash

      root@mycluster-redis-0:/# redis-cli -a kpz77mcs  --user default

      127.0.0.1:6379> config get parameter acllog-max-len
      1) "acllog-max-len"
      2) "256"
      ```

## Configure cluster parameters with OpsRequest

1. Define an OpsRequest file and configure the parameters in the OpsRequest in a yaml file named `mycluster-configuring-demo.yaml`. In this example, `acllog-max-len` is configured as `256`.

    ```yaml
    apiVersion: apps.kubeblocks.io/v1alpha1
    kind: OpsRequest
    metadata:
      name: mycluster-configuring-demo
      namespace: demo
    spec:
      clusterName: mycluster
      reconfigure:
        componentName: redis
        configurations:
        - keys:
          - key: redis.conf
            parameters:
            - key: acllog-max-len
              value: "256"
          name: redis-replication-config
      type: Reconfiguring
    ```

   * `metadata.name` specifies the name of this OpsRequest.
   * `metadata.namespace` specifies the namespace where this cluster is created.
   * `spec.clusterName` specifies the cluster name.
   * `spec.reconfigure` specifies the configuration information.
   * `componentName` specifies the component name of this cluster.
   * `configurations.keys.key` specifies the configuration file.
   * `configurations.keys.parameters` specifies the parameters you want to edit.
   * `configurations.keys.name` specifies the configuration spec name.

2. Apply the configuration opsRequest.

   ```bash
   kubectl apply -f mycluster-configuring-demo.yaml
   ```

3. Connect to this cluster to verify whether the configuration takes effect.

   1. Get the username and password.

      ```bash
      kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
      >
      default

      kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
      >
      kpz77mcs
      ```

   2. Connect to this cluster and verify whether the parameters are configured as expected.

      ```bash
      kubectl exec -ti -n demo mycluster-redis-0 -- bash

      root@mycluster-redis-0:/# redis-cli -a kpz77mcs  --user default
      
      127.0.0.1:6379> config get parameter acllog-max-len
      1) "acllog-max-len"
      2) "256"
      ```

:::note

Just in case you cannot find the configuration file of your cluster, you can use `kbcli` to view the current configuration file of a cluster.

```bash
kbcli cluster describe-config mycluster -n demo
```

From the meta information, the cluster `mycluster` has a configuration file named `redis.conf`. 

You can also view the details of this configuration file and parameters.

* View the details of the current configuration file. 

   ```bash
   kbcli cluster describe-config mycluster --show-detail -n demo
   ```

* View the parameter description.

  ```bash
  kbcli cluster explain-config mycluster -n demo | head -n 20
  ```

* View the user guide of a specified parameter.
  
  ```bash
  kbcli cluster explain-config mycluster --param=acllog-max-len  -n demo
  ```

  <details>

  <summary>Output</summary>

  ```bash
  component: redis
  template meta:
    ConfigSpec: redis-replication-config  ComponentName: redis    ClusterName: mycluster

  Configure Constraint:
    Parameter Name:     acllog-max-len
    Allowed Values:     [1-10000]
    Scope:              Global
    Dynamic:            true
    Type:               integer
    Description:    
  ```

  </details>

  * Allowed Values: It defines the valid value range of this parameter.
  * Dynamic: The value of `Dynamic` in `Configure Constraint` defines how the parameter configuration takes effect. There are two different configuration strategies based on the effectiveness type of modified parameters, i.e. **dynamic** and **static**.
    * When `Dynamic` is `true`, it means the effectiveness type of parameters is **dynamic** and can be configured online.
    * When `Dynamic` is `false`, it means the effectiveness type of parameters is **static** and a pod restarting is required to make the configuration effective.
  * Description: It describes the parameter definition.

:::