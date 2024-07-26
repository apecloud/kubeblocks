---
title: Apply instance template
description: Apply instance template
keywords: [apply instance template, instance template]
sidebar_position: 2
sidebar_label: Apply instance template
---

# Apply instance template

Instance templates can be applied to many scenarios. In this section, we take a RisingWave cluster as an example.

KubeBlocks supports the management of RisingWave clusters. The RisingWave addon is contributed by the RisingWave official team. For RisingWave to function optimally, it relies on an external storage solution, such as AWS S3 or Alibaba Cloud OSS, to serve as its state backend. When creating a RisingWave cluster, it is necessary to configure credentials and other information for the external storage to ensure normal operation, and this information may vary for each cluster.

In the official image of RisingWave, this information can be injected via environment variables. Therefore, in KubeBlocks 0.9, we can configure corresponding environment variables in the instance template and set the values of these environment variables each time a cluster is created, so as to inject credential information into the container of RisingWave.

## An example

In the default template of RisingWave addon, [the environment variables](https://github.com/apecloud/kubeblocks-addons/blob/main/addons/risingwave/templates/clusterdefinition.yaml#L334) are configured as follows:

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: ClusterDefinition
metadata:
  name: risingwave
# ...
spec:
  componentDefs:
  - name: compute
# ...
    podSpec:
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

After adding an instance template to the cluster resources:

```yaml
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: {{ include "risingwave-cluster.name" . }}
  namespace: {{ .Release.Namespace }}
# ...
spec:
  componentSpecs:
  - componentDefRef: compute
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

In the example above, we added an instance template through the `instances` field, named `instance`. This template defines several environment variables such as `RW_STATE_STORE` and `AWS_REGION`. These environment variables will be appended by KubeBlocks to the list of environment variables defined in the default template. Consequently, the rendered instance will contain both the default template and all the environment variables defined in this instance template.

Additionally, the `replicas` field in the instance template is identical to that in the `componentSpec` (both are `{{ .Values.risingwave.compute.replicas }}`), indicating that after overriding the default template, this instance template will be used to render all instances within this component.

## Detailed information on instance template

- `Name` field: For each component, multiple instance templates can be defined. The template name is configured with the `Name` field and must remain unique within the same component.
- `Replica` field: Each template can set the number of instances rendered based on that template via the `Replicas` field, of which the default value is 1. The sum of `Replicas` for all instance templates within the same component must be less than or equal to the `Replicas` value of the component. If the number of instances rendered based on the instance templates is less than the total number required by the component, the remaining instances will be rendered using the default template.

The pattern for the names of instances rendered based on instance templates is `$(cluster name)-$(component name)-$(instance template name)-ordinal`. For example, in the above RisingWave cluster, the cluster name is `risingwave`, the component name is `compute`, the instance template name is `instance`, and the number of `Replicas` is 3. Therefore, the rendered instance names are risingwave-compute-instance-0, risingwave-compute-instance-1, and risingwave-compute-instance-2.

Instance templates can be used during cluster creation and can be updated during the operations period. Specifically, this includes adding, deleting, or updating instance templates. Updating instance templates may update, delete, or reconstruct instances. You are recommended to carefully evaluate whether the final changes meet expectations before performing updates.

### Annotations

The `Annotations` in the instance template are used to override the `Annotations` field in the default template. If a Key in the `Annotations` of the instance template already exists in the default template, the `value` corresponding to the Key will use the value in the instance template; if the Key does not exist in the default template, the Key and Value will be added to the final `Annotations`.

***Example:***

The `annotations` in the default template are:

```yaml
annotations:
  "foo0": "bar0"
  "foo1": "bar"
```

And `annotations` in the instance templates are:

```yaml
annotations:
  "foo1": "bar1"
  "foo2": "bar2"
```

Then, after rendering, the actual annotations are:

```yaml
annotations:
  "foo0": "bar0"
  "foo1": "bar1"
  "foo2": "bar2"
```

:::note

KubeBlocks adds system `Annotations`, and do not overwrite them.

:::

### Labels

You can also set `Labels` with the instance template.

Similar to `Annotations`, `Labels` in instance templates follow the same overriding logic applied to existing labels.

:::note

KubeBlocks adds system `Labels`, and do not overwrite them.

:::

### Image

The `Image` field in the instance template is used to override the `Image` field of the first container in the default template.

:::note

`Image` field should be used with caution: for the StatefulSet like databases, changing the `Image` often involves compatibility issues with data formats. When changing this field, please ensure that the image version in the instance template is fully compatible with that in the default template.

:::

With KubeBlocks version 0.9 and above, detailed design for image versions is provided through `ComponentVersion`. It is recommended to manage versions using `ComponentVersion`.

### NodeName

`NodeName` in the instance template overrides the same field in the default template.

### NodeSelector

`NodeSelector` in the instance template overrides the same field in the default template.

### Tolerations

`Tolerations` in the instance template overrides the same field in the default template.

If the `Toleration` in the instance template is identical to a `Toleration` in the default template (with the same `Key`, `Operator`, `Value`, `Effect`, and `TolerationSeconds`), then that `Toleration` will be ignored. Otherwise, it will be added to the list of `Tolerations` in the default template.

### RuntimeClassName

`RuntimeClassName` in the instance template overrides the same field in the default template.

### Resources

`Resources` in the instance template overrides the same field in the default template and gets the highest priority.

### Env

The environment variables (`Env`) defined in the instance template will override any other environment variables except for the default `Env` set by KubeBlocks. The overriding logic is similar to `Annotations` and `Labels`. If an environment variable name is the same, the value or value source from the instance template will be used; if it's different, it will be added as a new environment variable.

### Volumes

Used to override the `Volumes` field of the first container in the default template. The overriding logic is similar to `Env`, if the `Volume Name` is the same, the `VolumeSource` from the instance template will be used; otherwise, it will be added as a new `Volume`.

### VolumeMounts

Used to override the `VolumeMounts` field of the first container in the default template. The overriding logic is similar to `Volumes`, meaning if the `VolumeMount` `Name` is the same, the `MountPath` and other values from the instance template will be used; otherwise, it will be added as a new `VolumeMount`.

### VolumeClaimTemplates

Used to override the `VolumeClaimTemplates` generated by `ClusterComponentVolumeClaimTemplate` within the Component. The overriding logic is similar to `Volumes`, meaning if the `PersistentVolumeClaim` `Name` is the same, the `PersistentVolumeClaimSpec` values from the instance template will be used; otherwise, it will be added as a new `PersistentVolumeClaim`.
