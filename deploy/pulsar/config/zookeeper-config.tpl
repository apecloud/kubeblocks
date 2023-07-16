PULSAR_GC: -XX:+UseG1GC -XX:MaxGCPauseMillis=10 -Dcom.sun.management.jmxremote -Djute.maxbuffer=10485760 -XX:+ParallelRefProcEnabled -XX:+UnlockExperimentalVMOptions -XX:+DoEscapeAnalysis -XX:+DisableExplicitGC -XX:+ExitOnOutOfMemoryError -XX:+PerfDisableSharedMem -XX:ActiveProcessorCount=1
PULSAR_MEM: -XX:MinRAMPercentage=40 -XX:MaxRAMPercentage=60
PULSAR_PREFIX_serverCnxnFactory: org.apache.zookeeper.server.NIOServerCnxnFactory
dataDir: /pulsar/data/zookeeper
serverCnxnFactory: org.apache.zookeeper.server.NIOServerCnxnFactory
{{- $clusterName := $.cluster.metadata.name }}
{{- $namespace := $.cluster.metadata.namespace }}
{{- $pulsar_zk_component := fromJson "{}" }}
{{- range $i, $e := $.cluster.spec.componentSpecs }}
  {{- if eq $e.componentDefRef "zookeeper" }}
    {{- $pulsar_zk_component = $e }}
  {{- end }}
{{- end }}
{{- $zk_servers := "" }}
{{- $zk_server_i := "" }}
{{- $replicas := $pulsar_zk_component.replicas | int }}
{{- range $i, $e := until $replicas }}
  {{- $zk_server_i = printf "%s-%s-%d" $clusterName $pulsar_zk_component.name $i }}
  {{- if ne $zk_servers "" }}
    {{- $zk_servers = printf "%s,%s" $zk_servers $zk_server_i }}
  {{- else }}
    {{- $zk_servers = printf "%s" $zk_server_i }}
  {{- end }}
{{- end }}
ZOOKEEPER_SERVERS: {{ $zk_servers }}