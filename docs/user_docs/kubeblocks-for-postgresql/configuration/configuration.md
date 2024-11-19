---
title: Configure cluster parameters
description: Configure cluster parameters
keywords: [postgresql, parameter, configuration, reconfiguration]
sidebar_position: 1
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Configure cluster parameters

This guide shows how to configure cluster parameters.

From v0.9.0, KubeBlocks supports dynamic configuration. When the specification of a database instance changes (e.g. a user vertically scales a cluster), KubeBlocks automatically matches the appropriate configuration template based on the new specification. This is because different specifications of a database instance may require different optimal configurations to optimize performance and resource utilization. When you choose a different database instance specification, KubeBlocks automatically detects it and determines the best database configuration for the new specification, ensuring optimal performance and configuration of the database under the new specifications.

This feature simplifies the process of configuring parameters, which saves you from manually configuring database parameters as KubeBlocks handles the updates and configurations automatically to adapt to the new specifications. This saves time and effort and reduces performance issues caused by incorrect configuration.

But it's also important to note that the dynamic parameter configuration doesn't apply to all parameters. Some parameters may require manual configuration. Additionally, if you have manually modified database parameters before, KubeBlocks may overwrite your customized configurations when updating the database configuration template. Therefore, when using the dynamic configuration feature, it is recommended to back up and record your custom configuration so that you can restore them if needed.

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

## View parameter information

View the current configuration file of a cluster.

```bash
kbcli cluster describe-config mycluster -n demo
```

From the meta information, you can find the configuration files of this PostgreSQL cluster.

You can also view the details of this configuration file and parameters.

* View the details of the current configuration file.

   ```bash
   kbcli cluster describe-config mycluster -n demo --show-detail
   ```

* View the parameter description.

  ```bash
  kbcli cluster explain-config mycluster -n demo | head -n 20
  ```

* View the user guide of a specified parameter.
  
  ```bash
  kbcli cluster explain-config mycluster -n demo --param=max_connections
  ```
  
  <details>

  <summary>Output</summary>
  
  ```bash
  template meta:
    ConfigSpec: postgresql-configuration ComponentName: postgresql ClusterName: mycluster

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
   kbcli cluster connect mycluster -n demo
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
   kbcli cluster configure mycluster -n demo --set=max_connections=200
   ```

:::note

Make sure the value you set is within the Allowed Values of this parameter. If you set a value that does not meet the value range, the system prompts an error. For example,

```bash
kbcli cluster configure mycluster -n demo  --set=max_connections=5
>
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
   kbcli cluster describe-ops mycluster-reconfiguring-fq6q7 -n demo
   ```

   <details>

   <summary>Output</summary>

   ```bash
   Spec:
     Name: mycluster-reconfiguring-fq6q7 NameSpace: demo Cluster: mycluster Type: Reconfiguring

   Command:
     kbcli cluster configure mycluster -n demo --components=postgresql --config-specs=postgresql-configuration --config-file=postgresql.conf --set max_connections=100 --namespace=demo

   Status:
     Start Time:         Mar 17,2023 19:25 UTC+0800
     Completion Time:    Mar 17,2023 19:25 UTC+0800
     Duration:           2s
     Status:             Succeed
     Progress:           1/1
                         OBJECT-KEY   STATUS   DURATION   MESSAGE

   Conditions:
   LAST-TRANSITION-TIME         TYPE                 REASON                            STATUS   MESSAGE
   Mar 17,2023 19:25 UTC+0800   Progressing          OpsRequestProgressingStarted      True     Start to process the OpsRequest: mycluster-reconfiguring-fq6q7 in Cluster: mycluster
   Mar 17,2023 19:25 UTC+0800   Validated            ValidateOpsRequestPassed          True     OpsRequest: mycluster-reconfiguring-fq6q7 is validated
   Mar 17,2023 19:25 UTC+0800   Reconfigure          ReconfigureStarted                True     Start to reconfigure in Cluster: mycluster, Component: postgresql
   Mar 17,2023 19:25 UTC+0800   ReconfigureMerged    ReconfigureMerged                 True     Reconfiguring in Cluster: mycluster, Component: postgresql, ConfigSpec: postgresql-configuration, info: updated: map[postgresql.conf:{"max_connections":"200"}], added: map[], deleted:map[]
   Mar 17,2023 19:25 UTC+0800   ReconfigureSucceed   ReconfigureSucceed                True     Reconfiguring in Cluster: mycluster, Component: postgresql, ConfigSpec: postgresql-configuration, info: updated policy: <operatorSyncUpdate>, updated: map[postgresql.conf:{"max_connections":"100"}], added: map[], deleted:map[]
   Mar 17,2023 19:25 UTC+0800   Succeed              OpsRequestProcessedSuccessfully   True     Successfully processed the OpsRequest: mycluster-reconfiguring-fq6q7 in Cluster: mycluster
   ```

   </details>

4. Connect to the database to verify whether the parameter is configured as expected.

   It takes about 30 seconds for the configuration to take effect because the kubelet requires some time to sync changes in the ConfigMap to the Pod's volume.

   ```bash
   kbcli cluster connect mycluster -n demo
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
   kbcli cluster edit-config mycluster -n demo
   ```

   :::note

   If there are multiple components in a cluster, use `--components` to specify a component.

   :::

2. View the status of the parameter configuration.

   ```bash
   kbcli cluster describe-ops mycluster-reconfiguring-njk23 -n demo
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
kbcli cluster describe-config mycluster -n demo
```

<details>

<summary>Output</summary>

```bash
ConfigSpecs Meta:
CONFIG-SPEC-NAME            FILE                  ENABLED   TEMPLATE                    CONSTRAINT        RENDERED                                         COMPONENT    CLUSTER
postgresql-configuration    kb_restore.conf       false     postgresql-configuration    postgresql14-cc   mycluster-postgresql-postgresql-configuration    postgresql   mycluster
postgresql-configuration    pg_hba.conf           false     postgresql-configuration    postgresql14-cc   mycluster-postgresql-postgresql-configuration    postgresql   mycluster
postgresql-configuration    postgresql.conf       true      postgresql-configuration    postgresql14-cc   mycluster-postgresql-postgresql-configuration    postgresql   mycluster
postgresql-configuration    kb_pitr.conf          false     postgresql-configuration    postgresql14-cc   mycluster-postgresql-postgresql-configuration    postgresql   mycluster
postgresql-custom-metrics   custom-metrics.yaml   false     postgresql-custom-metrics                     mycluster-postgresql-postgresql-custom-metrics   postgresql   mycluster

History modifications:
OPS-NAME                        CLUSTER     COMPONENT    CONFIG-SPEC-NAME           FILE              STATUS    POLICY    PROGRESS   CREATED-TIME                 VALID-UPDATED
mycluster-reconfiguring-fq6q7   mycluster   postgresql   postgresql-configuration   postgresql.conf   Succeed             1/1        Mar 17,2023 19:25 UTC+0800   {"postgresql.conf":"{\"max_connections\":\"100\"}"}
mycluster-reconfiguring-bm84z   mycluster   postgresql   postgresql-configuration   postgresql.conf   Succeed             1/1        Mar 17,2023 19:27 UTC+0800   {"postgresql.conf":"{\"max_connections\":\"200\"}"}
mycluster-reconfiguring-cbqxd   mycluster   postgresql   postgresql-configuration   postgresql.conf   Succeed             1/1        Mar 17,2023 19:35 UTC+0800   {"postgresql.conf":"{\"max_connections\":\"500\"}"}
mycluster-reconfiguring-rcnzb   mycluster   postgresql   postgresql-configuration   postgresql.conf   Succeed   restart   1/1        Mar 17,2023 19:38 UTC+0800   {"postgresql.conf":"{\"shared_buffers\":\"512MB\"}"}
```

</details>

From the above results, there are three parameter modifications.

Compare these modifications to view the configured parameters and their different values for different versions.

```bash
kbcli cluster diff-config mycluster-reconfiguring-bm84z mycluster-reconfiguring-rcnzb -n demo
>
DIFF-CONFIG RESULT:
  ConfigFile: postgresql.conf TemplateName: postgresql-configuration ComponentName: postgresql ClusterName: mycluster UpdateType: update

PARAMETERNAME     MYCLUSTER-RECONFIGURING-BM84Z    MYCLUSTER-RECONFIGURING-RCNZB
max_connections   200                              500
shared_buffers    256MB                            512MB
```

</TabItem>

<TabItem value="Edit config file" label="Edit config file">

KubeBlocks supports configuring cluster parameters by editing its configuration file.

1. Get the configuration file of this cluster.

   ```bash
   kubectl edit configurations.apps.kubeblocks.io mycluster-postgresql -n demo
   ```

2. Configure parameters according to your needs. The example below adds the `spec.configFileParams` part to configure `max_connections`.

   ```yaml
   spec:
     clusterRef: mycluster
     componentName: postgresql
     configItemDetails:
     - configFileParams:
         my.cnf:
           parameters:
             max_connections: "600"
       configSpec:
         constraintRef: postgresql14-cc
         defaultMode: 292
         keys:
         - postgresql.conf
         name: postgresql-configuration
         namespace: kb-system
         templateRef: postgresql-configuration
         volumeName: postgresql-config
       name: postgresql-configuration
     - configSpec:
         defaultMode: 292
   ```

3. Connect to this cluster to verify whether the configuration takes effect.

   1. Get the username and password.

      ```bash
      kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.username}' | base64 -d
      >
      root

      kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.password}' | base64 -d
      >
      2gvztbvz
      ```

   2. Connect to this cluster and verify whether the parameters are configured as expected.

      ```bash
      kubectl exec -ti -n demo mycluster-postgresql-0 -- bash

      root@mycluster-postgresql-0:/home/postgres# psql -U postgres -W
      Password: tf8fhsv2
      >
      postgres=# show max_connections;
      max_connections
      -----------------
      600
      (1 row)
      ```

:::note

Just in case you cannot find the configuration file of your cluster, you can switch to the `kbcli` tab to view the current configuration file of a cluster.

:::

</TabItem>

<TabItem value="OpsRequest" label="OpsRequest">

1. Define an OpsRequest file and configure the parameters in the OpsRequest in a YAML file named `mycluster-configuring-demo.yaml`. In this example, `max_connections` is configured as `600`.

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: OpsRequest
   metadata:
     name: mycluster-configuring-demo
     namespace: demo
   spec:
     clusterName: mycluster
     reconfigure:
       componentName: postgresql
       configurations:
       - keys:
         - key: postgresql.conf
           parameters:
           - key: max_connections
             value: "600"
         name: postgresql-configuration
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

2. Apply this OpsRequest.

   ```bash
   kubectl apply -f mycluster-configuring-demo.yaml
   ```

3. Connect to this cluster to verify whether the configuration takes effect.

   1. Get the username and password.

      ```bash
      kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.username}' | base64 -d
      >
      postgres

      kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.password}' | base64 -d
      >
      tf8fhsv2
      ```

   2. Connect to this cluster and verify whether the parameters are configured as expected.

      ```bash
      kubectl exec -ti -n demo mycluster-postgresql-0 -- bash

      root@mycluster-postgresql-0:/home/postgres# psql -U postgres -W
      Password: tf8fhsv2
      >
      postgres=# show max_connections;
      max_connections
      -----------------
      600
      (1 row)
      ```

:::note

Just in case you cannot find the configuration file of your cluster, you can switch to the `kbcli` tab to view the current configuration file of a cluster.

:::

</TabItem>

</Tabs>
