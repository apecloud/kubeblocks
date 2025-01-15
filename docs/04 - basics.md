## Basics
### Kubebuilder
`KubeBlocks` is using KubeBuilder as the operator framework, before your start to code, suggest to read [kubebuilder books](https://book.kubebuilder.io/).

### Makefile
`KubeBlocks` includes a [Makefile](../Makefile) in the root of the repo. This serves as a high-level interface for common commands. Running `make help` will produce a list of make targets with descriptions. These targets will be referenced throughout this document.
### kbcli

`kbcli` is a [cobra-based](https://github.com/spf13/cobra) command line interface for `KubeBlocks` which support to the basic interaction commands such as `install/uninstall/upgrade` KubeBlocks and all kinds of actions for KubeBlocks

The kbcli repository: https://github.com/apecloud/kbcli

### Code style
First, read the Go (Golang) coding guidelines again, looking for any style violations. It's easier to remember a style rule once you've violated it.

Run our suite of linters:

``` shell
make lint
```
This is not a fast command. On my machine, at the time of writing, it takes about a full minute to run. You can instead run
