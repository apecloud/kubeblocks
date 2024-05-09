---
title: Configure cluster parameters
description: Configure cluster parameters
keywords: [kafka, parameter, configuration, reconfiguration]
sidebar_position: 1
---

# Configure cluster parameters

The KubeBlocks configuration function provides a set of consistent default configuration generation strategies for all the databases running on KubeBlocks and also provides a unified parameter configuration interface to facilitate managing parameter configuration, searching the parameter user guide, and validating parameter effectiveness.

From v0.6.0, KubeBlocks supports `kbcli cluster configure` and `kbcli cluster edit-config` to configure parameters. The difference is that KubeBlocks configures parameters automatically with `kbcli cluster configure` but `kbcli cluster edit-config` provides a visualized way for you to edit parameters directly.

## View parameter information

View the current configuration file of a cluster.

```bash
kbcli cluster describe-config mykafka  
```

From the meta information, the cluster `mykafka` has a configuration file named `server.properties`.

You can also view the details of this configuration file and parameters.

* View the details of the current configuration file.

   ```bash
   kbcli cluster describe-config mykafka --show-detail
   ```

* View the parameter description.

  ```bash
  kbcli cluster explain-config mykafka | head -n 20
  ```

* View the user guide of a specified parameter.
  
  ```bash
  kbcli cluster explain-config mykafka --param=log.cleanup.policy
  ```

  <details>

  <summary>Output</summary>

  ```bash
  template meta:
    ConfigSpec: kafka-configuration-tpl	ComponentName: broker	ClusterName: mykafka

  Configure Constraint:
    Parameter Name:     log.cleanup.policy
    Allowed Values:     "compact","delete"
    Scope:              Global
    Dynamic:            false
    Type:               string
    Description:        The default cleanup policy for segments beyond the retention window. A comma separated list of valid policies. 
  ```
  
  </details>

  * Allowed Values: It defines the valid value range of this parameter.
  * Dynamic: The value of `Dynamic` in `Configure Constraint` defines how the parameter configuration takes effect. Currently, Kafka only supports static strategy, i.e. `Dynamic` is `false`. Restarting is required to make the configuration effective.
  * Description: It describes the parameter definition.

## Configure parameters

### Configure parameters with configure command

1. View the current value of `log.cleanup.policy`.

   ```bash
   kbcli cluster describe-config mykafka --show-detail | grep log.cleanup.policy
   >
   log.cleanup.policy=delete
   ```

2. Adjust the value of `log.cleanup.policy`.

   ```bash
   kbcli cluster configure mykafka --set=log.cleanup.policy=compact
   ```

   :::note

   Make sure the value you set is within the Allowed Values of this parameter. Otherwise, the configuration may fail.

   :::

3. View the status of the parameter configuration.

   `Status.Progress` and `Status.Status` shows the overall status of the parameter configuration and Conditions show the details.

   When the `Status.Status` shows `Succeed`, the configuration is completed.

   <details>

   <summary>Output</summary>

   ```bash
   # In progress
   kbcli cluster describe-ops mykafka-reconfiguring-wvqns -n default
   >
   Spec:
     Name: mykafka-reconfiguring-wvqns	NameSpace: default	Cluster: mykafka	Type: Reconfiguring

   Command:
     kbcli cluster configure mykafka --components=broker --config-spec=kafka-configuration-tpl --config-file=server.properties --set log.cleanup.policy=compact --namespace=default

   Status:
     Start Time:         Sep 14,2023 16:28 UTC+0800
     Duration:           5s
     Status:             Running
     Progress:           0/1
                         OBJECT-KEY   STATUS   DURATION   MESSAGE
   ```

   ```bash
   # Parameter reconfiguration is completed
   kbcli cluster describe-ops mykafka-reconfiguring-wvqns -n default
   >
   Spec:
     Name: mykafka-reconfiguring-wvqns	NameSpace: default	Cluster: mykafka	Type: Reconfiguring

   Command:
     kbcli cluster configure mykafka --components=broker --config-spec=kafka-configuration-tpl --config-file=server.properties --set log.cleanup.policy=compact --namespace=default

   Status:
     Start Time:         Sep 14,2023 16:28 UTC+0800
     Completion Time:    Sep 14,2023 16:28 UTC+0800
     Duration:           25s
     Status:             Succeed
     Progress:           1/1
                         OBJECT-KEY   STATUS   DURATION   MESSAGE
   ```

   </details>

4. View the configuration file to verify whether the parameter is configured as expected.

   The whole searching process has a 30-second delay.

   ```bash
   kbcli cluster describe-config mykafka --show-detail | grep log.cleanup.policy
   >
   log.cleanup.policy = compact
   mykafka-reconfiguring-wvqns   mykafka   broker      kafka-configuration-tpl   server.properties   Succeed   restart   1/1        Sep 14,2023 16:28 UTC+0800   {"server.properties":"{\"log.cleanup.policy\":\"compact\"}"}
   ```

### Configure parameters with edit-config command

For your convenience, KubeBlocks offers a tool `edit-config` to help you to configure parameter in a visualized way.

For Linux and macOS, you can edit configuration files by vi. For Windows, you can edit files on notepad.

1. Edit the configuration file.

   ```bash
   kbcli cluster edit-config mykafka
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
   kbcli cluster connect mykafka
   ```

   :::note

   1. For the `edit-config` function, static parameters and dynamic parameters cannot be edited at the same time.
   2. Deleting a parameter will be supported later.

   :::

## View history and compare differences

After the configuration is completed, you can search the configuration history and compare the parameter differences.

View the parameter configuration history.

```bash
kbcli cluster describe-config mykafka                 
```

From the above results, there are three parameter modifications.

Compare these modifications to view the configured parameters and their different values for different versions.

```bash
kbcli cluster diff-config mykafka-reconfiguring-wvqns mykafka-reconfiguring-hxqfx
>
DIFF-CONFIG RESULT:
  ConfigFile: server.properties	TemplateName: kafka-configuration-tpl	ComponentName: broker	ClusterName: mykafka	UpdateType: update

PARAMETERNAME         MYKAFKA-RECONFIGURING-WVQNS   MYKAFKA-RECONFIGURING-HXQFX
log.retention.hours   168                           200
```
