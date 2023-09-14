PULSAR_EXTRA_OPTS: -Dpulsar.allocator.exit_on_oom=true -Dio.netty.recycler.maxCapacity.default=1000 -Dio.netty.recycler.linkCapacity=1024 -Dnetworkaddress.cache.ttl=60 -XX:ActiveProcessorCount=1 -XshowSettings:vm -Ddepth=64
PULSAR_GC: -XX:+UseG1GC -XX:MaxGCPauseMillis=10 -XX:+ParallelRefProcEnabled -XX:+UnlockExperimentalVMOptions -XX:+DoEscapeAnalysis -XX:G1NewSizePercent=50 -XX:+DisableExplicitGC -XX:-ResizePLAB
{{- $MaxDirectMemorySize := "" }}
{{- $phy_memory := getContainerMemory ( index $.podSpec.containers 0 ) }}
{{- if gt $phy_memory 0 }}
  {{- $MaxDirectMemorySize = printf "-XX:MaxDirectMemorySize=%dm" (mul (div $phy_memory ( mul 1024 1024 10)) 6) }}
{{- end }}
PULSAR_MEM: -XX:MinRAMPercentage=30 -XX:MaxRAMPercentage=30 {{ $MaxDirectMemorySize }}

{{- $clusterName := $.cluster.metadata.name }}
{{- $namespace := $.cluster.metadata.namespace }}
{{- $pulsar_zk_from_service_ref := fromJson "{}" }}
{{- $pulsar_zk_from_component := fromJson "{}" }}

{{- if index $.component "serviceReferences" }}
  {{- range $i, $e := $.component.serviceReferences }}
    {{- if eq $i "pulsarZookeeper" }}
      {{- $pulsar_zk_from_service_ref = $e }}
      {{- break }}
    {{- end }}
  {{- end }}
{{- end }}
{{- range $i, $e := $.cluster.spec.componentSpecs }}
  {{- if eq $e.componentDefRef "zookeeper" }}
    {{- $pulsar_zk_from_component = $e }}
  {{- end }}
{{- end }}

# Try to get zookeeper from service reference first, if zookeeper service reference is empty, get default zookeeper componentDef in ClusterDefinition
{{- $zk_server := "" }}
{{- if $pulsar_zk_from_service_ref }}
  {{- if and (index $pulsar_zk_from_service_ref.spec "endpoint") (index $pulsar_zk_from_service_ref.spec "port") }}
     {{- $zk_server = printf "%s:%s" $pulsar_zk_from_service_ref.spec.endpoint.value $pulsar_zk_from_service_ref.spec.port.value }}
  {{- else }}
     {{- $zk_server = printf "%s-%s.%s.svc:2181" $clusterName $pulsar_zk_from_component.name $namespace }}
  {{- end }}
{{- else }}
  {{- $zk_server = printf "%s-%s.%s.svc:2181" $clusterName $pulsar_zk_from_component.name $namespace }}
{{- end }}
zookeeperServers: {{ $zk_server }}
configurationStoreServers: {{ $zk_server }}