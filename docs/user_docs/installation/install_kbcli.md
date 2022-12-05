# Install `kbcli`

`kbcli` is the KubeBlocks CLI tool. This section introduces how to install and uninstall `kbcli`. 

For more information on the KubeBlocks CLIs, refer to [KubeBlocks commands overview](../cli/kubeblocks%20commands%20overview.md).

## Before you start

The following dependencies are required for deploying `kbcli`.

- MySQL Shell
  Install MySQL Shell in your local host to visit MySQL instances. Refer to [Install MySQL Shell on macOS](https://dev.mysql.com/doc/mysql-shell/8.0/en/mysql-shell-install-macos-quick.html) for details.

- `kubectl`
  Run the following command to install `kubectl` in your local host for visiting Kubernetes clusters. Refer to [Install and Set Up kubectl on macOS](https://kubernetes.io/docs/tasks/tools/install-kubectl-macos/) for details.

    ```
    curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/darwin/arm64/kubectl"
    ```

- Kubernetes cluster
  `kbcli` visits a Kubernetes cluster which can be specified by the `kubeconfig` condition variable or the `kubeconfig` parameter. If no Kubernetes cluster is specified, `kbcli` reads the content in `~/.kube/config` file by default.

## Install `kbcli`

_Steps_:
1. Installing `kbcli` by curl and make are supported.

  - curl
  `kbcli` can run on macOS, Linux, and Windows. Copy and run the command below.
    ```
    curl -fsSL http://161.189.136.182:8000/apecloud/kubeblocks/install_kbcli.sh |bash
    ```

   - make
    Download the source code and execute the following commands under the root directory of the source code. Make and generate `kbcli` executive file. Make sure the executive file is under the `bin/kbcli` path.

      ```
      # Switch to the `main` branch
      git checkout main
      git pull

      # Make `kbcli`
      GIT_VERSION=`git describe --always --abbrev=0 --tag`
      VERSION=`echo "${GIT_VERSION/v/}"`
      make kbcli
      ```

2. Run this command to check the version after installation.
  ```
  kbcli version
  ```

## Uninstall `kbcli`

Run this command to uninstall `kbcli`.

```
sudo rm /usr/local/bin/kbcli
```

If you install `kbcli` by make, run the command below to clean the generated `kbcli`.

```
make clean-kbcli
```

## What's next

With `kbcli` installed, you can [install a playground](install_playground.md) to create a database environment.


## FAQ

- Q1: What do I do when an error, `- dial unix /var/run/docker.sock: connect: permission denied`, occurs?
  
  A1: 
  Execute the command below to authorize `kbcli` operation. 
  Docker is installed as the root user by default but running `kbcli` requires a non-root user. Therefore, calling Docker may trigger this error if you do not use `sudo` when running `kbcli` commands.

  ```
  sudo chown user_name /var/run/docker.sock
  ```

- Q2: Installing `kbcli` is very slow, and an error, `unexpected end of file，tar: linux-arm64`, occurs with the timestamp showing 2022-09-18 22:47:51, which is 406436.949402453s earlier than the current time.
  
  A2:
  Check whether the system time is set correctly.