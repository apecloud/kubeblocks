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
httpServerEnabled: "true"
httpServerPort: "8000"
prometheusStatsHttpPort: "8000"
useHostNameAsBookieID: "true"
PULSAR_GC: -XX:+UseG1GC -XX:MaxGCPauseMillis=10 -XX:+ParallelRefProcEnabled -XX:+UnlockExperimentalVMOptions -XX:+DoEscapeAnalysis -XX:ParallelGCThreads=4 -XX:ConcGCThreads=4 -XX:G1NewSizePercent=50 -XX:+DisableExplicitGC -XX:-ResizePLAB -XX:+ExitOnOutOfMemoryError -XX:+PerfDisableSharedMem -Xlog:gc* -Xlog:gc::utctime -Xlog:safepoint -Xlog:gc+heap=trace -verbosegc
PULSAR_MEM: -Xms128m -Xmx256m -XX:MaxDirectMemorySize=256m