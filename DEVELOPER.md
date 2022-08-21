# Developer Guide

## 代码结构

* 入口函数为 `cmd/opencli/main.go`。
* 所有支持的命令均在 `pkg/cmd` 目录中实现，每个子目录对应一个子命令，`root.go` 为 `opencli` 根命令。
* `cluster` 目录为部署 DBaaS 集群相关的代码。
* `provider` 目录为 `playgournd` 部署不同类型数据库集群的配置信息，比如依赖的 Helm Repo 等，如果要支持新的数据库类型，需要在此添加配置。

## 接口
### Command
命令行代码使用 [cobra](https://github.com/spf13/cobra) 生成。

### Helm
Helm 相关的接口在 `utils/helm` 中实现，包括：
  * `add repo`
  * `install`
  * `login`

Helm 接口的使用，请参考 `InstallDeps` 函数。

### K8s API
访问 K8s 对象请参考 `cmd/dbcluster/describe.go` 的实现。