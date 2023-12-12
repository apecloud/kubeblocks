---
title: 管理账户
description: 如何管理账户
keywords: [账户管理]
sidebar_position: 1
sidebar_label: 管理账户
---

# 管理账户

KubeBlocks 提供了多种服务增强数据库集群的可用性、易用性和可观测性。不同组件所需要的用户权限不同。

***步骤***

- 创建账户。
  
  ```bash
  kbcli cluster create-account <clustername> --name <username> --password <pwd> 
  ```

- 授予角色。

  ```bash
  kbcli cluster grant-role  <clustername> --name <username> --role <rolename>
  ```

  KubeBlocks提供了三个级别的角色。
  - Superuser：具有所有权限。
  - ReadWrite：具有读和写权限。
  - ReadOnly：只具有读权限。

  不同的数据库引擎的权限有所不同。详情请参阅下表。

    | 角色      | MySQL    | PostgreSQL | Redis  |
    | :------   | :------- | :------    | :----- |
    | Superuser | GRANT SELECT, INSERT, UPDATE, DELETE, CREATE, DROP, RELOAD, SHUTDOWN, PROCESS, FILE, REFERENCES, INDEX, ALTER, SHOW DATABASES, SUPER, CREATE TEMPORARY TABLES, LOCK TABLES, EXECUTE, REPLICATION SLAVE, REPLICATION CLIENT, CREATE VIEW, SHOW VIEW, CREATE ROUTINE, ALTER ROUTINE, CREATE USER, EVENT, TRIGGER, CREATE TABLESPACE, CREATE ROLE, DROP ROLE ON * a user | ALTER USER WITH SUPERUSER | +@ALL allkeys|
    | ReadWrite | GRANT SELECT, INSERT, DELETE ON * TO a user | GRANT pg_write_all_data TO a user | -@ALL +@Write +@READ allkeys |
    | ReadOnly  | GRANT SELECT, SHOW VIEW ON * TO a user | GRANT pg_read_all_data TO a user | -@ALL +@READ allkeys |

- 检查角色级别。
  
  ```bash
  kbcli cluster describe-account <clustername> --name <username>
  ```

- 撤销账户角色。

  ```bash
  kbcli cluster revoke-role <clustername> --name <name> --role <rolename> 
  ```

- 列出所有账户。

  ```bash
  kbcli cluster list-accounts  <clustername>  
  ```

  :::note

  出于安全原因，`list-accounts` 命令不显示所有账户。符合某些规则的高权限账户（例如操作账户和超级用户账户）会被隐藏。参考下表查看隐藏账户。

  :::

  | 数据库     | 隐藏账户                            |
  |------------|-------------------------------------|
  | MySQL      | root <br />kb* <br />Localhost = '' |
  | PostgreSQL | Postgres <br />kb*                  |

- 删除账户。

  ```bash
  kbcli cluster delete-account <clustername> --name <username> 
  ```
