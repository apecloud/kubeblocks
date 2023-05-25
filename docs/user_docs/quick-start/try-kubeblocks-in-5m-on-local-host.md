---
title: Try out KubeBlocks in 5 minutes on Local Host
description: A quick tour of KubeBlocks in 5 minutes on Local host on Playground
sidebar_position: 1
sidebar_label: Try out KubeBlocks on local host
---


# Try out KubeBlocks in 5 minutes on Local Host 
This guide walks you through the quickest way to get started with KubeBlocks, demonstrating how to easily create a KubeBlocks demo environment (Playground) with simply one `kbcli` command. 
With Playground, you can try out KubeBlocks both on your local host (macOS).


## Before you start 

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
         curl -fsSL https://www.kubeblocks.io/installer/install_cli.sh | bash
         ```
    2. Run `kbcli version` to check the `kbcli` version and make sure `kbcli` is installed successfully.

## Initialize Playground

***Steps:***

1. Install Playground.

   ```bash
   kbcli playground init
   ```

   This command:
   1. Creates a Kubernetes cluster in the container with [K3d](https://k3d.io/v5.4.6/).
   2. Deploys KubeBlocks in this Kubernetes cluster.
   3. Creates an ApeCloud MySQL Standalone by KubeBlocks.

2. View the created cluster and when the status is `Running`, this cluster is created successfully. 
   ```bash
   kbcli cluster list
   ```
   
   **Result:**

   You just created a cluster named `mycluster` in the default namespace.

   You can find the Playground user guide under the installation success tip. View this guide again by running `kbcli playground init -h`.



## Try KubeBlocks with Playground

You can explore KubeBlocks, by [Viewing an ApeCloud MySQL cluster](#view-an-apecloud-mysql-cluster), [Accessing an ApeCloud MySQL cluster](#access-an-apecloud-mysql-cluster), [Observability](#observability), and [High availability](#high-availability-of-apecloud-mysql). Refer to [Overview](./../introduction/introduction.md) to explore detailed KubeBlocks features and you can try all the features of KubeBlocks in a standard Kubernetes cluster.

KubeBlocks supports the complete life cycle management of a database cluster. Go through the following instructions to try basic features of KubeBlocks. 

:::note

The local host does not support volume expansion, backup, and restore functions.

:::

### View an ApeCloud MySQL cluster

***Steps:***

1. View the database cluster list.
    ```bash
    kbcli cluster list
    ```

2. View the details of a specified database cluster and get information like `STATUS`, `Endpoints`, `Topology`, `Images`, and `Events`.
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

KubeBlocks supports complete observability capabilities. This section demonstrates the monitoring function of KubeBlocks. 

***Steps:***

1. View the monitoring page to observe the service running status.
   ```bash
   kbcli dashboard open kubeblocks-grafana
   ```

   **Result**

   A monitoring page on Grafana website is loaded automatically after the command is executed. 

2. Click the Dashboard icon on the left bar and monitoring panels show on the page.
   ![Dashboards](./../../img/quick_start_dashboards.png)
3. Click **General** -> **MySQL** to monitor the status of the ApeCloud MySQL cluster deployed by Playground.
   ![MySQL_panel](./../../img/quick_start_mysql_panel.png)

### High availability of ApeCloud MySQL

ApeCloud MySQL Paxos group delivers high availability with RPO=0 and RTO in less than 30 seconds.
This guide shows a simple failure simulation to show you the failure recovery capability of ApeCloud MySQL.

#### Delete ApeCloud MySQL Standalone

Delete the ApeCloud MySQL Standalone before trying out high availability.
```bash
kbcli cluster delete mycluster
```

#### Create an ApeCloud MySQL Paxos group

Playground creates an ApeCloud MySQL standalone by default. You can also use `kbcli` to create a new Paxos group. The following is an example of creating an ApeCloud MySQL Paxos group with default configurations.

```bash
kbcli cluster create --cluster-definition='apecloud-mysql' --set replicas=3
```

#### Simulate leader pod failure recovery

In this example, delete the leader pod to simulate a failure.

***Steps:***

1. Run `kbcli cluster describe` to view the ApeCloud MySQL Paxos group information. View the leader pod name in `Topology`. In this example, the leader pod's name is maple05-mysql-1.
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
4. Delete the leader pod and view the influences on the ApeCloud MySQL cluster through the NYAN CAT page.
   
   ![NYAN CAT](./../../img/quick_start_nyan_cat.png)

5. Uninstall the NYAN CAT demo application after your trial.
   ```bash
   kbcli addon disable nyancat
   ```

## Destroy Playground

Destroying Playground cleans up relevant component services and data:

* Delete all KubeBlocks database clusters.
* Uninstall KubeBlocks.
* Delete the local Kubernetes clusters created by K3d.
  
Destroy Playground.
```bash
kbcli playground destroy
```

