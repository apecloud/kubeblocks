---
title: Configure cluster parameters
description: Configure cluster parameters
keywords: [postgresql, parameter, configuration, reconfiguration]
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
kbcli cluster describe-config pg-cluster 
```

From the meta information, you can find the configuration files of this PostgreSQL cluster.

You can also view the details of this configuration file and parameters.

* View the details of the current configuration file.

   ```bash
   kbcli cluster describe-config pg-cluster --show-detail
   ```

* View the parameter description.

  ```bash
  kbcli cluster explain-config pg-cluster |head -n 20
  ```

* View the user guide of a specified parameter.
  
  ```bash
  kbcli cluster explain-config pg-cluster --param=max_connections
  >
  template meta:
    ConfigSpec: postgresql-configuration ComponentName: postgresql ClusterName: pg-cluster

  Configure Constraint:
    Parameter Name:     max_connections
    Allowed Values:     [6-8388607]
    Scope:              Global
    Dynamic:            true
    Type:               integer
    Description:        Sets the maximum number of concurrent connections.
  ```

  * Allowed Values: It defines the valid value range of this parameter.
  * Dynamic: The value of `Dynamic` in `Configure Constraint` defines how the parameter reconfiguration takes effect. There are two different reconfiguration strategies based on the effectiveness type of modified parameters, i.e. **dynamic** and **static**.
    * When `Dynamic` is `true`, it means the effectiveness type of parameters is **dynamic** and can be updated online. Follow the instructions in [Reconfigure dynamic parameters](#reconfigure-dynamic-parameters).
    * When `Dynamic` is `false`, it means the effectiveness type of parameters is **static** and a pod restarting is required to make reconfiguration effective. Follow the instructions in [Reconfigure static parameters](#reconfigure-static-parameters).
  * Description: It describes the parameter definition.

## Reconfigure dynamic parameters

The example below reconfigures `max_connections`.

1. View the current values of `max_connections`.

   ```bash
   kbcli cluster connect pg-cluster
   ```

   ```bash
   postgres=# show max_connections;
    max_connections
   -----------------
    100
   (1 row)
   ```

2. Adjust the values of `max_connections`.

   ```bash
   kbcli cluster configure pg-cluster --set=max_connections=200
   ```

   :::note

   Make sure the value you set is within the Allowed Values of this parameter. If you set a value that does not meet the value range, the system prompts an error. For example,

   ```bash
   kbcli cluster configure redis-cluster  --set=max_connections=5
   error: failed to validate updated config: [failed to cue template render configure: [redis.acllog-max-len: invalid value 5 (out of bound 6-8388607):  #TODO: Confirm the error prompt
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
     Name: pg-cluster-reconfiguring-fq6q7 NameSpace: default Cluster: pg-cluster Type: Reconfiguring

   Command:
     kbcli cluster configure pg-cluster --components=postgresql --config-spec=postgresql-configuration --config-file=postgresql.conf --set max_connections=100 --namespace=default

   Status:
     Start Time:         Mar 17,2023 19:25 UTC+0800
     Completion Time:    Mar 17,2023 19:25 UTC+0800
     Duration:           2s
     Status:             Succeed
     Progress:           1/1
                         OBJECT-KEY   STATUS   DURATION   MESSAGE

   Conditions:
   LAST-TRANSITION-TIME         TYPE                 REASON                            STATUS   MESSAGE
   Mar 17,2023 19:25 UTC+0800   Progressing          OpsRequestProgressingStarted      True     Start to process the OpsRequest: pg-cluster-reconfiguring-fq6q7 in Cluster: pg-cluster
   Mar 17,2023 19:25 UTC+0800   Validated            ValidateOpsRequestPassed          True     OpsRequest: pg-cluster-reconfiguring-fq6q7 is validated
   Mar 17,2023 19:25 UTC+0800   Reconfigure          ReconfigureStarted                True     Start to reconfigure in Cluster: pg-cluster, Component: postgresql
   Mar 17,2023 19:25 UTC+0800   ReconfigureMerged    ReconfigureMerged                 True     Reconfiguring in Cluster: pg-cluster, Component: postgresql, ConfigSpec: postgresql-configuration, info: updated: map[postgresql.conf:{"max_connections":"200"}], added: map[], deleted:map[]
   Mar 17,2023 19:25 UTC+0800   ReconfigureSucceed   ReconfigureSucceed                True     Reconfiguring in Cluster: pg-cluster, Component: postgresql, ConfigSpec: postgresql-configuration, info: updated policy: <operatorSyncUpdate>, updated: map[postgresql.conf:{"max_connections":"100"}], added: map[], deleted:map[]
   Mar 17,2023 19:25 UTC+0800   Succeed              OpsRequestProcessedSuccessfully   True     Successfully processed the OpsRequest: pg-cluster-reconfiguring-fq6q7 in Cluster: pg-cluster
   ```

   </details>

4. Connect to the database to verify whether the parameters are modified.

   The whole searching process has a 30-second delay since it takes some time for kubelete to synchronize modifications to the volume of the pod.

   ```bash
   kbcli cluster connect pg-cluster
   ```

   ```bash
   postgres=# show max_connections;
    max_connections
   -----------------
    200
   (1 row)
   ```

## Reconfigure static parameters

The example below reconfigures `shared_buffers`.

1. View the current values of `shared_buffers`.

   ```bash
   kbcli cluster connect pg-cluster
   ```

   ```bash
   postgres=# show shared_buffers;
    shared_buffers
   ----------------
    204MB
   (1 row)
   ```

3. Adjust the values of `maxclients` and `databases`.

   ```bash
   kbcli cluster configure pg-cluster --set=shared_buffers=512M
   ```

   :::note

   Make sure the value you set is within the Allowed Values of this parameter. If you set a value that does not meet the value range, the system prompts an error. For example,

   ```bash
   kbcli cluster configure pg-cluster  --set=shared_buffers=5M
   error: failed to validate updated config: [failed to cue template render configure: [redis.maxclients: invalid value 5 (out of bound 16-107374182):
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
     Name: pg-cluster-reconfiguring-nkf8t NameSpace: default Cluster: pg-cluster Type: Reconfiguring

   Command:
     kbcli cluster configure pg-cluster --components=postgresql --config-spec=postgresql-configuration --config-file=postgresql.conf --set shared_buffers=512M --namespace=default

   Status:
     Start Time:         Mar 17,2023 19:31 UTC+0800
     Duration:           2s
     Status:             Running
     Progress:           0/1
                         OBJECT-KEY   STATUS   DURATION   MESSAGE

   Conditions:
   LAST-TRANSITION-TIME         TYPE                 REASON                         STATUS   MESSAGE
   Mar 17,2023 19:31 UTC+0800   Progressing          OpsRequestProgressingStarted   True     Start to process the OpsRequest: pg-cluster-reconfiguring-nkf8t in Cluster: pg-cluster
   Mar 17,2023 19:31 UTC+0800   Validated            ValidateOpsRequestPassed       True     OpsRequest: pg-cluster-reconfiguring-nkf8t is validated
   Mar 17,2023 19:31 UTC+0800   Reconfigure          ReconfigureStarted             True     Start to reconfigure in Cluster: pg-cluster, Component: postgresql
   Mar 17,2023 19:31 UTC+0800   ReconfigureMerged    ReconfigureMerged              True     Reconfiguring in Cluster: pg-cluster, Component: postgresql, ConfigSpec: postgresql-configuration, info: updated: map[postgresql.conf:{"shared_buffers":"512M"}], added: map[], deleted:map[]
   Mar 17,2023 19:31 UTC+0800   ReconfigureRunning   ReconfigureRunning             True     Reconfiguring in Cluster: pg-cluster, Component: postgresql, ConfigSpec: postgresql-configuration
   ```

   </details>

5. Connect to the database to verify whether the parameters are modified.

   The whole searching process has a 30-second delay since it takes some time for kubelete to synchronize modifications to the volume of the pod.

   ```bash
   kbcli cluster connect pg-cluster
   ```

   ```bash
   postgres=# show shared_buffers;
    shared_buffers
   ----------------
    512MB
   (1 row)
   ```

## View history and compare differences

After the reconfiguration is completed, you can search the reconfiguration history and compare the parameter differences.

View the parameter reconfiguration history.

```bash
kbcli cluster describe-config pg-cluster
```

<details>

<summary>Output</summary>

```bash
ConfigSpecs Meta:
CONFIG-SPEC-NAME            FILE                  ENABLED   TEMPLATE                    CONSTRAINT        RENDERED                                          COMPONENT    CLUSTER
postgresql-configuration    kb_restore.conf       false     postgresql-configuration    postgresql14-cc   pg-cluster-postgresql-postgresql-configuration    postgresql   pg-cluster
postgresql-configuration    pg_hba.conf           false     postgresql-configuration    postgresql14-cc   pg-cluster-postgresql-postgresql-configuration    postgresql   pg-cluster
postgresql-configuration    postgresql.conf       true      postgresql-configuration    postgresql14-cc   pg-cluster-postgresql-postgresql-configuration    postgresql   pg-cluster
postgresql-configuration    kb_pitr.conf          false     postgresql-configuration    postgresql14-cc   pg-cluster-postgresql-postgresql-configuration    postgresql   pg-cluster
postgresql-custom-metrics   custom-metrics.yaml   false     postgresql-custom-metrics                     pg-cluster-postgresql-postgresql-custom-metrics   postgresql   pg-cluster

History modifications:
OPS-NAME                         CLUSTER      COMPONENT    CONFIG-SPEC-NAME           FILE              STATUS    POLICY    PROGRESS   CREATED-TIME                 VALID-UPDATED
pg-cluster-reconfiguring-fq6q7   pg-cluster   postgresql   postgresql-configuration   postgresql.conf   Succeed             1/1        Mar 17,2023 19:25 UTC+0800   {"postgresql.conf":"{\"max_connections\":\"100\"}"}
pg-cluster-reconfiguring-bm84z   pg-cluster   postgresql   postgresql-configuration   postgresql.conf   Succeed             1/1        Mar 17,2023 19:27 UTC+0800   {"postgresql.conf":"{\"max_connections\":\"200\"}"}
pg-cluster-reconfiguring-cbqxd   pg-cluster   postgresql   postgresql-configuration   postgresql.conf   Succeed             1/1        Mar 17,2023 19:35 UTC+0800   {"postgresql.conf":"{\"max_connections\":\"500\"}"}
pg-cluster-reconfiguring-rcnzb   pg-cluster   postgresql   postgresql-configuration   postgresql.conf   Succeed   restart   1/1        Mar 17,2023 19:38 UTC+0800   {"postgresql.conf":"{\"shared_buffers\":\"512MB\"}"}
```

</details>

From the above results, there are three parameter modifications.

Compare these modifications to view the configured parameters and their different values for different versions.

```bash
kbcli cluster diff-config pg-cluster-reconfiguring-bm84z pg-cluster-reconfiguring-rcnzb
>
DIFF-CONFIG RESULT:
  ConfigFile: postgresql.conf TemplateName: postgresql-configuration ComponentName: postgresql ClusterName: pg-cluster UpdateType: update

PARAMETERNAME     PG-CLUSTER-RECONFIGURING-BM84Z   PG-CLUSTER-RECONFIGURING-RCNZB
max_connections   200                              500
shared_buffers    '256MB'                          512MB
```
