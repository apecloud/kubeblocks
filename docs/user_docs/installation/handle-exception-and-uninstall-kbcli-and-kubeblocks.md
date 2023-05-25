---
title: Handle exception and uninstall kbcli and KubeBlocks
description: Handle exception and uninstall kbcli and KubeBlocks
keywords: [kbcli, kubeblocks, exception, uninstall]
sidebar_position: 4
sidebar_label: kbcli and KubeBlocks
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Handle an exception

If installing KubeBlocks by `kbcli` fails, run preflight checks to find whether your environment meets the requirements of running KubeBlocks and clusters.

```bash
kbcli kubeblocks preflight
```

Add the `--verbose` sub-command to output the details of the preflight checks.

```bash
kbcli kubeblocks preflight --verbose
```

***Result***

There are three types of results:

* `fail`: The environment requirements for installing KubeBlocks are not met, and KubeBlocks can only be installed after these requirements are met. It is required to check these items again and re-run the preflight checks.
* `warn`: The target environment affects the stability and performance of KubeBlocks and clusters, but running KubeBlocks and clusters is not affected, and you can continue the following installation.
* `congratulation`: All checks pass and you can continue the following installation.

# Uninstall KubeBlocks and kbcli

:::note

Uninstallation order:

1. Delete your cluster if you have created a cluster.

   ```bash
   kbcli cluster delete <name>
   ```

2. Uninstall KubeBlocks.

3. Uninstall `kbcli`.

:::

## Uninstall KubeBlocks

Uninstall KubeBlocks if you want to delete KubeBlocks after your trial.

<Tabs>
<TabItem value="kbcli" label="kbcli" default>

```bash
kbcli kubeblocks uninstall
```

</TabItem>

<TabItem value="helm" label="helm" default>

```bash
helm uninstall kubeblocks -n kb-system
```

</TabItem>

## Uninstall kbcli

Uninstall `kbcli` if you want to delete `kbcli` after your trial.

<TabItem value="macOS" label="macOS" default>

<TabItem value="cURL" label="cURL" default>

```bash
sudo rm /usr/local/bin/kbcli
```

</TabItem>

<TabItem value="Homebrew" label="Homebrew">

```bash
brew uninstall kbcli
```

</TabItem>

</TabItem>

<TabItem value="Windows" label="Windows">

1. Go to the `kbcli` installation path and delete the installation folder.

   * If you install `kbcli` by script, go to `C:\Program Files` and delete the `kbcli-windows-amd64` folder.
   * If you customize the installation path, go to your specified path and delete the installation folder.

2. Delete the environment variable.

   1. Click the Windows icon and click **System**.
   2. Go to **Settings** -> **Related Settings** -> **Advanced system settings**.
   3. On the **Advanced** tab, click **Environment Variables**.
   4. Double-click **Path** in **User variables** or **System variables** list.
      * If you install `kbcli` by script, double-click **Path** in **User variables**.
      * If you customize the installation path, double-click **Path** based on where you created the variable before.
   5. Select `C:\Program Files\kbcli-windows-amd64` or your customized path and delete it. This operation requires double confirmation.

</TabItem>

</Tabs>
