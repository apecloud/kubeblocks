---
title: Manage user accounts
description: How to manage user accounts
keywords: [user account]
sidebar_position: 1
sidebar_label: Manage user accounts
---

# Manage user accounts

KubeBlocks offers a variety of services to enhance the usability, availability, and observability of database clusters. Different components require user accounts with different permissions to create connections.

***Steps***

- Create a user account

  ```bash
  kbcli cluster create-account <clustername> --name <username> --password <pwd> 
  ```

- Grant a role to a user

  ```bash
  kbcli cluster grant-role  <clustername> --name <username> --role <rolename>
  ```

  KubeBlocks provides three role levels of permission.

  - Superuser: with all permissions.
  - ReadWrite: read and write.
  - ReadOnly: read only.
  
  For different database engines, the detailed permission are varied. Check the table below.

    | Role      | MySQL    | PostgreSQL | Redis  |
    | :------   | :------- | :------    | :----- |
    | Superuser | GRANT SELECT, INSERT, UPDATE, DELETE, CREATE, DROP, RELOAD, SHUTDOWN, PROCESS, FILE, REFERENCES, INDEX, ALTER, SHOW DATABASES, SUPER, CREATE TEMPORARY TABLES, LOCK TABLES, EXECUTE, REPLICATION SLAVE, REPLICATION CLIENT, CREATE VIEW, SHOW VIEW, CREATE ROUTINE, ALTER ROUTINE, CREATE USER, EVENT, TRIGGER, CREATE TABLESPACE, CREATE ROLE, DROP ROLE ON * a user | ALTER USER WITH SUPERUSER | +@ALL allkeys|
    | ReadWrite | GRANT SELECT, INSERT, DELETE ON * TO a user | GRANT pg_write_all_data TO a user | -@ALL +@Write +@READ allkeys |
    | ReadOnly  | GRANT SELECT, SHOW VIEW ON * TO a user | GRANT pg_read_all_data TO a user | -@ALL +@READ allkeys |

- Check role level of a user account

  ```bash
  kbcli cluster describe-account <clustername> --name <username>
  ```

- Revoke role from a user account

  ```bash
  kbcli cluster revoke-role <clustername> --name <name> --role <rolename> 
  ```

- List all user accounts

  ```bash
  kbcli cluster list-accounts  <clustername>  
  ```

  :::note

  For security reasons, ```list-accounts```command does not showw all accounts. Accounts with high previledge such as operational accounts and superuser accounts that meet certain rules are hidden.
  | Database | Hidden Accounts                |
  | :---     | :---                           |
  | MySQL    | root <br />kb* <br />Localhost = '' |
  | postGre  | Postgres <br />kb*              |

  :::

- Delete a user account

  ```bash
  kbcli cluster delete-account <clustername> --name <username> 
  ```
