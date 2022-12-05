# Install playground

KuebBlocks playground creates an easy-to-use database environment for Kubernetes users.

## Before you start

- `kbcli`
  
  Install [`kbcli`](install_kbcli.md) first.

- Docker

  Install and run Docker before running a playground. The Docker version should be v20.10.5 or above.

  Refer to [Install Docker Desktop on Mac](https://docs.docker.com/desktop/install/mac-install/) for details.

## Deploy a playground on a local host

> **Note** <br>
> During the deployment process, files will be downloaded from Docker image and downloading might be slow. It is recommended to enable VPN while downloading. ***# Is this a promblem for overseas users?***

_Steps_:

1. Run this command to install `playground`.

```
kbcli playground init
```

2. Run this command to view the installed database instance status. When the instance status changes to `ONLINE`, the instance is installed successfully.

```
export KUBECONFIG=~/.kube/kbcli-playground
kbcli cluster list
```

## Deploy a playground on AWS EC2

You can also deploy `playground` on AWS EC2 by following the steps below.

### Before you start

Make sure the following requirements are met.

- AWS access key: An AWS access key is required and this account should have the searching and changing permission of VPC/Internet Gateway/Subnet/Route Table/Security Group/EC2 resources. ***Environment dependencies are required. Need to be confirmed and added. ***
- EKS
- Self-owned Kubernetes
- A fresh and clean start

> **Caution** <br>
> DO NOT switch your network during the deployment and using process. Switching network will change the IP address, which results in connection failure.

Replace `--access-key` and `--access-secret` with your AWS access key and run this command to deploy a playground on AWS.

```
kbcli playground init --access-key xxx --access-secret xxx --cloud-provider aws
```

### Result
  The following information will be displayed when a playground is installed successfully.

```
Notes:
KubeBlocks Playground v0.2.0 Start SUCCESSFULLY!
MySQL Standalone Cluster "mycluster" has been CREATED!

1. Basic commands for dbcluster:
  kbcli --kubeconfig ~/.kube/opencli-playground dbcluster list                          # list all database clusters
  kbcli --kubeconfig ~/.kube/opencli-playground dbcluster describe mycluster       # get dbcluster information
  kbcli bench --host 54.222.159.218 tpcc mycluster                                  # run tpcc benchmark 1min on dbcluster

2. To connect to mysql database:
  MYSQL_ROOT_PASSWORD=$(kubectl --kubeconfig ~/.kube/kbcli-playground get secret --namespace default mycluster-cluster-secret -o jsonpath="{.data.rootPassword}" | base64 -d)
  mysqlsh -h 54.222.159.218 -uroot -p"$MYSQL_ROOT_PASSWORD"

3. To view the Grafana:
  open http://54.222.159.218:9100/d/549c2bf8936f7767ea6ac47c47b00f2a/mysql_for_demo
  User: admin
  Password: prom-operator

4. Uninstall Playground:
  kbcli playground destroy

--------------------------------------------------------------------
To view this guide next time:         kbcli playground guide
To get more help information:         kbcli help
To login to remote host:              ssh -i ~/.kubeblocks/ssh/id_rsa ec2-user@54.222.159.218
Use "kbcli [command] --help" for more information about a command.
```

> **Note** <br>
> If the installation fails, run `kbcli playground destroy` to clean the environment and execute the above command again.
> Run `kbcli playground guide` to display the installation information again.

## Verify deployment

TBD

## Destroy

Run this command to destroy the instance created by `playground`.

```
kbcli playground destroy
```

## What's next

You can [install KubeBlock](install_kubeblocks.md) in this playground with `kbcli` commands.

## Reference

Refer to the following links to find detailed information about the CLIs used above.

- [`kbcli playground`](../cli/kbcli_playground.md)
- [`kbcli playground init`](../cli/kbcli_playground_init.md)
- [`kbcli playground guide`](../cli/kbcli_playground_guide.md)
- [`kbcli playground destroy`](../cli/kbcli_playground_destroy.md)