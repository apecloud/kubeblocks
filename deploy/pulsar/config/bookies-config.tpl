PULSAR_GC: -XX:+UseG1GC -XX:MaxGCPauseMillis=10 -XX:+ParallelRefProcEnabled -XX:+UnlockExperimentalVMOptions -XX:+DoEscapeAnalysis -XX:ParallelGCThreads=4 -XX:ConcGCThreads=4 -XX:G1NewSizePercent=50 -XX:+DisableExplicitGC -XX:-ResizePLAB -XX:+ExitOnOutOfMemoryError -XX:+PerfDisableSharedMem -Xlog:gc* -Xlog:gc::utctime -Xlog:safepoint -Xlog:gc+heap=trace -verbosegc
PULSAR_MEM: -Xms128m -Xmx256m -XX:MaxDirectMemorySize=256m
PULSAR_PREFIX_journalDirectories: /pulsar/data/bookkeeper/journal
dbStorage_readAheadCacheMaxSizeMb: "32"
dbStorage_rocksDB_blockCacheSize: "8388608"
dbStorage_rocksDB_writeBufferSizeMB: "8"
dbStorage_writeCacheMaxSizeMb: "32"
httpServerEnabled: "true"
httpServerPort: "8000"
journalDirectories: /pulsar/data/bookkeeper/journal
journalMaxBackups: "0"
ledgerDirectories: /pulsar/data/bookkeeper/ledgers
statsProviderClass: org.apache.bookkeeper.stats.prometheus.PrometheusMetricsProvider
useHostNameAsBookieID: "true"
zkLedgersRootPath: /ledgers
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
