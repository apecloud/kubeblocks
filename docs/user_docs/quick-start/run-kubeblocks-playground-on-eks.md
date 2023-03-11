---
title: Run KubeBlocks Playground on AWS EKS
description: How to run KubeBlocks on Playground
sidebar_position: 2
---

# Run KubeBlocks Playground on AWS EKS

KubeBlocks v0.4 supports using the `kbcli playground` command to deploy EKS cluster and deploy KubeBlocks on EKS. This tutorial introduces how to configure this command.

## How `kbcli` works on EKS

`kbcli` currently supports deploy Playground on [the local host](./run-kubeblocks-playground-on-localhost.md) and the cloud. For the cloud environment, `kbcli` only supports deploying AWS EKS clusters. When deploying on the cloud, cloud resources are initialized with the help of the terraform script maintained by ApeCloud. Find the script at [Github repository](https://github.com/apecloud/cloud-provider).
When deploying a Kubernetes cluster on the cloud, `kbcli` clones the above repository to the local host, calls the terraform commands to initialize the cluster, then deploys KubeBlocks on this cluster.

## Before you start

Install AWS CLI. Refer to [Installing or updating the latest version of the AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html) for details.

## Step 1. Configure access key

Before using `kbcli` commands, configure the Access Key of cloud resources.
For AWS, there are two options.

**Option 1.** Use `aws configure`

Fill in `AWS Access Key ID` and `AWS Secret Access Key` and run the command below to configure access permission.

```bash
aws configure
AWS Access Key ID [None]: AKIAIOSFODNN7EXAMPLE
AWS Secret Access Key [None]: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

You can refer to [Quick configuration with aws configure](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-quickstart.html#cli-configure-quickstart-config) to learn the detailed guide.

**Option 2.** Use environment variables

```bash
export AWS_ACCESS_KEY_ID="anaccesskey"
export AWS_SECRET_ACCESS_KEY="asecretkey"
```

## Step 2. Initialize Playground

Run the command below to initialize Playground.

```bash
kbbcli playground init --cloud-provider aws --region cn-northwest-1
```

* `cloud-provider` specifies the cloud provider. Currently, only `aws` is supported.
* `region` specifies the region to deploy a Kubernetes cluster.
   Frequently used regions are as follow. You can find the full region list on [the official website](https://aws.amazon.com/about-aws/global-infrastructure/regional-product-services/?nc1=h_ls).
  * Americas
     | Region ID  | Region name         |
     | :--        | :--                 |
     | us-east-1  | Northern Virginia   |
     | us-east-2  | Ohio                |
     | us-west-1  | Northern California |
     | us-west-2  | Oregon              |

  * Asia Pacific
     | Region ID         | Region name |
     | :--               | :--         |
     | ap-east-1         | Hong Kong   |
     | ap-southeast-1    | Singapore   |
     | cn-north-1        | Beijing     |
     | cn-northwest-1    | Ningxia     |
During the initialization, `kbcli` clones [the GitHub repository](https://github.com/apecloud/cloud-provider) to the path `~/.kbcli/playground` and calls the terraform script to create cloud resources. After that, `kbcli` deploys KubeBlocks automatically and installs a MySQL cluster.

After the `kbcli playground init` command is executed, `kbcli` automatically switches the context of the local kubeconfig to the current cluster. Run the command below to view the deplayed cluster.

```bash
# View kbcli version
kbcli version

# View the cluster list
kbcli cluster list
```

> ***Note:***
>
> The initialization lasts about 20 minutes. If the installation fails after a long time, please check your network environment.

## Step 3. Destroy Playground

Before destroying Playground, it is recommended to delete the clusters created by KubeBlocks

```bash
# View all clusters
kbcli cluster list -A

# Delete a cluster
# A double-check is required and you can add --auto-approve to check it automatically
kbcli cluster delete <my-cluster>

# Uninstall KubeBlocks
# A double-check is required and you can add --auto-approve to check it automatically
kbcli kubeblocks uninstall --remove-pvcs --remove-pvs
```

Run the command below to destroy Playground.

```bash
kbbcli playground destroy --cloud-provider aws --region cn-northwest-1
```

Like the parameters in `kbcli playground init`, use `--cloud-provider` and `--region` to specify the cloud provider and the region.

> ***Caution:***
>
> `kbcli playground destroy` directly deletes the Kubenetes cluster on the cloud but there might be residual resources in cloud, such as volumes. Please confirm whether there are residual resources after uninstalling and delete them in time to avoid unnecessary fees.