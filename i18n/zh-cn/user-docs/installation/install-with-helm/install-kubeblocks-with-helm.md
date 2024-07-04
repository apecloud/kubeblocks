---
title: 用 Helm 安装 KubeBlocks
description: 如何用 Helm 安装 KubeBlocks
keywords: [taints, affinity, tolerance, 安装, helm, KubeBlocks]
sidebar_position: 1
sidebar_label: 用 Helm 安装 KubeBlocks
---

# 用 Helm 安装 KubeBlocks

KubeBlocks 是基于 Kubernetes 的原生应用，你可以使用 Helm 来进行安装。

:::note

如果使用 Helm 安装 KubeBlocks，那么卸载也需要使用 Helm。

请确保已安装 [kubectl](https://kubernetes.io/zh-cn/docs/tasks/tools/) 和 [Helm](https://helm.sh/zh/docs/intro/install/)。

:::

## 环境准备

<table>
    <tr>
        <th colspan="3">资源要求</th>
    </tr >
    <tr>
        <td >控制面</td>
        <td colspan="2">建议创建 1 个具有 4 核 CPU、4GB 内存和 50GB 存储空间的节点。</td>
    </tr >
    <tr >
        <td rowspan="4">数据面</td>
        <td> MySQL </td>
        <td>建议至少创建 3 个具有 2 核 CPU、4GB 内存和 50GB 存储空间的节点。 </td>
    </tr>
    <tr>
        <td> PostgreSQL </td>
        <td>建议至少创建 2 个具有 2 核 CPU、4GB 内存和 50GB 存储空间的节点。</td>
    </tr>
    <tr>
        <td> Redis </td>
        <td>建议至少创建 2 个具有 2 核 CPU、4GB 内存和 50GB 存储空间的节点。</td>
    </tr>
    <tr>
        <td> MongoDB </td>
        <td>建议至少创建 3 个具有 2 核 CPU、4GB 内存和 50GB 存储空间的节点。</td>
    </tr>
</table>

## 安装步骤

**在线安装 KubeBlocks**

1. 创建 CRD 依赖。

   ```bash
   kubectl create -f https://github.com/apecloud/kubeblocks/releases/download/v0.8.1/kubeblocks_crds.yaml
   ```

2. 添加 Helm 仓库。

   ```bash
   helm repo add kubeblocks https://apecloud.github.io/helm-charts
   
   helm repo update
   ```

3. 安装 KubeBlocks。

   ```bash
   helm install kubeblocks kubeblocks/kubeblocks --namespace kb-system --create-namespace
   ```

   如果想要使用自定义的 tolerations 安装 KubeBlocks，可以使用以下命令：

   ```bash
   helm install kubeblocks kubeblocks/kubeblocks --namespace kb-system --create-namespace \
    --set-json 'tolerations=[ { "key": "control-plane-taint", "operator": "Equal", "effect": "NoSchedule", "value": "true" } ]' \
    --set-json 'dataPlane.tolerations=[{ "key": "data-plane-taint", "operator": "Equal", "effect": "NoSchedule", "value": "true"    }]'
   ```

   如果想安装 KubeBlocks 的指定版本，请按照以下步骤操作：

   1. 在 [KubeBlocks Release 页面](https://github.com/apecloud/kubeblocks/releases/)查看可用的版本。
   2. 使用 `--version` 指定版本，并执行以下命令。

      ```bash
      helm install kubeblocks kubeblocks/kubeblocks \
      --namespace kb-system --create-namespace --version="x.x.x"
      ```

:::note

默认安装最新版本。

:::

**离线安装 KubeBlocks**

在生产环境中，K8S 集群一般是不能连接外网的，只会允许办公环境的电脑通过 VPN 连接 K8S 集群。在线安装方式不可用，需要进行离线安装。
## 环境准备

<table>
    <tr>
        <td colspan="3">1. K8S 集群中有可用的镜像仓库（假定为registry.kb）</td>
    </tr >
    <tr>
        <td colspan="3">2. K8S 的部署节点安装 helm</td>
    </tr >
</table>
总得来说就是将在线的crd ， helm-charts，公有云镜像下载下来，然后helm install安装

1. 下载crd并安装
   1. 办公电脑访问下面地址，会自动下载 crd 文件
   ```bash
   https://github.com/apecloud/kubeblocks/releases/download/v0.8.4-beta.11/kubeblocks_crds.yaml
   ```
   2. 将文件上传到集群中，并安装 crd
   ```bash
   kubectl create -f crds.yaml // 这里假定下载为 crds.yaml
   ```
   
2. 下载远程 helmchart 文件
   ```bash
   helm repo add kubeblocks https://apecloud.github.io/helm-charts

   helm repo update

   helm fetch kubeblocks/kubeblocks --version 0.8.4-beta.11 // 这里指定了版本为 0.8.4-beta.11
   ```
3. 将仓库的 repo 改成集群可用的仓库，这里假定可用的是 registry.kb
   1. 将下载的 helmchart 包解压，会得到一个 kubeblocks 的文件夹，进入后修改 value.yaml
   ```bash
   tar zxvf kubeblocks-0.8.4-beta.11
   ``` 
   2. 改对应的 repo
   ```bash
   ## KubeBlocks container image settings
   ##
   ## @param image.registry KubeBlocks image registry
   ## @param image.repository KubeBlocks image repository
   ## @param image.pullPolicy KubeBlocks image pull policy
   ## @param image.tag KubeBlocks image tag (immutable tags are recommended)
   ## @param image.imagePullSecrets KubeBlocks image pull secrets
   ## @param image.tools.repository KubeBlocks tools image repository
   image:
     registry: registry.kb  //换成可用的 registry
     repository: apecloud/kubeblocks
     pullPolicy: IfNotPresent
     # Overrides the image tag whose default is the chart appVersion.
     tag: ""
     imagePullSecrets: []
     tools:
       repository: apecloud/kubeblocks-tools
     datascript:
       repository: apecloud/kubeblocks-datascript
    
   
   
   ## @param addonChartsImage - addon charts image, used to copy Helm charts to the addon job container.
   ## @param addonChartsImage.chartsPath - the helm charts path in the addon charts image.
   addonChartsImage:
     # if the value of addonChartsImage.registry is not specified using `--set`, it will be set to the    value of 'image.registry' by default
     registry: "registry.kb"  // 换成可用的 registry
     repository: apecloud/kubeblocks-charts
     pullPolicy: IfNotPresent
     tag: ""
     chartsPath: /charts
     pullSecrets: []
   ```
4. 将需要的远程镜像重新 tag 推到 K8S 的镜像仓库
   1. 基本需要的镜像是下面这几个
   ```bash
   infracreate-registry.cn-zhangjiakou.cr.aliyuncs.com/apecloud/kubeblocks-datascript:0.8.4-beta.11
   infracreate-registry.cn-zhangjiakou.cr.aliyuncs.com/apecloud/kubeblocks-tools:0.8.4-beta.11
   infracreate-registry.cn-zhangjiakou.cr.aliyuncs.com/apecloud/kubeblocks:0.8.4-beta.11
   infracreate-registry.cn-zhangjiakou.cr.aliyuncs.com/apecloud/snapshot-controller:v6.2.1
   infracreate-registry.cn-zhangjiakou.cr.aliyuncs.com/apecloud/kubeblocks-charts:0.8.4-beta.11
   ```
   2. 将他们重新 tag 后推到远端（以kubeblocks:0.8.4-beta.11举例）
   ```bash
   docker pull infracreate-registry.cn-zhangjiakou.cr.aliyuncs.com/apecloud/kubeblocks:0.8.4-beta.11
   docker tag infracreate-registry.cn-zhangjiakou.cr.aliyuncs.com/apecloud/kubeblocks:0.8.4-beta.11 registry.kb/apecloud/kubeblocks:0.8.4-beta.11
   docker push registry.kb/apecloud/kubeblocks:0.8.4-beta.11
   ```   
   后续有需要的镜像也可以按照上面的方式添加

5. 可以将部分自动安装的 addon 关闭，只保留需要的

   在kubeblocks/templates/addons 文件夹中，将对应的数据库的配置中的 autoInstall 改成 false（以 apecloud-mysql-addon.yaml举例）
   ```bash
   {{- $selectorLabels := include "kubeblocks.selectorLabels" . }}
   {{- include "kubeblocks.buildAddonCR" (merge (dict
     "kbVersion" ">=0.7.0"
     "selectorLabels" $selectorLabels
     "name" "apecloud-mysql"
     "version" "0.8.0-beta.8"
     "model" "RDBMS"
     "provider" "apecloud"
     "description" "ApeCloud MySQL is a database that is compatible with MySQL syntax and achieves high    availability through the utilization of the RAFT consensus protocol."
     "autoInstall" false ) .) -}}  // 将原本的 true 改成false
   ``` 
6. 部署
   
   将原本的 kubeblocks 文件夹打包上传至集群部署节点
   ```bash
   tar czvf kb0.8.4beta11.tgz kubeblocks
   helm install kubeblocks kubeblocks/. --create-namespace kb-system
   ``` 
:::note

中间有问题需要卸载的时候，一定要使用 helm uninstall 的方式，不能使用删除 kb-system 命名空间的方式，因为部分资源不在 kb-system 下，会导致异常。比如设置某些数据库自动安装的开关失效

:::


## 验证 KubeBlocks 安装

执行以下命令来检查 KubeBlocks 是否已成功安装。

```bash
kbcli kubeblocks status
```

***结果***

如果工作负载都已准备就绪，则表明 KubeBlocks 已成功安装。

```bash
NAME                                            READY   STATUS    RESTARTS      AGE
kb-addon-snapshot-controller-649f8b9949-2wzzk   1/1     Running   2 (24m ago)   147d
kubeblocks-dataprotection-f6dbdbf7f-5fdr9       1/1     Running   2 (24m ago)   147d
kubeblocks-6497f7947-mc7vc                      1/1     Running   2 (24m ago)   147d
```