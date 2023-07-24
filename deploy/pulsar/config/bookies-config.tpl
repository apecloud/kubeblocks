{{- $phy_memory := getContainerMemory ( index $.podSpec.containers 0 ) }}

journalDirectories=/pulsar/data/bookkeeper/journal
compactionRateByBytes=52428800
useTransactionalCompaction=true

## for bookies storage configuration

{{- if gt $phy_memory 0 }}
{{- $phy_memory_mb := div $phy_memory ( mul 1024 1024 ) }}
## 25% of the available direct memory
dbStorage_writeCacheMaxSizeMb={{ div $phy_memory_mb 4 }}
dbStorage_readAheadCacheMaxSizeMb={{ div $phy_memory_mb 4 }}
{{- end }}

dbStorage_readAheadCacheBatchSize=100
dbStorage_rocksDB_blockSize=268435456
dbStorage_rocksDB_writeBufferSizeMB=64
dbStorage_rocksDB_sstSizeInMB=64
dbStorage_rocksDB_blockSize=65536
dbStorage_rocksDB_bloomFilterBitsPerKey=10
dbStorage_rocksDB_numLevels=-1
dbStorage_rocksDB_numFilesInLevel0=10
dbStorage_rocksDB_maxSizeInLevel1MB=256

## for disk utilization
# For each ledger dir, maximum disk space which can be used. Default is 0.95f. i.e. 95% of disk can be used at most after which nothing will be written to that partition. If all ledger dir partions are full, then bookie will turn to readonly mode if 'readOnlyModeEnabled=true' is set, else it will shutdown. Valid values should be in between 0 and 1 (exclusive).
diskUsageThreshold=0.95
# The disk free space low water mark threshold. Disk is considered full when usage threshold is exceeded. Disk returns back to non-full state when usage is below low water mark threshold. This prevents it from going back and forth between these states frequently when concurrent writes and compaction are happening. This also prevent bookie from switching frequently between read-only and read-writes states in the same cases.
diskUsageWarnThreshold=0.95
# Set the disk free space low water mark threshold. Disk is considered full when usage threshold is exceeded. Disk returns back to non-full state when usage is below low water mark threshold. This prevents it from going back and forth between these states frequently when concurrent writes and compaction are happening. This also prevent bookie from switching frequently between read-only and read-writes states in the same cases.
diskUsageLwmThreshold=0.9
# Disk check interval in milliseconds. Interval to check the ledger dirs usage.
diskCheckInterval=10000

## for Journal settings
journalDirectories=/pulsar/data/bookkeeper/journal
#journalDirectory=/pulsar/data/bookkeeper/txn
journalFormatVersionToWrite=5
journalMaxSizeMB=2048
journalMaxBackups=5
journalPreAllocSizeMB=16
journalWriteBufferSizeKB=64
journalRemoveFromPageCache=true
journalSyncData=true
journalAdaptiveGroupWrites=true
journalMaxGroupWaitMSec=2
journalBufferedWritesThreshold=524288
journalFlushWhenQueueEmpty=false
journalAlignmentSize=512
# Maximum entries to buffer to impose on a journal write to achieve grouping.
journalBufferedEntriesThreshold=0
journalFlushWhenQueueEmpty=false
journalQueueSize=10000

# TODO fix: For persisiting explicitLac, journalFormatVersionToWrite should be >= 6and FileInfoFormatVersionToWrite should be >= 1
# fileInfoFormatVersionToWrite=1

# how long to wait, in seconds, before starting autorecovery of a lost bookie.
# TODO: set to 0 after opsRequest for rollingUpdate supports hooks
lostBookieRecoveryDelay=300
httpServerEnabled=true
httpServerPort=8000
ledgerDirectories=/pulsar/data/bookkeeper/ledgers
# statsProviderClass ref: https://bookkeeper.apache.org/docs/admin/metrics#stats-providers
statsProviderClass=org.apache.bookkeeper.stats.prometheus.PrometheusMetricsProvider
enableStatistics=true
useHostNameAsBookieID=true

zkLedgersRootPath=/ledgers

{{- $autoRecoveryDaemonEnabled := "true" }}
{{- range $i, $e := $.cluster.spec.componentSpecs }}
  {{- if eq $e.componentDefRef "bookies-recovery" }}
    {{- $autoRecoveryDaemonEnabled = "false" }}
  {{- end }}
{{- end }}
autoRecoveryDaemonEnabled={{ $autoRecoveryDaemonEnabled }}