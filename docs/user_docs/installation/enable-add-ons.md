---
title: Enable add-ons
description: Enable KubeBlocks add-ons
sidebar_position: 2
sidebar_label: Enable add-ons
---

# Enable add-ons

An add-on provides extension capabilities, i.e., manifests or application software, to KubeBlocks control plane. 
By default, all add-ons supported are automatically installed.
To list supported add-ons, run `kbcli addon list` command.

**Example**

```
kbcli addon list
```

:::note

Some add-ons have an environment requirement. If a certain requirement is not met, the automatic installation is invalid.

:::

You can perform the following steps to check and enable the add-on.

***Steps:***

1. Run `kbcli addon describe`, and check the *Installable* part in the output information.
  
    **Example**

    ```
    kbcli addon describe snapshot-controller
    ```
    
    For certain add-ons, the installable part might say when the kubeGitVersion content includes *eks* and *ack*, the auto-install is enabled.
    In this case, you can check the version of the Kubernetes cluster, and run the following command.
    ```
    kubectl version -ojson | jq '.serverVersion.gitVersion'
    >
    "v1.24.4+eks"
    >
    ```
    As the printed output suggested, *eks* is included. And you can go on with the next step. In case that *eks* is not included, it is invalid to enable the add-on.

2. To enable the add-on, use `kbcli addon enable`.
   
    **Example**

    ```
    kbcli addon enable snapshot-controller
    ```

3. List the add-ons to check whether it is enabled.

    ```
    kbcli addon list
    ```
