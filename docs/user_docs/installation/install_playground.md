# Install playground

KuebBlocks playground creates an easy-to-use database environment for Kubernetes users.

## Before you start

- `kbcli`
  
  Install [`kbcli`](install_kbcli.md) first.

- Docker

  Install and run Docker before running a playground. The Docker version should be v20.10.5 or above.

  Refer to [Install Docker Desktop on Mac](https://docs.docker.com/desktop/install/mac-install/) for details.

## Deploy a playground on a local host

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

- AWS access key: An AWS access key is required and this account should have the searching and changing permission of VPC/Internet Gateway/Subnet/Route Table/Security Group/EC2 resources. 

> **Caution**<br />
> DO NOT switch your network during the deployment and using process. Switching network will change the IP address, which results in connection failure.

Replace `--access-key` and `--access-secret` with your AWS access key and run this command to deploy a playground on AWS.

```
kbcli playground init --access-key xxx --access-secret xxx --cloud-provider aws
```

### Result
  The following information will be displayed when a playground is installed successfully.

```
KubeBlocks playground init SUCCESSFULLY!
Cluster "mycluster" has been CREATED!

1. Basic commands for cluster:

  export KUBECONFIG=/Users/admin/.kube/kubeblocks-playground

  kbcli cluster list                     # list database cluster and check its status
  kbcli cluster describe mycluster       # get cluster information

2. Connect to database

  kbcli cluster connect mycluster

3. View the Grafana:

  kbcli dashboard open kubeblocks-grafana

4. Uninstall Playground:

  kbcli playground destroy

--------------------------------------------------------------------
To view this guide: kbcli playground guide
To get more help: kbcli help

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

## Next step

You can [install KubeBlock](install_kubeblocks.md) in this playground with `kbcli` commands.

## Reference

Refer to the following links to find detailed information about the CLIs used above.

- [`kbcli playground`](../cli/kbcli_playground.md)
- [`kbcli playground init`](../cli/kbcli_playground_init.md)
- [`kbcli playground guide`](../cli/kbcli_playground_guide.md)
- [`kbcli playground destroy`](../cli/kbcli_playground_destroy.md)