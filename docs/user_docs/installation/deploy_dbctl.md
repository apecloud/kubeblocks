# Deploy `dbctl`

`dbctl` is the command line tool of `KubeBlocks`. This section introduces how to install `dbctl`. 

For more information of CLI, refer to [KubeBlocks commands overview](../cli/kubeblocks%20commands%20overview.md).

## Install dependencies

The following dependencies are required for deploying `dbctl`.

- MySQL Shell
Install MySQL Shell in your local host to visit MySQL instances. Refer to [Install MySQL Shell on macOS](https://dev.mysql.com/doc/mysql-shell/8.0/en/mysql-shell-install-macos-quick.html) for details.
  Note:
  Installation steps:

  1. Download the package from http://dev.mysql.com/downloads/shell/. 
  2. Double-click the downloaded DMG to mount it. Finder opens.
  3. Double-click the .pkg file shown in the Finder window.
  4. Follow the steps in the installation wizard.
  5. When the installer finishes, eject the DMG. (It can be deleted.)

- `kubectl`
Run the following command to install `kubectl` in your local host for visiting k8s clusters. Refer to [Install and Set Up kubectl on macOS](https://kubernetes.io/docs/tasks/tools/install-kubectl-macos/) for details.
  ```
  curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/darwin/arm64/kubectl"
  ```

- K8s cluster
`dbctl` will visit a k8s cluster which can be specified by the `kubeconfig` condition variable or the `kubeconfig` parameter. If no k8s cluster is specified, `dbctl` reads the content in `~/.kube/config` file by default.

## Install `dbctl`

Both curl installation and make installation are supported.

- curl
`dbctl` can be run on macOS, Linux, and Windows. Copy and run the command below.

```
curl -fsSL http://161.189.136.182:8000/apecloud/kubeblocks/install_dbctl.sh |bash
```

- make
Downlod source code and execute the following commands under the root directory of the source code. Make and generate `dbctl` executive file. Make sure the executive file under `bin/dbctl`.

  ```
  # Switch to the `main` branch
  git checkout main
  git pull

  # Make `dbctl`
  GIT_VERSION=`git describe --always --abbrev=0 --tag`
  VERSION=`echo "${GIT_VERSION/v/}"`
  make dbctl
  ```

Run the command below to view version after installation.

```
dbctl version
```

## Uninstall `dbctl`

Run the following command to unistall `dbctl`.

```
sudo rm /usr/local/bin/dbctl
```

If you install `dbctl` by make, run the command below to clean the generated `dbctl`.

```
make clean-dbctl
```