# Cluster restarting

This section gives you an overview of how KubeBlocks restarts a cluster.

## Before you start

Make sure you are familiar with the following `KubeBlocks` concepts:

- KubeBlocks #links to be completed
- [KubeBlocks OpsRequest](../configure_ops_request.md) 

## How the restarting process works

The diagram below illustrates how KubeBlocks restarts a WeSQL database cluster.

![Restart process](../../../img/docs_restart_process.jpg)

Restarting consists of the following steps:

1. A user creates a restart opsRequest `CR`.
2. Restart opsRequest `CR` passes the webhook validation.
3. Add restart annotation to the StatefulSets corresponding to the components.
4. The opsRequest controller changes the cluster phase to `Updating`.
5. The component controller watches for StatefulSet and pods.
6. When the component type is `Stateful`, Kubernetes StatefulSet controller performs a rolling update on the pods. When the component type is `consensus`/`replicationset`, the component controller restarts the pods.
7. When restarting is completed, the component controller changes the component phase to `Running`.
8. The cluster controller watches for the component phase and changes the cluster phase to `Running`.