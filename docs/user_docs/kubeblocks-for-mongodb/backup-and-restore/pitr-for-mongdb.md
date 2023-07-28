---
title: PITR for MongoDB
description: PITR guide for a MongoDB cluster
sidebar_position: 3
sidebar_label: PITR
---

# PITR for MongoDB

PITR (Point-in-time-recovery) for MongoDB by KubeBlocks is realized on the basis of object storage. This guide applies to both local and cloud environments.

This section shows how to back up and restore a MongoDB Raft Group.

## Steps

1. Enable PITR.

    ```bash
    kbcli cluster edit-backup-policy mongo-mongodb-backup-policy --set schedule.logfile.enable=true
    updated
    ```

    Check the backup task status.

    ```bash
    # Wait for the logfile backup running (AvailablePods: 1)
    kbcli cluster list-backups 
    NAME                                  SOURCE-CLUSTER   TYPE       STATUS                      TOTAL-SIZE   DURATION   CREATE-TIME                  COMPLETION-TIME              EXPIRATION                   
    2402de22-mongo-default-logfile        mongo            logfile    Running(AvailablePods: 1)    36K                        Jul 07,2023 17:41 UTC+0800                                Jul 14,2023 17:41 UTC+0800 
    ```

    :::note

    If you enable the PITR function for the first time, the system backup oplog only from the current timestamp. By default, an oplog backup is triggered when reaching 1 minute or writing in the database 20 Mi.

    :::

2. Connect to the database and insert test data continuously.

   ```bash
   kbcli cluster connect mongo
   ```

   ```bash
   use test;
   var arr = [];
   for(var i=1; i<=200000 ; i++){
      sleep(100);
      db.t1.insertOne({num:i,date:ISODate(),str: "test insert random string "+ i});
   }
   ```

3. Check the backup status again to verify whether the first oplog backup is successful. When the TOTAL-SIZE is not empty, the oplog backup is completed.

   ```bash
   kbcli cluster list-backups
   >
   NAME                             SOURCE-CLUSTER   TYPE      STATUS                      TOTAL-SIZE   DURATION   CREATE-TIME                  COMPLETION-TIME   EXPIRATION   
   2402de22-mongo-default-logfile   mongo            logfile   Running(AvailablePods: 1)   68K                     Jul 07,2023 17:41 UTC+0800
   ```

4. Backup the database by the snapshot or data file. Here is an example of datafile backup.

   ```bash
   kbcli cluster backup mongo --type datafile
   >
   Backup backup-default-mongo-20230707174559 created successfully, you can view the progress:
           kbcli cluster list-backups --name=backup-default-mongo-20230707174559 -n default
   ```

   Check whether the backup is completed.

   ```bash
   kbcli cluster list-backups mongo
   >
   NAME                                  SOURCE-CLUSTER   TYPE       STATUS      TOTAL-SIZE   DURATION   CREATE-TIME                  COMPLETION-TIME              EXPIRATION                   
   backup-default-mongo-20230707174559   mongo            datafile   Completed                   740K         12s        Jul 07,2023 17:45 UTC+0800   Jul 07,2023 17:46 UTC+0800   Jul 14,2023 17:45 UTC+0800   616K         15s        Jul 07,2023 17:33 UTC+0800   Jul 07,2023 17:33 UTC+0800   Jul 14,2023 17:33 UTC+0800
   ```

5. View the available point for restore.

   ```bash
   kbcli cluster describe mongo
   ```

   `RECOVERABLE-TIME` is the available point for restore.

   <details>
   <summary>Output</summary>
   ```bash
   Name: mongo      Created Time: Jul 07,2023 17:21 UTC+0800
   NAMESPACE   CLUSTER-DEFINITION   VERSION          STATUS    TERMINATION-POLICY   
   default     mongodb              mongodb-5.0.14   Running   Delete               

   Endpoints:
   COMPONENT   MODE        INTERNAL                                        EXTERNAL   
   mongodb     ReadWrite   mongo-mongodb.default.svc.cluster.local:27017   <none>     

   Topology:
   COMPONENT   INSTANCE          ROLE        STATUS    AZ       NODE                    CREATED-TIME                 
   mongodb     mongo-mongodb-0   primary     Running   <none>   minikube/192.168.49.2   Jul 07,2023 17:21 UTC+0800   
   mongodb     mongo-mongodb-1   secondary   Running   <none>   minikube/192.168.49.2   Jul 07,2023 17:21 UTC+0800   
   mongodb     mongo-mongodb-2   secondary   Running   <none>   minikube/192.168.49.2   Jul 07,2023 17:21 UTC+0800   

   Resources Allocation:
   COMPONENT   DEDICATED   CPU(REQUEST/LIMIT)   MEMORY(REQUEST/LIMIT)   STORAGE-SIZE   STORAGE-CLASS     
   mongodb     false       500m / 500m          500Mi / 500Mi           data:20Gi      csi-hostpath-sc   

   Images:
   COMPONENT   TYPE      IMAGE                                                     
   mongodb     mongodb   registry.cn-hangzhou.aliyuncs.com/apecloud/mongo:5.0.14   

   Data Protection:
   AUTO-BACKUP   BACKUP-SCHEDULE   TYPE     BACKUP-TTL   LAST-SCHEDULE   RECOVERABLE-TIME                                                
   Disabled      <none>            <none>   7d           <none>          Jul 07,2023 17:46:04 UTC+0800 ~ Jul 07,2023 17:53:53 UTC+0800
   ```
   </details>

6. Restore the database by PITR.

   ```bash
   kbcli cluster restore mongo-pitr --restore-to-time 'Jul 07,2023 17:52:53 UTC+0800' --source-cluster mongo
   ```

   View the new cluster and wait for the cluster to run.

   ```bash
   kbcli cluster list mongo-pitr
   ```

   View the restore oplog job and wait for this job to be completed.

   ```bash
   kubectl get job restore-logic-mongo-pitr-mongodb-0
   ```

7. Connect to this new cluster and view `test.t1 collection` to view whether this cluster is resotred to the specified point of the original cluster.

   ```bash
   kbcli cluster connect mongo-pitr
   >
   ...
   mongo-pitr-mongodb [primary] admin> use test;
   switched to db test
   mongo-pitr-mongodb [primary] test> db.t1.find().count()
   1447
   mongo-pitr-mongodb [primary] test> db.t1.find({date:{$lte:ISODate("2023-07-07T09:52:53Z")}}).count()
   1447
   ```
