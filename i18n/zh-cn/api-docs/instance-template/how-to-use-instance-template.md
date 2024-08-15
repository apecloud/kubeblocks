---
title: 应用实例模板
description: 应用实例模板
keywords: [应用实例模板, 实例模板]
sidebar_position: 2
sidebar_label: 应用实例模板
---

# 应用实例模板

实例模板可用于多种场景。本章节以 RisingWave 为实例。

KubeBlocks 中支持管理 RisingWave 集群，RisingWave 引擎由 RisingWave 官方贡献。RisingWave 需要一个外部存储来作为自己的存储后端（state backend），这个外部存储可以是 AWS S3、阿里云 OSS 等。RisingWave 集群在创建时需要配置外部存储的 Credential 等信息，以便能够正常工作，而这些信息对每个集群来说可能都不同。

在 RisingWave 的官方镜像中，这些信息可以通过环境变量（Env）方式注入，所以在 KubeBlocks v0.9 中，我们可以通过在实例模板中配置相应的环境变量，在每次创建集群时设置这些环境变量的值，以便将 Credential 等信息注入到 RisingWave 的容器中。

## 示例

在 RisingWave 引擎的默认模板中，[环境变量相关配置](https://github.com/apecloud/kubeblocks-addons/blob/main/addons/risingwave/templates/cmpd-compute.yaml#L26)如下：

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: ComponentDefinition
metadata:
  name: risingwave
# ...
spec:
#...
  runtime:
    containers:
      - name: compute
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
          privileged: false
        command:
        - /risingwave/bin/risingwave
        - compute-node
        env:
        - name: RUST_BACKTRACE
          value: "1"
        - name: RW_CONFIG_PATH
          value: /risingwave/config/risingwave.toml
        - name: RW_LISTEN_ADDR
          value: 0.0.0.0:5688
        - name: RW_ADVERTISE_ADDR
          value: $(KB_POD_FQDN):5688
        - name: RW_META_ADDR
          value: load-balance+http://$(metaSvc)-headless:5690
        - name: RW_METRICS_LEVEL
          value: "1"
        - name: RW_CONNECTOR_RPC_ENDPOINT
          value: $(connectorSvc):50051
        - name: RW_PROMETHEUS_LISTENER_ADDR
          value: 0.0.0.0:1222
# ...
```

在 [Cluster 资源](https://github.com/apecloud/kubeblocks-addons/blob/main/addons-cluster/risingwave/templates/cluster.yaml)中添加实例模板后如下：

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: {{ include "risingwave-cluster.name" . }}
  namespace: {{ .Release.Namespace }}
# ...
spec:
  componentSpecs:
  - componentDef: compute
    name: compute
    replicas: {{ .Values.risingwave.compute.replicas }}
    instances:
    - name: instance
      replicas: {{ .Values.risingwave.compute.replicas }}
      env:
      - name: RW_STATE_STORE
        value: "hummock+s3://{{ .Values.risingwave.stateStore.s3.bucket }}"
      - name: AWS_REGION
        value: "{{ .Values.risingwave.stateStore.s3.region }}"
      {{- if eq .Values.risingwave.stateStore.s3.authentication.serviceAccountName "" }}
      - name: AWS_ACCESS_KEY_ID
        value: "{{ .Values.risingwave.stateStore.s3.authentication.accessKey }}"
      - name: AWS_SECRET_ACCESS_KEY
        value: "{{ .Values.risingwave.stateStore.s3.authentication.secretAccessKey }}"
      {{- end }}
      - name: RW_DATA_DIRECTORY
        value: "{{ .Values.risingwave.stateStore.dataDirectory }}"
      {{- if .Values.risingwave.stateStore.s3.endpoint }}
      - name: RW_S3_ENDPOINT
        value: "{{ .Values.risingwave.stateStore.s3.endpoint }}"
      {{- end }}
      {{- if .Values.risingwave.metaStore.etcd.authentication.enabled }}
      - name: RW_ETCD_USERNAME
        value: "{{ .Values.risingwave.metaStore.etcd.authentication.username }}"
      - name: RW_ETCD_PASSWORD
        value: "{{ .Values.risingwave.metaStore.etcd.authentication.password }}"
      {{- end }}
      - name: RW_ETCD_ENDPOINTS
        value: "{{ .Values.risingwave.metaStore.etcd.endpoints }}"
      - name: RW_ETCD_AUTH
        value: "{{ .Values.risingwave.metaStore.etcd.authentication.enabled}}"
# ...
```

在上面的例子中，我们通过 `instances` 字段新增了一个实例模板，该实例模板的名字为 `instance`。模板中定义了 `RW_STATE_STORE`、`AWS_REGION` 等若干环境变量，这些环境变量会被 KubeBlocks append 到默认模板中定义的环境变量列表后面，最终渲染的实例中将包含默认模板和在该实例模板中定义的所有环境变量。

另外，实例模板中 `replicas` 与 `componentSpec` 中相同（都为 `{{ .Values.risingwave.compute.replicas }}`），意味着在覆盖默认模板后，该实例模板将用来渲染该 Component 中的所有实例。

## 实例模板详细说明

- `Name` 字段：每个 Component 中可以定义多个实例模板。每个模板都需要通过 `Name` 字段设置模板名称，同一个 Component 中的实例模板名字必须保持唯一。
- `Replica` 字段：每个模板可以通过 `Replicas` 字段设置基于该模板渲染的实例数量，Replicas 默认为 1。同一个 Component 中的所有实例模板的 `Replicas` 相加后必须小于或等于 Component 的 `Replicas` 值。若基于实例模板渲染的实例数量小于 Component 需要的总的实例数量，剩余的实例将使用默认模板进行渲染。

基于实例模板渲染的实例名称的模式（Pattern）为 `$(cluster name)-$(component name)-$(instance template name)-ordinal`。比如在上文 RisingWave 示例中， Cluster 名字为 `risingwave`，Component 名字为 `compute`，实例模板名称为 `instance`，实例数量 `Replicas` 为 3。那么最终渲染出的实例名称为：`risingwave-compute-instance-0`,`risingwave-compute-instance-1`,`risingwave-compute-instance-2`。

实例模板在集群创建阶段可以使用，并可以在后续运维中对实例模板进行更新，具体包括添加实例模板、删除实例模板或更新实例模板。实例模板更新可能会引起实例的更新、删除或重建，在更新前建议仔细分析最终的变化是否符合预期。

### Annotations

实例模板中的 `annotations` 用于覆盖默认模板中的 `annotations` 字段，若实例模板 `annotations` 中的某个 Key 在默认模板中已存在，该 Key 对应的值（Value）将使用实例模板中的值；若该 Key 在默认模板中不存在，该 Key 和 Value 将被添加到最终的 `annotations` 中。

***示例***

默认模板中 `annotations` 为：

```yaml
annotations:
  "foo0": "bar0"
  "foo1": "bar"
```

实例模板中 `annotations` 为：

```yaml
annotations:
  "foo1": "bar1"
  "foo2": "bar2"
```

则最终被渲染出的实例的 `annotations` 为：

```yaml
annotations:
  "foo0": "bar0"
  "foo1": "bar1"
  "foo2": "bar2"
```

:::note

KubeBlocks 会添加一些系统 `annotations`，需要避免对这些 `annotations` 造成覆盖。

:::

### Labels

您也可以通过实例模板设置 `Labels`。

与 `Annotations` 类似，实例模板中的 `Labels` 采用相同的覆盖逻辑应用到已有的 `Labels` 上，并形成最终的 `Labels`。

:::note

KubeBlocks 添加了系统 `Labels`，请勿覆盖这些 `Labels`。

:::

### Image

实例模板中的 `Image` 用于覆盖默认模板中第一个 Container 的 `Image` 字段。

:::warning

该字段需慎用：在数据库等有状态应用中，`Image` 改变通常涉及数据格式等的兼容性，使用该字段时请确认实例模板的镜像版本与默认模板中的完全兼容。

:::

同时 KubeBlocks 从 v0.9 开始，通过 `ComponentVersion` 对镜像版本进行了详细设计，建议通过 `ComponentVersion` 进行版本管理。

### NodeName

该字段用于覆盖默认模板中的 `NodeName` 字段。

### NodeSelector

该字段用于覆盖默认模板中的 `NodeSelector` 字段，覆盖逻辑与 `Annotations` 和 `Labels` 相同。

### Tolerations

该字段用于覆盖默认模板中的 `Tolerations` 字段。

若实例模板中的 `Toleration` 与默认模板中的某个 `Toleration` 完全相同（`Key`、`Operator`、`Value`、`Effect` 和 `TolerationSeconds` 都相同），则该 `Toleration` 会被忽略；否则，追加到默认模板中的 `Tolerations` 列表中。

### RuntimeClassName

该字段用于覆盖默认模板中的 `RuntimeClassName` 字段。

### Resources

在实例模板中可以进一步覆盖 `Resources` 的值，其优先级高于 `Component`。

### Env

实例模板中定义的 `Env` 将覆盖除 KubeBlocks 系统默认 `Env` 外的其他 `Env`。覆盖逻辑与 `Annotaions` 和 `Labels` 类似，即若 `Env Name` 相同，则用实例模板中的 `Value` 或 `ValueFrom`；不同，则添加为新的 `Env`。

### Volumes

用于覆盖默认模板第一个 Container 的 `Volumes` 字段。覆盖逻辑与 `Env` 类似，即若 `Volume Name` 相同，则用实例模板中的 `VolumeSource`；否则，添加为新的 `Volume`。

### VolumeMounts

用于覆盖默认模板第一个 Container 的 `VolumeMounts` 字段。覆盖逻辑与 `Volumes` 类似，即若 VolumeMount `Name` 相同，则用实例模板中的 `MountPath` 等值；否则，添加为新的 `VolumeMount`。

### VolumeClaimTemplates

用于覆盖 Component 中通过 `ClusterComponentVolumeClaimTemplate` 生成的 `VolumeClaimTemplates`。覆盖逻辑与 Volumes 类似，即若 `PersistentVolumeClaim Name` 相同，则用实例模板中的 `PersistentVolumeClaimSpec` 值；否则，添加为新的 `PersistentVolumeClaim`。
