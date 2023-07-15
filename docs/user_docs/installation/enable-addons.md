---
title: Enable add-ons when installing KubeBlocks
description: Enable add-ons when installing KubeBlocks
keywords: [addons, enable, KubeBlocks, prometheus, s3, alertmanager,]
sidebar_position: 4
sidebar_label: Enable add-ons 
---

# Enable add-ons

An add-on provides extension capabilities, i.e., manifests or application software, to the KubeBlocks control plane.

:::note

Using `kbcli playground init` command to install KubeBlocks enables prometheus and grafana for observability by default. But if you install KubeBlocks with `kbcli kubeblocks install`, prometheus and grafana are disabled by default.

:::

To list supported add-ons, run `kbcli addon list` command.

```bash
kbcli addon list
NAME                           TYPE   STATUS     EXTRAS         AUTO-INSTALL   AUTO-INSTALLABLE-SELECTOR                                                
snapshot-controller            Helm   Enabling                  false          {key=KubeGitVersion,op=DoesNotContain,values=[tke]}                      
kubeblocks-csi-driver          Helm   Disabled   node           false          {key=KubeGitVersion,op=Contains,values=[eks]}                            
grafana                        Helm   Enabling                  true                                                                                    
prometheus                     Helm   Enabling   alertmanager   true                                                                                    
migration                      Helm   Disabled                  false                                                                                   
postgresql                     Helm   Enabling                  true                                                                                    
mongodb                        Helm   Enabling                  true                                                                                    
aws-load-balancer-controller   Helm   Disabled                  false          {key=KubeGitVersion,op=Contains,values=[eks]}                            
apecloud-mysql                 Helm   Enabling                  true                                                                                    
redis                          Helm   Enabling                  true                                                                                    
milvus                         Helm   Enabling                  true                                                                                    
weaviate                       Helm   Enabling                  true                                                                                    
csi-hostpath-driver            Helm   Disabled                  false          {key=KubeGitVersion,op=DoesNotContain,values=[eks aliyun gke tke aks]}   
nyancat                        Helm   Disabled                  false                                                                                   
csi-s3                         Helm   Disabled                  false                                                                                   
alertmanager-webhook-adaptor   Helm   Enabled                   true                                                                                    
qdrant                         Helm   Enabled                   true         
```

`kubectl` command to list all supported add-ons:
```bash
kubectl get addons
```

:::note

Some add-ons have environment requirements. If a certain requirement is not met, the automatic installation is invalid. So you can check the *AUTO-INSTALLABLE-SELECTOR* item of the output. 
You can use `kbcli addon describe [addon name]` command to check the installation requirement.

`kubectl` command to describe the add-on.
```bash
kubectl describe addon [addon name]
```

:::

**To manually enable or disable add-ons**
***Steps:***
1. To enable the add-on, use `kbcli addon enable`.

    ```bash
    kbcli addon enable snapshot-controller
    ```

Use `kubectl` command to enable the add-on:
```bash
kubectl patch addon snapshot-controller --type=merge -p '{"spec":{"install":{"enabled":true}}}' 
```

    To disable the add-on, use `kbcli addon disable`.

Use `kubectl` command to disable the add-on:
```bash
kubectl patch addon snapshot-controller --type=merge -p '{"spec":{"install":{"enabled":false}}}' 
```

You can run `kubectl get addon snapshot-controller` to check the status of the add-on.


2. List the add-ons again to check whether it is enabled.

    ```bash
    kbcli addon list
    ```
