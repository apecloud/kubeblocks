---
title: Connect database in testing environment
description: How to connect to a database in testing environment
keywords: [connect to a database, testing environment, test environment]
sidebar_position: 2
sidebar_label: Testing environment
---

# Connect database in testing environment

## Procedure 1. Use kbcli cluster connect command

You can use the `kbcli cluster connect` command and specify the cluster name to be connected.

```bash
kbcli cluster connect ${cluster-name}
```

The lower-level command is actually `kubectl exec`. The command is functional as long as the K8s API server is accessible.

## Procedure 2. Connect database with CLI or SDK client

Execute the following command to get the network information of the targeted database and connect it with the printed IP address.

```bash
kbcli cluster connect --show-example ${cluster-name}
```

Information printed includes database addresses, port No., username, password. The figure below is an example of MySQL database network information.

- Address: -h specifies the server address. In the example below it is 127.0.0.1
- Port: -P specifies port No. , In the example below it is 3306.
- User: -u is the user name.
- Password: -p shows the password. In the example below, it is hQBCKZLI.

:::note

The password does not include -p.

:::

![Example](./../../img/connect_database_with_CLI_or_SDK_client.png)
