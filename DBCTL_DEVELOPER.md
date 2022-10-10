# dbctl Developer Guide

## 代码结构
* 入口函数为 `cmd/dbctl/main.go`。
* 所有支持的命令均在 `internal/dbctl/cmd` 目录中实现。
  * `root.go` 为 `dbctl` 根命令
  * `dbass` 部署 KubeBlocks 集群相关的命令
  * `cluster` 数据库集群相关命令
  * `backup` 备份相关命令
  * `bench` 基准测试相关命令
  * `playground` 本地快速部署测试环境，体验 KubeBlocks，目前仅支持 WeSQL 数据库引擎

## 接口
### Command
命令行代码使用 [cobra](https://github.com/spf13/cobra) 框架，可以参考 `internal/dbctl/cluster/cluster.go` 的实现，整体实现逻辑与
kubectl 保持一致，每个命令大体包括：
* 命令基本信息，如名称，描述等
* 命令选项，一般通过 `cmd.Flags` 设置，包括选项名，默认值以及描述信息等
* 运行函数，运行函数一般包括三部分：
  * `validate` 对命令参数和选项进行校验，若不符合预期，则返回错误
  * `complete` 根据传入的参数和选项获取运行命令的一些必要信息，如 `namespace`，`client` 等
  * `run` 命令执行逻辑

### 通用逻辑
dbctl 有一些同名的子命令，比如 `dbctl cluster list` 和 `dbctl backup list`，均用于查看特定资源列表，其实现逻辑也是一致的。
dbctl 对于类似通用子命令做了统一实现，包括：
* `list` 显示资源列表，参考 `internal/dbctl/cmd/cluster/list.go` 的实现
* `describe` 显示资源的描述信息，资源的描述信息通过 Go Template 模板进行渲染，模板位于 `internel/dbctl/cmd/describe/template`中，参考 `internal/dbctl/cmd/cluster/describe.go` 的实现
* `get` 从 Kubernetes 获取资源信息的通用逻辑，`list` 和 `describe` 均调用该逻辑实现

### Helm
dbctl 需要调用 Helm 接口安装/卸载 Helm 包，Helm 相关的接口在 `internal/dbctl/util/helm` 中实现，包括：
  * `add repo`
  * `install`
  * `uninstall`
  * `login`

### 访问 Kubernetes 集群
dbctl 对 Kubernetes 集群的访问与 kubectl 保持一致，一般通过 `cmdutil.Factory` 构造 K8s Client 或者通过 `NewBuilder` 方法访问 K8s 中的资源，
前者可以参考 `internal/dbctl/cmd/cluster/create.go` 的实现；后者可以参考 `internal/dbctl/cmd/get/get.go` 的实现。

## 如何添加一个新命令
1. 参考 `NewCreateCmd` 实现一个 `NewXXXCmd`
2. 设计一个命令选项结构体，比如 `XXXOptions`，存放该命令需要的一些信息，参考 `CreateOptions`
3. 添加命令行选项（flag）
4. 实现对应的 `validate`，`complete`，`run` 函数，如果没有相关逻辑，可以不实现
5. 添加测试用例，确保新增代码覆盖率在 60% 以上