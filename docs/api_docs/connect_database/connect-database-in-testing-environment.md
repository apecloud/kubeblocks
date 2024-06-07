---
title: Connect database in testing environment
description: How to connect to a database in testing environment
keywords: [connect to a database, testing environment, test environment]
sidebar_position: 2
sidebar_label: Testing environment
---

# Connect database in the testing environment

## Procedure 1. Use kubectl exec command

If the database address is not required, run the command below to connect to the cluster via the default address.

You can use the `kbcli cluster connect` command and specify the cluster name to be connected.

   ```bash
   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
   >
   root

   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
   >
   2gvztbvz
   ```

2. Run the `kubectl exec` command and specify the pod to be connected.

   ```bash
   kubectl exec -ti -n demo mycluster-mysql-0 -- bash
   ```

3. Connect to the cluster.

   ```bash
   mysql -u root -p2gvztbvz
   ```

## Procedure 2. Connect database with CLI or SDK client

For a pod without a client, you can follow the steps below to connect to the cluster. You can also connect to the cluster by other options, like CLI, SDK client, go connection, java connection, etc.

Below is an example of using CLI to connect to the cluster on the local host.

1. Get the `username` and `password` for the cluster.

   ```bash
   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
   >
   root

   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
   >
   2gvztbvz
   ```

2. Run the command below to connect to the cluster.

   ```bash
   kubectl port-forward svc/mycluster-mysql 3306:3306 -n demo

   mysql -h 127.0.0.1 -P 3306 -u root -p2gvztbvz
   ```
