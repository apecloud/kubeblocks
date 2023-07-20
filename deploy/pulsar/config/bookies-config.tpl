journalDirectories=/pulsar/data/bookkeeper/journal
compactionRateByBytes=52428800
useTransactionalCompaction=true
#dbStorage_readAheadCacheMaxSizeMb=32
#dbStorage_rocksDB_blockCacheSize=8388608
#dbStorage_rocksDB_writeBufferSizeMB=8
#dbStorage_writeCacheMaxSizeMb=32
# how long to wait, in seconds, before starting autorecovery of a lost bookie.
# TODO: set to 0 after opsRequest for rollingUpdate supports hooks
lostBookieRecoveryDelay=300
httpServerEnabled=true
httpServerPort=8000
journalDirectories=/pulsar/data/bookkeeper/journal
journalMaxBackups=0
ledgerDirectories=/pulsar/data/bookkeeper/ledgers
# statsProviderClass ref: https://bookkeeper.apache.org/docs/admin/metrics#stats-providers
statsProviderClass=org.apache.bookkeeper.stats.prometheus.PrometheusMetricsProvider
enableStatistics=true
useHostNameAsBookieID=true

{{- $autoRecoveryDaemonEnabled := "true" }}
{{- range $i, $e := $.cluster.spec.componentSpecs }}
  {{- if eq $e.componentDefRef "bookies-recovery" }}
    {{- $autoRecoveryDaemonEnabled = "false" }}
  {{- end }}
{{- end }}
autoRecoveryDaemonEnabled={{ $autoRecoveryDaemonEnabled }}