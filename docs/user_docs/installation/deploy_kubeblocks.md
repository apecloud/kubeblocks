# Deploy KubeBlocks

`KubeBlocks` is managed via `dbass` command.

## Dependencies

- K8s 
  K8s is installed.

- Storageclass
  This k8s cluster should have `storageclass` by default. Run the following command to check whether `storageclass` exists in this k8s cluster. 
  ```
  $ kubectl get storageclass
    NAME               PROVISIONER             RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
    gp2 (default)      kubernetes.io/aws-ebs   Delete          WaitForFirstConsumer   false                  5d2h
  ```

## Install KubeBlocks

Run this command to install `KubeBlocks`. ***#Whether the version in this command is required for installation?***

```
$ dbctl dbass install --version=0.2.0-alpha.0 
Installing KubeBlocks v0.2.0-alpha.0 ...
⡿  Install oci://yimeisun.azurecr.io/helm-chart/kubeblocks W1024 14:05:05.028500  ... 50779 warnings.go:70] policy/v1beta1 PodSecurityPolicy is deprecated in v1.21+, unavailable in v1.25+
⣻  Install oci://yimeisun.azurecr.io/helm-chart/kubeblocks W1024 14:05:46.753450  ... 50779 warnings.go:70] policy/v1beta1 PodSecurityPolicy is deprecated in v1.21+, unavailable in v1.25+
Install oci://yimeisun.azurecr.io/helm-chart/kubeblocks  ...OK

KubeBlocks v0.2.0-alpha.0 Install SUCCESSFULLY!
You can now create a database cluster by running the following command:
        dbctl cluster create <you cluster name>
        
        
KubeBlocks v0.2.0-alpha.0 Install SUCCESSFULLY!

1. Basic commands for cluster:
    dbctl cluster create -h     # help information about creating a database cluster
    dbctl cluster list          # list all database clusters
    dbctl cluster describe <cluster-name>  # get cluster information
    
2. Uninstall DBaaS:
    dbctl dbaas uninstall
```

If you need to enable the monitor by default when installing `KubeBlocks`, run the following command.

```
$ dbctl dbaas install --version=0.2.0-alpha.0 --monitor=true
Installing KubeBlocks v0.2.0-alpha.0 ...
⡿  Install oci://yimeisun.azurecr.io/helm-chart/kubeblocks W1024 14:05:05.028500  ... 50779 warnings.go:70] policy/v1beta1 PodSecurityPolicy is deprecated in v1.21+, unavailable in v1.25+
⣻  Install oci://yimeisun.azurecr.io/helm-chart/kubeblocks W1024 14:05:46.753450  ... 50779 warnings.go:70] policy/v1beta1 PodSecurityPolicy is deprecated in v1.21+, unavailable in v1.25+
Install oci://yimeisun.azurecr.io/helm-chart/kubeblocks  ...OK

KubeBlocks v0.2.0-alpha.0 Install SUCCESSFULLY!
You can now create a database cluster by running the following command:
        dbctl cluster create <you cluster name>
        
        
KubeBlocks v0.2.0-alpha.0 Install SUCCESSFULLY!

1. Basic commands for cluster:
    dbctl cluster create -h     # help information about creating a database cluster
    dbctl cluster list          # list all database clusters
    dbctl cluster describe <cluster-name>  # get cluster information
    
2. Uninstall DBaaS:
    dbctl dbaas uninstall

3. To view the Grafana console:
    export POD_NAME=...
    kubectl -A port-forward $POD_NAME 3000
    open http://127.0.0.1:3000
    User: admin, Password: abc

4. To view the Prometheus console
    export POD_NAME=...
    kubectl -A port-forward $POD_NAME 9090
    open http://127.0.0.1:9090
    
5. To view the Prometheus AlertManager console
    export ...
    kubectl -A port-forward $POD_NAME 9093
    open http://127.0.0.1:9093
```

## Unistall KubeBlocks

Run this command to unistall `KubeBlocks`.

```
dbctl dbass unistall
```

## Reference

Refer to the following links to find detailed information about the CLIs used above.

- [`dbctl dbass`](../cli/dbctl_dbaas.md)
- [`dbctl dbass install`](../cli/dbctl_dbaas_install.md)
- [`dbctl dbass unistall`](../cli/dbctl_dbaas_uninstall.md)

