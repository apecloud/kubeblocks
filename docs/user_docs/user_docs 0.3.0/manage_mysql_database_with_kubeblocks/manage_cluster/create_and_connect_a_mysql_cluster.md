# Create and connect to a MySQL Cluster
## Create a MySQL Cluster
***Before you start***
Run the command below to view all the database types available for creating a cluster. 
```
kbcli clusterdefinition list
```
***Result***
```
$ kbcli clusterdefinition list
NAME             MAIN-COMPONENT-TYPE   STATUS      AGE
apecloud-mysql   mysql                 Available   7m52s
```
***Steps:***
1. Run the command below to list all the available kernel versions and choose one that you need.
   ```
   kbcli clusterversion list
   ```
   ***Example***

   ```
   $ kbcli clusterversion list
   NAME              CLUSTER-DEFINITION   STATUS      AGE
   ac-mysql-8.0.30   apecloud-mysql       Available   2m40s
   ```
2. Run the command below to create a MySQL cluster. 
```
$ kbcli cluster create --cluster-definition='apecloud-mysql'
```
> Note:
> If you want to create a Paxos group, set `replicas` as 3.

***Example***
```
$ kbcli cluster create --cluster-definition="apecloud-mysql" --set -<<EOF
- name: mysql
  replicas: 3
  type: mysql
  volumeClaimTemplates:
  - name: data
    spec:
      accessModes:
      - ReadWriteOnce
      resources:
        requests:
          storage: 10Gi
EOF
```

## Connect to a MySQL Cluster
***Steps:***
Run the command below to connect to a cluster.
```
kbcli cluster connect NAME
```

***Example***

```
$ kbcli cluster connect mysql-01
Connect to instance mysql-01-mysql-0: out of mysql-01-mysql-0(leader)
Welcome to the MySQL monitor.  Commands end with ; or \g.
Your MySQL connection id is 16
Server version: 8.0.30 WeSQL Server - GPL, Release 5, Revision d6b8719

Copyright (c) 2000, 2022, Oracle and/or its affiliates.

Oracle is a registered trademark of Oracle Corporation and/or its
affiliates. Other names may be trademarks of their respective
owners.

Type 'help;' or '\h' for help. Type '\c' to clear the current input statement.

mysql>
```