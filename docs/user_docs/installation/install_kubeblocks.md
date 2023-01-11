# Install KubeBlocks

`dbcli kubeblocks` command is used to manage KubeBlocks.

## Before you start

- Kubernetes 
  Kubernetes is installed.

- Storageclass
  This Kubernetes cluster should have `storageclass` by default. Run the following command to check whether `storageclass` exists in this Kubernetes cluster. 
  
  ```
  $ kubectl get storageclass
    NAME               PROVISIONER             RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
    gp2 (default)      kubernetes.io/aws-ebs   Delete          WaitForFirstConsumer   false                  5d2h
  ```

- `kbcli`
  The KubeBlocks CLI tool, [`kbcli`](install_kbcli.md), is installed.

- Playground
  A [KubeBlocks playground](install_playground.md) is installed.

## Install KubeBlocks

Run this command to install KubeBlocks.

```
kbcli kubeblocks install 
```

To enable database monitoring by default, set `--monitor=true` and run the following command. For detailed information, see [Monitor database](../database_observability/monitor_database.md).

```
kbcli kubeblocks install --monitor=true
```


## Uninstall KubeBlocks

Run this command to uninstall `KubeBlocks`.

```
kbcli kubeblocks unistall
```

## Next step

You can [create and manage a KubeBlocks cluster](create_and_manege_a_cluster.md).

## Reference

Refer to the following links to find detailed information about the CLIs used above.

- [`kbcli kubeblocks`](cli/../../cli/kbcli_kubeblocks.md)
- [`kbcli kubeblocks install`](cli/../../cli/kbcli_kubeblocks_install.md)
- [`kbcli kubeblocks uninstall`](cli/../../cli/kbcli_kubeblocks_uninstall.md)