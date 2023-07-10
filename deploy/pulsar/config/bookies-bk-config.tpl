{{- $clusterName := $.cluster.metadata.name }}
{{- $namespace := $.cluster.metadata.namespace }}
{{- $pulsar_zk_component := fromJson "{}" }}
{{- range $i, $e := $.cluster.spec.componentSpecs }}
  {{- if eq $e.componentDefRef "zookeeper" }}
    {{- $pulsar_zk_component = $e }}
  {{- end }}
{{- end }}
{{- $zk_server := "" }}
{{- $zk_server = printf "%s-%s" $clusterName $pulsar_zk_component }}
BOOKIE_EXTRA_OPTS: -Dpulsar.allocator.exit_on_oom=true -Dio.netty.recycler.maxCapacity.default=1000
-Dio.netty.recycler.linkCapacity=1024 -XX:ActiveProcessorCount=1
BOOKIE_GC: -XX:+UseG1GC -XX:MaxGCPauseMillis=10 -XX:+ParallelRefProcEnabled -XX:+UnlockExperimentalVMOptions
-XX:+DoEscapeAnalysis -XX:G1NewSizePercent=50 -XX:+DisableExplicitGC -XX:-ResizePLAB
PULSAR_EXTRA_OPTS: ' '
PULSAR_GC: ' '
PULSAR_PREFIX_autoRecoveryDaemonEnabled: "false"
PULSAR_PREFIX_compactionRateByBytes: "52428800"
PULSAR_PREFIX_fileInfoFormatVersionToWrite: "1"
PULSAR_PREFIX_gcWaitTime: "300000"
PULSAR_PREFIX_isThrottleByBytes: "true"
PULSAR_PREFIX_journalDirectories: /pulsar/data/bookkeeper/journal-0
PULSAR_PREFIX_journalFormatVersionToWrite: "6"
PULSAR_PREFIX_journalMaxBackups: "0"
PULSAR_PREFIX_ledgerDirectories: /pulsar/data/bookkeeper/ledgers-0
PULSAR_PREFIX_numHighPriorityWorkerThreads: "1"
PULSAR_PREFIX_numReadWorkerThreads: "1"
PULSAR_PREFIX_persistBookieStatusEnabled: "false"
PULSAR_PREFIX_useTransactionalCompaction: "true"
httpServerEnabled: "true"
httpServerPort: "8000"
prometheusStatsHttpPort: "8000"
useHostNameAsBookieID: "true"
zkServers: {{ $zk_server }}:2181