---
title: Try out basic functions of KubeBlocks on Cloud
description: How to run KubeBlocks on Playground
sidebar_position: 1
sidebar_label: Try out KubeBlocks on cloud
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Try out basic functions of KubeBlocks on Cloud 
This guide walks you through the quickest way to get started with KubeBlocks, demonstrating how to easily create a KubeBlocks demo environment (Playground) with simply one `kbcli` command. 
With Playground, you can try out KubeBlocks both on your local host (macOS) and on a cloud environment (AWS).

<Tabs>
<TabItem value="AWS" label="AWS" default>

## Before you start to try KubeBlocks on Cloud (AWS)
  
When deploying on the cloud, cloud resources are initialized with the help of the terraform script maintained by ApeCloud. Find the script at [Github repository](https://github.com/apecloud/cloud-provider).

When deploying a Kubernetes cluster on the cloud, `kbcli` clones the above repository to the local host, calls the terraform commands to initialize the cluster, then deploys KubeBlocks on this cluster.
* Install AWS CLI. Refer to [Installing or updating the latest version of the AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html) for details.
* Make sure the following tools are installed on your local host.
  * Docker: v20.10.5 (runc ≥ v1.0.0-rc93) or above. For installation details, refer to [Get Docker](https://docs.docker.com/get-docker/).
  * `kubectl`: It is used to interact with Kubernetes clusters. For installation details, refer to [Install kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl).
  * `kbcli`: It is the command line tool of KubeBlocks and is used for the interaction between Playground and KubeBlocks. Follow the steps below to install `kbcli`.
    1. Install `kbcli`.
         ```bash
         curl -fsSL https://www.kubeblocks.io/installer/install_cli.sh | bash
         ```
    2. Run `kbcli version` to check the `kbcli` version and make sure `kbcli` is installed successfully.

## Configure access key

Configure the Access Key of cloud resources.
For AWS, there are two options.

**Option 1.** Use `aws configure`.

Fill in AWS Access Key ID and AWS Secret Access Key and run the command below to configure access permission.

```bash
aws configure
AWS Access Key ID [None]: AKIAIOSFODNN7EXAMPLE
AWS Secret Access Key [None]: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

You can refer to [Quick configuration with aws configure](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-quickstart.html#cli-configure-quickstart-config) for detailed information.

**Option 2.** Use environment variables.

```bash
export AWS_ACCESS_KEY_ID="anaccesskey"
export AWS_SECRET_ACCESS_KEY="asecretkey"
```

## Initialize Playground

Initialize Playground.

```bash
kbcli playground init --cloud-provider aws --region cn-northwest-1
```

* `cloud-provider` specifies the cloud provider. 
* `region` specifies the region to deploy a Kubernetes cluster.
   Frequently used regions are as follows. You can find the full region list on [the official website](https://aws.amazon.com/about-aws/global-infrastructure/regional-product-services/?nc1=h_ls).
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

During the initialization, `kbcli` clones [the GitHub repository](https://github.com/apecloud/cloud-provider) to the path `~/.kbcli/playground` and calls the terraform script to create cloud resources. And then `kbcli` deploys KubeBlocks automatically and installs a MySQL cluster.

After the `kbcli playground init` command is executed, `kbcli` automatically switches the context of the local kubeconfig to the current cluster. Run the command below to view the deployed cluster.

```bash
# View kbcli version
kbcli version

# View the cluster list
kbcli cluster list
```

:::note

The initialization lasts about 20 minutes. If the installation fails after a long time, please check your network environment.

:::

## Try KubeBlocks with Playground

You can explore KubeBlocks by [Viewing an ApeCloud MySQL cluster](#view-an-apecloud-mysql-cluster), [Accessing an ApeCloud MySQL Cluster](#access-an-apecloud-mysql-cluster), [Observability](#observability), and [High availability](#high-availability-of-apecloud-mysql). Refer to [Overview](./../introduction/introduction.md) to explore more KubeBlocks features and you can try the full features of KubeBlocks in a standard Kubernetes cluster.

KubeBlocks supports the complete life cycle management of a database cluster. Go through the following instruction to try basic features of KubeBlocks. 
For the full feature set, refer to [KubeBlocks Documentation](./../introduction/introduction.md) for details.

### View an ApeCloud MySQL cluster

***Steps:***

1. Run the command below to view the database cluster list.
    ```bash
    kbcli cluster list
    ```

2. Run `kbcli cluster describe` to view the details of a specified database cluster, such as `STATUS`, `Endpoints`, `Topology`, `Images`, and `Events`.
    ```bash
    kbcli cluster describe mycluster
    ```

### Access an ApeCloud MySQL cluster

**Option 1.** Connect database inside Kubernetes cluster.

If a database cluster has been created and its status is `Running`, run `kbcli cluster connect` to access a specified database cluster. For example, 
```bash
kbcli cluster connect mycluster
```

**Option 2.** Connect database outside Kubernetes cluster.

Get the MySQL client connection example.

```bash
kbcli cluster connect --show-example --client=cli mycluster
```

**Example**

```bash
kubectl port-forward service/mycluster-mysql 3306:3306
>
Forwarding from 127.0.0.1:3306 -> 3306
Forwarding from [::1]:3306 -> 3306


mysql -h 127.0.0.1 -P 3306 -u root -paiImelyt
>
...
Type 'help;' or '\h' for help. Type '\c' to clear the current input statement.

mysql> show databases;
>
+--------------------+
| Database           |
+--------------------+
| information_schema |
| mydb               |
| mysql              |
| performance_schema |
| sys                |
+--------------------+
5 rows in set (0.02 sec)
```

### Observability

KubeBlocks has complete observability capabilities. This section demonstrates the monitoring function of KubeBlocks. 

***Steps:***

1. Run the command below to view the monitoring page to observe the service running status.
   ```bash
   kbcli dashboard open kubeblocks-grafana
   ```

   ***Result***

   A monitoring page on Grafana website is loaded automatically after the command is executed. 

2. Click the Dashboard icon on the left bar and two monitoring panels show on the page.
   ![Dashboards](./../../img/quick_start_dashboards.png)
3. Click **General** -> **MySQL** to monitor the status of the ApeCloud MySQL cluster deployed by Playground.
   ![MySQL_panel](./../../img/quick_start_mysql_panel.png)

### High availability of ApeCloud MySQL

ApeCloud MySQL Paxos Group delivers high availability with RPO=0 and RTO in less than 30 seconds.
This section uses a simple failure simulation to show you the failure recovery capability of ApeCloud MySQL.

#### Delete ApeCloud MySQL Standalone

Delete the ApeCloud MySQL Standalone before trying out high availability.
```bash
kbcli cluster delete mycluster
```

#### Create an ApeCloud MySQL Paxos Group

Playground creates an ApeCloud MySQL Standalone by default. You can also use `kbcli` to create a new Paxos Group. The following is an example of creating an ApeCloud MySQL Paxos Group with default configurations.

```bash
kbcli cluster create --cluster-definition='apecloud-mysql' --set replicas=3
```

#### Simulate leader pod failure recovery

In this example, we delete the leader pod to simulate a failure.

***Steps:***

1. Run `kbcli cluster describe ` to view the ApeCloud MySQL Paxos group information. View the leader pod name in `Topology`. In this example, the leader pod's name is maple05-mysql-1.

   ```bash
   kbcli cluster describe maple05
   >
   Name: maple05         Created Time: Jan 27,2023 17:33 UTC+0800
   NAMESPACE        CLUSTER-DEFINITION        VERSION                STATUS         TERMINATION-POLICY
   default          apecloud-mysql            ac-mysql-8.0.30        Running        WipeOut

   Endpoints:
   COMPONENT        MODE             INTERNAL                EXTERNAL
   mysql            ReadWrite        10.43.29.51:3306        <none>

   Topology:
   COMPONENT        INSTANCE               ROLE            STATUS         AZ            NODE                                                 CREATED-TIME
   mysql            maple05-mysql-1        leader          Running        <none>        k3d-kubeblocks-playground-server-0/172.20.0.3        Jan 30,2023 17:33 UTC+0800
   mysql            maple05-mysql-2        follower        Running        <none>        k3d-kubeblocks-playground-server-0/172.20.0.3        Jan 30,2023 17:33 UTC+0800
   mysql            maple05-mysql-0        follower        Running        <none>        k3d-kubeblocks-playground-server-0/172.20.0.3        Jan 30,2023 17:33 UTC+0800

   Resources Allocation:
   COMPONENT        DEDICATED        CPU(REQUEST/LIMIT)        MEMORY(REQUEST/LIMIT)        STORAGE-SIZE        STORAGE-CLASS
   mysql            false            <none>                    <none>                       <none>              <none>

   Images:
   COMPONENT        TYPE         IMAGE
   mysql            mysql        docker.io/apecloud/wesql-server:8.0.30-5.alpha2.20230105.gd6b8719

   Events(last 5 warnings, see more:kbcli cluster list-events -n default mycluster):
   TIME        TYPE        REASON        OBJECT        MESSAGE
   ```
2. Delete the leader pod.
   ```bash
   kubectl delete pod maple05-mysql-1
   >
   pod "maple05-mysql-1" deleted
   ```

3. Run `kbcli cluster connect maple05` to connect to the ApeCloud MySQL Paxos Group to test its availability. You can find this group can still be accessed within seconds due to our HA strategy.
   ```bash
   kbcli cluster connect maple05
   >
   Connect to instance maple05-mysql-2: out of maple05-mysql-2(leader), maple05-mysql-1(follower), maple05-mysql-0(follower)
   Welcome to the MySQL monitor.  Commands end with ; or \g.
   Your MySQL connection id is 33
   Server version: 8.0.30 WeSQL Server - GPL, Release 5, Revision d6b8719

   Copyright (c) 2000, 2022, Oracle and/or its affiliates.

   Oracle is a registered trademark of Oracle Corporation and/or its
   affiliates. Other names may be trademarks of their respective
   owners.

   Type 'help;' or '\h' for help. Type '\c' to clear the current input statement.

   mysql>
   ```

#### Demonstrate availability failure by NON-STOP NYAN CAT (for fun)
The above example uses `kbcli cluster connect` to test availability, in which the changes are not obvious to see.
NON-STOP NYAN CAT is a demo application to observe how the database cluster exceptions affect actual businesses. Animations and real-time key information display provided by NON-STOP NYAN CAT can directly show the availability influences of database services.

***Steps:***

1. Run the command below to install the NYAN CAT demo application.
   ```bash
   kbcli addon enable nyancat
   ```

   **Result:**

   ```
   addon.extensions.kubeblocks.io/nyancat patched
   ```
2. Check the NYAN CAT add-on status and when its status is `Enabled`, this application is ready.
   ```bash
   kbcli addon list | grep nyancat 
   ```
3. Open the web page.
   ```bash
   kbcli dashboard open kubeblocks-nyancat
   ```
4. Delete the leader pod and view the influences on the ApeCloud MySQL clusters through the NYAN CAT page.
   
   ![NYAN CAT](./../../img/quick_start_nyan_cat.png)

5. Uninstall the NYAN CAT demo application after your trial.
   ```bash
   kbcli addon disable nyancat
   ```

## Destroy Playground

Before destroying Playground, it is recommended to delete the clusters created by KubeBlocks.

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
kbcli playground destroy --cloud-provider aws --region cn-northwest-1
```

Like the parameters in `kbcli playground init`, use `--cloud-provider` and `--region` to specify the cloud provider and the region.

:::caution

`kbcli playground destroy` directly deletes the Kubernetes cluster on the cloud but there might be residual resources in cloud, such as volumes. Please confirm whether there are residual resources after uninstalling and delete them in time to avoid unnecessary fees.

:::


</TabItem>
<TabItem value="GCP" label="GCP">

Coming soon!

</TabItem>
</Tabs>
