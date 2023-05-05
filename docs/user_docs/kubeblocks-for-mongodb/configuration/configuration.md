---
title: Configure cluster parameters
description: Configure cluster parameters
keywords: [parameter, configuration, reconfiguration]
sidebar_position: 1
---

# Configure cluster parameters

The KubeBlocks configuration function provides a set of consistent default configuration generation strategies for all the databases running on KubeBlocks and also provides a unified parameter configuration interface to facilitate managing parameter reconfiguration, searching the parameter user guide, and validating parameter effectiveness.

## View parameter information

View the current configuration file of a cluster.
```bash
kbcli cluster describe-config mongodb-cluster

ConfigSpecs Meta:
CONFIG-SPEC-NAME         FILE                  ENABLED   TEMPLATE                     CONSTRAINT                   RENDERED                                            COMPONENT    CLUSTER           
mongodb-config           keyfile               false     mongodb5.0-config-template   mongodb-config-constraints   mongodb-cluster-replicaset-mongodb-config           replicaset   mongodb-cluster   
mongodb-config           mongodb.conf          true      mongodb5.0-config-template   mongodb-config-constraints   mongodb-cluster-replicaset-mongodb-config           replicaset   mongodb-cluster   
mongodb-metrics-config   metrics-config.yaml   false     mongodb-metrics-config                                    mongodb-cluster-replicaset-mongodb-metrics-config   replicaset   mongodb-cluster   

History modifications:
OPS-NAME   CLUSTER   COMPONENT   CONFIG-SPEC-NAME   FILE   STATUS   POLICY   PROGRESS   CREATED-TIME   VALID-UPDATED 
```

From the meta information, the cluster `mongodb-cluster` has a configuration file named `mongodb.conf`.

You can also view the details of this configuration file and parameters.

* View the details of the current configuration file.

   ```bash
   kbcli cluster describe-config mongodb-cluster --show-detail
   ```


## Reconfigure parameters

The example below reconfigures velocity to 1.

1. Adjust the values of `velocity` to 1.

   ```
   $ kbcli cluster configure mongodb-cluster --component mongodb --config-spec mongodb-config --config-file mongodb.conf --set systemLog.verbosity=1
   Warning: The parameter change you modified needs to be restarted, which may cause the cluster to be unavailable for a period of time. Do you need to continue...
   Please type "yes" to confirm: yes
   Will updated configure file meta:
   ConfigSpec: mongodb-config      ConfigFile: mongodb.conf      ComponentName: mongodb  ClusterName: mongodb-cluster
   OpsRequest mongodb-cluster-reconfiguring-q8ndn created successfully, you can view the progress:
          kbcli cluster describe-ops mongodb-cluster-reconfiguring-q8ndn -n default
  ```

2. Check configure history.

  ```
    kbcli cluster describe-config mongodb-cluster
    ConfigSpecs Meta:
    CONFIG-SPEC-NAME         FILE                  ENABLED   TEMPLATE                     CONSTRAINT                   RENDERED                                         COMPONENT   CLUSTER
    mongodb-config           keyfile               false     mongodb5.0-config-template   mongodb-config-constraints   mongodb-cluster-mongodb-mongodb-config           mongodb     mongodb-cluster
    mongodb-config           mongodb.conf          true      mongodb5.0-config-template   mongodb-config-constraints   mongodb-cluster-mongodb-mongodb-config           mongodb     mongodb-cluster
    mongodb-metrics-config   metrics-config.yaml   false     mongodb-metrics-config                                    mongodb-cluster-mongodb-mongodb-metrics-config   mongodb     mongodb-cluster

    History modifications:
    OPS-NAME                              CLUSTER           COMPONENT   CONFIG-SPEC-NAME   FILE           STATUS    POLICY    PROGRESS   CREATED-TIME                 VALID-UPDATED
    mongodb-cluster-reconfiguring-q8ndn   mongodb-cluster   mongodb     mongodb-config     mongodb.conf   Succeed   restart   3/3        Apr 21,2023 18:56 UTC+0800   {"mongodb.conf":"{\"systemLog\":{\"verbosity\":\"1\"}}"}```
  ```

3. Verify change result.

```
    root@mongodb-cluster-mongodb-0:/# cat etc/mongodb/mongodb.conf |grep verbosity
    verbosity: "1"
```

