---
title: Install kbcli and KubeBlocks on a new Kubernetes cluster
description: Install kbcli and KubeBlocks on a new Kubernetes cluster and the environment is clean
keywords: [install, KubeBlocks, kbcli, on a new Kubernetes cluster, clean enivronment]
sidebar_position: 2
sidebar_label: On a new Kubernetes cluster
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Install kbcli and KubeBlocks on a new Kubernetes cluster

To install kbcli and KubeBlocks to an new Kubernetes cluster on both local and cloud environment, you can use Playground. To install kbcli and KubeBlocks on existed Kubernetes cluster, see [Install kbcli and KubeBlocks on the existed Kubernetes cluster](./install-kbcli-and-kubeblocks-on-the-existed-kubernetes-clusters.md).

## Install kbcli and KubeBlocks on local host

### Environment preparation

- Minimum system requirements:
  - MacOS:
    - CPU: 4 cores
    - Memory: 4 GB
    Check CPU with the following command: `sysctl hw.physicalcpu; Check memory with the following command: top -d`.
  - Windows:
    - 64-bit
- Ensure the following tools are installed on your local host:
  - Docker: v20.10.5 (runc ≥ v1.0.0-rc93) or higher. For installation details, see Get Docker.
  - kubectl: used to interact with Kubernetes clusters.
  - For Windows environment, PowerShell version 5.0 or higher is required.

### Installation steps

You can install the kbcli and KubeBlocks on your local host, and now MacOS and Windows are supported.

**Before you start**

Make sure you have kbcli installed, for detailed information, check [Install kbcli](#install-kbcli).

**One-click Deployment of KubeBlocks.**

Use the `kbcli playground init` command.

This command:

- Creates a Kubernetes cluster in a K3d container.
- Deploys KubeBlocks in the Kubernetes cluster.
- Creates a high-availability ApeCloud MySQL cluster named mycluster in the default namespace.

Check the created cluster. When the status is Running, it indicates that the cluster has been successfully created.

```bash
kbcli cluster list
```

## Install kbcli and KubeBlocks on cloud

This section shows how to install KubeBlocks on new Kubernetes clusters on cloud.

**Before you start**

Make sure you have kbcli installed, for detailed information, check [Install kbcli](#install-kbcli).

When deploying to the cloud, you can use the Terraform scripts maintained by ApeCloud to initialize the cloud resources. Click on [Terraform script](https://github.com/apecloud/cloud-provider) to use Terraform.

When deploying a Kubernetes cluster in the cloud, kbcli clones the above repository to the local host, invokes the Terraform command to initialize the cluster, and then deploys KubeBlocks on that cluster.

**Step 1. Configure and connect to cloud environment.**

See the table below.

    | Cloud Environment                 | Commands                                                                                                                                                                |
    |-----------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
    | EKS v1.22 / v1.23 / v1.24 / v1.25 | `export AWS_ACCESS_KEY_ID="anaccesskey"  $ export  AWS_SECRET_ACCESS_KEY="asecretkey"  kbcli playground init  --cloud-provider aws --region regionname`                 |
    | ACK v1.22 / v1.24                 | `export ALICLOUD_ACCESS_KEY="************"  export ALICLOUD_SECRET_KEY="************"  kbcli playground init --cloud-provider alicloud --region regionname`             |
    | TKE v1.22 / v1.24                 | `export TENCENTCLOUD_SECRET_ID=YOUR_SECRET_ID  export  TENCENTCLOUD_SECRET_KEY=YOUR_SECRET_KEY  kbcli playground init  --cloud-provider tencentcloud --region regionname` |
    | GKE v1.24 / v1.25                 | `gcloud init  gcloud auth application-default login   export GOOGLE_PROJECT= <project name> kbcli playground init --cloud-provider gcp  --region regionname`            |

**Step 2. One-click Deployment of KubeBlocks**

Use the `kbcli playground init` command.

This command:

- Creates a Kubernetes cluster in a K3d container.
- Deploys KubeBlocks in the Kubernetes cluster.
- Creates a high-availability ApeCloud MySQL cluster named mycluster in the default namespace.

Check the created cluster. When the status is `Running`, it indicates that the cluster has been successfully created.

```bash
kbcli cluster list
```