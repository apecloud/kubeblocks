# Deploy and manage a cluster

`dbctl cluster` command is used to create and manage the database cluster created by KubeBlocks.

## Create a cluster

Run this command to create a cluster.

```
dbctl cluster create NAME [flags]
```

| **Flag**             |  **Default**              |  **Description**                          |
| :--                  | :--                       |  :--                                      |
| cluster-definition   | wesql-clusterdefintiion   | The quoted `ClusterDefinition`.           |
| app-version          | wesql-appersion-8.0.29    | The default value is the latest version of WeSQL. It will change when a new version is released. |
| termination-policy   | Halt                      | The halt strategy.                        |
| components           | N/A                       | It specifies the path of the YAML file and is used to configure `component`. |
<!--| class                | N/A                       | The smallest class is set as the defalut. |

> **Note**
> `class` stands for the built-in specifications, including resource and node amount. But this feature is not supported.-->

_Example_

Here is an example of how to create a KubeBlocks cluster.

  _Steps_:

  1. Prepare a YAML file for configuring a component. You can find the following example file in GitHub repository `KubeBlocks/examples/dbaas`.

```
     - name: wesql-demo
      type: replicasets
      monitor: false
      volumeClaimTemplates:
        - name: data
          spec:
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 1Gi
            volumeMode: Filesystem
```

  2. Run this command to create a cluster in the default specification and engine.

```
    dbctl cluster create wesql-cluster --components=mycluster.yaml
```

  > **Note:**
  > You can specify engine type and version by adding `--cluster-definition` and `--app-version` flags. For example,
  >
  > ```
  > dbctl cluster create wesql-cluster --cluster-definiton=wesql-clusterdefinition --app-version=wesql-appversion-8.0.29 > --components=mycluster.yaml
  > ```

## Delete a cluster

Run this command to delete a cluster.

```
dbctl cluster delete NAME
```

_Example_

Delete a specified database cluster.

```
dbctl cluster delete wesql-demo
```

## Describe a cluster

`describe` is used to view the cluster information.

```
dbctl cluster describe NAME
```

## List all clusters

`list` is used to view the cluster list.

```
dbctl cluster list [flags]
```

| **Flag**     |  **Default**  |  **Description**                               |
| :--          | :--           |  :--                                           |
| -A           | N/A           | Get the data in all namespaces.                |
| -o           | N/A           | Set the output format. `YAML`, `JSON`, and `Wide` are supported. |
| --no-headers | False         | The system does not output header information. |

## Connect to a cluster

***Commands below do not use `dbctl` command. Need further development.***

Run this command to allow your local host to access the database instances.

```
kubectl --kubeconfig ~/.kube/dbctl-playground port-forward --address 0.0.0.0 service/mycluster 3306
```

Run the command below to connect to the database instance in another terminal.

```
  MYSQL_ROOT_PASSWORD=$(kubectl --kubeconfig ~/.kube/dbctl-playground get secret --namespace default mycluster-cluster-secret -o jsonpath="{.data.rootPassword}" | base64 -d)
  mysql -h 127.0.0.1 -uroot -p"$MYSQL_ROOT_PASSWORD"
```

## Restart a cluster

Run the command below to restart a cluster.

```
dbctl cluster restart NAME 
```

## Upgrade a cluster

You can specify an appversion and upgrade the cluster to this version.

```
dbctl cluster upgrade NAME --app-version=<AppVersionName>
```

## Vertically scale a cluster

Specify your requirements with options to vertically scale a cluster.

_Example_

```
dbctl cluster vertical-scaling wesql-demo \
--component-names="replicasets" \
--requests.memory="300Mi" --requests.cpu="0.3" \
--limits.memory="500Mi" --limits.cpu="0.5"
```

## Horizontally scale a cluster

Specify a cluster and its role group to horizontally scale this cluster.

_Example_

```
dbctl cluster horizontal-scaling wesql-demo \
 --component-names="replicasets" \
 --role-group-names="primary" --replicas=3
```

`role-group-names` stands for the roleGroupNames of the node to be expanded. It is an array and needs to be seperated by commas.

`role-group-replicas` stands for the node amount that the specified roleGroup needs to expand to.

## Expand the volume of a cluster

You can use `component-names` to specify a cluster and `storage` to specify the expected volume expansion size. 
`vct-names` stands for the name of the VolumeClaimTemplate. It is an array and needs to be seperated by commas. 

_Example_

```
dbctl cluster volume-expansion wesql-demo --component-names="replicasets" \
--vct-names="data" --storage="2Gi"
```

## Reference

For detailed information about `dbctl cluster`, refer to the CLI reference book.

- [KubeBlocks commands overview](../cli/kubeblocks%20commands%20overview.md)
- [`dbctl cluster create`](../cli/dbctl_cluster_create.md)
- [`dbctl cluster delete`](../cli/dbctl_cluster_delete.md)
- [`dbctl cluster describe`](../cli/dbctl_cluster_describe.md)
- [`dbctl cluster list`](../cli/dbctl_cluster_list.md)
- [`dbctl cluster connect`](../cli/dbctl_cluster_connect.md)
- [`dbctl cluster restart`](../cli/dbctl_cluster_connect.md)
- [`dbctl cluster upgrade`](../cli/dbctl_cluster_upgrade.md)
- [`dbctl cluster vertical-scaling`](../cli/dbctl_cluster_vertical-scaling.md)
- [`dbctl cluster horizontal-scaling`](../cli/dbctl_cluster_horizontal-scaling.md)
- [`dbctl cluster volume-expansion`](../cli/dbctl_cluster_volume-expansion.md)