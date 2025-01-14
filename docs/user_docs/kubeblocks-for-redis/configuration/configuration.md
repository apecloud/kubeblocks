---
title: Configure cluster parameters
description: Configure cluster parameters
keywords: [redis, parameter, configuration, reconfiguration]
sidebar_position: 1
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Configure cluster parameters

The KubeBlocks configuration function provides a set of consistent default configuration generation strategies for all the databases running on KubeBlocks and also provides a unified parameter configuration interface to facilitate managing parameter configuration, searching the parameter user guide, and validating parameter effectiveness.

From v0.6.0, KubeBlocks supports `kbcli cluster configure` and `kbcli cluster edit-config` to configure parameters. The difference is that KubeBlocks configures parameters automatically with `kbcli cluster configure` but `kbcli cluster edit-config` provides a visualized way for you to edit parameters directly.

<Tabs>

<TabItem value="Edit config file" label="Edit config file" default>

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
          namespace: demo
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
      kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.username}' | base64 -d
      >
      default

      kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.password}' | base64 -d
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

Just in case you cannot find the configuration file of your cluster, you can switch to the `kbcli` tab to view the current configuration file of a cluster.

:::

</TabItem>

<TabItem value="OpsRequest" label="OpsRequest">

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

2. Apply the configuration OpsRequest.

   ```bash
   kubectl apply -f mycluster-configuring-demo.yaml
   ```

3. Connect to this cluster to verify whether the configuration takes effect.

   1. Get the username and password.

      ```bash
      kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.username}' | base64 -d
      >
      default

      kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.password}' | base64 -d
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

Just in case you cannot find the configuration file of your cluster, you can switch to the `kbcli` tab to view the current configuration file of a cluster.

:::

</TabItem>

<TabItem value="kbcli" label="kbcli">

## View parameter information

View the current configuration file of a cluster.

```bash
kbcli cluster describe-config mycluster --components=redis -n demo
```

From the meta information, the cluster `mycluster` has a configuration file named `redis.cnf`.

You can also view the details of this configuration file and parameters.

* View the details of the current configuration file.

  ```bash
  kbcli cluster describe-config mycluster -n demo --components=redis --show-detail
  ```

* View the parameter description.

  ```bash
  kbcli cluster explain-config mycluster -n demo --components=redis |head -n 20
  ```

* View the user guide of a specified parameter.

  ```bash
  kbcli cluster explain-config mycluster -n demo --components=redis --param=acllog-max-len
  ```

  <details>
  <summary>Output</summary>

  ```bash
  template meta:
    ConfigSpec: redis-replication-config	ComponentName: redis	ClusterName: mycluster -n demo

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
    * When `Dynamic` is `true`, it means the effectiveness type of parameters is **dynamic** and can be updated online.
    * When `Dynamic` is `false`, it means the effectiveness type of parameters is **static** and a pod restarting is required to make the configuration effective.
  * Description: It describes the parameter definition.

## Configure parameters

### Configure parameters with configure command

The example below configures `acllog-max-len`.

1. View the current values of `acllog-max-len`.

   ```bash
   kbcli cluster connect mycluster -n demo
   ```

   ```bash
   127.0.0.1:6379> config get parameter acllog-max-len
   1) "acllog-max-len"
   2) "128"
   ```

2. Adjust the values of `acllog-max-len`.

   ```bash
   kbcli cluster configure mycluster -n demo --components=redis --set=acllog-max-len=256
   ```

   :::note

   Make sure the value you set is within the Allowed Values of this parameter. If you set a value that does not meet the value range, the system prompts an error. For example,

   ```bash
   kbcli cluster configure mycluster -n demo  --set=acllog-max-len=1000000
   >
   error: failed to validate updated config: [failed to cue template render configure: [configuration."acllog-max-len": 2 errors in empty disjunction:
   configuration."acllog-max-len": conflicting values 128 and 1000000:
       20:43
       155:16
   configuration."acllog-max-len": invalid value 1000000 (out of bound <=10000):
       20:32
   ]
   ]
   ```

   :::

3. View the status of the parameter configuration.

   `Status.Progress` and `Status.Status` shows the overall status of the parameter configuration and `Conditions` show the details.

   When the `Status.Status` shows `Succeed`, the configuration is completed.

   ```bash
   kbcli cluster describe-ops mycluster-reconfiguring-zjztm -n demo
   ```

   <details>
   <summary>Output</summary>

   ```bash
   Spec:
     Name: mycluster-reconfiguring-zjztm	NameSpace: demo	Cluster: mycluster	Type: Reconfiguring

   Command:
     kbcli cluster configure mycluster -n demo --components=redis --config-specs=redis-replication-config --config-file=redis.conf --set acllog-max-len=256 --namespace=demo

   Status:
     Start Time:         Sep 29,2024 10:46 UTC+0800
     Duration:           10s
     Status:             Running
     Progress:           1/2
                         OBJECT-KEY   STATUS   DURATION   MESSAGE

   Conditions:
   LAST-TRANSITION-TIME         TYPE                 REASON                         STATUS   MESSAGE
   Sep 29,2024 10:46 UTC+0800   Progressing          Progressing                    True     wait for the controller to process the OpsRequest: mycluster-reconfiguring-zjztm in Cluster: mycluster
   Sep 29,2024 10:46 UTC+0800   Validated            ValidateOpsRequestPassed       True     OpsRequest: mycluster-reconfiguring-zjztm is validated
   Sep 29,2024 10:46 UTC+0800   Reconfigure          ReconfigureStarted             True     Start to reconfigure in Cluster: mycluster, Component: redis
   ```

   </details>

4. Connect to the database to verify whether the parameter is configured as expected.

   It takes about 30 seconds for the configuration to take effect because the kubelet requires some time to sync changes in the ConfigMap to the Pod's volume.

   ```bash
   kbcli cluster connect mycluster -n demo
   ```

   ```bash
   127.0.0.1:6379> config get parameter acllog-max-len
   1) "acllog-max-len"
   2) "256"
   ```

### Configure parameters with edit-config command

For your convenience, KubeBlocks offers a tool `edit-config` to help you configure parameters in a visualized way.

For Linux and macOS, you can edit configuration files by vi. For Windows, you can edit files on the notepad.

1. Edit the configuration file.

   ```bash
   kbcli cluster edit-config mycluster -n demo
   ```

   :::note

   If there are multiple components in a cluster, use `--component` to specify a component.

   :::

2. View the status of the parameter configuration.

   ```bash
   kbcli cluster describe-ops mycluster-reconfiguring-nflq8 -n demo
   ```

3. Connect to the database to verify whether the parameters are configured as expected.

   ```bash
   kbcli cluster connect mycluster -n demo
   ```

   :::note

   1. For the `edit-config` function, static parameters and dynamic parameters cannot be edited at the same time.
   2. Deleting a parameter will be supported later.

   :::

## View history and compare differences

After the configuration is completed, you can search the configuration history and compare the parameter differences.

View the parameter configuration history.

```bash
kbcli cluster describe-config mycluster -n demo --components=redis
```

<details>
<summary>Output</summary>

```bash
ConfigSpecs Meta:
CONFIG-SPEC-NAME           FILE         ENABLED   TEMPLATE                 CONSTRAINT                  RENDERED                                       COMPONENT   CLUSTER
redis-replication-config   redis.conf   true      redis7-config-template   redis7-config-constraints   mycluster-redis-redis-replication-config   redis       mycluster

History modifications:
OPS-NAME                        CLUSTER     COMPONENT   CONFIG-SPEC-NAME           FILE         STATUS    POLICY    PROGRESS   CREATED-TIME                 VALID-UPDATED
mycluster-reconfiguring-zjztm   mycluster   redis       redis-replication-config   redis.conf   Succeed   restart   1/1        Sep 29,2024 10:46 UTC+0800
mycluster-reconfiguring-zrkq7   mycluster   redis       redis-replication-config   redis.conf   Succeed   restart   1/1        Sep 29,2024 11:08 UTC+0800   {"redis.conf":"{\"databases\":\"32\",\"maxclients\":\"20000\"}"}
mycluster-reconfiguring-mwbnw   mycluster   redis       redis-replication-config   redis.conf   Succeed   restart   1/1        Sep 29,2024 11:20 UTC+0800   {"redis.conf":"{\"maxclients\":\"40000\"}"}
```

</details>

From the above results, there are three parameter modifications.

Compare these modifications to view the configured parameters and their different values for different versions.

```bash
kbcli cluster diff-config mycluster-reconfiguring-zrkq7 mycluster-reconfiguring-mwbnw
>
DIFF-CONFIG RESULT:
  ConfigFile: redis.conf	TemplateName: redis-replication-config	ComponentName: redis	ClusterName: mycluster	UpdateType: update

PARAMETERNAME   MYCLUSTER-RECONFIGURING-ZRKQ7   MYCLUSTER-RECONFIGURING-MWBNW
maxclients      20000                           40000
```

</TabItem>

</Tabs>
