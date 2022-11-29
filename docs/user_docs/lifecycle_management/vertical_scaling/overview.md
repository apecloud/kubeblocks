# Vertical scaling

This section gives you an overview of how KubeBlocks scales a MySQL database cluster.

## Before you start

You should be familiar with the following `KubeBlocks` concepts:

- KubeBlocks #links to be completed
- [KubeBlocks OpsRequest](../configure_ops_request.md)
  
## How vertical scaling works

The below diagram illustrates how KubeBlocks increases MySQL database capacity.

![Vertical scaling process](../../../img/docs_vertical_scaling_process.jpg)

The vertical scaling consists of the following steps:

1. A user creates a vertical scaling OpsRequest.
2. This OpsRequest passes webhook validation.
3. The OpsRequest controller applies this OpsRequest requirement to the specified components.
4. The WeSQL cluster phase changes to `Updating`.
5. The cluster controller watches for the WeSQL cluster.
6. The cluster controller applies scaling to StatefulSet block.
7. The StatefulSet block applies vertical scaling to pods.
8. The StatefulSet controller watches for the StatefulSet block.
9. Vertical scaling completes and reports to the cluster controller.
10. The WeSQL cluster phase changes to `Running`.