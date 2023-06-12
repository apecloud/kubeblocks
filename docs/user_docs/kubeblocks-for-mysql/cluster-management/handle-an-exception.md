---
title: Handle an exception
description: How to handle an exception in a MySQL cluster
keywords: [mysql, exception]
sidebar_position: 7
sidebar_label: Handle an exception
---

# Handle an exception

When an exception occurs during your operation, you can perform the following steps to solve it.

## Steps

1. Check the cluster status.

    ```bash
    kbcli cluster list mysql-cluster
    ```

2. Handle the exception according to the status information.

    | **Status**       | **Information** |
    | :---             | :---            |
    | Abnormal         | The cluster can be accessed but exceptions occur in some pods. This might be a mediate status of the operation process and the system recovers automatically without executing any extra operation. Wait until the cluster status changes to `Running`. |
    | ConditionsError  | The cluster is normal but an exception occurs to the condition. It might be caused by configuration loss or exception, which further leads to operation failure. Manual recovery is required. |
    | Failed | The cluster cannot be accessed. Check the `status.message` string and get the exception reason. Then manually recover it according to the hints. |

    You can check the cluster's status for more information.

## Fallback strategies

If the above operations can not solve the problem, try the following steps:

- Restart this cluster. If the restart fails, you can delete the pod manually.
- Roll the cluster status back to the status before changes.
