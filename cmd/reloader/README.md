<h1>Reloader</h1>

# 1. Introduction

Reloader is a service that watch changes in `ConfigMap` and trigger a config dynamic reload without process restart. Reloader is capable of killing containers or processes in pod, serviced through GRPC API, the controller do rolling upgrades on Pods by using the API.

. [Design Details](https://infracreate.feishu.cn/wiki/wikcn24AWAgXXBedVZZ0YgvjGuc)

# 2. Getting Started

You can get started with Reloader, by any of the following methods:
* Download the release for your platform from github.com/apecloud/kubeblocks/release
* Use the available Reloader docker image `docker run -it apecloud/kubeblocks`
* Build `reloader` from sources

## 2.1 Build

Compiler `Go 1.20+` (Generics Programming Support), checking the [Go Installation](https://go.dev/doc/install) to see how to install Go on your platform.

Use `make reloader` to build and produce the `reloader` binary file. The executable is produced under current directory.

```shell
$ cd kubeblocks
$ make reloader
```

## 2.2 Run

You can run the following command to start Reloader once built

```shell
reloader Provides a mechanism to implement reload config files in a sidecar for kubeblocks.

Usage:
  reloader [flags]

Flags:
      --container-runtime string   the config set cri runtime type. (default "auto")
      --debug                      the config set debug.
      --disable-runtime            the config set disable runtime.
  -h, --help                       help for reloader
      --log-level string           the config set log level. enum: [error, info, debug] (default "info")
      --notify-type notifyType     the config describe how to process notification messages. (default signal)
      --pod-ip string              the config set pod ip address. (default "127.0.0.1")
      --process string             the config describe what is db program.
      --regex string               the config set filter config file.
      --runtime-endpoint string    the config set cri runtime endpoint.
      --signal string              the config describe reload unix signal. (default "SIGHUP")
      --tcp int                    the config set service port. (default 9901)
      --volume-dir stringArray     the config map volume directory to watch for updates; may be used multiple times.
      
```

```shell
./bin/reloader --disable-runtime --log-level debug --process mysqld --signal SIGHUP --volume-dir /opt/mysql --volume-dir /etc/mysql

```


# 7. License

Reloader is under the Apache 2.0 license. See the [LICENSE](../../LICENSE) file for details.
