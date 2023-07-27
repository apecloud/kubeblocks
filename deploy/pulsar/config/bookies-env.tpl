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
{{- $pulsar_zk_component := fromJson "{}" }}
{{- range $i, $e := $.cluster.spec.componentSpecs }}
  {{- if eq $e.componentDefRef "zookeeper" }}
    {{- $pulsar_zk_component = $e }}
  {{- end }}
{{- end }}
{{- $zk_server := "" }}
{{- $zk_server = printf "%s-%s.%s.svc" $clusterName $pulsar_zk_component.name $namespace }}
zkServers: {{ $zk_server }}:2181
