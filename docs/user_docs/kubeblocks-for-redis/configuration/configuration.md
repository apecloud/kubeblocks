---
title: Configure cluster parameters
description: Configure cluster parameters
keywords: [redis, parameter, configuration, reconfiguration]
sidebar_position: 1
---

# Configure cluster parameters

The KubeBlocks configuration function provides a set of consistent default configuration generation strategies for all the databases running on KubeBlocks and also provides a unified parameter configuration interface to facilitate managing parameter reconfiguration, searching the parameter user guide, and validating parameter effectiveness.

## Before you start

1. Install KubeBlocks. For details, refer to [Install KubeBlocks](./../../installation/install-and-uninstall-kbcli-and-kubeblocks.md).
2. Create a Redis cluster and wait until the cluster status is Running. For details, refer to [Create and connect a Redis cluster](./../cluster-management/create-and-connect-a-redis-cluster.md)

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
  >
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

  * Allowed Values: It defines the valid value range of this parameter.
  * Dynamic: The value of `Dynamic` in `Configure Constraint` defines how the parameter reconfiguration takes effect. There are two different reconfiguration strategies based on the effectiveness type of modified parameters, i.e. **dynamic** and **static**.
    * When `Dynamic` is `true`, it means the effectiveness type of parameters is **dynamic** and can be updated online. Follow the instructions in [Reconfigure dynamic parameters](#reconfigure-dynamic-parameters).
    * When `Dynamic` is `false`, it means the effectiveness type of parameters is **static** and a pod restarting is required to make reconfiguration effective. Follow the instructions in [Reconfigure static parameters](#reconfigure-static-parameters).
  * Description: It describes the parameter definition.

## Reconfigure dynamic parameters

The example below reconfigures `acllog-max-len`.

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
   kbcli cluster configure redis-cluster  --set=acllog-max-len=20000
   error: failed to validate updated config: [failed to cue template render configure: [redis.acllog-max-len: invalid value 10001 (out of bound >=10000):  #TODO: Confirm the error prompt
    343:34
   ]
   ]
   ```

   :::

3. View the status of the parameter reconfiguration.

   `Status.Progress` and `Status.Status` shows the overall status of the parameter reconfiguration and `Conditions` show the details.

   When the `Status.Status` shows `Succeed`, the reconfiguration is completed.

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

4. Connect to the database to verify whether the parameters are modified.

   The whole searching process has a 30-second delay since it takes some time for kubelete to synchronize modifications to the volume of the pod.

   ```bash
   kbcli cluster connect redis-cluster
   ```

   ```bash
   127.0.0.1:6379> config get parameter acllog-max-len
   1) "acllog-max-len"
   2) "256"
   ```

## Reconfigure static parameters

The example below reconfigures `maxclients` and `databases`.

1. View the current values of `maxclients` and `databases`.

   ```bash
   kbcli cluster connect redis-cluster
   ```

   ```bash
   127.0.0.1:6379> config get parameter maxclients databases
   1) "databases"
   2) "16"
   3) "maxclients"
   4) "10000"
   ```

3. Adjust the values of `maxclients` and `databases`.

   ```bash
   kbcli cluster configure redis-cluster --component=redis --set=maxclients=20000,databases=32
   ```

   :::note

   Make sure the value you set is within the Allowed Values of this parameter. If you set a value that does not meet the value range, the system prompts an error. For example,

   ```bash
   kbcli cluster configure redis-cluster  --set=maxclients=65001,databases=32
   error: failed to validate updated config: [failed to cue template render configure: [redis.maxclients: invalid value 65001 (out of bound >=65000):
    343:34
   ]
   ]
   ```

   :::

4. View the status of the parameter reconfiguration.

   `Status.Progress` and `Status.Status` shows the overall status of the parameter reconfiguration and `Conditions` show the details.

   When the `Status.Status` shows `Succeed`, the reconfiguration is completed.

   ```bash
   kbcli cluster describe-ops redis-cluster-reconfiguring-zrkq7 -n default
   ```

   <details>

   <summary>Output</summary>


   ```bash 
   Spec:
     Name: redis-cluster-reconfiguring-zrkq7	NameSpace: default	Cluster: redis-cluster	Type: Reconfiguring

   Command:
     kbcli cluster configure redis-cluster --components=redis --config-spec=redis-replication-config --config-file=redis.conf --set databases=32 --set maxclients=20000 --namespace=default

   Status:
     Start Time:         Apr 17,2023 17:28 UTC+0800
     Duration:           2s
     Status:             Running
     Progress:           0/1
                         OBJECT-KEY   STATUS   DURATION   MESSAGE

   Conditions:
   LAST-TRANSITION-TIME         TYPE                 REASON                         STATUS   MESSAGE
   Apr 17,2023 17:28 UTC+0800   Progressing          OpsRequestProgressingStarted   True     Start to process the OpsRequest: redis-cluster-reconfiguring-zrkq7 in Cluster: redis-cluster
   Apr 17,2023 17:28 UTC+0800   Validated            ValidateOpsRequestPassed       True     OpsRequest: redis-cluster-reconfiguring-zrkq7 is validated
   Apr 17,2023 17:28 UTC+0800   Reconfigure          ReconfigureStarted             True     Start to reconfigure in Cluster: redis-cluster, Component: redis
   Apr 17,2023 17:28 UTC+0800   ReconfigureMerged    ReconfigureMerged              True     Reconfiguring in Cluster: redis-cluster, Component: redis, ConfigSpec: redis-replication-config, info: updated: map[redis.conf:{"databases":"32","maxclients":"20000"}], added: map[], deleted:map[]
   Apr 17,2023 17:28 UTC+0800   ReconfigureRunning   ReconfigureRunning             True     Reconfiguring in Cluster: redis-cluster, Component: redis, ConfigSpec: redis-replication-config
   ```


   </details>

5. Connect to the database to verify whether the parameters are modified.

   The whole searching process has a 30-second delay since it takes some time for kubelete to synchronize modifications to the volume of the pod.

   ```bash
   kbcli cluster connect redis-cluster
   ```

   ```bash
   127.0.0.1:6379> config get parameter maxclients databases
   1) "databases"
   2) "32"
   3) "maxclients"
   4) "20000"
   ```

## View history and compare differences

After the reconfiguration is completed, you can search the reconfiguration history and compare the parameter differences.

View the parameter reconfiguration history.

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