---
title: kbcli migration create
---

Create a migration task.

```
kbcli migration create NAME [flags]
```

### Examples

```
  # Create a migration task to migrate the entire database under mysql: mydb1 and mytable1 under database: mydb2 to the target mysql
  kbcli migration create mytask --template apecloud-mysql2mysql
  --source user:123456@127.0.0.1:3306
  --sink user:123456@127.0.0.1:3305
  --migration-object '"mydb1","mydb2.mytable1"'
  
  # Create a migration task to migrate the schema: myschema under database: mydb1 under PostgreSQL to the target PostgreSQL
  kbcli migration create mytask --template apecloud-pg2pg
  --source user:123456@127.0.0.1:3306/mydb1
  --sink user:123456@127.0.0.1:3305/mydb1
  --migration-object '"myschema"'
  
  # Use prechecks, data initialization, CDC, but do not perform structure initialization
  kbcli migration create mytask --template apecloud-pg2pg
  --source user:123456@127.0.0.1:3306/mydb1
  --sink user:123456@127.0.0.1:3305/mydb1
  --migration-object '"myschema"'
  --steps precheck=true,init-struct=false,init-data=true,cdc=true
  
  # Create a migration task with two tolerations
  kbcli migration create mytask --template apecloud-pg2pg
  --source user:123456@127.0.0.1:3306/mydb1
  --sink user:123456@127.0.0.1:3305/mydb1
  --migration-object '"myschema"'
  --tolerations '"step=global,key=engineType,value=pg,operator=Equal,effect=NoSchedule","step=init-data,key=diskType,value=ssd,operator=Equal,effect=NoSchedule"'
  
  # Limit resource usage when performing data initialization
  kbcli migration create mytask --template apecloud-pg2pg
  --source user:123456@127.0.0.1:3306/mydb1
  --sink user:123456@127.0.0.1:3305/mydb1
  --migration-object '"myschema"'
  --resources '"step=init-data,cpu=1000m,memory=1Gi"'
```

### Options

```
  -h, --help                       help for create
      --migration-object strings   Set the data objects that need to be migrated,such as '"db1.table1","db2"'
      --resources strings          Resources limit for migration, such as '"cpu=3000m,memory=3Gi"'
      --sink string                Set the sink database information for migration.such as '{username}:{password}@{connection_address}:{connection_port}/[{database}]
      --source string              Set the source database information for migration.such as '{username}:{password}@{connection_address}:{connection_port}/[{database}]'
      --steps strings              Set up migration steps,such as: precheck=true,init-struct=true,init-data=true,cdc=true
      --template string            Specify migration template, run "kbcli migration templates" to show all available migration templates
      --tolerations strings        Tolerations for migration, such as '"key=engineType,value=pg,operator=Equal,effect=NoSchedule"'
```

### Options inherited from parent commands

```
      --as string                      Username to impersonate for the operation. User could be a regular user or a service account in a namespace.
      --as-group stringArray           Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --as-uid string                  UID to impersonate for the operation.
      --cache-dir string               Default cache directory (default "$HOME/.kube/cache")
      --certificate-authority string   Path to a cert file for the certificate authority
      --client-certificate string      Path to a client certificate file for TLS
      --client-key string              Path to a client key file for TLS
      --cluster string                 The name of the kubeconfig cluster to use
      --context string                 The name of the kubeconfig context to use
      --disable-compression            If true, opt-out of response compression for all requests to the server
      --insecure-skip-tls-verify       If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string              Path to the kubeconfig file to use for CLI requests.
      --match-server-version           Require server version to match client version
  -n, --namespace string               If present, the namespace scope for this CLI request
      --request-timeout string         The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
  -s, --server string                  The address and port of the Kubernetes API server
      --tls-server-name string         Server name to use for server certificate validation. If it is not provided, the hostname used to contact the server is used
      --token string                   Bearer token for authentication to the API server
      --user string                    The name of the kubeconfig user to use
```

### SEE ALSO

* [kbcli migration](kbcli_migration.md)	 - Data migration between two data sources.

#### Go Back to [CLI Overview](cli.md) Homepage.

