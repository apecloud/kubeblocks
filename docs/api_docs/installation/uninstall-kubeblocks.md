---
title: Uninstall KubeBlocks
description: Handle exception and uninstall KubeBlocks
keywords: [kubeblocks, exception, uninstall]
sidebar_position: 5
sidebar_label: Uninstall KubeBlocks
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Uninstall KubeBlocks

Uninstallation order:

1. Delete your cluster if you have created a cluster.

   ```bash
   kbcli cluster delete <name>
   ```

2. Uninstall KubeBlocks.

## Uninstall KubeBlocks

<Tabs>

<TabItem value="Helm" label="Helm" default>

Delete all the clusters and resources created before performing the following command, otherwise the uninstallation may not be successful.

```bash
helm uninstall kubeblocks --namespace kb-system
```

Helm does not delete CRD objects. You can delete the ones KubeBlocks created with the following commands:

```bash
kubectl get crd -o name | grep kubeblocks.io | xargs kubectl delete
```

</TabItem>

<TabItem value="YAML" label="YAML">

You can generate YAMLs from the KubeBlocks chart and uninstall using `kubectl`.

```bash
helm template kubeblocks kubeblocks/kubeblocks --namespace kb-system | kubectl delete -f -
```

</TabItem>

</Tabs>
