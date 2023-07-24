{{- $clusterName := $.cluster.metadata.name }}
{{- $namespace := $.cluster.metadata.namespace }}
<clickhouse>
  <!-- Macros -->
  <macros>
    <shard from_env="CLICKHOUSE_SHARD_ID"></shard>
    <replica from_env="CLICKHOUSE_REPLICA_ID"></replica>
    <layer>{{ $clusterName }}</layer>
  </macros>
  <!-- Log Level -->
  <logger>
    <level>information</level>
  </logger>
  <!-- Cluster configuration - Any update of the shards and replicas requires helm upgrade -->
  <remote_servers>
    <default>
{{- range $.cluster.spec.componentSpecs }}
  {{ $compIter := . }}
  {{- if eq $compIter.componentDefRef "clickhouse" }}
      <shard>
    {{- $replicas := $compIter.replicas | int }}
    {{- range $i, $_e := until $replicas }}
        <replica>
            <host>{{ $clusterName }}-{{ $compIter.name }}-{{ $i }}.{{ $clusterName }}-{{ $compIter.name }}-headless.{{ $namespace }}.svc.{{- $.clusterDomain }}</host>
            <port>9000</port>
        </replica>
    {{- end }}
      </shard>
  {{- end }}
{{- end }}
    </default>
  </remote_servers>
{{- range $.cluster.spec.componentSpecs }}
  {{ $compIter := . }}
  {{- if or (eq $compIter.componentDefRef "zookeeper") (eq $compIter.componentDefRef "ch-keeper") }}
  <!-- Zookeeper configuration -->
  <zookeeper>
    {{- $replicas := $compIter.replicas | int }}
    {{- range $i, $_e := until $replicas }}
    <node>
      <host>{{ $clusterName }}-{{ $compIter.name }}-{{ $i }}.{{ $clusterName }}-{{ $compIter.name }}-headless.{{ $namespace }}.svc.{{- $.clusterDomain }}</host>
      <port>2181</port>
    </node>
    {{- end }}
  </zookeeper>
  {{- end }}
{{- end }}
{{- if $.component.monitor.enable }}
  <!-- Prometheus metrics -->
  <prometheus>
    <endpoint>/metrics</endpoint>
    <port from_env="CLICKHOUSE_METRICS_PORT"></port>
    <metrics>true</metrics>
    <events>true</events>
    <asynchronous_metrics>true</asynchronous_metrics>
  </prometheus>
{{- end }}
</clickhouse>
