---
title: Connect database in testing environment
description: How to connect to a database in testing environment
keywords: [connect to a database, testing environment, test environment]
sidebar_position: 2
sidebar_label: Testing environment
---

# Connect database in testing environment

## Procedure 1. Use kbcli cluster connect command

1. Get the `username` and `password` for the cluster.

   ```bash
   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\username}' | base64 -d
   >
   root

   kubectl get secrets -n demo mycluster-conn-credential -o jsonpath='{.data.\password}' | base64 -d
   >
   2gvztbvz
   ```

2. Run the `kubectl exec` command and specify the cluster name to be connected.

   ```bash
   kubectl exec -ti -n demo mycluster-mysql-0 -- bash
   ```

The `kubectl exec` command is functional as long as the K8s API server is accessible.

## Procedure 2. Connect database with CLI or SDK client

You can use `kubectl exec` to connet to this cluster by following the steps in Procedure 1, and then connect to the cluster with the client you prefer.

```bash
kubectl exec -ti -n demo mycluster-mysql-0 -- bash
```
