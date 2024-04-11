# Apply instance template

Instance template can be applied to many scenarios. In this section, we take RisingWave cluster as an example.
KubeBlocks supports the management of RisingWave clusters. The RisingWave addon is contributed by the RisingWave official team.  For RisingWave to function optimally, it relies on an external storage solution, such as AWS S3 or Alibaba Cloud OSS, to serve as its state backend. When creating a RisingWave cluster, it is necessary to configure credentials and other information for the external storage to ensure normal operation, and these information may vary for each cluster.

In the official image of RisingWave, these information can be injected via environment variables. Therefore, in KubeBlocks 0.9, we can configure corresponding environment variables in the instance template and set the values of these environment variables each time a cluster is created, so as to inject credential information into the container of RisingWave.

## An example

In the default template of RisingWave addon, the environment viriables are configured as follows:
```
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
```
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

## Detailed information of instance template

For each component, multiple instance templates can be defined. Template name is configured with `name` field, and must remain unique within the same component.