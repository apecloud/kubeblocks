# 概览
BackupRepo 是备份数据的存储库，它使用 CSI Driver 将备份数据上传到各种存储系统，例如对象存储系统（如 S3 和 GCS）或存储服务器（如 FTP 和 NFS）。目前，Kubeblocks 的 BackupRepo 仅支持 S3 和其他兼容 S3 的存储系统，将来会支持更多存储类型。
用户可以创建多个 BackupRepo 以适应不同的场景。例如，根据不同的业务需求，可以把业务 A 的数据存储在 A 仓库，把业务 B 的数据存储在 B 仓库，或者可以按地区配置多个仓库以实现异地容灾。在创建备份时，你需要指定备份仓库。你也可以创建一个默认的备份仓库，如果在创建备份时未指定具体的仓库，KubeBlocks 将使用此默认仓库来存储备份数据。
下面以 AWS S3 为例，演示如何配置 BackupRepo。一共有两种配置 BackupRepo 的选项。

- 自动配置 BackupRepo：在安装 KubeBlocks 时，你可以提供必要的配置信息（例如，对象存储的 AccessKey等），安装程序会自动创建一个默认的 BackupRepo 和所需的 CSI Driver 附加组件。

- 手动配置 BackupRepo：你可以先安装 S3 CSI Driver，然后手动创建 BackupRepo。

## 自动配置 BackupRepo

在安装 KubeBlocks 时，你可以在 Yaml 配置文件中指定 BackupRepo 的信息， KubeBlocks 会自动创建一个 BackupRepo。

1. 将以下配置写入 backuprepo.yaml 文件中。
backupRepo:
  create: true
  storageProvider: s3
  config:
    region: cn-northwest-1
    bucket: test-kb-backup
  secrets:
    accessKeyId: <ACCESS KEY>
    secretAccessKey: <SECRET KEY>
    * region: 表示 S3 所在的区域。
    * bucket: 表示 S3 的桶名称。
    * accessKeyId: 表示 AWS 的 Access Key。
    * secretAccessKey: 表示 AWS 的 Secret Key。

:::note
1. 对于 KubeBlocks v0.6.0，可用的 storageProvider 是 s3、oss 和 minio。
2. 不同 storageprovider 所需的配置信息可能会有所不同。上面示例中的 config 和 secrets 适用于 S3。
3. 你可以执行以下命令查看支持的 storageprovider。
```kubectl get storageproviders.storage.kubeblocks.io```
    :::

2. 安装  KubeBlocks。
```kbcli kubeblocks install -f backuprepo.yaml```

## 手动配置 BackupRepo
如果在安装 KubeBlocks 时没有配置 BackupRepo 信息，你可以按照以下说明进行手动配置。

### 在开始之前 
有很多方法可以配置 BackupRepo。请确保在创建之前完成了所有必要的准备工作。如果你想使用 MinIO，请提前进行以下配置。
1. 安装 MinIO。
```
helm install minio oci://registry-1.docker.io/bitnamicharts/minio --set "extraEnvVars[0].name=MINIO_BROWSER_LOGIN_ANIMATION" --set "extraEnvVars[0].value=off"
```
获取初始的用户名和密码：
```
# 初始用户名
echo $(kubectl get secret --namespace default minio -o jsonpath="{.data.root-user}" | base64 -d)

# 初始密码
echo $(kubectl get secret --namespace default minio -o jsonpath="{.data.root-password}" | base64 -d)       
```
2. 生成连接凭证。
执行 `kubectl port-forward --namespace default svc/minio 9001:9001`，然后访问 `127.0.0.1:9001`进入登录页面。
登录到仪表盘后，生成 access key 和 secret key。
[图片]

3. 创建 Bucket。
在 MinIO 仪表盘上创建一个名为 test-minio 的存储桶。
[图片]
[图片]

### 安装 S3 CSI driver

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

# 启动 CSI-S3 插件
```
kbcli addon enable csi-s3
```
# 你可以添加标志来自定义此插件的安装
# CSI-S3 默认在所有节点上安装一个 DaemonSet Pod，你可以设置 tolerations，安装在指定 node 上
```
kbcli addon enable csi-s3 \
  --tolerations '[{"key":"taintkey","operator":"Equal","effect":"NoSchedule","value":"true"}]' \
  --tolerations 'daemonset:[{"key":"taintkey","operator":"Equal","effect":"NoSchedule","value":"true"}]'
```

# 查看 CSI-S3 Driver 的状态，确保其状态为 enabled
```
kbcli addon list csi-s3
```
</TabItem>

<TabItem value="Helm" label="Helm">

```
helm repo add kubeblocks https://jihulab.com/api/v4/projects/85949/packages/helm/stable
helm install csi-s3 kubeblocks/csi-s3 --version=0.6.0 -n kb-system
```
# 你可以添加标志来自定义此插件的安装
# CSI-S3 默认在所有节点上安装一个 DaemonSet Pod，你可以设置 tolerations，安装在指定 node 上
```
--set-json tolerations='[{"key":"taintkey","operator":"Equal","effect":"NoSchedule","value":"taintValue"}]'
--set-json daemonsetTolerations='[{"key":"taintkey","operator":"Equal","effect":"NoSchedule","value":"taintValue"}]'
```
</TabItem>

</Tabs>

### 创建 BackupRepo

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

   <Tabs>

   <TabItem value="S3" label="S3" default>
 ```
 kbcli backuprepo create my-repo \
   --provider s3 \
   --region cn-northwest-1 \
   --bucket test-kb-backup \
   --access-key-id <ACCESS KEY> \
   --secret-access-key <SECRET KEY> \
   --default
 ```
上述命令创建了一个名为 my-repo 的默认备份仓库。
- `my-repo` 是仓库名，可以留空不填。如果未指定名称，系统将随机使用类似 backuprepo-xxxxx 的名字。
- `--default` 表示将该仓库为默认仓库。请注意，全局只能有一个默认仓库。如果存在多个默认仓库，KubeBlocks 将无法决定应该使用哪一个仓库（类似于 Kubernetes 的默认 StorageClass），就会导致备份失败。使用 kbcli 创建 BackupRepo 可以避免此类问题，因为 kbcli 在创建新仓库之前会检查是否存在其他默认仓库。
- `--provider` 指定了存储类型，即 storageProvider，在创建 BackupRepo 时会用到此参数。它可用的值为 s3、oss 和 minio。不同存储所需的命令行参数各不相同，你可以执行 `kbcli backuprepo create --provider STORAGE-PROVIDER-NAME -h` 来查看参数信息。
  
   </TabItem>

   <TabItem value="OSS" label="OSS">
```
kbcli backuprepo create my-repo \
 --provider oss \
 --region cn-zhangjiakou \
 --bucket  test-kb-backup \
 # --端点 https://oss-cn-zhangjiakou-internal.aliyuncs.com \ 展示指定了的 oss 端点
 --access-key-id <ACCESS KEY> \
 --secret-access-key <SECRET KEY> \
 --default
 ```
   上述命令创建了一个名为 my-repo 的默认备份仓库。
- `my-repo` 是仓库名，可以留空不填。如果未指定名称，系统将随机使用类似 backuprepo-xxxxx 的名字。
- `--default` 表示将该仓库为默认仓库。请注意，全局只能有一个默认仓库。如果存在多个默认仓库，KubeBlocks 将无法决定应该使用哪一个仓库（类似于 Kubernetes 的默认 StorageClass），就会导致备份失败。使用 kbcli 创建 BackupRepo 可以避免此类问题，因为 kbcli 在创建新仓库之前会检查是否存在其他默认仓库。
- `--provider` 指定了存储类型，即 storageProvider，在创建 BackupRepo 时会用到此参数。它可用的值为 s3、oss 和 minio。不同存储所需的命令行参数各不相同，你可以执行 `kbcli backuprepo create --provider STORAGE-PROVIDER-NAME -h` 来查看参数信息。

   </TabItem>

   <TabItem value="MinIO" label="MinIO">
```
kbcli backuprepo create my-repo \
  --provider minio \
  --endpoint <ip:port> \
  --bucket test-minio \
  --access-key-id <ACCESS KEY> \
  --secret-access-key <SECRET KEY> \
  --default
```
上述命令创建了一个名为 `my-repo` 的默认备份仓库。
- `my-repo` 是仓库名，可以留空不填。如果未指定名称，系统将随机使用类似 backuprepo-xxxxx 的名字。
- `--default` 表示将该仓库为默认仓库。请注意，全局只能有一个默认仓库。如果存在多个默认仓库，KubeBlocks 将无法决定应该使用哪一个仓库（类似于 Kubernetes 的默认 StorageClass），就会导致备份失败。使用 kbcli 创建 BackupRepo 可以避免此类问题，因为 kbcli 在创建新仓库之前会检查是否存在其他默认仓库。
- `--provider` 指定了存储类型，即 storageProvider，在创建 BackupRepo 时会用到此参数。它可用的值为 s3、oss 和 minio。不同存储所需的命令行参数各不相同，你可以执行 `kbcli backuprepo create --provider STORAGE-PROVIDER-NAME -h` 来查看参数信息。
- `--endpoint` 参数指定了 MinIO 服务器的端点。如果你按照上述说明安装了 MinIO，端点将是 http://minio.default.svc.cluster.local:9000，其中 default 是 MinIO 安装所在的命名空间。

   </TabItem>

   </Tabs>
成功执行 `kbcli backuprepo create` 后，系统将创建一个类型为 BackupRepo 的 K8s 资源。你可以修改此资源的 annotation 以调整默认仓库。
# 取消默认仓库
```
kubectl annotate backuprepo old-default-repo \
  --overwrite=true \
  dataprotection.kubeblocks.io/is-default-repo=false
# 设置新的默认仓库
kubectl annotate backuprepo backuprepo-4qms6 \
  --overwrite=true \
  dataprotection.kubeblocks.io/is-default-repo=true
```
</TabItem>

<TabItem value="kubectl" label="kubectl">
另一个方法是使用 kubectl 创建 BackupRepo。与 kbcli 相比，kubectl 的命令缺少参数校验和默认仓库检查，所以相对来说不是很方便。

   <Tabs>

   <TabItem value="S3" label="S3" default>

# 创建用于保存 S3 access key 的密钥
```
kubectl create secret generic s3-credential-for-backuprepo \
  -n kb-system \
  --from-literal=accessKeyId=<ACCESS KEY> \
  --from-literal=secretAccessKey=<SECRET KEY>
```
# 创建 BackupRepo 资源

```
kubectl apply -f - <<-'EOF'
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: BackupRepo
metadata:
  name: my-repo
  annotations:
    dataprotection.kubeblocks.io/is-default-repo: "true"
spec:
  storageProviderRef: s3
  pvReclaimPolicy: Retain
  volumeCapacity: 100Gi
  config:
    bucket: test-kb-backup
    endpoint: ""
    mountOptions: --memory-limit 1000 --dir-mode 0777 --file-mode 0666
    region: cn-northwest-1
  credential:
    name: s3-credential-for-backuprepo
    namespace: kb-system
EOF
```
   </TabItem>

   <TabItem value="OSS" label="OSS">
# 创建用于保存 OSS access key 的密钥
```
kubectl create secret generic oss-credential-for-backuprepo \
  -n kb-system \
  --from-literal=accessKeyId=<ACCESS KEY> \
  --from-literal=secretAccessKey=<SECRET KEY>
```
# 创建 BackupRepo 资源
```
kubectl apply -f - <<-'EOF'
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: BackupRepo
metadata:
  name: my-repo
  annotations:
    dataprotection.kubeblocks.io/is-default-repo: "true"
spec:
  storageProviderRef: s3
  pvReclaimPolicy: Retain
  volumeCapacity: 100Gi
  config:
    bucket: test-kb-backup
    mountOptions: ""
    endpoint: ""
    region: cn-zhangjiakou
  credential:
    name: oss-credential-for-backuprepo
    namespace: kb-system
EOF
```
   </TabItem>

   <TabItem value="MinIO" label="MinIO">
# 创建用于保存 MinIO access key 的密钥
```
kubectl create secret generic minio-credential-for-backuprepo \
  -n kb-system \
  --from-literal=accessKeyId=<ACCESS KEY> \
  --from-literal=secretAccessKey=<SECRET KEY>
```
# 创建 BackupRepo 资源
```
kubectl apply -f - <<-'EOF'
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: BackupRepo
metadata:
  name: my-repo
  annotations:
    dataprotection.kubeblocks.io/is-default-repo: "true"
spec:
  storageProviderRef: minio
  pvReclaimPolicy: Retain
  volumeCapacity: 100Gi
  config:
    bucket: test-kb-backup
    mountOptions: ""
    endpoint: <ip:port>
  credential:
    name: minio-credential-for-backuprepo
    namespace: kb-system
EOF
 ``` 
   </TabItem>

   </Tabs>

</TabItem>

</Tabs>

## (可选) 更改集群的 BackupRepo
默认情况下，在创建集群时，所有备份都存储在全局默认仓库中。你可以通过编辑 BackupPolicy 来指定集群使用另一个 BackupRepo。

<Tabs>

<TabItem value="kbcli" label="kbcli" default>

```kbcli cluster edit-backup-policy mysql-cluster --set="datafile.backupRepoName=my-repo"
```
</TabItem>

<TabItem value="kubectl" label="kubectl">

```kubectl edit backuppolicy mysql-cluster-mysql-backup-policy
...
spec:
  datafile:
    ... 
    # 编辑 BackupRepo 名称
    backupRepoName: my-repo
```
</TabItem>

</Tabs>

