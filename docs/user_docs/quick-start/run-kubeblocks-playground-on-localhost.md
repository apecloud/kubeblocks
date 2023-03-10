---
title: Run KubeBlocks Playground on localhost
description: How to run KubeBlocks on Playground
sidebar_position: 1
---

# Run KubeBlocks Playground on localhost
This guide uses one `kbcli` command to create a KubeBlocks demo environment (Playground) quickly on your local host.
With Playground, you can try KubeBlocks and some ApeCloud MySQL features. This guide introduces how to install Playground and how to try KubeBlocks on Playground.

***Before you start***

Ensure the following requirements are met, so Playground and other functions can run fluently.

* Minimum system requirements:
  * CPU: 4 cores
  * RAM: 4 GB

* The following tools are installed on your local host.
  * Docker: It acts as a workload environment and the version should be v20.10.5 (runc ≥ v1.0.0-rc93) or above. For installation details, refer to [Get Docker](https://docs.docker.com/get-docker/).
  * `kbcli`: It is the command line tool of KubeBlocks and is used for the interaction between Playground and KubeBlocks. Follow the steps below to install `kbcli`.
    1. Run the command below to install `kbcli`.
         ```bash
         curl -fsSL https://kubeblocks.io/installer/install_cli.sh | bash
         ```
    2. Run `kbcli version` to check the `kbcli` version and make sure `kbcli` is installed successfully.
    3. Run the command below to uninstall `kbcli` if you want to delete `kbcli` after your trial.
         ```bash
         sudo rm /usr/local/bin/kbcli
         ```
  * `kubectl`: It is used to interact with Kubernetes clusters. For installation details, refer to [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl).

## Install Playground

Run the command below to install Playground.

```bash
kbcli playground init
```

How this command works on your local host:
1. Create a Kubernetes cluster in the container by using [K3d](https://k3d.io/v5.4.6/).
2. Deploy KubeBlocks in this Kubernetes cluster.
3. Create an ApeCloud MySQL Paxos group by KubeBlocks.

***Result***

You can see the following information indicating relevant modules have been installed successfully.
```
Create playground k3d cluster: kubeblocks-playground OK
Generate kubernetes config /Users/${username}/.kube/kubeblocks-playground OK
⣟  Install KubeBlocks 0.3.2
Install KubeBlocks 0.3.2                 OK
Create cluster mycluster (ClusterDefinition: apecloud-mysql, ClusterVersion: ac-mysql-8.0.30) OK

KubeBlocks playground init SUCCESSFULLY!
Cluster "mycluster" has been CREATED!
```

You can find the Playground user guide under the installation success tip. View this guide again by running `kbcli playground guide`.
```
1. Basic commands for cluster:

  export KUBECONFIG=/Users/${username}/.kube/kubeblocks-playground

  kbcli cluster list                     # list database cluster and check its status
  kbcli cluster describe mycluster       # get cluster information

2. Connect to database

  kbcli cluster connect mycluster

3. View the Grafana:

  kbcli dashboard open kubeblocks-grafana

4. Destroy Playground:

  kbcli playground destroy

--------------------------------------------------------------------
To view this guide: kbcli playground guide
To get more help: kbcli help

Use "kbcli [command] --help" for more information about a command.
```

Following the instructions in "1. Basic commands for cluster", switch to the Kubernetes local cluster created by Playground by running `export KUBECONFIG=xxx` to start your trip on KubeBlocks and ApeCloud MySQL.

> ***Caution:***  
> 
> Running `export KUBECONFIG` is a necessity for using KubeBlocks and ApeCloud MySQL.

## Run Playground

The Playground guide includes three sections, namely [Basic functions](#basic-functions), [Observability](#observability), and [High availability](#high-availability-of-apecloud-mysql). Refer to [Feature list](../introduction_and_feature_list/feature_list.md) to explore more KubeBlocks features and you can try the full features of KubeBlocks in a standard Kubernetes cluster.

### Basic functions

KubeBlocks supports the complete life cycle management of a database cluster and follows the best practice of application development. The following instructions demonstrate the basic features of KubeBlocks. For the full feature set, refer to [KubeBlocks Documentation](../../../../docs) for details.

> ***Note:***
>
> The local host does not support volume expansion, backup, and restore functions.

#### Create an ApeCloud MySQL Paxos group

Playground creates an ApeCloud-MySQL Paxos group by default. You can also use `kbcli` to create a new cluster. The following is an example of creating an ApeCloud-MySQL Paxos group with default configurations.

```bash
export KBCLI_CLUSTER_DEFAULT_REPLICAS=3

kbcli cluster create --cluster-definition='apecloud-mysql' 

```

#### View an ApeCloud MySQL Paxos group

***Steps:***

1. Run the command below to view the database cluster list.
    ```bash
    kbcli cluster list
    >
    NAME     	NAMESPACE	CLUSTER-DEFINITION	VERSION        	TERMINATION-POLICY	STATUS 	CREATED-TIME
    mycluster	default  	apecloud-mysql    	ac-mysql-8.0.30	WipeOut           	Running	Jan 31,2023 16:06 UTC+0800
    maple05  	default  	apecloud-mysql    	ac-mysql-8.0.30	Delete            	Running	Jan 31,2023 16:17 UTC+0800 
    ```

2. Run `kbcli cluster describe` to view the details of a specified database cluster, such as `STATUS`, `Endpoints`, `Topology`, `Images`, and `Events`.
    ```bash
    kbcli cluster describe maple05
    >
    Name: mycluster  Created Time: Jan 30,2023 17:33 UTC+0800
    NAMESPACE  CLUSTER-DEFINITION  VERSION          STATUS  TERMINATION-POLICY
    default    apecloud-mysql      ac-mysql-8.0.30  Running WipeOut

    Endpoints:
    COMPONENT  MODE       INTERNAL          EXTERNAL
    mysql      ReadWrite  10.43.29.51:3306  <none>

    Topology:
    COMPONENT  INSTANCE         ROLE      STATUS  AZ      NODE                               
    mysql      maple05-mysql-0  follower  Running <none>  k3d-kubeblocks-playground-server-0/xxx
    mysql      maple05-mysql-2  leader    Running <none>  k3d-kubeblocks-playground-server-0/xxx
    mysql      maple05-mysql-1  follower  Running <none>  k3d-kubeblocks-playground-server-0/xxx

    Resources Allocation:
    COMPONENT   DEDICATED  CPU(REQUEST/LIMIT)  MEMORY(REQUEST/LIMIT)    STORAGE-SIZE    STORAGE-CLASS
    mysql       false      <none>              <none>                   <none>          <none>

    Images:
    COMPONENT  TYPE   IMAGE
    mysql      mysql  docker.io/apecloud/wesql-server:8.0.30-5.alpha2.20230105.gd6b8719

    Events(last 5 warnings, see more:kbcli cluster list-events -n default mycluster):
    TIME                        TYPE    REASON      OBJECT                      MESSAGE
    Jan 31,2023 18:11 UTC+0800  Warning Unhealthy   Instance/mycluster-mysql-2  Readiness probe failed: {"event":"roleChanged","role":"Leader"}
    ```

#### Access an ApeCloud MySQL Paxos group

**Option 1.** Use `kbcli`.

If a database cluster has been created and its status is `Running`, run `kbcli cluster connect` to access a specified database cluster. For example, 
```bash
kbcli cluster connect maple05
>
Connect to instance maple05-mysql-2: out of maple05-mysql-2(leader), maple05-mysql-1(follower), maple05-mysql-0(follower)
Welcome to the MySQL monitor.  Commands end with ; or \g.
Your MySQL connection id is 25
Server version: 8.0.30 WeSQL Server - GPL, Release 5, Revision d6b8719

Copyright (c) 2000, 2022, Oracle and/or its affiliates.

Oracle is a registered trademark of Oracle Corporation and/or its
affiliates. Other names may be trademarks of their respective
owners.

Type 'help;' or '\h' for help. Type '\c' to clear the current input statement.

mysql>
```

You can also run the command below to access a cluster by MySQL client.
```bash
kbcli cluster connect --show-example --client=cli maple05
# cluster maple05 does not have public endpoints, you can run following command and connect cluster from local host
kubectl port-forward service/maple05-mysql 3306:3306

# mysql client connection example
mysql -h 127.0.0.1 -P 3306 -u root -paiImelyt


kubectl port-forward service/maple05-mysql 3306:3306
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
kbcli cluster describe maple05

...
Endpoints:
COMPONENT  MODE       INTERNAL          EXTERNAL
mysql      ReadWrite  10.43.29.51:3306  <none>
...
```

Besides accessing a cluster by `IP:PORT` in `Endpoints`, you can also use [the DNS service of Kubernetes](https://kubernetes.io/docs/concepts/services-networking/service/#dns) for service discovery. The format of DNS access is `${service_name}.${namespace}:${port}`.
For example, run the command below and the results show that there are two services in this database cluster. The namespace of Kubernetes is `default`. Then this database can be accessed via `mysql-cluster.default:3306` and `mysql-cluster-ac-mysql-headless.default:3306`.
```bash
kubectl get service | grep maple05

NAME                     CLUSTER-IP      EXTERNAL-IP   PORT(S)                                                          
maple05-mysql-headless   None            <none>        3306/TCP,13306/TCP,9104/TCP,3501/TCP 
maple05-mysql            10.43.206.161   <none>        3306/TCP 
```

#### Delete an ApeCloud MySQL Paxos group

Run the command below to delete a specified database cluster. For example, 
```bash
kbcli cluster delete maple05

Please enter the name again(separate with commas when more than one): maple05
Cluster maple05 deleted
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
   ![Dashboards](../../img/quick_start_dashboards.png)
3. Click **General** -> **MySQL** to monitor the status of the ApeCloud MySQL cluster deployed by Playground.
   ![MySQL_panel](../../img/quick_start_mysql_panel.png)

### High availability of ApeCloud MySQL

ApeCloud MySQL Paxos group delivers high availability with RPO=0 and RTO in less than 30 seconds.
Here we use a simple failure simulation to show you the failure recovery capability of ApeCloud MySQL.

#### Simulate leader pod failure recovery

In this example, we delete the leader pod to simulate a failure.

***Steps:***

1. Run the command below to view the ApeCloud MySQL Paxos group information. View the leader pod name in `Topology`. In this example, the leader pod's name is maple05-mysql-2.
   ```bash
   kbcli cluster describe maple05
   >
   Name: mysql-cluster         Created Time: Jan 27,2023 17:33 UTC+0800
   NAMESPACE        CLUSTER-DEFINITION        VERSION                STATUS         TERMINATION-POLICY
   default          apecloud-mysql            ac-mysql-8.0.30        Running        WipeOut

   Endpoints:
   COMPONENT        MODE             INTERNAL                EXTERNAL
   mysql            ReadWrite        10.43.29.51:3306        <none>

   Topology:
   COMPONENT        INSTANCE               ROLE            STATUS         AZ            NODE                                                 CREATED-TIME
   mysql            maple05-mysql-2        leader          Running        <none>        k3d-kubeblocks-playground-server-0/172.20.0.3        Jan 30,2023 17:33 UTC+0800
   mysql            maple05-mysql-1        follower        Running        <none>        k3d-kubeblocks-playground-server-0/172.20.0.3        Jan 30,2023 17:33 UTC+0800
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
   kubectl delete pod maple05-mysql-2

   pod "maple05-mysql-2" deleted
   ```

3. Run `kbcli cluster connect maple05` to connect to the ApeCloud MySQL Paxos group to test its availability. You can find this group can be accessed within seconds.
   ```bash
   kbcli cluster connect maple05
   >
   Connect to instance maple05-mysql-1: out of maple05-mysql-1(leader), maple05-mysql-2(follower), maple05-mysql-0(follower)
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
   ![NYAN CAT](../../img/quick_start_nyan_cat.png)
4. Uninstall the NYAN CAT demo application.
   ```bash
   kbcli app uninstall nyancat
   ```

## Destroy Playground

Destroying Playground cleans up relevant component services and data:

* Delete all KubeBlocks database clusters, such as ApeCloud MySQL Paxos group
* Uninstall KubeBlocks
* Delete the local Kubernetes clusters created by K3d.
  
Run the command below to destroy Playground.
```bash
kbcli playground destroy
```
