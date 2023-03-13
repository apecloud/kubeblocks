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

## Step 3. Run Playground

The Playground guide includes three sections, namely [Basic functions](#basic-functions), [Observability](#observability), and [High availability](#high-availability-of-apecloud-mysql). Refer to [Feature list](./../introduction/feature-list.md) to explore more KubeBlocks features and you can try the full features of KubeBlocks in a standard Kubernetes cluster.

### Basic functions

KubeBlocks supports the complete life cycle management of a database cluster and follows the best practice of application development. The following instructions demonstrate the basic features of KubeBlocks. For the full feature set, refer to [KubeBlocks Documentation](./docs/user_docs) for details.

> ***Note:***
>
> The local host does not support volume expansion, backup, and restore functions.

#### View an ApeCloud MySQL cluster

***Steps:***

1. Run the command below to view the database cluster list.
    ```bash
    kbcli cluster list
    >
    NAME     	NAMESPACE	CLUSTER-DEFINITION	VERSION        	TERMINATION-POLICY	STATUS 	CREATED-TIME
    mycluster	default  	apecloud-mysql    	ac-mysql-8.0.30	WipeOut           	Running	Jan 31,2023 16:06 UTC+0800
    ```

2. Run `kbcli cluster describe` to view the details of a specified database cluster, such as `STATUS`, `Endpoints`, `Topology`, `Images`, and `Events`.
    ```bash
    kbcli cluster describe mycluster
    ```

#### Access an ApeCloud MySQL cluster

**Option 1.** Use `kbcli`.

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

mysql>
```

**Option 2.** Use an access address.

If you want to access a cluster via MySQL client, get the access address from `Endpoints` in the cluster details.
```bash
kbcli cluster describe mycluster
```

#### Delete an ApeCloud MySQL cluster

Run the command below to delete a specified database cluster. For example, 
```bash
kbcli cluster delete mycluster
```

### Observability

KubeBlocks has complete observability capabilities. This section demonstrates the monitoring function of KubeBlocks. 

***Steps:***

1. Run the command below to view the monitoring page to observe the service running status.
   ```bash
   kbcli dashboard open kubeblocks-grafana

   Forwarding from 127.0.0.1:3000 -> 3000
   Forward successfully! Opening browser ...
   ```

   ***Result***

   A monitoring page is loaded automatically after the command is executed. 

2. Click the Dashboard icon on the left bar and two monitoring panels show on the page.
   ![Dashboards](./../../img/quick_start_dashboards.png)
3. Click **General** -> **MySQL** to monitor the status of the ApeCloud MySQL cluster deployed by Playground.
   ![MySQL_panel](./../../img/quick_start_mysql_panel.png)

### High availability of ApeCloud MySQL

ApeCloud MySQL Paxos group delivers high availability with RPO=0 and RTO in less than 30 seconds.
Here we use a simple failure simulation to show you the failure recovery capability of ApeCloud MySQL.

#### Create an ApeCloud MySQL Paxos group

Playground creates an ApeCloud-MySQL standalone by default. You can also use `kbcli` to create a new Paxos group. The following is an example of creating an ApeCloud-MySQL Paxos group with default configurations.

```bash
kbcli cluster create --cluster-definition='apecloud-mysql' --set replicas=3
```

#### Simulate leader pod failure recovery

In this example, we delete the leader pod to simulate a failure.

***Steps:***

1. Run the command below to view the ApeCloud MySQL Paxos group information. View the leader pod name in `Topology`. In this example, the leader pod's name is maple05-mysql-1.
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
2. Run the command below to delete the leader pod.
   ```bash
   kubectl delete pod maple05-mysql-1

   pod "maple05-mysql-1" deleted
   ```

3. Run `kbcli cluster connect maple05` to connect to the ApeCloud MySQL Paxos group to test its availability. You can find this group can be accessed within seconds.
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

#### Observe clusters by NON-STOP NYAN CAT
The above example uses `kbcli cluster connect` to test availability, in which the changes are not obvious to see.
NON-STOP NYAN CAT is a demo application to observe how the database cluster exceptions affect actual businesses. Animations and real-time key information display provided by NON-STOP NYAN CAT can directly show the availability influences of database services.

***Steps:***

1. Run the command below to install the NYAN CAT demo application.
   ```bash
   kbcli app install nyancat
   >
   Installing application nyancat OK
   Install nyancat SUCCESSFULLY!
   1. Get the application URL by running these commands:
   kubectl --namespace default port-forward service/nyancat 8087:8087
   echo "Visit http://127.0.0.1:8087 to use Nyan Cat demo application."
   ```

2. Use `port-forward` according to the hints above to expose an application port as available access for your local host, then visit this application via http://127.0.0.1:8087.
3. Delete the leader pod and view the influences on the ApeCloud MySQL clusters through the NYAN CAT page.
   ![NYAN CAT](./../../img/quick_start_nyan_cat.png)
4. Uninstall the NYAN CAT demo application.
   ```bash
   kbcli app uninstall nyancat
   ```


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