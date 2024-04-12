---
title: Configure cluster parameters
description: Configure cluster parameters
keywords: [mysql, parameter, configuration, reconfiguration]
sidebar_position: 1
sidebar_label: Configuration
---

# Configure cluster parameters

This guide shows how to configure cluster parameters by creating an opsRequest.

KubeBlocks supports dynamic parameter configuration. When the specifications of a database instance change (e.g., a user vertically scales a cluster), KubeBlocks automatically matches the appropriate parameter template based on the new specifications. This is because different specifications of a database instance may require different optimal parameter configurations to optimize performance and resource utilization. When you choose a different database instance specification, KubeBlocks automatically detects and determines the best database configuration for the new specification, ensuring optimal performance and configuration of the database under the new specifications.

The benefit of this feature is that it simplifies the process of adjusting database specifications. You don't need to manually change database parameters as KubeBlocks handles the updates and configurations automatically to adapt to the new specifications. This saves time and effort and reduces performance issues caused by incorrect parameter settings.

It's important to note that the dynamic adjustment of database parameters doesn't apply to all parameters. Some parameters may require manual adjustment and configuration. Additionally, if you manually modify database parameters, KubeBlocks may overwrite your modifications when refreshing the database parameter template. Therefore, when using the dynamic adjustment feature, it is recommended to back up and record your custom parameter settings so that you can restore them if needed.

## Before you start

1. [Install KubeBlocks](./../../installation/install-with-helm/install-kubeblocks-with-helm.md).
2. [Create a MySQL cluster](./../cluster-management/create-and-connect-a-mysql-cluster.md).

## Configure cluster parameters with OpsRequest

1. Define the OpsRequest file and configure the parameters in the OpsRequest in a yaml file named `mycluster-configuring-demo.yaml`. In this example, `max_connections` is configured as `600`.

```bash
apiVersion: apps.kubeblocks.io/v1alpha1
kind: OpsRequest
metadata:
  name: mycluster-configuring-demo
  namespace:kb-system
spec:
  clusterRef: mycluster
  reconfigure:
    componentName: mysql
    configurations:
    - keys:
      - key: my.conf
        parameters:
        - key: max_connections
          value:"600"
      name: mysql-configuration
  ttlSecondBeforeAbort: 0
  type: Reconfiguring
EOF
```

* `metadata.name` specifies the name of this OpsRequest.
* `metadata.namespace` specifies the namespace where this cluster is created.
* `spec.clusterRef` specifies the cluster name.
* `spec.reconfigure` specifies the configuration information. `componentName`specifies the component name of this cluster. `configurations.keys.key` specifies the configuration file. `configurations.keys.parameters` specifies the parameters you want to edit. `configurations.keys.name` specifies the configuration spec name.

2. Perform the configuration opsRequest.

```bash
kubectl apply -f mycluster-configuring-demo.yaml
```

3. Connect to this cluster to verify whether the configuration takes effect.

3.1. Get the username and password.

```bash
kubectl get secrets -n demo mysql-cluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
>
root

kubectl get secrets -n demo mysql-cluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
>
2gvztbvz
```

3.2. Connect to this cluster and verify whether the parameters are configured as expected.

```bash
kubectl exec -ti -n demo mycluster-mysql-0 -- bash

mysql -uroot -p2gvztbvz
>
mysql> show variables like 'max_connections';
+-----------------+-------+
| Variable_name   | Value |
+-----------------+-------+
| max_connections | 600   |
+-----------------+-------+
1 row in set (0.00 sec)
```

## Configure cluster parameters by configuration file

1. Get the configuration file of this cluster.

```bash
kubectl edit configurations.apps.kubeblocks.io mycluster-mysql
```

2. Configure parameters according to your needs.

```yaml
TBD
```

3. Connect to this cluster to verify whether the configuration takes effect.

3.1. Get the username and password.

```bash
kubectl get secrets -n demo mysql-cluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
>
root

kubectl get secrets -n demo mysql-cluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
>
2gvztbvz
```

3.2. Connect to this cluster and verify whether the parameters are configured as expected.

```bash
kubectl exec -ti -n demo mycluster-mysql-0 -- bash

mysql -uroot -p2gvztbvz
>
mysql> show variables like 'max_connections';
+-----------------+-------+
| Variable_name   | Value |
+-----------------+-------+
| max_connections | 600   |
+-----------------+-------+
1 row in set (0.00 sec)
```

:::note

Just in case you cannot find the configuration file of your cluster, you can use `kbcli` to view the current configuration file of a cluster.

```bash
kbcli cluster describe-config mycluster  
```

From the meta information, the cluster `mycluster` has a configuration file named `my.cnf`.

You can also view the details of this configuration file and parameters.

* View the details of the current configuration file.

   ```bash
   kbcli cluster describe-config mycluster --show-detail
   ```

* View the parameter description.

  ```bash
  kbcli cluster explain-config mycluster | head -n 20
  ```

* View the user guide of a specified parameter.
  
  ```bash
  kbcli cluster explain-config mycluster --param=innodb_buffer_pool_size --config-spec=mysql-consensusset-config
  ```

  `--config-spec` is required to specify a configuration template since ApeCloud MySQL currently supports multiple templates. You can run `kbcli cluster describe-config mycluster` to view the all template names.

  <details>

  <summary>Output</summary>

  ```bash
  template meta:
    ConfigSpec: mysql-consensusset-config        ComponentName: mysql        ClusterName: mycluster

  Configure Constraint:
    Parameter Name:     innodb_buffer_pool_size
    Allowed Values:     [5242880-18446744073709552000]
    Scope:              Global
    Dynamic:            false
    Type:               integer
    Description:        The size in bytes of the memory buffer innodb uses to cache data and indexes of its tables  
  ```
  
  </details>

  * Allowed Values: It defines the valid value range of this parameter.
  * Dynamic: The value of `Dynamic` in `Configure Constraint` defines how the parameter configuration takes effect. There are two different configuration strategies based on the effectiveness type of modified parameters, i.e. **dynamic** and **static**.
    * When `Dynamic` is `true`, it means the effectiveness type of parameters is **dynamic** and can be configured online.
    * When `Dynamic` is `false`, it means the effectiveness type of parameters is **static** and a pod restarting is required to make the configuration effective.
  * Description: It describes the parameter definition.
:::
