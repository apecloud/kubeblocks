---
title: Configure cluster parameters
description: Configure cluster parameters
keywords: [postgresql, parameter, configuration, reconfiguration]
sidebar_position: 1
---

# Configure cluster parameters

The KubeBlocks configuration function provides a set of consistent default configuration generation strategies for all the databases running on KubeBlocks and also provides a unified interface to facilitate managing parameter configuration, searching the parameter user guide, and validating parameter effectiveness.

From v0.6.0, KubeBlocks supports `kbcli cluster configure` and `kbcli cluster edit-config` to configure parameters. The difference is that KubeBlocks configures parameters automatically with `kbcli cluster configure` but `kbcli cluster edit-config` provides a visualized way for you to edit parameters directly.

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
  kbcli cluster explain-config pg-cluster | head -n 20
  ```

* View the user guide of a specified parameter.
  
  ```bash
  kbcli cluster explain-config pg-cluster --param=max_connections
  ```
  
  <details>

  <summary>Output</summary>
  
  ```bash
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
  </details>

  * Allowed Values: It defines the valid value range of this parameter.
  * Dynamic: The value of `Dynamic` in `Configure Constraint` defines how the parameter configuration takes effect. There are two different configuration strategies based on the effectiveness type of modified parameters, i.e. **dynamic** and **static**.
    * When `Dynamic` is `true`, it means the effectiveness type of parameters is **dynamic** and can be configured online.
    * When `Dynamic` is `false`, it means the effectiveness type of parameters is **static** and a pod restarting is required to make the configuration effective.
  * Description: It describes the parameter definition.

## Configure parameters

### Configure parameters with configure command

The example below takes configuring `max_connections` as an example.

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
   kbcli cluster configure pg-cluster  --set=max_connections=5
   error: failed to validate updated config: [failed to cue template render configure: [pg.acllog-max-len: invalid value 5 (out of bound 6-8388607):
    343:34
   ]
   ]
   ```

   :::

3. View the status of the parameter configuration.

   `Status.Progress` and `Status.Status` shows the overall status of the parameter configuration and `Conditions` show the details.

   When the `Status.Status` shows `Succeed`, the configuration is completed.

   ```bash
   kbcli cluster describe-ops pg-cluster-reconfiguring-fq6q7 -n default
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

4. Connect to the database to verify whether the parameter is configured as expected.

   The whole searching process has a 30-second delay since it takes some time for kubelet to synchronize modifications to the volume of the pod.

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

### Configure parameters with edit-config command

For your convenience, KubeBlocks offers a tool `edit-config` to help you configure parameters in a visualized way.

For Linux and macOS, you can edit configuration files by vi. For Windows, you can edit files on the notepad.

1. Edit the configuration file.

   ```bash
   kbcli cluster edit-config pg-cluster
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
   kbcli cluster connect pg-cluster
   ```

:::note

1. For the `edit-config` function, static parameters and dynamic parameters cannot be edited at the same time.
2. Deleting a parameter will be supported later.

:::

## View history and compare differences

After the configuration is completed, you can search the configuration history and compare the parameter differences.

View the parameter configuration history.

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
shared_buffers    256MB                            512MB
```
