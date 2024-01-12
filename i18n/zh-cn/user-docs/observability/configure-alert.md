---
title: 配置告警
description: 如何配置告警
keywords: [mysql, 告警, 告警信息, 邮件告警]
sidebar_position: 2
---

# 配置 IM 告警

告警主要用于日常故障响应，以提高系统的可用性。KubeBlocks 内置了一组通用的告警规则，并集成了多个通知渠道，可满足生产级线上集群的运维需求。

:::note

告警功能对所有数据库都是一样的。

:::

## 告警规则

KubeBlocks 内置的告警系统可以满足各种数据产品的需求，并提供了开箱即用的体验，无需进一步配置。这些告警规则为集群运维提供了最佳实践，通过经验总结的平滑窗口、告警阈值、告警级别和告警指标，进一步提高了告警的准确性，降低了误报率和漏报率。

以 PostgreSQL 为例，告警规则中内置了常见的异常事件，例如实例宕机、实例重启、慢查询、连接数、死锁和缓存命中率等。

以下示例显示了 PostgreSQL 的告警规则（可参考 [Prometheus 的语法](https://prometheus.io/docs/prometheus/latest/querying/basics/)）。当活跃连接数超过阈值的 80% 并持续 2 分钟时，Prometheus 会触发 warning 警告，并将其发送到 AlertManager。

```bash
alert: PostgreSQLTooManyConnections
  expr: |
    sum by (namespace,app_kubernetes_io_instance,pod) (pg_stat_activity_count{datname!~"template.*|postgres"})
    > on(namespace,app_kubernetes_io_instance,pod)
    (pg_settings_max_connections - pg_settings_superuser_reserved_connections) * 0.8
  for: 2m
  labels:
    severity: warning
  annotations:
    summary: "PostgreSQL too many connections (> 80%)"
    description: "PostgreSQL has too many connections and the value is {{ $value }}. (instance: {{ $labels.pod }})"
```

你可以在  **Prometheus 仪表盘**的 **Alerts** 选项卡中查看所有的告警规则。执行以下命令打开 Prometheus 仪表盘：

```bash
# 查看仪表盘列表
kbcli dashboard list

# 打开 Prometheus 仪表盘
kbcli dashboard open kubeblocks-prometheus-server # 这是一个示例，请根据上述仪表盘列表中的实际名称填写
```

## 配置 IM 告警

KubeBlocks 的告警通知主要采用 AlertManager 的原生能力。在接收到 Prometheus 告警后，KubeBlocks 执行去重、分组、静默、抑制和路由等步骤，最终将告警发送到相应的通知渠道。

AlertManager 集成了一组通知渠道，例如电子邮件和 Slack。KubeBlocks 通过 AlertManager Webhook 扩展了新的 IM 类通知渠道。

下面以配置飞书为例。

**开始之前**

首先，部署监控组件并启用集群监控。更多信息请参阅[监控数据库](./monitor-database.md)。

**步骤**

1. 配置告警渠道。
请参考以下指南配置告警渠道。
     - [飞书自定义机器人](https://open.feishu.cn/document/ukTMukTMukTM/ucTM5YjL3ETO24yNxkjN)
     - [钉钉自定义机器人](https://open.dingtalk.com/document/orgapp/custom-robot-access)
     - [企业微信自定义机器人](https://developer.work.weixin.qq.com/document/path/91770)
     - [Slack](https://api.slack.com/messaging/webhooks)

    :::note

    - 每个通知渠道都有接口调用量和频率限制。超过限制后该渠道将会被限流，导致无法接收到告警。
    - 每个渠道的服务级别协议（SLA）也无法确保告警一定会成功发送。因此，建议配置多个渠道以确保可用性。

    :::

2. 配置接收者。
   
   为了提高易用性，`kbcli` 开发了 `alert` 子命令来简化接收者的配置。你可以通过 `alert` 子命令设置通知渠道和接收者。该命令还支持过滤集群名称和严重程度级别。配置成功后即会动态生效，无需重启服务。

   1. 添加告警接收者。

      ```bash
      kbcli alert add-receiver --webhook='xxx' --cluster=xx --severity=xx
      ```

      ***示例***
      
      下面以飞书为例，展示如何添加接收者。请注意，下述命令中的 webhook 地址仅为示例，请使用真实的 webhook 地址替换。

      ```bash
      # 用户未开启签名检验
      kbcli alert add-receiver \
      --webhook='url=https://open.feishu.cn/open-apis/bot/v2/hook/foo'

      # 用户开启签名认证，签名 sign 作为 token 参数值
      kbcli alert add-receiver \
      --webhook='url=https://open.feishu.cn/open-apis/bot/v2/hook/foo,token=sign'

      # 仅接收来自 mysql-cluster 集群的告警
      kbcli alert add-receiver \
      --webhook='url=https://open.feishu.cn/open-apis/bot/v2/hook/foo' --cluster=mysql-cluster

      # 仅接收来自 mysql-cluster 集群的 critical 告警
      kbcli alert add-receiver \
      --webhook='url=https://open.feishu.cn/open-apis/bot/v2/hook/foo' --cluster=mysql-cluster --severity=critical
      ```

      :::note

      如需查看详细的命令，请执行 `kbcli alert add-receiver -h`.

      :::

   2. 查看接收者列表，验证新接收者是否已添加。

        还可以通过此命令查看通知配置。

        ```bash
        kbcli alert list-receivers
        ```

   3. （可选）如果要禁用告警功能，可以删除通知渠道和接收者。

        ```bash
        kbcli alert delete-receiver <receiver-name>
        ```

## IM 告警故障排除

如果无法接收到告警通知，请排查 AlertManager 和 AlertManager-Webhook-Adaptor 两个监控组件的日志。

```bash
# 查找 AlertManager 对应的 Pod 并获取 Pod 名称
kubectl get pods -n kb-system -l 'app=prometheus,component=alertmanager'

# 查看 AlertManeger 的日志
kubectl logs <pod-name> -n kb-system -c prometheus-alertmanager

# 查找 AlertManager-Webhook-Adaptor 对应的 Pod 并获取 Pod 名称
kubectl get pods -n kb-system -l 'app.kubernetes.io/name=alertmanager-webhook-adaptor'

# 查看 AlertManager-Webhook-Adaptor 的日志
kubectl logs <pod-name> -n kb-system -c alertmanager-webhook-adaptor
```

## 配置邮件告警

KubeBlocks 还支持电子邮件告警。

1. 配置 SMTP 服务器。

    ```bash
    kbcli alert config-smtpserver 
    --smtp-from alert-test@apecloud.com \
    --smtp-smarthost smtp.feishu.cn:587 \
    --smtp-auth-username alert-test@apecloud.com \
    --smtp-auth-password 123456abc \
    --smtp-auth-identity alert-test@apecloud.com
    ```

2. 查看 SMTP 服务器列表，确保上述服务器已成功添加。

   你还可以通过此命令查看配置的详细信息。

    ```bash
    kbcli alert list-smtpserver
    ```

3. 添加电子邮件接收者。

    ```bash
    kbcli alert add-receiver --email='user1@kubeblocks.io'
    ```

    KubeBlocks 支持从指定集群接收邮件，或接受特定严重级别的邮件。你可以使用 `--cluster` 和 `--severity` 标志来设置此功能。
    - `--cluster`：表示仅从指定集群接收邮件。

      ```bash
      kbcli alert add-receiver --email='user1@kubeblocks.io,user2@kubeblocks.io' --cluster=mycluster
      ```

    - `--severity`：表示仅从指定集群接收严重级别为 `warning` 的电子邮件。

      ```bash
      kbcli alert add-receiver --email='user1@kubeblocks.io,user2@kubeblocks.io' --cluster=mycluster --severity=warning
      ```
