// Copyright (C) 2022-2023 ApeCloud Co., Ltd
//
// This file is part of KubeBlocks project
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// AUTO generate: ./bin/cue-tools https://raw.githubusercontent.com/apache/pulsar/master/pulsar-broker-common/src/main/java/org/apache/pulsar/broker/ServiceConfiguration.java "ServiceConfiguration" "PulsarBrokersParameter" > broker-config-constraint.cue
#PulsarBrokersParameter: {
	// The Zookeeper quorum connection string (as a comma-separated list). Deprecated in favour of metadataStoreUrl
	// @deprecated
	zookeeperServers: string

	// The metadata store URL. \n Examples: \n  * zk:my-zk-1:2181,my-zk-2:2181,my-zk-3:2181\n  * my-zk-1:2181,my-zk-2:2181,my-zk-3:2181 (will default to ZooKeeper when the schema is not specified)\n  * zk:my-zk-1:2181,my-zk-2:2181,my-zk-3:2181/my-chroot-path (to add a ZK chroot path)\n
	metadataStoreUrl: string

	// Global Zookeeper quorum connection string (as a comma-separated list). Deprecated in favor of using `configurationStoreServers`
	// @deprecated
	globalZookeeperServers: string

	// Configuration store connection string (as a comma-separated list). Deprecated in favor of `configurationMetadataStoreUrl`
	// @deprecated
	configurationStoreServers: string

	// The metadata store URL for the configuration data. If empty, we fall back to use metadataStoreUrl
	configurationMetadataStoreUrl: string

	// The port for serving binary protobuf requests. If set, defines a server binding for bindAddress:brokerServicePort. The Default value is 6650.
	brokerServicePort: int

	// The port for serving TLS-secured binary protobuf requests. If set, defines a server binding for bindAddress:brokerServicePortTls.
	brokerServicePortTls: int

	// The port for serving http requests
	webServicePort: int

	// The port for serving https requests
	webServicePortTls: int

	// Specify the TLS provider for the web service: SunJSSE, Conscrypt and etc.
	webServiceTlsProvider: string

	// Specify the tls protocols the proxy's web service will use to negotiate during TLS Handshake.\n\nExample:- [TLSv1.3, TLSv1.2]
	webServiceTlsProtocols: string

	// Specify the tls cipher the proxy's web service will use to negotiate during TLS Handshake.\n\nExample:- [TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256]
	webServiceTlsCiphers: string

	// Hostname or IP address the service binds on
	bindAddress: string

	// Hostname or IP address the service advertises to the outside world. If not set, the value of `InetAddress.getLocalHost().getCanonicalHostName()` is used.
	advertisedAddress: string

	// Used to specify multiple advertised listeners for the broker. The value must format as <listener_name>:pulsar://<host>:<port>,multiple listeners should separate with commas.Do not use this configuration with advertisedAddress and brokerServicePort.The Default value is absent means use advertisedAddress and brokerServicePort.
	advertisedListeners: string

	// Used to specify the internal listener name for the broker.The listener name must contain in the advertisedListeners.The Default value is absent, the broker uses the first listener as the internal listener.
	internalListenerName: string

	// Used to specify additional bind addresses for the broker. The value must format as <listener_name>:<scheme>://<host>:<port>, multiple bind addresses should be separated with commas. Associates each bind address with an advertised listener and protocol handler. Note that the brokerServicePort, brokerServicePortTls, webServicePort, and webServicePortTls properties define additional bindings.
	bindAddresses: string

	// Enable or disable the proxy protocol. If true, the real IP addresses of consumers and producers can be obtained when getting topic statistics data.
	haProxyProtocolEnabled: bool

	// Number of threads to use for Netty Acceptor. Default is set to `1`
	numAcceptorThreads: int

	// Number of threads to use for Netty IO. Default is set to `2 * Runtime.getRuntime().availableProcessors()`
	numIOThreads: int

	// Number of threads to use for orderedExecutor. The ordered executor is used to operate with zookeeper, such as init zookeeper client, get namespace policies from zookeeper etc. It also used to split bundle. Default is 8
	numOrderedExecutorThreads: int

	// Number of threads to use for HTTP requests processing Default is set to `2 * Runtime.getRuntime().availableProcessors()`
	numHttpServerThreads: int

	// Number of threads to use for pulsar broker service. The executor in thread pool will do basic broker operation like load/unload bundle, update managedLedgerConfig, update topic/subscription/replicator message dispatch rate, do leader election etc. Default is set to 20
	numExecutorThreadPoolSize: int

	// Number of thread pool size to use for pulsar zookeeper callback service.The cache executor thread pool is used for restarting global zookeeper session. Default is 10
	numCacheExecutorThreadPoolSize: int

	// Option to enable busy-wait settings. Default is false. WARNING: This option will enable spin-waiting on executors and IO threads in order to reduce latency during context switches. The spinning will consume 100% CPU even when the broker is not doing any work. It is recommended to reduce the number of IO threads and BK client threads to only have few CPU cores busy.
	enableBusyWait: bool

	// Max concurrent web requests
	maxConcurrentHttpRequests: int

	// Capacity for thread pool queue in the HTTP server Default is set to 8192.
	httpServerThreadPoolQueueSize: int

	// Capacity for accept queue in the HTTP server Default is set to 8192.
	httpServerAcceptQueueSize: int

	// Maximum number of inbound http connections. (0 to disable limiting)
	maxHttpServerConnections: int

	// Whether to enable the delayed delivery for messages.
	delayedDeliveryEnabled: bool

	// Class name of the factory that implements the delayed deliver tracker.            If value is "org.apache.pulsar.broker.delayed.BucketDelayedDeliveryTrackerFactory", \            will create bucket based delayed message index tracker.
	delayedDeliveryTrackerFactoryClassName: string

	// Control the tick time for when retrying on delayed delivery, affecting the accuracy of the delivery time compared to the scheduled time. Default is 1 second. Note that this time is used to configure the HashedWheelTimer's tick time.
	delayedDeliveryTickTimeMillis: int

	// Whether the deliverAt time is strictly followed. When false (default), messages may be sent to consumers before the deliverAt time by as much as the tickTimeMillis. This can reduce the overhead on the broker of maintaining the delayed index for a potentially very short time period. When true, messages will not be sent to consumer until the deliverAt time has passed, and they may be as late as the deliverAt time plus the tickTimeMillis for the topic plus the delayedDeliveryTickTimeMillis.
	isDelayedDeliveryDeliverAtTimeStrict: bool

	// The delayed message index bucket min index count. When the index count of the current bucket is more than \            this value and all message indexes of current ledger have already been added to the tracker \            we will seal the bucket.
	delayedDeliveryMinIndexCountPerBucket: int

	// The delayed message index time step(in seconds) in per bucket snapshot segment, \            after reaching the max time step limitation, the snapshot segment will be cut off.
	delayedDeliveryMaxTimeStepPerBucketSnapshotSegmentSeconds: int

	// The max number of delayed message index in per bucket snapshot segment, -1 means no limitation, \            after reaching the max number limitation, the snapshot segment will be cut off.
	delayedDeliveryMaxIndexesPerBucketSnapshotSegment: int

	// The max number of delayed message index bucket, \            after reaching the max buckets limitation, the adjacent buckets will be merged.\            (disable with value -1)
	delayedDeliveryMaxNumBuckets: int

	// Size of the lookahead window to use when detecting if all the messages in the topic have a fixed delay for InMemoryDelayedDeliveryTracker (the default DelayedDeliverTracker). Default is 50,000. Setting the lookahead window to 0 will disable the logic to handle fixed delays in messages in a different way.
	delayedDeliveryFixedDelayDetectionLookahead: int

	// Whether to enable the acknowledge of batch local index
	acknowledgmentAtBatchIndexLevelEnabled: bool

	// Enable the WebSocket API service in broker
	webSocketServiceEnabled: bool

	// Flag indicates whether to run broker in standalone mode
	isRunningStandalone: bool

	// Name of the cluster to which this broker belongs to
	clusterName: string

	// The maximum number of tenants that each pulsar cluster can create.This configuration is not precise control, in a concurrent scenario, the threshold will be exceeded.
	maxTenants: int

	// Enable cluster's failure-domain which can distribute brokers into logical region
	failureDomainsEnabled: bool

	// Metadata store session timeout in milliseconds.
	metadataStoreSessionTimeoutMillis: int

	// Metadata store operation timeout in seconds.
	metadataStoreOperationTimeoutSeconds: int

	// Metadata store cache expiry time in seconds.
	metadataStoreCacheExpirySeconds: int

	// Is metadata store read-only operations.
	metadataStoreAllowReadOnlyOperations: bool

	// ZooKeeper session timeout in milliseconds. @deprecated - Use metadataStoreSessionTimeoutMillis instead.
	// @deprecated
	zooKeeperSessionTimeoutMillis: int

	// ZooKeeper operation timeout in seconds. @deprecated - Use metadataStoreOperationTimeoutSeconds instead.
	// @deprecated
	zooKeeperOperationTimeoutSeconds: int

	// ZooKeeper cache expiry time in seconds. @deprecated - Use metadataStoreCacheExpirySeconds instead.
	// @deprecated
	zooKeeperCacheExpirySeconds: int

	// Is zookeeper allow read-only operations.
	// @deprecated
	zooKeeperAllowReadOnlyOperations: bool

	// Time to wait for broker graceful shutdown. After this time elapses, the process will be killed
	brokerShutdownTimeoutMs: int

	// Flag to skip broker shutdown when broker handles Out of memory error
	skipBrokerShutdownOnOOM: bool

	// Amount of seconds to timeout when loading a topic. In situations with many geo-replicated clusters, this may need raised.
	topicLoadTimeoutSeconds: int

	// Whether we should enable metadata operations batching
	metadataStoreBatchingEnabled: bool

	// Maximum delay to impose on batching grouping
	metadataStoreBatchingMaxDelayMillis: int

	// Maximum number of operations to include in a singular batch
	metadataStoreBatchingMaxOperations: int

	// Maximum size of a batch
	metadataStoreBatchingMaxSizeKb: int

	// Configuration file path for local metadata store. It's supported by RocksdbMetadataStore for now.
	metadataStoreConfigPath: string

	// Event topic to sync metadata between separate pulsar clusters on different cloud platforms.
	metadataSyncEventTopic: string

	// Event topic to sync configuration-metadata between separate pulsar clusters on different cloud platforms.
	configurationMetadataSyncEventTopic: string

	// Factory class-name to create topic with custom workflow
	topicFactoryClassName: string

	// Enable backlog quota check. Enforces actions on topic when the quota is reached
	backlogQuotaCheckEnabled: bool

	// Whether to enable precise time based backlog quota check. Enabling precise time based backlog quota check will cause broker to read first entry in backlog of the slowest cursor on a ledger which will mostly result in reading entry from BookKeeper's disk which can have negative impact on overall performance. Disabling precise time based backlog quota check will just use the timestamp indicating when a ledger was closed, which is of coarser granularity.
	preciseTimeBasedBacklogQuotaCheck: bool

	// How often to check for topics that have reached the quota. It only takes effects when `backlogQuotaCheckEnabled` is true
	backlogQuotaCheckIntervalInSeconds: int

	// @deprecated - Use backlogQuotaDefaultLimitByte instead.
	backlogQuotaDefaultLimitGB: float

	// Default per-topic backlog quota limit by size, less than 0 means no limitation. default is -1. Increase it if you want to allow larger msg backlog
	backlogQuotaDefaultLimitBytes: int

	// Default per-topic backlog quota limit by time in second, less than 0 means no limitation. default is -1. Increase it if you want to allow larger msg backlog
	backlogQuotaDefaultLimitSecond: int

	// Default backlog quota retention policy. Default is producer_request_hold\n\n'producer_request_hold' Policy which holds producer's send request until theresource becomes available (or holding times out)\n'producer_exception' Policy which throws javax.jms.ResourceAllocationException to the producer\n'consumer_backlog_eviction' Policy which evicts the oldest message from the slowest consumer's backlog
	backlogQuotaDefaultRetentionPolicy: string

	// Default ttl for namespaces if ttl is not already configured at namespace policies. (disable default-ttl with value 0)
	ttlDurationDefaultInSeconds: int

	// Enable the deletion of inactive topics.\nIf only enable this option, will not clean the metadata of partitioned topic.
	brokerDeleteInactiveTopicsEnabled: bool

	// Metadata of inactive partitioned topic will not be automatically cleaned up by default.\nNote: If `allowAutoTopicCreation` and this option are enabled at the same time,\nit may appear that a partitioned topic has just been deleted but is automatically created as a non-partitioned topic.
	brokerDeleteInactivePartitionedTopicMetadataEnabled: bool

	// How often to check for inactive topics
	brokerDeleteInactiveTopicsFrequencySeconds: int

	// Set the inactive topic delete mode. Default is delete_when_no_subscriptions\n'delete_when_no_subscriptions' mode only delete the topic which has no subscriptions and no active producers\n'delete_when_subscriptions_caught_up' mode only delete the topic that all subscriptions has no backlogs(caught up) and no active producers/consumers
	brokerDeleteInactiveTopicsMode: string

	// Max duration of topic inactivity in seconds, default is not present\nIf not present, 'brokerDeleteInactiveTopicsFrequencySeconds' will be used\nTopics that are inactive for longer than this value will be deleted
	brokerDeleteInactiveTopicsMaxInactiveDurationSeconds: int

	// Allow forced deletion of tenants. Default is false.
	forceDeleteTenantAllowed: bool

	// Allow forced deletion of namespaces. Default is false.
	forceDeleteNamespaceAllowed: bool

	// Max pending publish requests per connection to avoid keeping large number of pending requests in memory. Default: 1000
	maxPendingPublishRequestsPerConnection: int

	// How frequently to proactively check and purge expired messages
	messageExpiryCheckIntervalInMinutes: int

	// How long to delay rewinding cursor and dispatching messages when active consumer is changed
	activeConsumerFailoverDelayTimeMillis: int

	// Maximum time to spend while scanning a subscription to calculate the accurate backlog
	subscriptionBacklogScanMaxTimeMs: int

	// Maximum number of entries to process while scanning a subscription to calculate the accurate backlog
	subscriptionBacklogScanMaxEntries: int

	// How long to delete inactive subscriptions from last consuming. When it is 0, inactive subscriptions are not deleted automatically
	subscriptionExpirationTimeMinutes: int

	// Enable subscription message redelivery tracker to send redelivery count to consumer (default is enabled)
	subscriptionRedeliveryTrackerEnabled: bool

	// How frequently to proactively check and purge expired subscription
	subscriptionExpiryCheckIntervalInMinutes: int

	// Enable subscription types (default is all type enabled)
	subscriptionTypesEnabled: string

	// Enable Key_Shared subscription (default is enabled).\n@deprecated - use subscriptionTypesEnabled instead.
	subscriptionKeySharedEnable: bool

	// On KeyShared subscriptions, with default AUTO_SPLIT mode, use splitting ranges or consistent hashing to reassign keys to new consumers (default is consistent hashing)
	subscriptionKeySharedUseConsistentHashing: bool

	// On KeyShared subscriptions, number of points in the consistent-hashing ring. The higher the number, the more equal the assignment of keys to consumers
	subscriptionKeySharedConsistentHashingReplicaPoints: int

	// Set the default behavior for message deduplication in the broker.\n\nThis can be overridden per-namespace. If enabled, broker will reject messages that were already stored in the topic
	brokerDeduplicationEnabled: bool

	// Maximum number of producer information that it's going to be persisted for deduplication purposes
	brokerDeduplicationMaxNumberOfProducers: int

	// How often is the thread pool scheduled to check whether a snapshot needs to be taken.(disable with value 0)
	brokerDeduplicationSnapshotFrequencyInSeconds: int

	// If this time interval is exceeded, a snapshot will be taken.It will run simultaneously with `brokerDeduplicationEntriesInterval`
	brokerDeduplicationSnapshotIntervalSeconds: int

	// Number of entries after which a dedup info snapshot is taken.\n\nA bigger interval will lead to less snapshots being taken though it would increase the topic recovery time, when the entries published after the snapshot need to be replayed
	brokerDeduplicationEntriesInterval: int

	// Time of inactivity after which the broker will discard the deduplication information relative to a disconnected producer. Default is 6 hours.
	brokerDeduplicationProducerInactivityTimeoutMinutes: int

	// When a namespace is created without specifying the number of bundle, this value will be used as the default
	defaultNumberOfNamespaceBundles: int

	// The maximum number of namespaces that each tenant can create.This configuration is not precise control, in a concurrent scenario, the threshold will be exceeded
	maxNamespacesPerTenant: int

	// Max number of topics allowed to be created in the namespace. When the topics reach the max topics of the namespace, the broker should reject the new topic request(include topic auto-created by the producer or consumer) until the number of connected consumers decrease.  Using a value of 0, is disabling maxTopicsPerNamespace-limit check.
	maxTopicsPerNamespace: int

	// The maximum number of connections in the broker. If it exceeds, new connections are rejected.
	brokerMaxConnections: int

	// The maximum number of connections per IP. If it exceeds, new connections are rejected.
	brokerMaxConnectionsPerIp: int

	// Allow schema to be auto updated at broker level. User can override this by 'is_allow_auto_update_schema' of namespace policy. This is enabled by default.
	isAllowAutoUpdateSchemaEnabled: bool

	// Whether to enable the automatic shrink of pendingAcks map, the default is false, which means it is not enabled. When there are a large number of share or key share consumers in the cluster, it can be enabled to reduce the memory consumption caused by pendingAcks.
	autoShrinkForConsumerPendingAcksMap: bool

	// Enable check for minimum allowed client library version
	clientLibraryVersionCheckEnabled: bool

	// Path for the file used to determine the rotation status for the broker when responding to service discovery health checks
	statusFilePath: string

	// Max number of unacknowledged messages allowed to receive messages by a consumer on a shared subscription.\n\n Broker will stop sending messages to consumer once, this limit reaches until consumer starts acknowledging messages back and unack count reaches to `maxUnackedMessagesPerConsumer/2`. Using a value of 0, it is disabling  unackedMessage-limit check and consumer can receive messages without any restriction
	maxUnackedMessagesPerConsumer: int

	// Max number of unacknowledged messages allowed per shared subscription. \n\n Broker will stop dispatching messages to all consumers of the subscription once this  limit reaches until consumer starts acknowledging messages back and unack count reaches to `limit/2`. Using a value of 0, is disabling unackedMessage-limit check and dispatcher can dispatch messages without any restriction
	maxUnackedMessagesPerSubscription: int

	// Max number of unacknowledged messages allowed per broker. \n\n Once this limit reaches, broker will stop dispatching messages to all shared subscription  which has higher number of unack messages until subscriptions start acknowledging messages  back and unack count reaches to `limit/2`. Using a value of 0, is disabling unackedMessage-limit check and broker doesn't block dispatchers
	maxUnackedMessagesPerBroker: int

	// Once broker reaches maxUnackedMessagesPerBroker limit, it blocks subscriptions which has higher  unacked messages than this percentage limit and subscription will not receive any new messages  until that subscription acks back `limit/2` messages
	maxUnackedMessagesPerSubscriptionOnBrokerBlocked: float

	// Maximum size of Consumer metadata
	maxConsumerMetadataSize: int

	// Broker periodically checks if subscription is stuck and unblock if flag is enabled. (Default is disabled)
	unblockStuckSubscriptionEnabled: bool

	// Tick time to schedule task that checks topic publish rate limiting across all topics  Reducing to lower value can give more accuracy while throttling publish but it uses more CPU to perform frequent check. (Disable publish throttling with value 0)
	topicPublisherThrottlingTickTimeMillis: int

	// Enable precise rate limit for topic publish
	preciseTopicPublishRateLimiterEnable: bool

	// Tick time to schedule task that checks broker publish rate limiting across all topics  Reducing to lower value can give more accuracy while throttling publish but it uses more CPU to perform frequent check. (Disable publish throttling with value 0)
	brokerPublisherThrottlingTickTimeMillis: int

	// Max Rate(in 1 seconds) of Message allowed to publish for a broker when broker publish rate limiting enabled. (Disable message rate limit with value 0)
	brokerPublisherThrottlingMaxMessageRate: int

	// Max Rate(in 1 seconds) of Byte allowed to publish for a broker when broker publish rate limiting enabled. (Disable byte rate limit with value 0)
	brokerPublisherThrottlingMaxByteRate: int

	// Default messages per second dispatch throttling-limit for whole broker. Using a value of 0, is disabling default message-byte dispatch-throttling
	dispatchThrottlingRateInMsg: int

	// Default bytes per second dispatch throttling-limit for whole broker. Using a value of 0, is disabling default message-byte dispatch-throttling
	dispatchThrottlingRateInByte: int

	// Max Rate(in 1 seconds) of Message allowed to publish for a topic when topic publish rate limiting enabled. (Disable byte rate limit with value 0)
	maxPublishRatePerTopicInMessages: int

	// Max Rate(in 1 seconds) of Byte allowed to publish for a topic when topic publish rate limiting enabled. (Disable byte rate limit with value 0)
	maxPublishRatePerTopicInBytes: int

	// Too many subscribe requests from a consumer can cause broker rewinding consumer cursors  and loading data from bookies, hence causing high network bandwidth usage When the positive value is set, broker will throttle the subscribe requests for one consumer. Otherwise, the throttling will be disabled. The default value of this setting is 0 - throttling is disabled.
	subscribeThrottlingRatePerConsumer: int

	// Rate period for {subscribeThrottlingRatePerConsumer}. Default is 30s.
	subscribeRatePeriodPerConsumerInSecond: int

	// Default number of message dispatching throttling-limit for every topic. \n\nUsing a value of 0, is disabling default message dispatch-throttling
	dispatchThrottlingRatePerTopicInMsg: int

	// Default number of message-bytes dispatching throttling-limit for every topic. \n\nUsing a value of 0, is disabling default message-byte dispatch-throttling
	dispatchThrottlingRatePerTopicInByte: int

	// Apply dispatch rate limiting on batch message instead individual messages with in batch message. (Default is disabled)
	dispatchThrottlingOnBatchMessageEnabled: bool

	// Default number of message dispatching throttling-limit for a subscription. \n\nUsing a value of 0, is disabling default message dispatch-throttling.
	dispatchThrottlingRatePerSubscriptionInMsg: int

	// Default number of message-bytes dispatching throttling-limit for a subscription. \n\nUsing a value of 0, is disabling default message-byte dispatch-throttling.
	dispatchThrottlingRatePerSubscriptionInByte: int

	// Default number of message dispatching throttling-limit for every replicator in replication. \n\nUsing a value of 0, is disabling replication message dispatch-throttling
	dispatchThrottlingRatePerReplicatorInMsg: int

	// Default number of message-bytes dispatching throttling-limit for every replicator in replication. \n\nUsing a value of 0, is disabling replication message-byte dispatch-throttling
	dispatchThrottlingRatePerReplicatorInByte: int

	// Dispatch rate-limiting relative to publish rate. (Enabling flag will make broker to dynamically update dispatch-rate relatively to publish-rate: throttle-dispatch-rate = (publish-rate + configured dispatch-rate)
	dispatchThrottlingRateRelativeToPublishRate: bool

	// Default dispatch-throttling is disabled for consumers which already caught-up with published messages and don't have backlog. This enables dispatch-throttling for  non-backlog consumers as well.
	dispatchThrottlingOnNonBacklogConsumerEnabled: bool

	// Default policy for publishing usage reports to system topic is disabled.This enables publishing of usage reports
	resourceUsageTransportClassName: string

	// Default interval to publish usage reports if resourceUsagePublishToTopic is enabled.
	resourceUsageTransportPublishIntervalInSecs: int

	// Enables evaluating subscription pattern on broker side.
	enableBrokerSideSubscriptionPatternEvaluation: bool

	// Max length of subscription pattern
	subscriptionPatternMaxLength: int

	// Max number of entries to read from bookkeeper. By default it is 100 entries.
	dispatcherMaxReadBatchSize: int

	// Dispatch messages and execute broker side filters in a per-subscription thread
	dispatcherDispatchMessagesInSubscriptionThread: bool

	// Whether the broker should count filtered entries in dispatch rate limit calculations. When disabled, only messages sent to a consumer count towards a dispatch rate limit at the broker, topic, and subscription level. When enabled, messages filtered out due to entry filter logic are counted towards each relevant rate limit.
	dispatchThrottlingForFilteredEntriesEnabled: bool

	// Max size in bytes of entries to read from bookkeeper. By default it is 5MB.
	dispatcherMaxReadSizeBytes: int

	// Min number of entries to read from bookkeeper. By default it is 1 entries.When there is an error occurred on reading entries from bookkeeper, the broker will backoff the batch size to this minimum number.
	dispatcherMinReadBatchSize: int

	// The read failure backoff initial time in milliseconds. By default it is 15s.
	dispatcherReadFailureBackoffInitialTimeInMs: int

	// The read failure backoff max time in milliseconds. By default it is 60s.
	dispatcherReadFailureBackoffMaxTimeInMs: int

	// The read failure backoff mandatory stop time in milliseconds. By default it is 0s.
	dispatcherReadFailureBackoffMandatoryStopTimeInMs: int

	// Time in milliseconds to delay the new delivery of a message when an EntryFilter returns RESCHEDULE.
	dispatcherEntryFilterRescheduledMessageDelay: int

	// Max number of entries to dispatch for a shared subscription. By default it is 20 entries.
	dispatcherMaxRoundRobinBatchSize: int

	// Precise dispatcher flow control according to history message number of each entry
	preciseDispatcherFlowControl: bool

	// Class name of pluggable entry filter that decides whether the entry needs to be filtered.You can use this class to decide which entries can be sent to consumers.Multiple names need to be separated by commas.
	entryFilterNames: string

	// The directory for all the entry filter implementations.
	entryFiltersDirectory: string

	// Whether allow topic level entry filters policies overrides broker configuration.
	allowOverrideEntryFilters: bool

	// Max number of concurrent lookup request broker allows to throttle heavy incoming lookup traffic
	maxConcurrentLookupRequest: int

	// Max number of concurrent topic loading request broker allows to control number of zk-operations
	maxConcurrentTopicLoadRequest: int

	// Max concurrent non-persistent message can be processed per connection
	maxConcurrentNonPersistentMessagePerConnection: int

	// Number of worker threads to serve non-persistent topic.\n@deprecated - use topicOrderedExecutorThreadNum instead.
	// @deprecated
	numWorkerThreadsForNonPersistentTopic: int

	// Number of worker threads to serve topic ordered executor
	topicOrderedExecutorThreadNum: int

	// Enable broker to load persistent topics
	enablePersistentTopics: bool

	// Enable broker to load non-persistent topics
	enableNonPersistentTopics: bool

	// Enable to run bookie along with broker
	enableRunBookieTogether: bool

	// Enable to run bookie autorecovery along with broker
	enableRunBookieAutoRecoveryTogether: bool

	// Max number of producers allowed to connect to topic. \n\nOnce this limit reaches, Broker will reject new producers until the number of connected producers decrease. Using a value of 0, is disabling maxProducersPerTopic-limit check.
	maxProducersPerTopic: int

	// Max number of producers with the same IP address allowed to connect to topic. \n\nOnce this limit reaches, Broker will reject new producers until the number of connected producers with the same IP address decrease. Using a value of 0, is disabling maxSameAddressProducersPerTopic-limit check.
	maxSameAddressProducersPerTopic: int

	// Enforce producer to publish encrypted messages.(default disable).
	encryptionRequireOnProducer: bool

	// Max number of consumers allowed to connect to topic. \n\nOnce this limit reaches, Broker will reject new consumers until the number of connected consumers decrease. Using a value of 0, is disabling maxConsumersPerTopic-limit check.
	maxConsumersPerTopic: int

	// Max number of consumers with the same IP address allowed to connect to topic. \n\nOnce this limit reaches, Broker will reject new consumers until the number of connected consumers with the same IP address decrease. Using a value of 0, is disabling maxSameAddressConsumersPerTopic-limit check.
	maxSameAddressConsumersPerTopic: int

	// Max number of subscriptions allowed to subscribe to topic. \n\nOnce this limit reaches,  broker will reject new subscription until the number of subscribed subscriptions decrease.\n Using a value of 0, is disabling maxSubscriptionsPerTopic limit check.
	maxSubscriptionsPerTopic: int

	// Max number of consumers allowed to connect to subscription. \n\nOnce this limit reaches, Broker will reject new consumers until the number of connected consumers decrease. Using a value of 0, is disabling maxConsumersPerSubscription-limit check.
	maxConsumersPerSubscription: int

	// Max size of messages.
	maxMessageSize: int

	// Enable tracking of replicated subscriptions state across clusters.
	enableReplicatedSubscriptions: bool

	// Frequency of snapshots for replicated subscriptions tracking.
	replicatedSubscriptionsSnapshotFrequencyMillis: int

	// Timeout for building a consistent snapshot for tracking replicated subscriptions state.
	replicatedSubscriptionsSnapshotTimeoutSeconds: int

	// Max number of snapshot to be cached per subscription.
	replicatedSubscriptionsSnapshotMaxCachedPerSubscription: int

	// Max memory size for broker handling messages sending from producers.\n\n If the processing message size exceed this value, broker will stop read data from the connection. The processing messages means messages are sends to broker but broker have not send response to client, usually waiting to write to bookies.\n\n It's shared across all the topics running in the same broker.\n\n Use -1 to disable the memory limitation. Default is 1/2 of direct memory.\n\n
	maxMessagePublishBufferSizeInMB: int

	// Interval between checks to see if message publish buffer size is exceed the max message publish buffer size
	messagePublishBufferCheckIntervalInMillis: int

	// Whether to recover cursors lazily when trying to recover a managed ledger backing a persistent topic. It can improve write availability of topics.\nThe caveat is now when recovered ledger is ready to write we're not sure if all old consumers last mark delete position can be recovered or not.
	lazyCursorRecovery: bool

	// Check between intervals to see if consumed ledgers need to be trimmed
	retentionCheckIntervalInSeconds: int

	// The number of partitions per partitioned topic.\nIf try to create or update partitioned topics by exceeded number of partitions, then fail.
	maxNumPartitionsPerPartitionedTopic: int

	// The directory to locate broker interceptors
	brokerInterceptorsDirectory: string

	// List of broker interceptor to load, which is a list of broker interceptor names
	brokerInterceptors: string

	// Enable or disable the broker interceptor, which is only used for testing for now
	disableBrokerInterceptors: bool

	// List of interceptors for payload processing.
	brokerEntryPayloadProcessors: string

	// There are two policies to apply when broker metadata session expires: session expired happens, \"shutdown\" or \"reconnect\". \n\n With \"shutdown\", the broker will be restarted.\n\n With \"reconnect\", the broker will keep serving the topics, while attempting to recreate a new session.
	zookeeperSessionExpiredPolicy: string

	// If a topic remains fenced for this number of seconds, it will be closed forcefully.\n If it is set to 0 or a negative number, the fenced topic will not be closed.
	topicFencingTimeoutSeconds: int

	// The directory to locate messaging protocol handlers
	protocolHandlerDirectory: string

	// Use a separate ThreadPool for each Protocol Handler
	useSeparateThreadPoolForProtocolHandlers: bool

	// List of messaging protocols to load, which is a list of protocol names
	messagingProtocols: string

	// Enable or disable system topic.
	systemTopicEnabled: bool

	// # Enable strict topic name check. Which includes two parts as follows:\n# 1. Mark `-partition-` as a keyword.\n# E.g.\n    Create a non-partitioned topic.\n      No corresponding partitioned topic\n       - persistent://public/default/local-name (passed)\n       - persistent://public/default/local-name-partition-z (rejected by keyword)\n       - persistent://public/default/local-name-partition-0 (rejected by keyword)\n      Has corresponding partitioned topic, partitions=2 and topic partition name is persistent://public/default/local-name\n       - persistent://public/default/local-name-partition-0 (passed, Because it is the partition topic's sub-partition)\n       - persistent://public/default/local-name-partition-z (rejected by keyword)\n       - persistent://public/default/local-name-partition-4 (rejected, Because it exceeds the number of maximum partitions)\n    Create a partitioned topic(topic metadata)\n       - persistent://public/default/local-name (passed)\n       - persistent://public/default/local-name-partition-z (rejected by keyword)\n       - persistent://public/default/local-name-partition-0 (rejected by keyword)\n# 2. Allowed alphanumeric (a-zA-Z_0-9) and these special chars -=:. for topic name.\n# NOTE: This flag will be removed in some major releases in the future.\n
	strictTopicNameEnabled: bool

	// The schema compatibility strategy to use for system topics
	systemTopicSchemaCompatibilityStrategy: string

	// Enable or disable topic level policies, topic level policies depends on the system topic, please enable the system topic first.
	topicLevelPoliciesEnabled: bool

	// List of interceptors for entry metadata.
	brokerEntryMetadataInterceptors: string

	// Enable or disable exposing broker entry metadata to client.
	exposingBrokerEntryMetadataToClientEnabled: bool

	// Enable namespaceIsolation policy update take effect ontime or not, if set to ture, then the related namespaces will be unloaded after reset policy to make it take effect.
	enableNamespaceIsolationUpdateOnTime: bool

	// Enable or disable strict bookie affinity.
	strictBookieAffinityEnabled: bool

	// Enable TLS
	tlsEnabled: bool

	// Tls cert refresh duration in seconds (set 0 to check on every new connection)
	tlsCertRefreshCheckDurationSec: int

	// Path for the TLS certificate file
	tlsCertificateFilePath: string

	// Path for the TLS private key file
	tlsKeyFilePath: string

	// Path for the trusted TLS certificate file
	tlsTrustCertsFilePath: string

	// Accept untrusted TLS certificate from client
	tlsAllowInsecureConnection: bool

	// Whether the hostname is validated when the broker creates a TLS connection with other brokers
	tlsHostnameVerificationEnabled: bool

	// Specify the tls protocols the broker will use to negotiate during TLS Handshake.\n\nExample:- [TLSv1.3, TLSv1.2]
	tlsProtocols: string

	// Specify the tls cipher the broker will use to negotiate during TLS Handshake.\n\nExample:- [TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256]
	tlsCiphers: string

	// Specify whether Client certificates are required for TLS Reject.\nthe Connection if the Client Certificate is not trusted
	tlsRequireTrustedClientCertOnConnect: bool

	// Enable authentication
	authenticationEnabled: bool

	// Authentication provider name list, which is a list of class names
	authenticationProviders: string

	// Interval of time for checking for expired authentication credentials
	authenticationRefreshCheckSeconds: int

	// Enforce authorization
	authorizationEnabled: bool

	// Authorization provider fully qualified class-name
	authorizationProvider: string

	// Role names that are treated as `super-user`, meaning they will be able to do all admin operations and publish/consume from all topics
	superUserRoles: string

	// Role names that are treated as `proxy roles`. \n\nIf the broker sees a request with role as proxyRoles - it will demand to see the original client role or certificate.
	proxyRoles: string

	// If this flag is set then the broker authenticates the original Auth data else it just accepts the originalPrincipal and authorizes it (if required)
	authenticateOriginalAuthData: bool

	// Allow wildcard matching in authorization\n\n(wildcard matching only applicable if wildcard-char: * presents at first or last position eg: *.pulsar.service, pulsar.service.*)
	authorizationAllowWildcardsMatching: bool

	// When this parameter is not empty, unauthenticated users perform as anonymousUserRole
	anonymousUserRole: string

	// If >0, it will reject all HTTP requests with bodies larged than the configured limit
	httpMaxRequestSize: int

	// The maximum size in bytes of the request header.                Larger headers will allow for more and/or larger cookies plus larger form content encoded in a URL.                However, larger headers consume more memory and can make a server more vulnerable to denial of service                attacks.
	httpMaxRequestHeaderSize: int

	// If true, the broker will reject all HTTP requests using the TRACE and TRACK verbs.\n This setting may be necessary if the broker is deployed into an environment that uses http port\n scanning and flags web servers allowing the TRACE method as insecure.
	disableHttpDebugMethods: bool

	// Enable the enforcement of limits on the incoming HTTP requests
	httpRequestsLimitEnabled: bool

	// Max HTTP requests per seconds allowed. The excess of requests will be rejected with HTTP code 429 (Too many requests)
	httpRequestsMaxPerSecond: float

	// Admin API fail on unknown request parameter in request-body. see PIP-179. Default false.
	httpRequestsFailOnUnknownPropertiesEnabled: bool

	// This is a regexp, which limits the range of possible ids which can connect to the Broker using SASL.\n Default value is: \".*pulsar.*\", so only clients whose id contains 'pulsar' are allowed to connect.
	saslJaasClientAllowedIds: string

	// Service Principal, for login context name. Default value is \"PulsarBroker\".
	saslJaasServerSectionName: string

	// Path to file containing the secret to be used to SaslRoleTokenSigner\nThe secret can be specified like:\nsaslJaasServerRoleTokenSignerSecretPath=file:///my/saslRoleTokenSignerSecret.key.
	saslJaasServerRoleTokenSignerSecretPath: string

	// kerberos kinit command.
	kinitCommand: string

	// how often the broker expires the inflight SASL context.
	inflightSaslContextExpiryMs: int

	// Maximum number of inflight sasl context.
	maxInflightSaslContext: int

	// Metadata service uri that bookkeeper is used for loading corresponding metadata driver and resolving its metadata service location
	bookkeeperMetadataServiceUri: string

	// Authentication plugin to use when connecting to bookies
	bookkeeperClientAuthenticationPlugin: string

	// BookKeeper auth plugin implementation specifics parameters name and values
	bookkeeperClientAuthenticationParametersName: string

	// Parameters for bookkeeper auth plugin
	bookkeeperClientAuthenticationParameters: string

	// Timeout for BK add / read operations
	bookkeeperClientTimeoutInSeconds: int

	// Speculative reads are initiated if a read request doesn't complete within a certain time Using a value of 0, is disabling the speculative reads
	bookkeeperClientSpeculativeReadTimeoutInMillis: int

	// Number of channels per bookie
	bookkeeperNumberOfChannelsPerBookie: int

	// Use older Bookkeeper wire protocol with bookie
	bookkeeperUseV2WireProtocol: bool

	// Enable bookies health check. \n\n Bookies that have more than the configured number of failure within the interval will be quarantined for some time. During this period, new ledgers won't be created on these bookies
	bookkeeperClientHealthCheckEnabled: bool

	// Bookies health check interval in seconds
	bookkeeperClientHealthCheckIntervalSeconds: int

	// Bookies health check error threshold per check interval
	bookkeeperClientHealthCheckErrorThresholdPerInterval: int

	// Bookie health check quarantined time in seconds
	bookkeeperClientHealthCheckQuarantineTimeInSeconds: int

	// bookie quarantine ratio to avoid all clients quarantine the high pressure bookie servers at the same time
	bookkeeperClientQuarantineRatio: float

	// Enable rack-aware bookie selection policy. \n\nBK will chose bookies from different racks when forming a new bookie ensemble
	bookkeeperClientRackawarePolicyEnabled: bool

	// Enable region-aware bookie selection policy. \n\nBK will chose bookies from different regions and racks when forming a new bookie ensemble
	bookkeeperClientRegionawarePolicyEnabled: bool

	// Minimum number of racks per write quorum. \n\nBK rack-aware bookie selection policy will try to get bookies from at least 'bookkeeperClientMinNumRacksPerWriteQuorum' racks for a write quorum.
	bookkeeperClientMinNumRacksPerWriteQuorum: int

	// Enforces rack-aware bookie selection policy to pick bookies from 'bookkeeperClientMinNumRacksPerWriteQuorum' racks for  a writeQuorum. \n\nIf BK can't find bookie then it would throw BKNotEnoughBookiesException instead of picking random one.
	bookkeeperClientEnforceMinNumRacksPerWriteQuorum: bool

	// Enable/disable reordering read sequence on reading entries
	bookkeeperClientReorderReadSequenceEnabled: bool

	// Enable bookie isolation by specifying a list of bookie groups to choose from. \n\nAny bookie outside the specified groups will not be used by the broker
	bookkeeperClientIsolationGroups: string

	// Enable bookie secondary-isolation group if bookkeeperClientIsolationGroups doesn't have enough bookie available.
	bookkeeperClientSecondaryIsolationGroups: string

	// Set the interval to periodically check bookie info
	bookkeeperClientGetBookieInfoIntervalSeconds: int

	// Set the interval to retry a failed bookie info lookup
	bookkeeperClientGetBookieInfoRetryIntervalSeconds: int

	// Enable/disable having read operations for a ledger to be sticky to a single bookie.\nIf this flag is enabled, the client will use one single bookie (by preference) to read all entries for a ledger.
	bookkeeperEnableStickyReads: bool

	// Set the client security provider factory class name. Default: org.apache.bookkeeper.tls.TLSContextFactory
	bookkeeperTLSProviderFactoryClass: string

	// Enable tls authentication with bookie
	bookkeeperTLSClientAuthentication: bool

	// Supported type: PEM, JKS, PKCS12. Default value: PEM
	bookkeeperTLSKeyFileType: string

	// Supported type: PEM, JKS, PKCS12. Default value: PEM
	bookkeeperTLSTrustCertTypes: string

	// Path to file containing keystore password, if the client keystore is password protected.
	bookkeeperTLSKeyStorePasswordPath: string

	// Path to file containing truststore password, if the client truststore is password protected.
	bookkeeperTLSTrustStorePasswordPath: string

	// Path for the TLS private key file
	bookkeeperTLSKeyFilePath: string

	// Path for the TLS certificate file
	bookkeeperTLSCertificateFilePath: string

	// Path for the trusted TLS certificate file
	bookkeeperTLSTrustCertsFilePath: string

	// Tls cert refresh duration at bookKeeper-client in seconds (0 to disable check)
	bookkeeperTlsCertFilesRefreshDurationSeconds: int

	// Enable/disable disk weight based placement. Default is false
	bookkeeperDiskWeightBasedPlacementEnabled: bool

	// Set the interval to check the need for sending an explicit LAC
	bookkeeperExplicitLacIntervalInMills: int

	// whether expose managed ledger client stats to prometheus
	bookkeeperClientExposeStatsToPrometheus: bool

	// whether limit per_channel_bookie_client metrics of bookkeeper client stats
	bookkeeperClientLimitStatsLogging: bool

	// Throttle value for bookkeeper client
	bookkeeperClientThrottleValue: int

	// Number of BookKeeper client worker threads. Default is Runtime.getRuntime().availableProcessors()
	bookkeeperClientNumWorkerThreads: int

	// Number of BookKeeper client IO threads. Default is Runtime.getRuntime().availableProcessors() * 2
	bookkeeperClientNumIoThreads: int

	// Use separated IO threads for BookKeeper client. Default is false, which will use Pulsar IO threads
	bookkeeperClientSeparatedIoThreadsEnabled: bool

	// Ensemble (E) size, Number of bookies to use for storing entries in a ledger.\nPlease notice that sticky reads enabled by bookkeeperEnableStickyReads=true aren’t used  unless ensemble size (E) equals write quorum (Qw) size.
	managedLedgerDefaultEnsembleSize: int

	// Write quorum (Qw) size, Replication factor for storing entries (messages) in a ledger.
	managedLedgerDefaultWriteQuorum: int

	// Ack quorum (Qa) size, Number of guaranteed copies (acks to wait for before a write is considered completed)
	managedLedgerDefaultAckQuorum: int

	// How frequently to flush the cursor positions that were accumulated due to rate limiting. (seconds). Default is 60 seconds
	managedLedgerCursorPositionFlushSeconds: int

	// How frequently to refresh the stats. (seconds). Default is 60 seconds
	managedLedgerStatsPeriodSeconds: int

	// Default type of checksum to use when writing to BookKeeper. \n\nDefault is `CRC32C`. Other possible options are `CRC32`, `MAC` or `DUMMY` (no checksum).
	managedLedgerDigestType: string

	// Default  password to use when writing to BookKeeper. \n\nDefault is ``.
	managedLedgerPassword: string

	// Max number of bookies to use when creating a ledger
	managedLedgerMaxEnsembleSize: int

	// Max number of copies to store for each message
	managedLedgerMaxWriteQuorum: int

	// Max number of guaranteed copies (acks to wait before write is complete)
	managedLedgerMaxAckQuorum: int

	// Amount of memory to use for caching data payload in managed ledger. \n\nThis memory is allocated from JVM direct memory and it's shared across all the topics running in the same broker. By default, uses 1/5th of available direct memory
	managedLedgerCacheSizeMB: int

	// Whether we should make a copy of the entry payloads when inserting in cache
	managedLedgerCacheCopyEntries: bool

	// Maximum buffer size for bytes read from storage. This is the memory retained by data read from storage (or cache) until it has been delivered to the Consumer Netty channel. Use O to disable
	managedLedgerMaxReadsInFlightSizeInMB: int

	// Threshold to which bring down the cache level when eviction is triggered
	managedLedgerCacheEvictionWatermark: float

	// Configure the cache eviction frequency for the managed ledger cache.
	managedLedgerCacheEvictionFrequency: float

	// Configure the cache eviction interval in milliseconds for the managed ledger cache, default is 10ms
	managedLedgerCacheEvictionIntervalMs: int

	// All entries that have stayed in cache for more than the configured time, will be evicted
	managedLedgerCacheEvictionTimeThresholdMillis: int

	// Configure the threshold (in number of entries) from where a cursor should be considered 'backlogged' and thus should be set as inactive.
	managedLedgerCursorBackloggedThreshold: int

	// Rate limit the amount of writes per second generated by consumer acking the messages
	managedLedgerDefaultMarkDeleteRateLimit: float

	// Allow automated creation of topics if set to true (default value).
	allowAutoTopicCreation: bool

	// The type of topic that is allowed to be automatically created.(partitioned/non-partitioned)
	allowAutoTopicCreationType: string

	// Allow automated creation of subscriptions if set to true (default value).
	allowAutoSubscriptionCreation: bool

	// The number of partitioned topics that is allowed to be automatically created if allowAutoTopicCreationType is partitioned.
	defaultNumPartitions: int

	// The class of the managed ledger storage
	managedLedgerStorageClassName: string

	// Number of threads to be used for managed ledger scheduled tasks
	managedLedgerNumSchedulerThreads: int

	// Max number of entries to append to a ledger before triggering a rollover.\n\nA ledger rollover is triggered after the min rollover time has passed and one of the following conditions is true: the max rollover time has been reached, the max entries have been written to the ledger, or the max ledger size has been written to the ledger
	managedLedgerMaxEntriesPerLedger: int

	// Minimum time between ledger rollover for a topic
	managedLedgerMinLedgerRolloverTimeMinutes: int

	// Maximum time before forcing a ledger rollover for a topic
	managedLedgerMaxLedgerRolloverTimeMinutes: int

	// Maximum ledger size before triggering a rollover for a topic (MB)
	managedLedgerMaxSizePerLedgerMbytes: int

	// Delay between a ledger being successfully offloaded to long term storage, and the ledger being deleted from bookkeeper
	managedLedgerOffloadDeletionLagMs: int

	// The number of bytes before triggering automatic offload to long term storage
	managedLedgerOffloadAutoTriggerSizeThresholdBytes: int

	// The threshold to triggering automatic offload to long term storage
	managedLedgerOffloadThresholdInSeconds: int

	// Max number of entries to append to a cursor ledger
	managedLedgerCursorMaxEntriesPerLedger: int

	// Max time before triggering a rollover on a cursor ledger
	managedLedgerCursorRolloverTimeInSeconds: int

	// Max number of `acknowledgment holes` that are going to be persistently stored.\n\nWhen acknowledging out of order, a consumer will leave holes that are supposed to be quickly filled by acking all the messages. The information of which messages are acknowledged is persisted by compressing in `ranges` of messages that were acknowledged. After the max number of ranges is reached, the information will only be tracked in memory and messages will be redelivered in case of crashes.
	managedLedgerMaxUnackedRangesToPersist: int

	// If enabled, the maximum \"acknowledgment holes\" will not be limited and \"acknowledgment holes\" are stored in multiple entries.
	persistentUnackedRangesWithMultipleEntriesEnabled: bool

	// Max number of `acknowledgment holes` that can be stored in Zookeeper.\n\nIf number of unack message range is higher than this limit then broker will persist unacked ranges into bookkeeper to avoid additional data overhead into zookeeper.\n@deprecated - use managedLedgerMaxUnackedRangesToPersistInMetadataStore.
	// @deprecated
	managedLedgerMaxUnackedRangesToPersistInZooKeeper: int

	// Max number of `acknowledgment holes` that can be stored in MetadataStore.\n\nIf number of unack message range is higher than this limit then broker will persist unacked ranges into bookkeeper to avoid additional data overhead into MetadataStore.
	managedLedgerMaxUnackedRangesToPersistInMetadataStore: int

	// Use Open Range-Set to cache unacked messages (it is memory efficient but it can take more cpu)
	managedLedgerUnackedRangesOpenCacheSetEnabled: bool

	// Skip reading non-recoverable/unreadable data-ledger under managed-ledger's list.\n\n It helps when data-ledgers gets corrupted at bookkeeper and managed-cursor is stuck at that ledger.
	autoSkipNonRecoverableData: bool

	// operation timeout while updating managed-ledger metadata.
	managedLedgerMetadataOperationsTimeoutSeconds: int

	// Read entries timeout when broker tries to read messages from bookkeeper (0 to disable it)
	managedLedgerReadEntryTimeoutSeconds: int

	// Add entry timeout when broker tries to publish message to bookkeeper.(0 to disable it)
	managedLedgerAddEntryTimeoutSeconds: int

	// Managed ledger prometheus stats latency rollover seconds
	managedLedgerPrometheusStatsLatencyRolloverSeconds: int

	// Whether trace managed ledger task execution time
	managedLedgerTraceTaskExecution: bool

	// New entries check delay for the cursor under the managed ledger. \nIf no new messages in the topic, the cursor will try to check again after the delay time. \nFor consumption latency sensitive scenario, can set to a smaller value or set to 0.\nOf course, this may degrade consumption throughput. Default is 10ms.
	managedLedgerNewEntriesCheckDelayInMillis: int

	// Read priority when ledgers exists in both bookkeeper and the second layer storage.
	managedLedgerDataReadPriority: string

	// ManagedLedgerInfo compression type, option values (NONE, LZ4, ZLIB, ZSTD, SNAPPY). \nIf value is invalid or NONE, then save the ManagedLedgerInfo bytes data directly.
	managedLedgerInfoCompressionType: string

	// ManagedLedgerInfo compression size threshold (bytes), only compress metadata when origin size more then this value.\n0 means compression will always apply.\n
	managedLedgerInfoCompressionThresholdInBytes: int

	// ManagedCursorInfo compression type, option values (NONE, LZ4, ZLIB, ZSTD, SNAPPY). \nIf value is NONE, then save the ManagedCursorInfo bytes data directly.
	managedCursorInfoCompressionType: string

	// ManagedCursorInfo compression size threshold (bytes), only compress metadata when origin size more then this value.\n0 means compression will always apply.\n
	managedCursorInfoCompressionThresholdInBytes: int

	// Minimum cursors that must be in backlog state to cache and reuse the read entries.(Default =0 to disable backlog reach cache)
	managedLedgerMinimumBacklogCursorsForCaching: int

	// Minimum backlog entries for any cursor before start caching reads
	managedLedgerMinimumBacklogEntriesForCaching: int

	// Maximum backlog entry difference to prevent caching entries that can't be reused
	managedLedgerMaxBacklogBetweenCursorsForCaching: int

	// Enable load balancer
	loadBalancerEnabled: bool

	// load placement strategy[weightedRandomSelection/leastLoadedServer] (only used by SimpleLoadManagerImpl)
	// @deprecated
	loadBalancerPlacementStrategy: string

	// load balance load shedding strategy (It requires broker restart if value is changed using dynamic config). Default is ThresholdShedder since 2.10.0
	loadBalancerLoadSheddingStrategy: string

	// When [current usage < average usage - threshold], the broker with the highest load will be triggered to unload
	lowerBoundarySheddingEnabled: bool

	// load balance placement strategy
	loadBalancerLoadPlacementStrategy: string

	// Percentage of change to trigger load report update
	loadBalancerReportUpdateThresholdPercentage: int

	// maximum interval to update load report
	loadBalancerReportUpdateMinIntervalMillis: int

	// Min delay of load report to collect, in milli-seconds
	loadBalancerReportUpdateMaxIntervalMinutes: int

	// Frequency of report to collect, in minutes
	loadBalancerHostUsageCheckIntervalMinutes: int

	// Enable/disable automatic bundle unloading for load-shedding
	loadBalancerSheddingEnabled: bool

	// Load shedding interval. \n\nBroker periodically checks whether some traffic should be offload from some over-loaded broker to other under-loaded brokers
	loadBalancerSheddingIntervalMinutes: int

	// enable/disable distribute bundles evenly
	loadBalancerDistributeBundlesEvenlyEnabled: bool

	// Prevent the same topics to be shed and moved to other broker more than once within this timeframe
	loadBalancerSheddingGracePeriodMinutes: int

	// Usage threshold to determine a broker as under-loaded (only used by SimpleLoadManagerImpl)
	// @deprecated
	loadBalancerBrokerUnderloadedThresholdPercentage: int

	// Usage threshold to allocate max number of topics to broker
	loadBalancerBrokerMaxTopics: int

	// Usage threshold to determine a broker as over-loaded
	loadBalancerBrokerOverloadedThresholdPercentage: int

	// Usage threshold to determine a broker whether to start threshold shedder
	loadBalancerBrokerThresholdShedderPercentage: int

	// Average resource usage difference threshold to determine a broker whether to be a best candidate in LeastResourceUsageWithWeight.(eg: broker1 with 10% resource usage with weight and broker2 with 30% and broker3 with 80% will have 40% average resource usage. The placement strategy can select broker1 and broker2 as best candidates.)
	loadBalancerAverageResourceUsageDifferenceThresholdPercentage: int

	// In FlowOrQpsEquallyDivideBundleSplitAlgorithm, if msgRate >= loadBalancerNamespaceBundleMaxMsgRate *  (100 + flowOrQpsDifferenceThresholdPercentage)/100.0  or throughput >=  loadBalancerNamespaceBundleMaxBandwidthMbytes *  (100 + flowOrQpsDifferenceThresholdPercentage)/100.0,  execute split bundle
	flowOrQpsDifferenceThresholdPercentage: int

	// In the UniformLoadShedder strategy, the minimum message that triggers unload.
	minUnloadMessage: int

	// In the UniformLoadShedder strategy, the minimum throughput that triggers unload.
	minUnloadMessageThroughput: int

	// In the UniformLoadShedder strategy, the maximum unload ratio.
	maxUnloadPercentage: float

	// Message-rate percentage threshold between highest and least loaded brokers for uniform load shedding. (eg: broker1 with 50K msgRate and broker2 with 30K msgRate will have 66% msgRate difference and load balancer can unload bundles from broker-1 to broker-2)
	loadBalancerMsgRateDifferenceShedderThreshold: float

	// Message-throughput threshold between highest and least loaded brokers for uniform load shedding. (eg: broker1 with 450MB msgRate and broker2 with 100MB msgRate will have 4.5 times msgThroughout difference and load balancer can unload bundles from broker-1 to broker-2)
	loadBalancerMsgThroughputMultiplierDifferenceShedderThreshold: float

	// For each uniform balanced unload, the maximum number of bundles that can be unloaded. The default value is -1, which means no limit
	maxUnloadBundleNumPerShedding: int

	// Resource history Usage Percentage When adding new resource usage info
	loadBalancerHistoryResourcePercentage: float

	// BandwithIn Resource Usage Weight
	loadBalancerBandwithInResourceWeight: float

	// BandwithOut Resource Usage Weight
	loadBalancerBandwithOutResourceWeight: float

	// CPU Resource Usage Weight
	loadBalancerCPUResourceWeight: float

	// Memory Resource Usage Weight. Deprecated: Memory is no longer used as a load balancing item.
	// @deprecated
	loadBalancerMemoryResourceWeight: float

	// Direct Memory Resource Usage Weight
	loadBalancerDirectMemoryResourceWeight: float

	// Bundle unload minimum throughput threshold (MB)
	loadBalancerBundleUnloadMinThroughputThreshold: float

	// Interval to flush dynamic resource quota to ZooKeeper
	loadBalancerResourceQuotaUpdateIntervalMinutes: int

	// Usage threshold to determine a broker is having just right level of load (only used by SimpleLoadManagerImpl)
	// @deprecated
	loadBalancerBrokerComfortLoadLevelPercentage: int

	// enable/disable automatic namespace bundle split
	loadBalancerAutoBundleSplitEnabled: bool

	// enable/disable automatic unloading of split bundles
	loadBalancerAutoUnloadSplitBundlesEnabled: bool

	// maximum topics in a bundle, otherwise bundle split will be triggered
	loadBalancerNamespaceBundleMaxTopics: int

	// maximum sessions (producers + consumers) in a bundle, otherwise bundle split will be triggered(disable threshold check with value -1)
	loadBalancerNamespaceBundleMaxSessions: int

	// maximum msgRate (in + out) in a bundle, otherwise bundle split will be triggered
	loadBalancerNamespaceBundleMaxMsgRate: int

	// maximum bandwidth (in + out) in a bundle, otherwise bundle split will be triggered
	loadBalancerNamespaceBundleMaxBandwidthMbytes: int

	// maximum number of bundles in a namespace
	loadBalancerNamespaceMaximumBundles: int

	// Name of load manager to use
	loadManagerClassName: string

	// Name of topic bundle assignment strategy to use
	topicBundleAssignmentStrategy: string

	// Supported algorithms name for namespace bundle split
	supportedNamespaceBundleSplitAlgorithms: string

	// Default algorithm name for namespace bundle split
	defaultNamespaceBundleSplitAlgorithm: string

	// Option to override the auto-detected network interfaces max speed
	loadBalancerOverrideBrokerNicSpeedGbps: float

	// Time to wait for the unloading of a namespace bundle
	namespaceBundleUnloadingTimeoutMs: int

	// Option to enable the debug mode for the load balancer logics. The debug mode prints more logs to provide more information such as load balance states and decisions. (only used in load balancer extension logics)
	loadBalancerDebugModeEnabled: bool

	// The target standard deviation of the resource usage across brokers (100% resource usage is 1.0 load). The shedder logic tries to distribute bundle load across brokers to meet this target std. The smaller value will incur load balancing more frequently. (only used in load balancer extension TransferSheddeer)
	loadBalancerBrokerLoadTargetStd: float

	// Threshold to the consecutive count of fulfilled shedding(unload) conditions. If the unload scheduler consecutively finds bundles that meet unload conditions many times bigger than this threshold, the scheduler will shed the bundles. The bigger value will incur less bundle unloading/transfers. (only used in load balancer extension TransferSheddeer)
	loadBalancerSheddingConditionHitCountThreshold: int

	// Option to enable the bundle transfer mode when distributing bundle loads. On: transfer bundles from overloaded brokers to underloaded -- pre-assigns the destination broker upon unloading). Off: unload bundles from overloaded brokers -- post-assigns the destination broker upon lookups). (only used in load balancer extension TransferSheddeer)
	loadBalancerTransferEnabled: bool

	// Maximum number of brokers to unload bundle load for each unloading cycle. The bigger value will incur more unloading/transfers for each unloading cycle. (only used in load balancer extension TransferSheddeer)
	loadBalancerMaxNumberOfBrokerSheddingPerCycle: int

	// Delay (in seconds) to the next unloading cycle after unloading. The logic tries to give enough time for brokers to recompute load after unloading. The bigger value will delay the next unloading cycle longer. (only used in load balancer extension TransferSheddeer)
	loadBalanceSheddingDelayInSeconds: int

	// Broker load data time to live (TTL in seconds). The logic tries to avoid (possibly unavailable) brokers with out-dated load data, and those brokers will be ignored in the load computation. When tuning this value, please consider loadBalancerReportUpdateMaxIntervalMinutes. The current default is loadBalancerReportUpdateMaxIntervalMinutes * 2. (only used in load balancer extension TransferSheddeer)
	loadBalancerBrokerLoadDataTTLInSeconds: int

	// Max number of bundles in bundle load report from each broker. The load balancer distributes bundles across brokers, based on topK bundle load data and other broker load data.The bigger value will increase the overhead of reporting many bundles in load data. (only used in load balancer extension logics)
	loadBalancerMaxNumberOfBundlesInBundleLoadReport: int

	// Service units'(bundles) split interval. Broker periodically checks whether some service units(e.g. bundles) should split if they become hot-spots. (only used in load balancer extension logics)
	loadBalancerSplitIntervalMinutes: int

	// Max number of bundles to split to per cycle. (only used in load balancer extension logics)
	loadBalancerMaxNumberOfBundlesToSplitPerCycle: int

	// Threshold to the consecutive count of fulfilled split conditions. If the split scheduler consecutively finds bundles that meet split conditions many times bigger than this threshold, the scheduler will trigger splits on the bundles (if the number of bundles is less than loadBalancerNamespaceMaximumBundles). (only used in load balancer extension logics)
	loadBalancerNamespaceBundleSplitConditionHitCountThreshold: int

	// After this delay, the service-unit state channel tombstones any service units (e.g., bundles) in semi-terminal states. For example, after splits, parent bundles will be `deleted`, and then after this delay, the parent bundles' state will be `tombstoned` in the service-unit state channel. Pulsar does not immediately remove such semi-terminal states to avoid unnecessary system confusion, as the bundles in the `tombstoned` state might temporarily look available to reassign. Rarely, one could lower this delay in order to aggressively clean the service-unit state channel when there are a large number of bundles. minimum value = 30 secs(only used in load balancer extension logics)
	loadBalancerServiceUnitStateTombstoneDelayTimeInSeconds: int

	// Option to automatically unload namespace bundles with affinity(isolation) or anti-affinity group policies.Such bundles are not ideal targets to auto-unload as destination brokers are limited.(only used in load balancer extension logics)
	loadBalancerSheddingBundlesWithPoliciesEnabled: bool

	// Enable replication metrics
	replicationMetricsEnabled: bool

	// Max number of connections to open for each broker in a remote cluster.\n\nMore connections host-to-host lead to better throughput over high-latency links
	replicationConnectionsPerBroker: int

	// replicator prefix used for replicator producer name and cursor name
	replicatorPrefix: string

	// Replicator producer queue size. When dynamically modified, it only takes effect for the newly added replicators
	replicationProducerQueueSize: int

	// Duration to check replication policy to avoid replicator inconsistency due to missing ZooKeeper watch (disable with value 0)
	replicationPolicyCheckDurationSeconds: int

	// @deprecated - Use brokerClientTlsEnabled instead.
	// @deprecated
	replicationTlsEnabled: bool

	// Default message retention time. 0 means retention is disabled. -1 means data is not removed by time quota
	defaultRetentionTimeInMinutes: int

	// Default retention size. 0 means retention is disabled. -1 means data is not removed by size quota
	defaultRetentionSizeInMB: int

	// How often to check pulsar connection is still alive
	keepAliveIntervalSeconds: int

	// Timeout for connection liveness check used to check liveness of possible consumer or producer duplicates. Helps prevent ProducerFencedException with exclusive producer, ConsumerAssignException with range conflict for Key Shared with sticky hash ranges or ConsumerBusyException in the case of an exclusive consumer. Set to 0 to disable connection liveness check.
	connectionLivenessCheckTimeoutMillis: int

	// How often broker checks for inactive topics to be deleted (topics with no subscriptions and no one connected) Deprecated in favor of using `brokerDeleteInactiveTopicsFrequencySeconds`\n@deprecated - unused.
	// @deprecated
	brokerServicePurgeInactiveFrequencyInSeconds: int

	// A comma-separated list of namespaces to bootstrap
	bootstrapNamespaces: string

	// If true, (and ModularLoadManagerImpl is being used), the load manager will attempt to use only brokers running the latest software version (to minimize impact to bundles)
	preferLaterVersions: bool

	// Interval between checks to see if topics with compaction policies need to be compacted
	brokerServiceCompactionMonitorIntervalInSeconds: int

	// The estimated backlog size is greater than this threshold, compression will be triggered.\nUsing a value of 0, is disabling compression check.
	brokerServiceCompactionThresholdInBytes: int

	// Timeout for the compaction phase one loop, If the execution time of the compaction phase one loop exceeds this time, the compaction will not proceed.
	brokerServiceCompactionPhaseOneLoopTimeInSeconds: int

	// Interval between checks to see if cluster is migrated and marks topic migrated  if cluster is marked migrated. Disable with value 0. (Default disabled).
	clusterMigrationCheckDurationSeconds: int

	// Enforce schema validation on following cases:\n\n - if a producer without a schema attempts to produce to a topic with schema, the producer will be\n   failed to connect. PLEASE be carefully on using this, since non-java clients don't support schema.\n   if you enable this setting, it will cause non-java clients failed to produce.
	isSchemaValidationEnforced: bool

	// The schema storage implementation used by this broker
	schemaRegistryStorageClassName: string

	// The list compatibility checkers to be used in schema registry
	schemaRegistryCompatibilityCheckers: string

	// The schema compatibility strategy in broker level
	schemaCompatibilityStrategy: string

	// Number of IO threads in Pulsar Client used in WebSocket proxy
	webSocketNumIoThreads: int

	// Number of threads used by Websocket service
	webSocketNumServiceThreads: int

	// Number of connections per Broker in Pulsar Client used in WebSocket proxy
	webSocketConnectionsPerBroker: int

	// Time in milliseconds that idle WebSocket session times out
	webSocketSessionIdleTimeoutMillis: int

	// Interval of time to sending the ping to keep alive in WebSocket proxy. This value greater than 0 means enabled
	webSocketPingDurationSeconds: int

	// The maximum size of a text message during parsing in WebSocket proxy.
	webSocketMaxTextFrameSize: int

	// Whether the '/metrics' endpoint requires authentication. Defaults to false.'authenticationEnabled' must also be set for this to take effect.
	authenticateMetricsEndpoint: bool

	// If true, export topic level metrics otherwise namespace level
	exposeTopicLevelMetricsInPrometheus: bool

	// If true, export buffered metrics
	metricsBufferResponse: bool

	// If true, export consumer level metrics otherwise namespace level
	exposeConsumerLevelMetricsInPrometheus: bool

	// If true, export producer level metrics otherwise namespace level
	exposeProducerLevelMetricsInPrometheus: bool

	// If true, export managed ledger metrics (aggregated by namespace)
	exposeManagedLedgerMetricsInPrometheus: bool

	// If true, export managed cursor metrics
	exposeManagedCursorMetricsInPrometheus: bool

	// Classname of Pluggable JVM GC metrics logger that can log GC specific metrics
	jvmGCMetricsLoggerClassName: string

	// Enable expose the precise backlog stats.\n Set false to use published counter and consumed counter to calculate,\n this would be more efficient but may be inaccurate. Default is false.
	exposePreciseBacklogInPrometheus: bool

	// Time in milliseconds that metrics endpoint would time out. Default is 30s.\n Increase it if there are a lot of topics to expose topic-level metrics.\n Set it to 0 to disable timeout.
	metricsServletTimeoutMs: int

	// Enable expose the backlog size for each subscription when generating stats.\n Locking is used for fetching the status so default to false.
	exposeSubscriptionBacklogSizeInPrometheus: bool

	// Enable splitting topic and partition label in Prometheus.\n If enabled, a topic name will split into 2 parts, one is topic name without partition index,\n another one is partition index, e.g. (topic=xxx, partition=0).\n If the topic is a non-partitioned topic, -1 will be used for the partition index.\n If disabled, one label to represent the topic and partition, e.g. (topic=xxx-partition-0)\n Default is false.
	splitTopicAndPartitionLabelInPrometheus: bool

	// Enable expose the broker bundles metrics.
	exposeBundlesMetricsInPrometheus: bool

	// Flag indicates enabling or disabling function worker on brokers
	functionsWorkerEnabled: bool

	// The nar package for the function worker service
	functionsWorkerServiceNarPackage: string

	// Flag indicates enabling or disabling function worker using unified PackageManagement service.
	functionsWorkerEnablePackageManagement: bool

	// If true, export publisher stats when returning topics stats from the admin rest api
	exposePublisherStats: bool

	// Stats update frequency in seconds
	statsUpdateFrequencyInSecs: int

	// Stats update initial delay in seconds
	statsUpdateInitialDelayInSecs: int

	// If true, aggregate publisher stats of PartitionedTopicStats by producerName
	aggregatePublisherStatsByProducerName: bool

	// The directory to locate offloaders
	offloadersDirectory: string

	// Driver to use to offload old data to long term storage
	managedLedgerOffloadDriver: string

	// Maximum number of thread pool threads for ledger offloading
	managedLedgerOffloadMaxThreads: int

	// The directory where nar Extraction of offloaders happens
	narExtractionDirectory: string

	// Maximum prefetch rounds for ledger reading for offloading
	managedLedgerOffloadPrefetchRounds: int

	// Time to rollover ledger for inactive topic (duration without any publish on that topic). Disable rollover with value 0 (Default value 0)
	managedLedgerInactiveLedgerRolloverTimeSeconds: int

	// Evicting cache data by the slowest markDeletedPosition or readPosition. The default is to evict through readPosition.
	cacheEvictionByMarkDeletedPosition: bool

	// Enable transaction coordinator in broker
	transactionCoordinatorEnabled: bool

	// Class name for transaction metadata store provider
	transactionMetadataStoreProviderClassName: string

	// Class name for transaction buffer provider
	transactionBufferProviderClassName: string

	// Class name for transaction pending ack store provider
	transactionPendingAckStoreProviderClassName: string

	// Number of threads to use for pulsar transaction replay PendingAckStore or TransactionBuffer.Default is 5
	numTransactionReplayThreadPoolSize: int

	// Transaction buffer take snapshot transaction countIf transaction buffer enables snapshot segment, transaction buffer updates snapshot metadataafter the number of transaction operations reaches this value.
	transactionBufferSnapshotMaxTransactionCount: int

	// The interval time for transaction buffer to take snapshots.If transaction buffer enables snapshot segment, it is the interval time for transaction buffer to update snapshot metadata.
	transactionBufferSnapshotMinTimeInMillis: int

	// Transaction buffer stores the transaction ID of aborted transactions and takes snapshots.This configuration determines the size of the snapshot segment. The default value is 256 KB (262144 bytes).
	transactionBufferSnapshotSegmentSize: int

	// Whether to enable segmented transaction buffer snapshot to handle a large number of aborted transactions.
	transactionBufferSegmentedSnapshotEnabled: bool

	// The max concurrent requests for transaction buffer client.
	transactionBufferClientMaxConcurrentRequests: int

	// The transaction buffer client's operation timeout in milliseconds.
	transactionBufferClientOperationTimeoutInMills: int

	// The max active transactions per transaction coordinator, default value 0 indicates no limit.
	maxActiveTransactionsPerCoordinator: int

	// MLPendingAckStore maintain a ConcurrentSkipListMap pendingAckLogIndex`,it store the position in pendingAckStore as value and save a position used to determinewhether the previous data can be cleaned up as a key.transactionPendingAckLogIndexMinLag is used to configure the minimum lag between indexes
	transactionPendingAckLogIndexMinLag: int

	// Provide a mechanism allowing the Transaction Log Store to aggregate multiple records into a batched record and persist into a single BK entry. This will make Pulsar transactions work more efficiently, aka batched log. see: https://github.com/apache/pulsar/issues/15370. Default false
	transactionLogBatchedWriteEnabled: bool

	// If enabled the feature that transaction log batch, this attribute means maximum log records count in a batch, default 512.
	transactionLogBatchedWriteMaxRecords: int

	// If enabled the feature that transaction log batch, this attribute means bytes size in a batch, default 4m.
	transactionLogBatchedWriteMaxSize: int

	// If enabled the feature that transaction log batch, this attribute means maximum wait time(in millis) for the first record in a batch, default 1 millisecond.
	transactionLogBatchedWriteMaxDelayInMillis: int

	// Provide a mechanism allowing the transaction pending ack Log Store to aggregate multiple records into a batched record and persist into a single BK entry. This will make Pulsar transactions work more efficiently, aka batched log. see: https://github.com/apache/pulsar/issues/15370. Default false.
	transactionPendingAckBatchedWriteEnabled: bool

	// If enabled the feature that transaction log batch, this attribute means maximum log records count in a batch, default 512.
	transactionPendingAckBatchedWriteMaxRecords: int

	// If enabled the feature that transaction pending ack log batch, this attribute means bytes size in a batch, default 4m.
	transactionPendingAckBatchedWriteMaxSize: int

	// If enabled the feature that transaction pending ack log batch, this attribute means maximum wait time(in millis) for the first record in a batch, default 1 millisecond.
	transactionPendingAckBatchedWriteMaxDelayInMillis: int

	// The class name of the factory that implements the topic compaction service.
	compactionServiceFactoryClassName: string

	// Enable TLS with KeyStore type configuration in broker
	tlsEnabledWithKeyStore: bool

	// Specify the TLS provider for the broker service: \nWhen using TLS authentication with CACert, the valid value is either OPENSSL or JDK.\nWhen using TLS authentication with KeyStore, available values can be SunJSSE, Conscrypt and etc.
	tlsProvider: string

	// TLS KeyStore type configuration in broker: JKS, PKCS12
	tlsKeyStoreType: string

	// TLS KeyStore path in broker
	tlsKeyStore: string

	// TLS KeyStore password for broker
	tlsKeyStorePassword: string

	// TLS TrustStore type configuration in broker: JKS, PKCS12
	tlsTrustStoreType: string

	// TLS TrustStore path in broker
	tlsTrustStore: string

	// TLS TrustStore password for broker, null means empty password.
	tlsTrustStorePassword: string

	// Authentication settings of the broker itself. \n\nUsed when the broker connects to other brokers, either in same or other clusters. Default uses plugin which disables authentication
	brokerClientAuthenticationPlugin: string

	// Authentication parameters of the authentication plugin the broker is using to connect to other brokers
	brokerClientAuthenticationParameters: string

	// Enable TLS when talking with other brokers in the same cluster (admin operation) or different clusters (replication)
	brokerClientTlsEnabled: bool

	// Whether internal client use KeyStore type to authenticate with other Pulsar brokers
	brokerClientTlsEnabledWithKeyStore: bool

	// The TLS Provider used by internal client to authenticate with other Pulsar brokers
	brokerClientSslProvider: string

	// TLS trusted certificate file for internal client, used by the internal client to authenticate with Pulsar brokers
	brokerClientTrustCertsFilePath: string

	// TLS private key file for internal client, used by the internal client to authenticate with Pulsar brokers
	brokerClientKeyFilePath: string

	// TLS certificate file for internal client, used by the internal client to authenticate with Pulsar brokers
	brokerClientCertificateFilePath: string

	// TLS TrustStore type configuration for internal client: JKS, PKCS12  used by the internal client to authenticate with Pulsar brokers
	brokerClientTlsTrustStoreType: string

	// TLS TrustStore path for internal client,  used by the internal client to authenticate with Pulsar brokers
	brokerClientTlsTrustStore: string

	// TLS TrustStore password for internal client,  used by the internal client to authenticate with Pulsar brokers
	brokerClientTlsTrustStorePassword: string

	// TLS KeyStore type configuration for internal client: JKS, PKCS12, used by the internal client to authenticate with Pulsar brokers
	brokerClientTlsKeyStoreType: string

	// TLS KeyStore path for internal client,  used by the internal client to authenticate with Pulsar brokers
	brokerClientTlsKeyStore: string

	// TLS KeyStore password for internal client,  used by the internal client to authenticate with Pulsar brokers
	brokerClientTlsKeyStorePassword: string

	// Specify the tls cipher the internal client will use to negotiate during TLS Handshake (a comma-separated list of ciphers).\n\nExamples:- [TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256].\n used by the internal client to authenticate with Pulsar brokers
	brokerClientTlsCiphers: string

	// Specify the tls protocols the broker will use to negotiate during TLS handshake (a comma-separated list of protocol names).\n\nExamples:- [TLSv1.3, TLSv1.2] \n used by the internal client to authenticate with Pulsar brokers
	brokerClientTlsProtocols: string

	// Enable the packages management service or not
	enablePackagesManagement: bool

	// The packages management service storage service provider
	packagesManagementStorageProvider: string

	// When the packages storage provider is bookkeeper, you can use this configuration to\ncontrol the number of replicas for storing the package
	packagesReplicas: int

	// The bookkeeper ledger root path
	packagesManagementLedgerRootPath: string

	// The directory to locate broker additional servlet
	additionalServletDirectory: string

	// List of broker additional servlet to load, which is a list of broker additional servlet names
	additionalServlets: string

	...
}

configuration: #PulsarBrokersParameter & {
}
