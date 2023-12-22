---
title: 添加数据库引擎
description: 在 KubeBlocks 中添加数据库引擎
keywords: [数据库引擎, 添加数据库引擎]
sidebar_position: 2
sidebar_label: 添加数据库引擎
---

# 添加数据库引擎

本文档以 Oracle MySQL 为例，介绍如何在 KubeBlocks 中添加数据库引擎（点击参考[完整 PR](https://github.com/apecloud/learn-kubeblocks-addon)）。

整个接入过程分成三大步：

1. 规划集群蓝图。
2. 准备集群模板。
3. 添加 `addon.yaml` 文件。

## 步骤 1. 规划集群蓝图

首先需要规划一个集群蓝图，知道你想要的集群长什么样：

- 集群包含哪些组件；
- 每个组件是什么形态：
  - 有状态/无状态。
  - 单机版/主备版/集群版。
  
本文档要部署的集群只包含一个组件，该组件是有状态的，且只有一个节点。Table 1. 中展示了部署集群所需要的基本信息。

部署一个 MySQL 8.0 单机版集群。

:paperclip: Table 1. Oracle MySQL 集群蓝图

| 术语              | 设置                                                                                                     |
|-------------------|--------------------------------------------------------------------------------------------------------------|
| CLusterDefinition | 启动脚本：默认。<br /> 配置文件：默认。<br /> 服务端口：3306。<br /> Component 个数：1，即 MySQL。 |
| ClusterVersion    | Image: docker.io/mysql:8.0.34                                                                                |
| Cluster.yaml      | 由用户创建时指定。                                                           |

## 步骤 2. 准备集群模板

### 2.1 创建 Helm chart

选项 1.`helm create oracle-mysql`

选项 2. 直接创建一个文件夹 `mkdir oracle-mysql`

包含以下文件内容：

```bash
> tree oracle-mysql
.
├── Chart.yaml        #  包含 chart 相关信息的 YAML 文件
├── templates         # 模板目录，赋值后将生成有效的 Kubernetes 清单文件
│   ├── NOTES.txt     # 可选：纯文本文件，包含简短的使用说明
│   ├── _helpers.tpl  # 放置 helpers 的位置，可以在整个 chart 中重复使用
│   ├── clusterdefinition.yaml  
│   └── clusterversion.yaml
└── values.yaml       # 该 chart 的默认配置值

2 directories, 6 files
```

这里着重分析 `templates` 下的 YAML 文件。

`templates` 目录下只有两个 YAML 文件：`clusterDefintion.yaml` 和 `clusterVersion.yaml`。前者描述组件拓扑，后者描述组件版本信息。

- `clusterDefinition.yaml`

  整个 YAML 文件看上去很长，下面逐个说明各个字段的用途。

  - `ConnectionCredential`

    ```yaml
      connectionCredential:
        username: root
        password: "$(RANDOM_PASSWD)"
        endpoint: "$(SVC_FQDN):$(SVC_PORT_mysql)"
        host: "$(SVC_FQDN)"
        port: "$(SVC_PORT_mysql)"
    ```

    这里会创建一个 secret，其命名规则为 `{clusterName}-conn-credential`。它包含用户名、密码、endpoint、port 等常规信息，在其他服务访问该集群时使用（这个 secret 会先于其他资源创建，可以在其他地方引用该对象）。

    `$(RANDOM_PASSWD)` 在创建时会替换为一个随机密码。

    `$(SVC_PORT_mysql)` 通过选择端口名来指定要暴露的端口号，这里为 `mysql`。

    更多关于环境变量的说明，参看[环境变量与占位符](./environment-variables-and-placeholders.md)文档。

  - `ComponentDefs`

      ```yaml
        componentDefs:
          - name: mysql-compdef
            characterType: mysql
            workloadType: Stateful
            service:
              ports:
                - name: mysql
                  port: 3306
                  targetPort: mysql
            podSpec:
              containers:
              ...
      ```

    `componentDefs`，即组件定义。它定义了每个组件运行所需的基本信息，包括启动脚本、配置、端口等等。

    这里我们只有一个 MySQL 组件。为了方便区分，我们把它命名为 `mysql-compdef`，表示这是 MySQL 的一个组件定义。

  - `name` [必填]

    即组件名称。这个名称没有特定标准，选择一个易区分有表达力的名字就可以。
    
    如果我们将拓扑、版本和资源解耦（就像数据库中的 table normalize 一样），我们不仅可以让每一个对象描述的信息都更清晰和聚焦, 而且还可以通过这些对象的组合来生成更丰富的集群。因此，我们可以将 Cluster 表示为

    $$Cluster = ClusterDefinition.yaml \Join ClusterVersion.yaml \Join ...$$

    这个 `name` 就是那个join key。
    
    记住这个 `name`，后面要考的。

  - `characterType` [可选]

    `characterType` 是一个字符串类型，用来识别这是一个什么引擎，例如 `mysql`、`postgresql`、`redis` 等。它主要用于数据库连接，在操作数据库时，它能快速识别引擎类型，找到匹配的操作命令。
    
    它可以是一个任意的字符串，或者你也可以给你的引擎起一个独特的名称。前期我们没有数据库引擎相关的操作，因此也可以空缺。

  - `workloadType` [必填]

    顾名思义就是 Workload 类型。K8s 提供了几个基础款，比如 Deployment，StatefulSet 等。Kubeblocks 在此基础上做了抽象并提供了更丰富的 Workload。比如：
    - Stateless，无状态服务；
    - Stateful，有状态服务；
    - Consensus，有状态服务，且有自选举能力，有角色。

    之后我们会更深入地介绍 Workload（包括设计、实现、如何使用等）。
    
    因为这里使用的是 MySQL 单机版，因此使用 `Stateful` 就足够了。

  - `service` [可选]

    ```yaml
          service:
            ports:
              - name: mysql  # 端口名 mysql，connectionCredential 会查找这个名称找到对应的 port
                port: 3306
                targetPort: mysql
    ```

    它定义如何为该组件创建一个 Service，暴露哪些端口。
    
    还记得 `connectionCredential` 中介绍的，集群对外暴露的 port 和 endpoint 吗？ 
    
    通过 `$(SVC_PORT_mysql)$` 来选择端口，`mysql` 就是这里的 `service.ports[0].name` mysql。

     :::note

     如果 `connectionCredential` 中填写了端口名，必须确保端口名出现在这里。

     :::

  - `podSpec`

    podSpec 的定义和 Kubernetes 的 podSpec 相同。

    ```yaml
          podSpec:
            containers:
              - name: mysql-container
                imagePullPolicy: IfNotPresent
                volumeMounts:
                  - mountPath: /var/lib/mysql
                    name: data
                ports:
                  - containerPort: 3306
                    name: mysql
                env:
                  - name: MYSQL_ROOT_HOST
                    value: {{ .Values.auth.rootHost | default "%" | quote }}
                  - name: MYSQL_ROOT_USER
                    valueFrom:
                      secretKeyRef:
                        name: $(CONN_CREDENTIAL_SECRET_NAME)
                        key: username
                  - name: MYSQL_ROOT_PASSWORD
                    valueFrom:
                      secretKeyRef:
                        name: $(CONN_CREDENTIAL_SECRET_NAME)
                        key: password
    ```

    这里定义了只包含一个 container 的 Pod，即 `mysql-container`，以及所需的环境变量、端口等常规信息。
    
    我们从名为 `$(CONN_CREDENTIAL_SECRET_NAME)` 的 secret 中获取用户名和密码作为 pod environment variable。

    这是一个 placeholder, 用来指代前文中提到的 Connection credential Secret。

- ClusterVersion

   所有版本相关的信息都配置在 `ClusterVersion.yaml` 中。
   
   现在可以为每一个 Component 需要的每一个 container 补充 image 信息。

   ```yaml
     clusterDefinitionRef: oracle-mysql
     componentVersions:
     - componentDefRef: mysql-compdef
       versionsContext:
         containers:
         - name: mysql-container
           image: {{ .Values.image.registry | default "docker.io" }}/{{ .Values.image.repository }}:{{ .Values.image.tag }}
           imagePullPolicy: {{ default .Values.image.pullPolicy "IfNotPresent" }}
   ```

   还记得在 ClusterDefinition 中采用的 ComponentDef Name 吗？对，`mysql-compdef`，在这里填写该组件的每一个 container 的 image 信息。

:::note

写好了 ClusterDefinition 和 ClusterVersion之后，可以快速测试一下，在本地安装一下试试。

:::

### 2.2 按照 Helm chart

安装 Helm。

```bash
helm install oracle-mysql ./oracle-mysql
```

成功安装后，可以看到如下信息：

```yaml
NAME: oracle-mysql
LAST DEPLOYED: Wed Aug  2 20:50:33 2023
NAMESPACE: default
STATUS: deployed
REVISION: 1
TEST SUITE: None
```

### 2.3 创建集群

通过 `kbcli cluster create`，就可以快速拉起一个 MySQL 集群。

```bash
kbcli cluster create mycluster --cluster-definition oracle-mysql
>
Info: --cluster-version is not specified, ClusterVersion oracle-mysql-8.0.34 is applied by default
Cluster mycluster created
```

通过 `--cluster-definition` 指定 ClusterDefinition 的名字。

:::note

如果只有一个 ClusterVersion 对象关联该 ClusterDefinition，kbcli在创建集群时，会使用该 ClusterVersion。

如果有多个 ClusterVersion 对象该关联 ClusterDefinition，则需要显式指定。

:::

创建完之后，就可以：

**A. 查看集群状态**

   ```bash
   kbcli cluster list mycluster
   >
   NAME        NAMESPACE   CLUSTER-DEFINITION   VERSION               TERMINATION-POLICY   STATUS    CREATED-TIME
   mycluster   default     oracle-mysql         oracle-mysql-8.0.34   Delete               Running   Aug 02,2023 20:52 UTC+0800
   ```

**B. 连接集群**

   ```bash
   kbcli cluster connect mycluster
   >
   Connect to instance mycluster-mysql-compdef-0
   mysql: [Warning] Using a password on the command line interface can be insecure.
   Welcome to the MySQL monitor.  Commands end with ; or \g.
   Your MySQL connection id is 8
   Server version: 8.0.34 MySQL Community Server - GPL

   Copyright (c) 2000, 2023, Oracle and/or its affiliates.

   Oracle is a registered trademark of Oracle Corporation and/or its
   affiliates. Other names may be trademarks of their respective
   owners.

   Type 'help;' or '\h' for help. Type '\c' to clear the current input statement.

   mysql>
   ```

**C. 简单运维，如 Scale up**

   ```bash
   kbcli cluster vscale mycluster --components mysql-compdef --cpu='2' --memory=2Gi
   ```

**D. 简单运维，如 Stop**

   Stopping the cluster releases all computing resources.

   ```bash
   kbcli cluster stop mycluster
   ```

## 步骤 3. 添加 `addon.yaml` 文件

还有最后一步，你就可以通过 KubeBlocks 平台发布你的数据库集群，让更多的用户快速创建你定义的集群。

那就是添加一个 add-on 文件，成为 KubeBlocks add-on 的一员。详情请参考 `tutorial-1-create-an-addon/oracle-mysql-addon.yaml`。

```bash
apiVersion: extensions.kubeblocks.io/v1alpha1
kind: Addon
metadata:
  name: tutorial-mysql
spec:
  description: 'MySQL is a widely used, open-source....'
  type: Helm
  helm:                                     
    chartsImage: registry-of-your-helm-chart
  installable:
    autoInstall: false
    
  defaultInstallValues:
    - enabled: true
```

然后通过 `chartsImage` 来配置你的 Helm chart 远端仓库地址。

## 步骤 4. 发布到 Kubeblocks 社区（可选）

你可以将 Helm chart 贡献给 [KubeBlocks add-ons](https://github.com/apecloud/kubeblocks-addons)，将 `addon.yaml` 贡献给 [KubeBlocks](https://github.com/apecloud/kubeblocks)。

`addon.yaml` 文件放在 `kubeblocks/deploy/helm/templates/addons` 目录下。

## 附录

### A.1 如何为同一个引擎配置多个版本？

在日常生产环境中碰到的一个常见问题就是：要支持多个版本。而在 KubeBlocks 中，可以通过**多个 ClusterVersion 关联同一个 ClusterDefinition** 的方式来解决这个问题。

以本文中的 MySQL 为例。

1. 修改 `ClusterVersion.yaml` 文件，支持多个版本。

   ```yaml
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: ClusterVersion
   metadata:
     name: oracle-mysql-8.0.32
   spec:
     clusterDefinitionRef: oracle-mysql   ## 关联同一个 clusterdefinition: oracle-mysql
     componentVersions:
     - componentDefRef: mysql-compdef
       versionsContext:
         containers:
           - name: mysql-container
             image: <image-of-mysql-8.0.32> ## 镜像地址为 8.0.32
   ---
   apiVersion: apps.kubeblocks.io/v1alpha1
   kind: ClusterVersion
   metadata:
     name: oracle-mysql-8.0.18
   spec:
     clusterDefinitionRef: oracle-mysql  ## 关联同一个 clusterdefinition: oracle-mysql
     componentVersions:
     - componentDefRef: mysql-compdef
       versionsContext:
         containers:
           - name: mysql-container
             image: <image-of-mysql-8.0.18> ## 镜像地址为 8.0.18
   ```

2. 创建 Cluster 时，指定版本信息。

   - 创建一个 8.0.32 版本的集群。

     ```bash
     kbcli cluster create mycluster --cluster-definition oracle-mysql --cluster-version oracle-mysql-8.0.32
     ```

   - 创建一个 8.0.18 版本的集群。

     ```bash
     kbcli cluster create mycluster --cluster-definition oracle-mysql --cluster-version oracle-mysql-8.0.18
     ```

   这样就可以快速为你的引擎实现多版本支持了。

### A.2 kbcli 创建 Cluster 不能满足需求怎么办？

`kbcli` 提供了一套便捷且通用的方案来创建集群。创建时会设置一些值，例如资源大小，但这并不能满足每个引擎的需求，尤其是当集群包含多个组件且需要选择性使用时。

为了解决这个问题，你可以用一个 Helm chart 来渲染 Cluster，或者通过 `cluster.yaml` 文件来创建，例如：

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: mycluster
  namespace: default
spec:
  clusterDefinitionRef: oracle-mysql        # Specify ClusterDefinition
  clusterVersionRef: oracle-mysql-8.0.32    # Specify ClusterVersion
  componentSpecs:                           # List required components
  - componentDefRef: mysql-compdef          # The type of the first component: mysql-compdef
    name: mysql-comp                        # The name of the first component: mysql-comp
    replicas: 1 
    resources:                              # Specify CPU and memory size
      limits:
        cpu: "1"
        memory: 1Gi
      requests:
        cpu: "1"
        memory: 1Gi
    volumeClaimTemplates:                   # Set the PVC information, where the name must correspond to that of the Component Def.
    - name: data
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 20Gi
  terminationPolicy: Delete
```
