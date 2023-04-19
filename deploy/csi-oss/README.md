# Helm chart for csi-oss

This chart adds oss volume support to your cluster.

## Install chart

- Helm 2.x: `helm install [--set secret.akId=... --set secret.akSecret=... ...] --namespace kube-system --name csi-oss .`
- Helm 3.x: `helm install [--set secret.akId=... --set secret.akSecret=... ...] --namespace kube-system csi-oss`

After installation succeeds, you can get a status of Chart: `helm status csi-oss`.

## Delete Chart

- Helm 2.x: `helm delete --purge csi-oss`
- Helm 3.x: `helm uninstall csi-oss --namespace kube-system`

## Configuration

By default, this chart creates a secret and a configmap with persistentVolume. You should at least set `secret.akId`, `secret.akSecret` and `storageConfig.bucket`
to your [Alibaba OSS CSI-DRIVER](https://github.com/kubernetes-sigs/alibaba-cloud-csi-driver/blob/master/docs/oss.md) keys for it to work.

The following table lists all configuration parameters and their default values.

| Parameter                    | Description                                                                        | Default                                                |
| ---------------------------- |------------------------------------------------------------------------------------|--------------------------------------------------------|
| `storageConfig.endpoint`        | Mount OSS access domain name                                                       | oss-cn-hangzhou.aliyuncs.com                           |
| `storageConfig.bucket`          | The OSS bucket that needs to be mounted                                            |                                                        |
| `storageConfig.path`  | Indicates the directory structure of the relative bucket root file during mounting | /                                                      |
| `storageConfig.otherOpts`       | Support for inputting customized parameters when mounting OSS                      | -o max_stat_cache_size=0 -o allow_other                                                |
| `secret.name`                | Name of the secret                                                                 | csi-oss-secret                                         |
| `secret.akId`           | OSS Access Key                                                                     |                                                        |
| `secret.akSecret`           | OSS Secret Key                                                                     |                                                        |