PULSAR_EXTRA_OPTS: -Dpulsar.allocator.exit_on_oom=true -Dio.netty.recycler.maxCapacity.default=1000 -Dio.netty.recycler.linkCapacity=1024 -Dnetworkaddress.cache.ttl=60 -XX:ActiveProcessorCount=1 -XshowSettings:vm -Ddepth=64
PULSAR_GC: -XX:+UseG1GC -XX:MaxGCPauseMillis=10 -XX:+ParallelRefProcEnabled -XX:+UnlockExperimentalVMOptions -XX:+DoEscapeAnalysis -XX:G1NewSizePercent=50 -XX:+DisableExplicitGC -XX:-ResizePLAB
{{- $MaxDirectMemorySize := "" }}
{{- $phy_memory := getContainerMemory ( index $.podSpec.containers 0 ) }}
{{- if gt $phy_memory 0 }}
  {{- $MaxDirectMemorySize = printf "-XX:MaxDirectMemorySize=%dm" (mul (div $phy_memory ( mul 1024 1024 10)) 6) }}
{{- end }}
PULSAR_MEM: -XX:MinRAMPercentage=30 -XX:MaxRAMPercentage=30 {{ $MaxDirectMemorySize }}
PULSAR_PREFIX_acknowledgmentAtBatchIndexLevelEnabled: "false"
PULSAR_PREFIX_backlogQuotaDefaultLimitGB: "-1"
PULSAR_PREFIX_backlogQuotaDefaultRetentionPolicy: producer_request_hold
PULSAR_PREFIX_bookkeeperClientReorderReadSequenceEnabled: "true"
PULSAR_PREFIX_bookkeeperDiskWeightBasedPlacementEnabled: "true"
PULSAR_PREFIX_bookkeeperEnableStickyReads: "true"
PULSAR_PREFIX_brokerDeleteInactiveTopicsEnabled: "false"
PULSAR_PREFIX_brokerInterceptors: ""
PULSAR_PREFIX_defaultNamespaceBundleSplitAlgorithm: topic_count_equally_divide
PULSAR_PREFIX_defaultNumPartitions: "1"
PULSAR_PREFIX_defaultNumberOfNamespaceBundles: "4"
PULSAR_PREFIX_delayedDeliveryEnabled: "true"
PULSAR_PREFIX_dispatcherMaxReadBatchSize: "1000"
PULSAR_PREFIX_enableReplicatedSubscriptions: "false"
PULSAR_PREFIX_managedLedgerNumSchedulerThreads: "2"
PULSAR_PREFIX_managedLedgerNumWorkerThreads: "2"
PULSAR_PREFIX_maxConcurrentLookupRequest: "1000"
PULSAR_PREFIX_maxConcurrentTopicLoadRequest: "1000"
PULSAR_PREFIX_maxConsumersPerSubscription: "5000"
PULSAR_PREFIX_maxConsumersPerTopic: "50000"
PULSAR_PREFIX_maxNumPartitionsPerPartitionedTopic: "2048"
PULSAR_PREFIX_maxProducersPerTopic: "10000"
PULSAR_PREFIX_maxUnackedMessagesPerConsumer: "10000"
PULSAR_PREFIX_maxUnackedMessagesPerSubscription: "50000"
PULSAR_PREFIX_preciseDispatcherFlowControl: "true"
PULSAR_PREFIX_subscriptionKeySharedUseConsistentHashing: "true"
PULSAR_PREFIX_transactionCoordinatorEnabled: "true"
PULSAR_PREFIX_systemTopicEnabled: "true"
PULSAR_PREFIX_acknowledgmentAtBatchIndexLevelEnabled: "true"
PULSAR_PREFIX_statusFilePath: /pulsar/status

# @deprecated since 2.8.0 subscriptionTypesEnabled is preferred over subscriptionKeySharedEnable.
PULSAR_PREFIX_subscriptionKeySharedEnable: "true"

# KoP config
# ref: 
# - https://github.com/streamnative/kop/blob/master/docs/kop.md
# - https://github.com/streamnative/kop/blob/master/docs/configuration.md 
PULSAR_PREFIX_messagingProtocols: kafka
#PULSAR_PREFIX_protocolHandlerDirectory: ./protocols
#PULSAR_PREFIX_narExtractionDirectory: /tmp/pulsar-nar
#PULSAR_PREFIX_kafkaListeners: kafka_external://0.0.0.0:9092
#PULSAR_PREFIX_kafkaProtocolMap: kafka_external:PLAINTEXT

# Set offset management as below, since offset management for KoP depeocalnds on Pulsar "Broker Entry Metadata".
# It’s required for KoP 2.8.0 or higher version.
PULSAR_PREFIX_brokerEntryMetadataInterceptors: org.apache.pulsar.common.intercept.AppendIndexMetadataInterceptor
# Disable the deletion of inactive topics. It’s not required but very important in KoP. Currently, 
# Pulsar deletes inactive partitions of a partitioned topic while the metadata of the partitioned topic is not deleted.
# KoP cannot create missed partitions in this case.
PULSAR_PREFIX_brokerDeleteInactiveTopicsEnabled: false

# KoP is compatible with Kafka clients 0.9 or higher. For Kafka clients 3.2.0 or higher, you have to add the following 
# configurations in KoP because of KIP-679.
PULSAR_PREFIX_kafkaTransactionCoordinatorEnabled: true
PULSAR_PREFIX_brokerDeduplicationEnabled: true