---
title: Try out basic functions of KubeBlocks on Cloud
description: How to run KubeBlocks on Playground
keywords: [Playground, try out, cloud]
sidebar_position: 1
sidebar_label: Try out KubeBlocks on cloud
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Try out basic functions of KubeBlocks on Cloud

This guide walks you through the quickest way to get started with KubeBlocks on any cloud, demonstrating how to easily create a demo environment (Playground) with simply one `kbcli` command.

With Playground, you can try out KubeBlocks both on your local host (macOS) and on a cloud environment. And KubeBlocks supports multiple cloud platforms.

## Preparation

When deploying on the cloud, cloud resources are initialized with the help of the terraform script maintained by ApeCloud. Find this script at [Github repository](https://github.com/apecloud/cloud-provider).

When deploying a Kubernetes cluster on the cloud, `kbcli` clones the above repository to the local host, calls the terraform commands to initialize the cluster, then deploys KubeBlocks on this cluster.

<Tabs>
<TabItem value="AWS" label="AWS" default>

### Before you start to try KubeBlocks on Cloud (AWS)

Make sure you have all the followings prepared.

* [Install AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html).
* Make sure the following tools are installed.
  * [Install kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl): It is used to interact with Kubernetes clusters.
  * [Install `kbcli`](./../../installation/install-kbcli.md). `kbcli` is the command line tool of KubeBlocks and is used for the interaction between Playground and KubeBlocks.

### Configure access key

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

### Initialize Playground

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

During the initialization, `kbcli` clones [the GitHub repository](https://github.com/apecloud/cloud-provider) to the directory `~/.kbcli/playground` and calls the terraform script to create cloud resources. And then `kbcli` deploys KubeBlocks automatically and installs a MySQL cluster.

After the `kbcli playground init` command is executed, `kbcli` automatically switches the context of the local kubeconfig to the current cluster. Run the command below to view the created cluster.

```bash
# View kbcli version
kbcli version

# View the cluster list
kbcli cluster list
```

:::note

The initialization lasts about 20 minutes. If the installation fails after a long time, please check your network environment.

:::

</TabItem>
<TabItem value="GCP" label="GCP">

### Before you start to try KubeBlocks on Cloud (GCP)

Make sure you have all the followings prepared.

* Google Cloud account.
* [Install kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl): It is used to interact with Kubernetes clusters.
* [Install `kbcli`](./../installation/install-kbcli.md): `kbcli` is the command line tool of KubeBlocks and is used for the interaction between Playground and KubeBlocks.
  
### Configure GCP environment

***Steps：***

1. Install Google Cloud SDK.

   ```bash
   # macOS brew install
   brew install --cask google-cloud-sdk

   # windows
   choco install gcloudsdk
   ```

2. Initialize GCP.

   ```bash
   gcloud init
   ```

3. Log in to GCP.

   ```bash
   gcloud auth application-default login
   ```

4. Configure GOOGLE_PROJECT environment variables，```kbcli playground``` creates GKE cluster in the project.

   ```bash
   export GOOGLE_PROJECT=<project-name>
   ```

### Initialize Playground

:::note

Execute `export GOOGLE_PROJECT=<project-name>` to specify the project name.

:::

The following command deploys a GKE service in the available region `us-central1` on Google Cloud, and installs KubeBlocks.

```bash
kbcli playground init --cloud-provider gcp --region us-central1
```

* `cloud-provider` specifies the cloud provider.
* `region` specifies the region to deploy a Kubernetes cluster.

During the initialization, `kbcli` clones [the GitHub repository](https://github.com/apecloud/cloud-provider) to the directory `~/.kbcli/playground` and calls the terraform script to create cloud resources. And then `kbcli` deploys KubeBlocks automatically and installs a MySQL cluster.

After the `kbcli playground init` command is executed, `kbcli` automatically switches the context of the local kubeconfig to the current cluster. Run the command below to view the created cluster.

```bash
# View kbcli version
kbcli version

# View the cluster list
kbcli cluster list
```

:::note

The initialization takes about 20 minutes. If the installation fails after a long time, please check your network environment.

:::

</TabItem>
<TabItem value="TKE" label="TKE">

### Before you start to try KubeBlocks on Cloud (TKE)

Make sure you have all the followings prepared.

* Tencent Cloud account.
* [Install kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl): It is used to interact with Kubernetes clusters.
* [Install `kbcli`](./../../installation/install-kbcli.md). `kbcli` is the command line tool of KubeBlocks and is used for the interaction between Playground and KubeBlocks.

  
### Configure TKE environment

***Steps：***

1. Log in to Tencent Cloud.
2. Go to [Tencent Kubernetes Engine (TKE)](https://console.cloud.tencent.com/tke2) to grant resource operation permission to your account before using the container service.
3. Go to [API Console](https://console.cloud.tencent.com/cam/overview) -> **Access Key** -> **API Keys** and click **Create Key** to create a pair of Secret ID and Secret Key.

   :::note

   If you have an available security certificate, skip Step 3 and 4.

   :::

4. Add the Secret ID and Secret Key to the environment variables.

   ```bash
   export TENCENTCLOUD_SECRET_ID=YOUR_SECRET_ID
   export TENCENTCLOUD_SECRET_KEY=YOUR_SECRET_KEY
   ```

### Initialize Playground

The following command deploys a Kubernetes service in the available region `ap-chengdu` on Tencent Cloud and installs KubeBlocks.

```bash
kbcli playground init --cloud-provider tencentcloud --region ap-chengdu
```

* `cloud-provider` specifies the cloud provider.
* `region` specifies the region to deploy a Kubernetes cluster.

During the initialization, `kbcli` clones [the GitHub repository](https://github.com/apecloud/cloud-provider) to the directory `~/.kbcli/playground` and calls the terraform script to create cloud resources. And then `kbcli` deploys KubeBlocks automatically and installs a MySQL cluster.

After the `kbcli playground init` command is executed, `kbcli` automatically switches the context of the local kubeconfig to the current cluster. Run the command below to view the created cluster.

```bash
# View kbcli version
kbcli version

# View the cluster list
kbcli cluster list
```

:::note

The initialization takes about 20 minutes. If the installation fails after a long time, please check your network environment.

:::

</TabItem>
<TabItem value="ACK" label="ACK">

### Before you start to try KubeBlocks on Cloud (ACK)

Make sure you have all the followings prepared.

* Alibaba Cloud account.
* [Install kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl): It is used to interact with Kubernetes clusters.
* [Install `kbcli`](./../installation/install-kbcli.md): Choose one guide that fits your actual environments. `kbcli` is the command line tool of KubeBlocks and is used for the interaction between Playground and KubeBlocks.
  
### Configure ACK environment

***Steps：***

1. Log in to Alibaba Cloud.
2. Follow the instructions in [Quick start for first-time users](https://www.alibabacloud.com/help/en/container-service-for-kubernetes/latest/quick-start-for-first-time-users) to check whether you have activated Alibaba Cloud Container Service for Kubernetes (ACK) and assigned roles.
3. Click [AliyunOOSLifecycleHook4CSRole](https://ram.console.aliyun.com/role/authorize?spm=5176.2020520152.0.0.5b4716ddI6QevL&request=%7B%22ReturnUrl%22%3A%22https%3A%2F%2Fram.console.aliyun.com%22%2C%22Services%22%3A%5B%7B%22Roles%22%3A%5B%7B%22RoleName%22%3A%22AliyunOOSLifecycleHook4CSRole%22%2C%22TemplateId%22%3A%22AliyunOOSLifecycleHook4CSRole%22%7D%5D%2C%22Service%22%3A%22OOS%22%7D%5D%7D) and click **Agree to Authorization** to create an AliyunOOSLifecycleHook4CSRole role.

   This operation grant permissions to access Operation Orchestration Service (OOS) and to access the resources in other cloud products since creating and managing a node pool is required for creating an ACK cluster.

   Refer to [Scale a node pool](https://www.alibabacloud.com/help/zh/container-service-for-kubernetes/latest/scale-up-and-down-node-pools) for details.
4. Create an AccessKey ID and the corresponding AccessKey secret.

   1. Go to [Alibaba Cloud Management Console](https://homenew.console.aliyun.com/home/dashboard/ProductAndService). Hover the pointer over your account console and click **AccessKey Management**.
   2. Click **Create AccessKey** to create the AccessKey ID and the corresponding AccessKey secret.
   3. Add the AccessKey ID and AccessKey secret to the environment variable to configure identity authorization information.

       ```bash
       export ALICLOUD_ACCESS_KEY="************"
       export ALICLOUD_SECRET_KEY="************"
       ```

   :::note

   Refer to [Create an AccessKey pair](https://www.alibabacloud.com/help/en/resource-access-management/latest/accesskey-pairs-create-an-accesskey-pair-for-a-ram-user) for details.

   :::

### Initialize Playground

The following command deploys an ACK cluster in the available region `cn-hangzhou` on Alibaba Cloud, and installs KubeBlocks.

```bash
kbcli playground init --cloud-provider alicloud --region cn-hangzhou
```

* `cloud-provider` specifies the cloud provider.
* `region` specifies the region to deploy a Kubernetes cluster.

During the initialization, `kbcli` clones [the GitHub repository](https://github.com/apecloud/cloud-provider) to the directory `~/.kbcli/playground` and calls the terraform script to create cloud resources. And then `kbcli` deploys KubeBlocks automatically and installs a MySQL cluster.

After the `kbcli playground init` command is executed, `kbcli` automatically switches the context of the local kubeconfig to the current cluster. Run the command below to view the created cluster.

```bash
# View kbcli version
kbcli version

# View the cluster list
kbcli cluster list
```

:::note

The initialization takes about 20 minutes. If the installation fails after a long time, please check your network environment.

:::

</TabItem>
</Tabs>

## Try KubeBlocks with Playground

KubeBlocks supports the complete life cycle management of a database cluster. Go through the following instructions to try basic features of KubeBlocks.

For the full feature set, refer to [KubeBlocks Documentation](./../introduction/introduction.md) for details.

### View an ApeCloud MySQL cluster

***Steps:***

1. View the database cluster list.

    ```bash
    kbcli cluster list
    ```

2. View the details of a specified database cluster, such as `STATUS`, `Endpoints`, `Topology`, `Images`, and `Events`.

    ```bash
    kbcli cluster describe mycluster
    ```

### Access an ApeCloud MySQL cluster

**Option 1.** Connect database inside Kubernetes cluster.

Wait until the status of this cluster is `Running`, then run `kbcli cluster connect` to access a specified database cluster. For example,

```bash
kbcli cluster connect mycluster
```

**Option 2.** Connect database outside Kubernetes cluster.

Get the MySQL Client connection example.

```bash
kbcli cluster connect --show-example --client=cli mycluster
```

**Example**

1. Run `port-forward` to connect the cluster from the local host.

   ```bash
   kubectl port-forward service/mycluster-mysql 3306:3306
   >
   Forwarding from 127.0.0.1:3306 -> 3306
   Forwarding from [::1]:3306 -> 3306
   ```

2. Open another terminal tab to connect the MySQL Client.

   ```bash
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

1. View the monitoring page to observe the service running status.

   ```bash
   kbcli dashboard open kubeblocks-grafana
   ```

   ***Result***

   A monitoring page on Grafana website is loaded automatically after the command is executed.

2. Click the Dashboard icon on the left bar and two monitoring panels show on the page.
   ![Dashboards](./../../img/quick_start_dashboards.png)
3. Click **General** -> **MySQL** to monitor the status of the ApeCloud MySQL cluster created by Playground.
   ![MySQL_panel](./../../img/quick_start_mysql_panel.png)

### High availability of ApeCloud MySQL

ApeCloud MySQL Raft Group delivers high availability with RPO=0 and RTO in less than 30 seconds.

This section uses a simple failure simulation to show you the failure recovery capability of ApeCloud MySQL.

#### Delete ApeCloud MySQL Standalone

Delete the ApeCloud MySQL Standalone before trying out high availability.

```bash
kbcli cluster delete mycluster
```

#### Create an ApeCloud MySQL Raft Group

Playground creates an ApeCloud MySQL Standalone by default. You can use `kbcli` to create a new Raft Group. The following is an example of creating an ApeCloud MySQL Raft Group with default configurations.

```bash
kbcli cluster create --cluster-definition='apecloud-mysql' --set replicas=3
```

#### Simulate leader pod failure recovery

In this example, delete the leader pod to simulate a failure.

***Steps:***

1. Make sure the newly created cluster is `Running`.

   ```bash
   kbcli cluster list
   ```

2. View the ApeCloud MySQL Raft group information. View the leader pod name in `Topology`. In this example, the leader pod's name is maple05-mysql-1.

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

3. Delete the leader pod.

   ```bash
   kubectl delete pod maple05-mysql-1
   >
   pod "maple05-mysql-1" deleted
   ```

4. Connect to the ApeCloud MySQL Raft Group to test its availability. You can find this group can still be accessed within seconds due to our HA strategy.

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

1. Install the NYAN CAT demo application.

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

   ```bash
   kubectl delete pod maple05-mysql-1
   ```

   ![NYAN CAT](./../../img/quick_start_nyan_cat.png)

5. Uninstall the NYAN CAT demo application after your trial.

   ```bash
   kbcli addon disable nyancat
   ```

## Destroy Playground

1. Before destroying Playground, it is recommended to delete the clusters created by KubeBlocks.

   ```bash
   # View all clusters
   kbcli cluster list -A

   # Delete a cluster
   # A double-check is required and you can add --auto-approve to check it automatically
   kbcli cluster delete <name>

   # Uninstall KubeBlocks
   # A double-check is required and you can add --auto-approve to check it automatically
   kbcli kubeblocks uninstall --remove-pvcs --remove-pvs
   ```

2. Destroy Playground.

   ```bash
   kbcli playground destroy 
   ```

:::caution

`kbcli playground destroy` directly deletes the Kubernetes cluster on the cloud but there might be residual resources on the cloud, such as volumes. Please confirm whether there are residual resources after uninstallation and delete them in time to avoid unnecessary fees.

:::
