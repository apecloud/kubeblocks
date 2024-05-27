---
title: Configure cluster parameters
description: Configure cluster parameters
keywords: [redis, parameter, configuration, reconfiguration]
sidebar_position: 1
sidebar_label: Configuration
---

# Configure cluster parameters

This guide shows how to configure cluster parameters.

## Before you start

1. [Install KubeBlocks](./../../installation/install-kubeblocks.md).
2. [Create a Redis cluster](./../cluster-management/create-and-connect-a-redis-cluster.md).

## Configure cluster parameters by editing configuration file

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
      preConditionDeadlineSeconds: 0
      type: Reconfiguring
    ```

   | Field                                                  | Definition     |
   |--------------------------------------------------------|--------------------------------|
   | `metadata.name`                                        | It specifies the name of this OpsRequest. |
   | `metadata.namespace`                                   | It specifies the namespace where this cluster is created. |
   | `spec.clusterName`                                     | It specifies the cluster name that this operation is targeted at. |
   | `spec.reconfigure`                                     | It specifies a component and its configuration updates. |
   | `spec.reconfigure.componentName`                       | It specifies the component name of this cluster.  |
   | `spec.configurations`                                  | It contains a list of ConfigurationItem objects, specifying the component's configuration template name, upgrade policy, and parameter key-value pairs to be updated. |
   | `spec.reconfigure.configurations.keys.key`             | It specifies the configuration map. |
   | `spec.reconfigure.configurations.keys.parameters`      | It defines a list of key-value pairs for a single configuration file. |
   | `spec.reconfigure.configurations.keys.parameter.key`   | It represents the name of the parameter you want to edit. |
   | `spec.reconfigure.configurations.keys.parameter.value` | It represents the parameter values that are to be updated. If set to nil, the parameter defined by the Key field will be removed from the configuration file.  |
   | `spec.reconfigure.configurations.name`                 | It specifies the configuration template name.  |
   | `preConditionDeadlineSeconds`                          | It specifies the maximum number of seconds this OpsRequest will wait for its start conditions to be met before aborting. If set to 0 (default), the start conditions must be met immediately for the OpsRequest to proceed. |

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
