![Release status](https://jihulab.com/infracreate/dbaas-system/opencli/-/badges/release.svg)
![Pipeline](https://jihulab.com/infracreate/dbaas-system/opencli/badges/develop/pipeline.svg)
![Codecoverage](https://jihulab.com/infracreate/dbaas-system/opencli/badges/develop/coverage.svg)

# opencli

DBaaS 的命令行工具，有如下主要功能：
* DBaaS 部署与运维
* 数据库集群运维
* 快速部署实验环境

##   前置依赖
* [docker](https://docs.docker.com/get-docker/)
  * `playground init` 命令依赖 docker 部署实验用 K3d 集群。
* [kubectl](https://kubernetes.io/docs/tasks/tools/)
  * DBaaS 基于 K8s 部署，kubectl 用于查看 DBaaS 部署的资源对象。

## 安装
### 脚本安装

执行如下命令安装 `opencli`:
```bash
curl -fsSL http://54.223.93.54:8000/infracreate/v0.2.0/install_opencli.sh | bash
```

### 编译安装

1. 克隆源码，如 `git clone git@jihulab.com:infracreate/dbaas-system/opencli.git`
2. 在源码目录，运行如下命令：
   * `make` 编译生成 `opencli`，位于 `bin/opencli`
   * `make clean` 清理之前生成的 `opencli`

## 快速开始

执行 `opencli --help` 获取支持的命令。

执行如下命令，在本地快速部署一个实验环境：
```bash
opencli playground init
```
>注意：执行该命令需要确保已经安装 `docker` 并启动。

更多信息请参考[使用手册](https://infracreate.feishu.cn/wiki/wikcnwuZElgGMyaRyEqeI6W44Sd)。

## 贡献

开发者请参考[开发指南](./DEVELOPER.md)。