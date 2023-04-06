# Manage user accounts
KubeBlocks offers a variety of services to enhance the usability, availability, and observability of database clusters. Different components require user accounts with different permissions to create connections. 
***Steps***
- Creating a user account
  ```
  kbcli cluster create-account <clustername> --username <name> --password <pwd> 
  ```
- Deleting a user account
  ```
  kbcli cluster delete-account <clustername> --username <name> 
  ```
- Grant role to a user
  ```
  kbcli cluster grant-role  <clustername> --username <name> --rolename <rname>
  ```
     KubeBlocks provides three role levels of permission.
    - Superuser: with all permissions.
    - ReadWrite: read and writer.
    - ReadOnly: read only.
    For different database engines, the detailed permission are varied. Check the table below.

 | Role | MySQL | PostgreSQL | Redis |
 |------|-------|------|-----|
 |Superuser|GRANT SELECT, INSERT, UPDATE, DELETE, CREATE, DROP, RELOAD, SHUTDOWN, PROCESS, FILE, REFERENCES, INDEX, ALTER, SHOW DATABASES, SUPER, CREATE TEMPORARY TABLES, LOCK TABLES, EXECUTE, REPLICATION SLAVE, REPLICATION CLIENT, CREATE VIEW, SHOW VIEW, CREATE ROUTINE, ALTER ROUTINE, CREATE USER, EVENT, TRIGGER, CREATE TABLESPACE, CREATE ROLE, DROP ROLE ON . <username>|ALTER USER <username> WITH SUPERUSER|+@ALL allkeys|
|ReadWrite|GRANT SELECT, INSERT, DELETE ON . TO <username>|GRANT pg_write_all_data TO <username>|-@ALL +@Write +@READ allkeys|
|ReadOnly|GRANT SELECT, SHOW VIEW ON . TO <username>|GRANT pg_read_all_data TO <username>| -@ALL +@READ allkeys|
  - Check user account role level
  ```
  kbcli cluster describe-account <clustername> --username <name>
  ```
  - Revoke role from user account 
  ```
  kbcli cluster revoke-role <clustername> --username <name> --rolename <rname> 
  ```
- List all user accounts
```
kbcli cluster list-accounts  <clustername>  
```
:::Note:
For security reasons, ```list-accounts```command does not showw all accounts. Accounts with high previledge such as operational accounts and superuser accounts that meet certain rules are hidden. 
| Database|Hidden Accounts|
|---|---|
|MySQL| root<br>kb* <br>Localhost = ''|
|postGre|Postgres <br> kb*|
:::
















