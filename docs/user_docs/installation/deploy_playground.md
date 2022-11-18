# Deploy Playground

Description to be filled.

## Install dependency

- Docker

  Install and run Docker before running `playground`. Docker version should be V20.10.5 or above.

  Refer to [Install Docker Desktop on Mac](https://docs.docker.com/desktop/install/mac-install/) for details.

## Deploy `playground` on local host

> **Note**
> During the deployment process, files will be downloaded from image and the downloading might be slow. It is recommended to enable VPN while downloading. ***# Is this a promblem for overseas users?***

Run this command to deploy `playground`.

```
dbctl playground init
```

Run the following command to view the deployed database instance status.

```
export KUBECONFIG=~/.kube/dbctl-playground
dbctl cluster list
```

When the instance changes to `ONLINE` status, the instance is deployed successfully.

## Deploy `playground` on AWS EC2

You can also deploy `playgournd` on AWS EC2 by following the steps below.

### Dependency

An AWS access key is required and this account should have the searching and changing permission of VPC/Internet Gateway/Subnet/Route Table/Security Group/EC2 resources. ***# This whole part is written based on the version in August. Need to be updated.***

> **Caution**
> DO NOT switch your network connection during the deployment and using process. Switching network changes the IP, which results in connection failure.

Replace `--access-key` and `--access-secret` with your AWS access key and run this command to deploy `playground` on AWS.

```
dbctl playground init --access-key xxx --acces-secret xxx --cloud-provider aws
```

The following information wil be displayed when `playground` is deployed successfully.

```
Notes:
Open DBaaS Playground v0.2.0 Start SUCCESSFULLY!
MySQL Standalone Cluster "mycluster" has been CREATED!

1. Basic commands for dbcluster:
  dbctl --kubeconfig ~/.kube/opencli-playground dbcluster list                          # list all database clusters
  dbctl --kubeconfig ~/.kube/opencli-playground dbcluster describe mycluster       # get dbcluster information
  dbctl bench --host 54.222.159.218 tpcc mycluster                                  # run tpcc benchmark 1min on dbcluster

2. To connect to mysql database:
  MYSQL_ROOT_PASSWORD=$(kubectl --kubeconfig ~/.kube/dbctl-playground get secret --namespace default mycluster-cluster-secret -o jsonpath="{.data.rootPassword}" | base64 -d)
  mysqlsh -h 54.222.159.218 -uroot -p"$MYSQL_ROOT_PASSWORD"

3. To view the Grafana:
  open http://54.222.159.218:9100/d/549c2bf8936f7767ea6ac47c47b00f2a/mysql_for_demo
  User: admin
  Password: prom-operator

4. Uninstall Playground:
  dbctl playground destroy

--------------------------------------------------------------------
To view this guide next time:         opencli playground guide
To get more help information:         opencli help
To login to remote host:              ssh -i ~/.opendbaas/ssh/id_rsa ec2-user@54.222.159.218
Use "dbctl [command] --help" for more information about a command.
```

> **Note**
> If the deployment fails, run `dbctl playground destroy` to clean the environment and execute the above command again.
> Run `dbctl playground guide` to display the installation information again.

## Verify deployment

TBD

## Destory

Run this command to destroy the instance created by `playground`.

```
dbctl playground destroy
```

## Reference

Refer to the following links to find detailed information about the CLIs used above.

- [`dbctl playground`](../cli/dbctl_playground.md)
- [`dbctl playground init`](../cli/dbctl_playground_init.md)
- [`dbctl playground guide`](../cli/dbctl_playground_guide.md)
- [`dbctl playground destroy`](../cli/dbctl_playground_destroy.md)