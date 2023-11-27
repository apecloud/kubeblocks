---
title: Configure cluster parameters
description: Configure cluster parameters
keywords: [redis, parameter, configuration, reconfiguration]
sidebar_position: 1
---

# Configure cluster parameters

The KubeBlocks configuration function provides a set of consistent default configuration generation strategies for all the databases running on KubeBlocks and also provides a unified parameter configuration interface to facilitate managing parameter configuration, searching the parameter user guide, and validating parameter effectiveness.

From v0.6.0, KubeBlocks supports `kbcli cluster configure` and `kbcli cluster edit-config` to configure parameters. The difference is that KubeBlocks configures parameters automatically with `kbcli cluster configure` but `kbcli cluster edit-config` provides a visualized way for you to edit parameters directly.

## View parameter information

View the current configuration file of a cluster.

```bash
kbcli cluster describe-config redis-cluster --component=redis
```

From the meta information, the cluster `redis-cluster` has a configuration file named `redis.cnf`.

You can also view the details of this configuration file and parameters.

* View the details of the current configuration file.

  ```bash
  kbcli cluster describe-config redis-cluster --component=redis --show-detail
  ```

* View the parameter description.

  ```bash
  kbcli cluster explain-config redis-cluster --component=redis |head -n 20
  ```

* View the user guide of a specified parameter.

  ```bash
  kbcli cluster explain-config redis-cluster --component=redis --param=acllog-max-len
  ```

  <details>
  <summary>Output</summary>

  ```bash
  template meta:
    ConfigSpec: redis-replication-config	ComponentName: redis	ClusterName: redis-cluster

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
   kbcli cluster connect redis-cluster
   ```

   ```bash
   127.0.0.1:6379> config get parameter acllog-max-len
   1) "acllog-max-len"
   2) "128"
   ```

2. Adjust the values of `acllog-max-len`.

   ```bash
   kbcli cluster configure redis-cluster --component=redis --set=acllog-max-len=256
   ```

   :::note

   Make sure the value you set is within the Allowed Values of this parameter. If you set a value that does not meet the value range, the system prompts an error. For example,

   ```bash
   kbcli cluster configure redis-cluster  --set=acllog-max-len=1000000
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
   kbcli cluster describe-ops redis-cluster-reconfiguring-zjztm -n default
   ```

   <details>
   <summary>Output</summary>

   ```bash
   Spec:
     Name: redis-cluster-reconfiguring-zjztm	NameSpace: default	Cluster: redis-cluster	Type: Reconfiguring

   Command:
     kbcli cluster configure redis-cluster --components=redis --config-spec=redis-replication-config --config-file=redis.conf --set acllog-max-len=256 --namespace=default

   Status:
     Start Time:         Apr 17,2023 17:22 UTC+0800
     Duration:           10s
     Status:             Running
     Progress:           0/1
                         OBJECT-KEY   STATUS   DURATION   MESSAGE

   Conditions:
   LAST-TRANSITION-TIME         TYPE                 REASON                         STATUS   MESSAGE
   Apr 17,2023 17:22 UTC+0800   Progressing          OpsRequestProgressingStarted   True     Start to process the OpsRequest: redis-cluster-reconfiguring-zjztm in Cluster: redis-cluster
   Apr 17,2023 17:22 UTC+0800   Validated            ValidateOpsRequestPassed       True     OpsRequest: redis-cluster-reconfiguring-zjztm is validated
   Apr 17,2023 17:22 UTC+0800   Reconfigure          ReconfigureStarted             True     Start to reconfigure in Cluster: redis-cluster, Component: redis
   Apr 17,2023 17:22 UTC+0800   ReconfigureRunning   ReconfigureRunning             True     Reconfiguring in Cluster: redis-cluster, Component: redis, ConfigSpec: redis-replication-config
   ```

   </details>

4. Connect to the database to verify whether the parameter is configured as expected.

   The whole searching process has a 30-second delay since it takes some time for kubelet to synchronize modifications to the volume of the pod.

   ```bash
   kbcli cluster connect redis-cluster
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
   kbcli cluster edit-config redis-cluster
   ```

:::note

If there are multiple components in a cluster, use `--component` to specify a component.

:::

2. View the status of the parameter configuration.

   ```bash
   kbcli cluster describe-ops xxx -n default
   ```

3. Connect to the database to verify whether the parameters are configured as expected.

   ```bash
   kbcli cluster connect redis-cluster
   ```

:::note

1. For the `edit-config` function, static parameters and dynamic parameters cannot be edited at the same time.
2. Deleting a parameter will be supported later.

:::

## View history and compare differences

After the configuration is completed, you can search the configuration history and compare the parameter differences.

View the parameter configuration history.

```bash
kbcli cluster describe-config redis-cluster --component=redis
```

<details>
<summary>Output</summary>

```bash
ConfigSpecs Meta:
CONFIG-SPEC-NAME           FILE         ENABLED   TEMPLATE                 CONSTRAINT                  RENDERED                                       COMPONENT   CLUSTER
redis-replication-config   redis.conf   true      redis7-config-template   redis7-config-constraints   redis-cluster-redis-redis-replication-config   redis       redis-cluster

History modifications:
OPS-NAME                            CLUSTER         COMPONENT   CONFIG-SPEC-NAME           FILE         STATUS    POLICY    PROGRESS   CREATED-TIME                 VALID-UPDATED
redis-cluster-reconfiguring-zjztm   redis-cluster   redis       redis-replication-config   redis.conf   Succeed   restart   1/1        Apr 17,2023 17:22 UTC+0800
redis-cluster-reconfiguring-zrkq7   redis-cluster   redis       redis-replication-config   redis.conf   Succeed   restart   1/1        Apr 17,2023 17:28 UTC+0800   {"redis.conf":"{\"databases\":\"32\",\"maxclients\":\"20000\"}"}
redis-cluster-reconfiguring-mwbnw   redis-cluster   redis       redis-replication-config   redis.conf   Succeed   restart   1/1        Apr 17,2023 17:35 UTC+0800   {"redis.conf":"{\"maxclients\":\"40000\"}"}
```

</details>

From the above results, there are three parameter modifications.

Compare these modifications to view the configured parameters and their different values for different versions.

```bash
kbcli cluster diff-config redis-cluster-reconfiguring-zrkq7 redis-cluster-reconfiguring-mwbnw
>
DIFF-CONFIG RESULT:
  ConfigFile: redis.conf	TemplateName: redis-replication-config	ComponentName: redis	ClusterName: redis-cluster	UpdateType: update

PARAMETERNAME   REDIS-CLUSTER-RECONFIGURING-ZRKQ7   REDIS-CLUSTER-RECONFIGURING-MWBNW
maxclients      20000                               40000
```
