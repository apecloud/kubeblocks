## Setup development environment
There are two options for getting an environment up and running for `KubeBlocks` development.

### Bring your own toolbox
To build `KubeBlocks` on your own host, needs to install the following tools:
- Go (Golang)
- Make

#### Install Go
Download and install [Go 1.21 or later](https://go.dev/doc/install).
#### Install Make
`KubeBlocks` uses `make` for a variety of build and test actions, and needs to be installed as appropriate for your platform:

- Linux
    1. Install the `build-essential` package:
       ```shell
       sudo apt-get install build-essential
       ```
- macOS
    1. Ensure that build tools are installed:
       ```shell
       xcode-select --install
       ```
    2. When completed, you should see `make` and other command line developer tools in `/usr/bin`.

#### Build KubeBlocks
When `go` and `make` are installed, you can clone the `KubeBlocks` repository, and build `KubeBlocks`  binaries with the `make` tool.
- To build for your current local environment:
  ```shell
  make all
  ```
- To cross-compile for a different platform, use the `GOOS` and `GOARCH` environmental variables:
  ```shell
  make all GOOS=windows GOARCH=amd64
  ```

### Use VSCode and development container
If you are using Visual Studio Code, you can connect to a [development container](https://code.visualstudio.com/docs/devcontainers/containers) configured for KubeBlocks development. With development container, you don't need to manually install all of the tools and frameworks needed.

#### Setup the development container
1. Install VSCode [Dev Containers extension](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers)
2. Open the `KubeBlocks` project folder in VSCode
    - VSCode will detect the presence of a dev container definition in the repo and will prompt you to reopen the project in a container:
      ![reopen dev container by pop notification](./img/reopen_dev_container_notification.png)
    - Alternatively, you can open the command palette and use the Remote-Containers: Reopen in Container command:
      ![reopen dev container by command](./img/reopen_dev_container_command.png)
    - VSCode will pull image and start dev container automatically, once the container is loaded, open an integrated terminal in VS Code and you're ready to develop `KubeBlocks` in a containerized environment.
3. And you can run `make all` to build `KubeBlocks` in the dev container.

#### Customize your dev container
##### Use a custom dev container image
The [devcontainer.json](../.devcontainer/devcontainer.json) uses the latest image from [ApeCloud Docker hub](https://hub.docker.com/r/apecloud/kubeblocks-dev), you can customize image to suit your need.
1. Edit the [docker/Dockerfile-dev](../docker/Dockerfile-dev) dev container image definition, you can change the `[Option]` configuration or install additional tools.
    ```Dockerfile
    # Copy library scripts to execute
    COPY library-scripts/*.sh library-scripts/*.env /tmp/library-scripts/

    # [Option] Install zsh
    ARG INSTALL_ZSH="true"

    # [Optional] Uncomment the next line to use go get to install anything else you need
    # RUN go get -x <your-dependency-or-tool>

    # [Optional] Uncomment this section to install additional OS packages.
    # RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    #     && apt-get -y install --no-install-recommends <your-package-list-here>

    # [Optional] Uncomment this line to install global node packages.
    # RUN su vscode -c "source /usr/local/share/nvm/nvm.sh && npm install -g <your-package-here>" 2>&
    ```
2. Build a new image using updated `Dockerfile`, and replace the `"image"` property in [devcontainer.json](../.devcontainer/devcontainer.json).
    ```json
    {
        // Update to your dev container image
        "image": "docker.io/apecloud/kubeblocks-dev:latest",
    }

    ```
3. Rebuild and reopen the workspace in the dev container via the command palette and the `Remote-Containers: Rebuild and Reopen` in Container command.
4. When you are satisfied with your changes, you can optionally publish your container image to your own registry to speed up rebuilding the container when you only want to make changes to the `devcontainer.json` configuration in the future. For a docker image named `docker.io/xxxx/kubeblocks-dev`:
    ```shell
    make build-dev-image DEV_CONTAINER_IMAGE_NAME=docker.io/xxxx/kubeblocks-dev
    ```

##### Connect existing Kubernetes cluster
If you want to reuse an existing Kubernetes config, such as your [`EKS`](https://aws.amazon.com/eks/) cluster or local [`Minikube`](https://minikube.sigs.k8s.io/docs/) cluster, you can configure the `devcontainer.json` copy those settings into the dev container. This requires:

1. Enabling the `SYNC_LOCALHOST_KUBECONFIG` environment variable
2. Bind mounting the locations of your Kubernetes and Minikube config paths to `/home/kubeblocks/.kube-localhost` and `/home/kubeblocks/.minikube-localhost` respectively.
    - You don't need to bind the Minikube path if you're not using it.
    ```json
    "containerEnv": {
        // Uncomment to overwrite devcontainer .kube/config and .minikube certs with the localhost versions
        // each time the devcontainer starts, if the respective .kube-localhost/config and .minikube-localhost
        // folders respectively are bind mounted to the devcontainer.
        "SYNC_LOCALHOST_KUBECONFIG": "true",
    },

    ...

    "mounts": [
    // Uncomment to clone local .kube/config into devcontainer
    "type=bind,source=${env:HOME}${env:USERPROFILE}/.kube,target=/home/kubeblocks/.kube-localhost",

    // Uncomment to additionally clone minikube certs into devcontainer for use with .kube/config
    "type=bind,source=${env:HOME}${env:USERPROFILE}/.minikube,target=/home/kubeblocks/.minikube-localhost"

    ]
    ```
> ⚠ The `SYNC_LOCALHOST_KUBECONFIG` option only supports providing the dev container with the snapshot configuration from the host and does not support updating the host Kubernetes configuration from the dev container directly.



### Setup a Kubernetes development environment
To run `KubeBlocks`, you needs `Docker` and a `Kubernetes` 1.24.1+ cluster for development.

#### Docker environment
1. Install [Docker](https://docs.docker.com/install/)
   > For Linux, you'll have to configure docker to run without `sudo` for the KubeBlocks build scripts to work. Follow the instructions to [manage Docker as a non-root user](https://docs.docker.com/engine/install/linux-postinstall/#manage-docker-as-a-non-root-user).
2. Create your [Docker Hub account](https://hub.docker.com/signup) if you don't already have one.

#### Kubernetes environment
- Kubernetes cluster
  You can use cloud Kubernetes service, such as [`EKS`](https://aws.amazon.com/eks/) [`GKE`](https://cloud.google.com/kubernetes-engine) [`AKS`](https://azure.microsoft.com/en-us/products/kubernetes-service/), or use local Kubernetes cluster, such as [`Minikube`](https://minikube.sigs.k8s.io/docs/) [`k3d`](https://k3d.io/stable/).
- For development purposes, you will also want to follow the optional steps to install [Helm 3.x](https://helm.sh/docs/intro/install/).
