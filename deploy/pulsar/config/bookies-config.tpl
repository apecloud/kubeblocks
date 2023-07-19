PULSAR_GC: -XX:+UseG1GC -XX:MaxGCPauseMillis=10 -XX:+ParallelRefProcEnabled -XX:+UnlockExperimentalVMOptions -XX:+DoEscapeAnalysis -XX:ParallelGCThreads=4 -XX:ConcGCThreads=4 -XX:G1NewSizePercent=50 -XX:+DisableExplicitGC -XX:-ResizePLAB -XX:+ExitOnOutOfMemoryError -XX:+PerfDisableSharedMem -XshowSettings:vm -Ddepth=64
PULSAR_PREFIX_journalDirectories: /pulsar/data/bookkeeper/journal
PULSAR_PREFIX_compactionRateByBytes: "52428800"
PULSAR_PREFIX_useTransactionalCompaction: "true"
#dbStorage_readAheadCacheMaxSizeMb: "32"
#dbStorage_rocksDB_blockCacheSize: "8388608"
#dbStorage_rocksDB_writeBufferSizeMB: "8"
#dbStorage_writeCacheMaxSizeMb: "32"
# how long to wait, in seconds, before starting autorecovery of a lost bookie.
# TODO: set to 0 after opsRequest for rollingUpdate supports hooks
lostBookieRecoveryDelay: "300"
httpServerEnabled: "true"
httpServerPort: "8000"
journalDirectories: /pulsar/data/bookkeeper/journal
journalMaxBackups: "0"
ledgerDirectories: /pulsar/data/bookkeeper/ledgers
# statsProviderClass ref: https://bookkeeper.apache.org/docs/admin/metrics#stats-providers
statsProviderClass: org.apache.bookkeeper.stats.prometheus.PrometheusMetricsProvider
enableStatistics: "true"
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
{{- $MaxDirectMemorySize := "" }}
{{- $phy_memory := getContainerMemory ( index $.podSpec.containers 0 ) }}
{{- if gt $phy_memory 0 }}
  {{- $MaxDirectMemorySize = printf "-XX:MaxDirectMemorySize=%dm" (div $phy_memory ( mul 1024 1024 2 )) }}
{{- end }}
PULSAR_MEM: -XX:MinRAMPercentage=25 -XX:MaxRAMPercentage=50 {{ $MaxDirectMemorySize }}

{{- $autoRecoveryDaemonEnabled := "true" }}
{{- range $i, $e := $.cluster.spec.componentSpecs }}
  {{- if eq $e.componentDefRef "bookies-recovery" }}
    {{- $autoRecoveryDaemonEnabled = "false" }}
  {{- end }}
{{- end }}
PULSAR_PREFIX_autoRecoveryDaemonEnabled: {{ $autoRecoveryDaemonEnabled }}