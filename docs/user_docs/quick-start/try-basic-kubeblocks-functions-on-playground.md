---
title: Try out basic function of KubeBlocks on Playground
description: How to run KubeBlocks on Playground
sidebar_position: 1
sidebar_label: Try out basic functions on Playground
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# Try out basic functions of KubeBlocks on Playground 
This guide walks you through the quickest way to get started with KubeBlocks, demonstrating how to easily create a KubeBlocks demo environment (Playground) with simply one `kbcli` command. 
With Playground, you can try out KubeBlocks both on your local host(maszXaacOS) and on cloud environment(AWS).

<Tabs>
  <TabItem value="macOS" label="Local Host(macOS)" default>
  
  ## Before you start try KubeBlocks on Local Host (macOS)
  Meet the following requirements for smooth operation of Playground and other functions.
    * Minimum system requirements:
  * CPU: 4 cores
  * RAM: 4 GB
  
  To check CPU, use `sysctl hw.physicalcpu` command;
  To check memory, use `top -d` command.

* Make sure the following tools are installed on your local host.
  * Docker: v20.10.5 (runc ≥ v1.0.0-rc93) or above. For installation details, refer to [Get Docker](https://docs.docker.com/get-docker/).
  * `kubectl`: It is used to interact with Kubernetes clusters. For installation details, refer to [Install kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl).
  * `kbcli`: It is the command line tool of KubeBlocks and is used for the interaction between Playground and KubeBlocks. Follow the steps below to install `kbcli`.
    1. Install `kbcli`.
         ```bash
         curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash
         ```
    2. Run `kbcli version` to check the `kbcli` version and make sure `kbcli` is installed successfully.
   ## Initialize Playground
***Steps***

1. Install Playground.

   ```bash
   kbcli playground init
   ```

   This command:
   1. Creates a Kubernetes cluster in the container with [K3d](https://k3d.io/v5.4.6/).
   2. Deploys KubeBlocks in this Kubernetes cluster.
   3. Creates an ApeCloud MySQL Paxos group by KubeBlocks.

2. Run `kbcli cluster list` to view the created cluster and when the status is `Running`, this cluster is created successfully. 
   **Result:**
   You just created a cluster named `mycluster` in the default namespace.

   You can find the Playground user guide under the installation success tip. View this guide again by running `kbcli playground guide`.



## Try KubeBlocks with Playground

You can explore three parts of KubeBlocks, the [Basic functions](#basic-functions), [Observability](#observability), and [High availability](#high-availability-of-apecloud-mysql). Refer to [Feature list](./../introduction/introduction.md) to explore detailed KubeBlocks features and you can try all the features of KubeBlocks in a standard Kubernetes cluster.

### Basic functions

KubeBlocks supports the complete life cycle management of a database cluster. Go through the following instruction to try basic features of KubeBlocks. 

:::note

The local host does not support volume expansion, backup, and restore functions.

:::

#### View an ApeCloud MySQL cluster

***Steps:***

1. View the database cluster list.
    ```bash
    kbcli cluster list
    >
    NAME     	NAMESPACE	CLUSTER-DEFINITION	VERSION        	TERMINATION-POLICY	STATUS 	CREATED-TIME
    mycluster	default  	apecloud-mysql    	ac-mysql-8.0.30	WipeOut           	Running	Jan 31,2023 16:06 UTC+0800
    ```

2. Run `kbcli cluster describe` to view the details of a specified database cluster, get information like `STATUS`, `Endpoints`, `Topology`, `Images`, and `Events`.
    ```bash
    kbcli cluster describe mycluster
    ```

#### Access an ApeCloud MySQL cluster

**Option 1.** Use kbcli.

If a database cluster has been created and its status is `Running`, run `kbcli cluster connect` to access a specified database cluster. For example, 
```bash
kbcli cluster connect mycluster
```

You can also run the command below to access a cluster by MySQL client.
```bash
kbcli cluster connect --show-example --client=cli mycluster
# cluster mycluster does not have public endpoints, you can run following command and connect cluster from local host
kubectl port-forward service/mycluster-mysql 3306:3306

# mysql client connection example
mysql -h 127.0.0.1 -P 3306 -u root -paiImelyt


kubectl port-forward service/mycluster-mysql 3306:3306
Forwarding from 127.0.0.1:3306 -> 3306
Forwarding from [::1]:3306 -> 3306


mysql -h 127.0.0.1 -P 3306 -u root -paiImelyt
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

**Option 2.** Use client.

If you want to access a cluster via MySQL client, get the access address from `Endpoints` in the cluster details.
```bash
kbcli cluster describe mycluster
```

#### (Optional)Delete an ApeCloud MySQL cluster

Run `kbcli cluster delete` to delete a specified database cluster. For example, 
```bash
kbcli cluster delete mycluster
```

### Observability

KubeBlocks supports complete observability capabilities. This section demonstrates the monitoring function of KubeBlocks. 

***Steps:***

1. View the monitoring page to observe the service running status.
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

ApeCloud MySQL Paxos group delivers high availability with RPO=0 and RTO in less than 30 seconds.
This guide shows a simple failure simulation to show you the failure recovery capability of ApeCloud MySQL.

#### Create an ApeCloud MySQL Paxos group

Playground creates an ApeCloud MySQL standalone by default. You can also use `kbcli` to create a new Paxos group. The following is an example of creating an ApeCloud MySQL Paxos group with default configurations.

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

3. Run `kbcli cluster connect maple05` to connect to the ApeCloud MySQL Paxos group to test its availability. You can find this group can still be accessed within seconds due to our HA strategy.
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
   kbcli app install nyancat
   ```
***Result:***
   ```
   Installing application nyancat OK
   Install nyancat SUCCESSFULLY!
   1. Get the application URL by running these commands:
   kubectl --namespace default port-forward service/nyancat 8087:8087
   echo "Visit http://127.0.0.1:8087 to use Nyan Cat demo application."
   ```
2. Use `port-forward` according to the hints above to expose an application port as available access for your local host, then visit this application via http://127.0.0.1:8087.
3. Delete the leader pod and view the influences on the ApeCloud MySQL clusters through the NYAN CAT page.
   ![NYAN CAT](./../../img/quick_start_nyan_cat.png)
4. Uninstall the NYAN CAT demo application after your trial.
   ```bash
   kbcli app uninstall nyancat
   ```



  </TabItem>
  <TabItem value="Cloud" label="Cloud(AWS)">
  
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
         curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash
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
kbbcli playground init --cloud-provider aws --region cn-northwest-1
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

You can explore three parts of KubeBlocks, the [Basic functions](#basic-functions), [Observability](#observability), and [High availability](#high-availability-of-apecloud-mysql). Refer to [Feature list](./../introduction/introduction.md) to explore more KubeBlocks features and you can try the full features of KubeBlocks in a standard Kubernetes cluster.

### Basic functions

KubeBlocks supports the complete life cycle management of a database cluster. Go through the following instruction to try basic features of KubeBlocks. 
 For the full feature set, refer to [KubeBlocks Documentation](./../introduction/introduction.md) for details.



#### View an ApeCloud MySQL cluster

***Steps:***

1. Run the command below to view the database cluster list.
    ```bash
    kbcli cluster list
   
    ```

2. Run `kbcli cluster describe` to view the details of a specified database cluster, such as `STATUS`, `Endpoints`, `Topology`, `Images`, and `Events`.
    ```bash
    kbcli cluster describe mycluster
    ```

#### Access an ApeCloud MySQL cluster

**Option 1.** Use kbcli

If a database cluster has been created and its status is `Running`, run `kbcli cluster connect` to access a specified database cluster. For example, 
```bash
kbcli cluster connect mycluster
```

You can also run the command below to access a cluster by MySQL client.
```bash
kbcli cluster connect --show-example --client=cli mycluster
# cluster mycluster does not have public endpoints, you can run following command and connect cluster from local host
kubectl port-forward service/mycluster-mysql 3306:3306

# mysql client connection example
mysql -h 127.0.0.1 -P 3306 -u root -paiImelyt


kubectl port-forward service/mycluster-mysql 3306:3306
Forwarding from 127.0.0.1:3306 -> 3306
Forwarding from [::1]:3306 -> 3306


mysql -h 127.0.0.1 -P 3306 -u root -paiImelyt
...
Type 'help;' or '\h' for help. Type '\c' to clear the current input statement.

mysql>> show databases;
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

**Option 2.** Access with MySQL client 

If you want to access a cluster via MySQL client, get the access address from `Endpoints` in the cluster details.
```bash
kbcli cluster describe mycluster
```

#### (Optional)Delete an ApeCloud MySQL cluster

Delete a specified database cluster with `kbcli cluster delete`. For example, 
```bash
kbcli cluster delete mycluster
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
This section use a simple failure simulation to show you the failure recovery capability of ApeCloud MySQL.

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
   kbcli app install nyancat
   ```
***Result:***
   ```
   Installing application nyancat OK
   Install nyancat SUCCESSFULLY!
   1. Get the application URL by running these commands:
   kubectl --namespace default port-forward service/nyancat 8087:8087
   echo "Visit http://127.0.0.1:8087 to use Nyan Cat demo application."
   ```
2. Use `port-forward` according to the hints above to expose an application port as available access for your local host, then visit this application via http://127.0.0.1:8087.
3. Delete the leader pod and view the influences on the ApeCloud MySQL clusters through the NYAN CAT page.
   ![NYAN CAT](./../../img/quick_start_nyan_cat.png)
4. Uninstall the NYAN CAT demo application after your trial.
   ```bash
   kbcli app uninstall nyancat
   ```

</TabItem>
</Tabs>

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
kbbcli playground destroy --cloud-provider aws --region cn-northwest-1
```

Like the parameters in `kbcli playground init`, use `--cloud-provider` and `--region` to specify the cloud provider and the region.

:::caution

`kbcli playground destroy` directly deletes the Kubernetes cluster on the cloud but there might be residual resources in cloud, such as volumes. Please confirm whether there are residual resources after uninstalling and delete them in time to avoid unnecessary fees.

:::


