# Create and manage a cluster

The `kbcli cluster` command is used to create and manage the database cluster created by KubeBlocks.

## Create a cluster

Run this command to create a cluster.

```
kbcli cluster create NAME [flags]
```

| **Flag**             |  **Default**  |  **Description**                                                             |
| :--                  | :--           |  :--                                                                         |
| cluster-definition   | N/A           | It is required. It refers to the quoted `ClusterDefinition`.                 |
| cluster-version      | N/A           | It is required. It refers to the version applied to the cluster.             |
| termination-policy   | N/A           | It is required. It refers to the termination policy.                         |
| components           | N/A           | It specifies the path of the YAML file and is used to configure `component`. |
<!--| class            | N/A           | The smallest class is set as the default.                                    |

> **Note**
> `class` stands for the built-in specifications, including resource and node amount. But this feature is not supported.-->

_Example_

Here is an example of how to create a KubeBlocks cluster using a YAML file.

  _Steps_:

  1. Prepare a YAML file for configuring a component. 

    ```
    - name: ac-mysql
      type: replicasets
      replicas: 1
      volumeClaimTemplates:
      - name: data
        spec:
          accessModes:
            - ReadWriteOnce
          resources:
            requests:
              storage: 1Gi
    ```

  2. Specify the `cluster-definition`, `cluster-version`, `terminationPolicy`, and `components` and run `kbcli cluster create NAME` to create a cluster.
   
    ```
    kbcli cluster create ac-cluster --cluster-definition=apecloud-mysql  --cluster-version=ac-mysql-8.0.30 --set-file=mycluster.yaml --termination-policy=WipeOut
    ```

## Delete a cluster

Run this command to delete a cluster.

```
kbcli cluster delete NAME
```

_Example_

Add the cluster name and run this command to delete this specified database cluster.

```
kbcli cluster delete ac-cluster
```

## Describe a cluster

Run this command to view the cluster information.

```
kbcli cluster describe NAME
```

## List all clusters

Run this command to view the cluster list.

```
kbcli cluster list [flags]
```

| **Flag**     |  **Default**  |  **Description**                               |
| :--          | :--           |  :--                                           |
| -A           | N/A           | Get the data in all namespaces.                |
| -o           | table         | Set the output format. `table`, `YAML`, `JSON`, and `Wide` are supported. |

## Connect to a cluster

Run the command below to allow your local host to access the database instances.

```
kbcli cluster connect NAME
```

You can use the option, `-i`, to specify an instance name. For example, 

```
kbcli cluster connect ac-cluster -i ac-cluster-ac-mysql-0
```

Run the command below to view the instance list.

```
kbcli cluster list-instances ac-cluster
```

## Restart a cluster

Run this command to restart a cluster.

```
kbcli cluster restart NAME 
```

## Upgrade a cluster

Specify a cluster version by using the option `--cluster-version` and upgrade the cluster to this version. For more options information, see [`kbcli cluster upgrade`](../cli/kbcli_cluster_upgrade.md).

```
kbcli cluster upgrade NAME --cluster-version=<ClusterVersionName>
```

## Vertically scale a cluster

Specify your requirements by using options to scale up a cluster. For more options information, see [`kbcli cluster vertical-scaling`](../cli/kbcli_cluster_vertical-scaling.md).

_Example_

```
kbcli cluster vertical-scaling ac-cluster \
--component-names="ac-mysql" \
--requests.memory="300Mi" --requests.cpu="0.3" \
--limits.memory="500Mi" --limits.cpu="0.5"
```

## Horizontally scale a cluster

Specify a cluster and its role group by using options to scale out this cluster. For more options information, see [`kbcli cluster horizontal-scale`](../cli/kbcli_cluster_horizontal-scaling.md).

- The `role-group-names` option stands for the roleGroupNames of the nodes that need to be expanded. It is an array and needs to be separated by commas.

- The `role-group-replicas` option stands for the node amount that the specified roleGroup needs to expand to.

_Example_

```
kbcli cluster horizontal-scaling ac-cluster \
--component-names="ac-mysql" \
--role-group-names="primary" --replicas=3
```

## Expand the cluster volume

Run `kbcli cluster volume-expansion` to expand the cluster volume. For more options information, see [`kbcli cluster volume-expansion`](../cli/kbcli_cluster_volume-expansion.md).

- The `component-names` option can be used to specify a cluster.
- The `storage` option can be used to specify the expected volume expansion size. 
- The `volume-claim-template-names` option stands for the name of the VolumeClaimTemplate. It is an array and needs to be separated by commas. 

_Example_

```
kbcli cluster volume-expansion ac-cluster \
--component-names="ac-mysql" \
--volume-claim-template-names="data" \
--storage="2Gi"
```

## Reference

For detailed information about `kbcli cluster` commands, refer to the CLI reference book.

- [KubeBlocks commands overview](../cli/kubeblocks_commands_overview.md)
- [`kbcli cluster create`](../cli/kbcli_cluster_create.md)
- [`kbcli cluster delete`](../cli/kbcli_cluster_delete.md)
- [`kbcli cluster describe`](../cli/kbcli_cluster_describe.md)
- [`kbcli cluster list`](../cli/kbcli_cluster_list.md)
- [`kbcli cluster connect`](../cli/kbcli_cluster_connect.md)
- [`kbcli cluster restart`](../cli/kbcli_cluster_connect.md)
- [`kbcli cluster upgrade`](../cli/kbcli_cluster_upgrade.md)
- [`kbcli cluster vertical-scaling`](../cli/kbcli_cluster_vertical-scaling.md)
- [`kbcli cluster horizontal-scaling`](../cli/kbcli_cluster_horizontal-scaling.md)
- [`kbcli cluster volume-expansion`](../cli/kbcli_cluster_volume-expansion.md)
