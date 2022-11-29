# Deploy KubeBlocks

`kubeblocks` command is used to manage KubeBlocks.

> **Note** <br>
> `kubeblocks` command is named as `dbaas` in v0.2.0-alpha.3 and below versions.

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

## Install KubeBlocks

Run this command to install KubeBlocks.

```
dbctl kubeblocks install 
```

To enable monitor for database observability, run the following command. For detailed information, see _Monitor database_. ***Links to be added.***

```
dbctl kubeblocks install --monitor=true
```


## Uninstall KubeBlocks

Run this command to uninstall `KubeBlocks`.

```
dbctl kubeblocks unistall
```

## Reference

Refer to the following links to find detailed information about the CLIs used above.

- [`dbctl kubeblocks`](cli/../../cli/dbctl_kubeblocks.md)
- [`dbctl kubeblocks install`](cli/../../cli/dbctl_kubeblocks_install.md)
- [`dbctl kubeblocks uninstall`](cli/../../cli/dbctl_kubeblocks_uninstall.md)
- [`dbctl dbass`](../cli/dbctl_dbaas.md)
- [`dbctl dbass install`](../cli/dbctl_dbaas_install.md)
- [`dbctl dbass uninstall`](../cli/dbctl_dbaas_uninstall.md)