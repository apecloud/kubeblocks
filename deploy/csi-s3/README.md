# Helm chart for csi-s3

This chart adds S3 volume support to your cluster.

## Install chart

- Helm 2.x: `helm install [--set secret.accessKey=... --set secret.secretKey=... ...] --namespace kube-system --name csi-s3 .`
- Helm 3.x: `helm install [--set secret.accessKey=... --set secret.secretKey=... ...] --namespace kube-system csi-s3 .`

After installation succeeds, you can get a status of Chart: `helm status csi-s3`.

## Delete Chart

- Helm 2.x: `helm delete --purge csi-s3`
- Helm 3.x: `helm uninstall csi-s3 --namespace kube-system`

## Configuration

By default, this chart creates a secret and a storage class. You should at least set `secret.accessKey` and `secret.secretKey`
to your [Yandex Object Storage](https://cloud.yandex.com/en-ru/services/storage) keys for it to work.

The following table lists all configuration parameters and their default values.

| Parameter                    | Description                                                                                         | Default                                                |
|------------------------------|-----------------------------------------------------------------------------------------------------|--------------------------------------------------------|
| `storageClass.create`        | Specifies whether the storage class should be created                                               | true                                                   |                                                   | csi-s3                                                 |
| `storageClass.bucket`        | Use a single bucket for all dynamically provisioned persistent volumes                              |                                                        |
| `storageClass.mounter`       | Mounter to use. Either geesefs, s3fs or rclone. geesefs recommended                                 | geesefs                                                |
| `storageClass.mountOptions`  | GeeseFS mount options                                                                               | `--memory-limit 1000 --dir-mode 0777 --file-mode 0666` |
| `storageClass.reclaimPolicy` | Volume reclaim policy                                                                               | Delete                                                 |
| `storageClass.annotations`   | Annotations for the storage class                                                                   |                                                        |
| `secret.create`              | Specifies whether the secret should be created                                                      | true                                                   |
| `secret.accessKey`           | S3 Access Key                                                                                       |                                                        |
| `secret.secretKey`           | S3 Secret Key                                                                                       |                                                        |
| `secret.endpoint`            | Endpoint                                                                                            | https://storage.yandexcloud.net                        |
| `secret.region`              | Region                                                                                              | eu-central-1                                           |
| `secret.cloudProvider`       | cloud provider: [aws,aliyun]                                                                        |                                                        |
| `multiCSI`                   | Check if this CSI has been installed multiple times. if true, only install storageClass and secret. | https://storage.yandexcloud.net                        |