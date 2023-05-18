---
title: Enable add-ons when installing KubeBlocks
description: Enable add-ons when installing KubeBlocks
keywords: [addons, enable, KubeBlocks, prometheus, s3, alertmanager,]
sidebar_position: 5
sidebar_label: Enable add-ons 
---



# Enable add-ons

An add-on provides extension capabilities, i.e., manifests or application software, to the KubeBlocks control plane.

By default, all add-ons supported are automatically installed.

To list supported add-ons, run `kbcli addon list` command.

**Example**

```bash
kbcli addon list
```

:::note

Some add-ons have an environment requirement. If a certain requirement is not met, the automatic installation is invalid.

:::

You can perform the following steps to check and enable the add-on.

***Steps:***

1. Check the *Installable* part in the output information.
  
    **Example**

    ```bash
    kbcli addon describe snapshot-controller
    ```

    For certain add-ons, the installable part might say when the kubeGitVersion content includes *eks* and *ack*, the auto-install is enabled.

    In this case, you can check the version of the Kubernetes cluster.

    ```bash
    kubectl version -ojson | jq '.serverVersion.gitVersion'
    >
    "v1.24.4+eks"
    >
    ```

    As the printed output suggested, *eks* is included. And you can go on with the next step. In the case that *eks* is not included, it is invalid to enable the add-on.

2. To enable the add-on, use `kbcli addon enable`.

    **Example**

    ```bash
    kbcli addon enable snapshot-controller
    ```

3. List the add-ons again to check whether it is enabled.

    ```bash
    kbcli addon list
    ```
