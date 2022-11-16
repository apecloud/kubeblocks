# Deploy Playground

## Dependencies

Make sure the following softwares are installed before `KubeBlocks Playground` deployment.

- Download Docker.dmg，run the following commands to install and start Docker in your computer. Refer to [Install Docker Desktop on Mac](https://docs.docker.com/desktop/install/mac-install/) for details.
  ```
  sudo hdiutil attach Docker.dmg
  sudo /Volumes/Docker/Docker.app/Contents/MacOS/install
  sudo hdiutil detach /Volumes/Docker
  ```
- Install MySQL Shell in your compouter to visit MySQL instances. Refer to [Install MySQL Shell on macOS](https://dev.mysql.com/doc/mysql-shell/8.0/en/mysql-shell-install-macos-quick.html) for details.
  Note:
  Installation steps:

  1. Download the package from http://dev.mysql.com/downloads/shell/. 
  2. Double-click the downloaded DMG to mount it. Finder opens.
  3. Double-click the .pkg file shown in the Finder window.
  4. Follow the steps in the installation wizard.
  5. When the installer finishes, eject the DMG. (It can be deleted.)

- Run the following command to install `kubectl` in your computer. Refer to [Install and Set Up kubectl on macOS](https://kubernetes.io/docs/tasks/tools/install-kubectl-macos/) for details.
  ```
  curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/darwin/arm64/kubectl"
  ```
