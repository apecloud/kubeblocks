PULSAR_GC: -XX:+UseG1GC -XX:MaxGCPauseMillis=10 -XX:+ParallelRefProcEnabled -XX:+UnlockExperimentalVMOptions -XX:+DoEscapeAnalysis -XX:ParallelGCThreads=4 -XX:ConcGCThreads=4 -XX:G1NewSizePercent=50 -XX:+DisableExplicitGC -XX:-ResizePLAB -XX:+ExitOnOutOfMemoryError -XX:+PerfDisableSharedMem -XshowSettings:vm -Ddepth=64

{{- $MaxDirectMemorySize := "" }}
{{- $phy_memory := getContainerMemory ( index $.podSpec.containers 0 ) }}
{{- if gt $phy_memory 0 }}
  {{- $phy_memory_mb := div $phy_memory ( mul 1024 1024 ) }}
  {{- $MaxDirectMemorySize = printf "-XX:MaxDirectMemorySize=%dm" (div ( mul $phy_memory_mb 3 ) 4 ) }}
{{- end }}
PULSAR_MEM: -XX:MinRAMPercentage=25 -XX:MaxRAMPercentage=50 {{ $MaxDirectMemorySize }}

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
zkServers: {{ $zk_server }}
