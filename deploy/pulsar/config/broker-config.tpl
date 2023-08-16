acknowledgmentAtBatchIndexLevelEnabled=false
backlogQuotaDefaultLimitGB=-1
backlogQuotaDefaultRetentionPolicy=producer_request_hold
bookkeeperClientReorderReadSequenceEnabled=true
bookkeeperDiskWeightBasedPlacementEnabled=true
bookkeeperEnableStickyReads=true
defaultNamespaceBundleSplitAlgorithm=topic_count_equally_divide
defaultNumPartitions=1
defaultNumberOfNamespaceBundles=4
delayedDeliveryEnabled=true
dispatcherMaxReadBatchSize=1000
enableReplicatedSubscriptions=false
managedLedgerNumSchedulerThreads=2
managedLedgerNumWorkerThreads=2
maxConcurrentLookupRequest=1000
maxConcurrentTopicLoadRequest=1000
maxConsumersPerSubscription=5000
maxConsumersPerTopic=50000
maxNumPartitionsPerPartitionedTopic=2048
maxProducersPerTopic=10000
maxUnackedMessagesPerConsumer=10000
maxUnackedMessagesPerSubscription=50000
preciseDispatcherFlowControl=true
subscriptionKeySharedUseConsistentHashing=true
transactionCoordinatorEnabled=true
systemTopicEnabled=true
acknowledgmentAtBatchIndexLevelEnabled=true
statusFilePath=/pulsar/status

# @deprecated since 2.8.0 subscriptionTypesEnabled is preferred over subscriptionKeySharedEnable.
subscriptionKeySharedEnable=true

# List of broker interceptor to load, which is a list of broker interceptor names
brokerInterceptors=

# KoP config
# ref:
# - https://github.com/streamnative/kop/blob/master/docs/kop.md
# - https://github.com/streamnative/kop/blob/master/docs/configuration.md
# List of messaging protocols to load, which is a list of protocol names
messagingProtocols=kafka
#protocolHandlerDirectory=./protocols
#narExtractionDirectory=/tmp/pulsar-nar
#kafkaListeners=kafka_external://0.0.0.0:9092
#kafkaProtocolMap=kafka_external:PLAINTEXT
allowAutoTopicCreationType=partitioned

# Set offset management as below, since offset management for KoP depeocalnds on Pulsar "Broker Entry Metadata".
# It’s required for KoP 2.8.0 or higher version.
brokerEntryMetadataInterceptors=org.apache.pulsar.common.intercept.AppendIndexMetadataInterceptor
# Disable the deletion of inactive topics. It’s not required but very important in KoP. Currently,
# Pulsar deletes inactive partitions of a partitioned topic while the metadata of the partitioned topic is not deleted.
# KoP cannot create missed partitions in this case.
brokerDeleteInactiveTopicsEnabled=false

# KoP is compatible with Kafka clients 0.9 or higher. For Kafka clients 3.2.0 or higher, you have to add the following
# configurations in KoP because of KIP-679.
kafkaTransactionCoordinatorEnabled=true
brokerDeduplicationEnabled=true
