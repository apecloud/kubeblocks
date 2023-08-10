//Copyright (C) 2022-2023 ApeCloud Co., Ltd
//
//This file is part of KubeBlocks project
//
//This program is free software: you can redistribute it and/or modify
//it under the terms of the GNU Affero General Public License as published by
//the Free Software Foundation, either version 3 of the License, or
//(at your option) any later version.
//
//This program is distributed in the hope that it will be useful
//but WITHOUT ANY WARRANTY; without even the implied warranty of
//MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//GNU Affero General Public License for more details.
//
//You should have received a copy of the GNU Affero General Public License
//along with this program.  If not, see <http://www.gnu.org/licenses/>.

// https://kafka.apache.org/documentation/#brokerconfigs
#KafkaParameter: {

  "allow.everyone.if.no.acl.found"?: bool

  // The replication factor for the offsets topic (set higher to ensure availability). Internal topic creation will fail until the cluster size meets this replication factor requirement.
  "offsets.topic.replication.factor"?: int & >=1 & <=32767

  // The replication factor for the transaction topic (set higher to ensure availability). Internal topic creation will fail until the cluster size meets this replication factor requirement.
  "transaction.state.log.replication.factor"?: int & >=1 & <=32767

  // The maximum time in ms that a message in any topic is kept in memory before flushed to disk. If not set, the value in log.flush.scheduler.interval.ms is used
  "log.flush.interval.ms"?: int

  // The number of messages accumulated on a log partition before messages are flushed to disk
  "log.flush.interval.messages"?: int & >=1

  // Overridden min.insync.replicas config for the transaction topic.
  "transaction.state.log.min.isr"?: int & >=1

  // Enables delete topic. Delete topic through the admin tool will have no effect if this config is turned off
  "delete.topic.enable"?: bool

  // The largest record batch size allowed by Kafka
  "message.max.bytes"?: int & >=0

  // The number of threads that the server uses for receiving requests from the network and sending responses to the network
  "num.network.threads"?: int & >=1

  // The number of threads that the server uses for processing requests, which may include disk I/O
  "num.io.threads"?: int & >=1

  // The number of threads that can move replicas between log directories, which may include disk I/O
  "num.replica.alter.log.dirs.threads"?: int

  // The number of threads to use for various background processing tasks
  "background.threads"?: int & >=1

  // The number of queued requests allowed for data-plane, before blocking the network threads
  "queued.max.requests"?: int & >=1

  // The number of queued bytes allowed before no more requests are read
  "queued.max.request.bytes"?: int

  // The configuration controls the maximum amount of time the client will wait for the response of a request
  "request.timeout.ms"?: int & >=0

  // The amount of time the client will wait for the socket connection to be established. If the connection is not built before the timeout elapses, clients will close the socket channel.
  "socket.connection.setup.timeout.ms"?: int

  // The maximum amount of time the client will wait for the socket connection to be established.
  "socket.connection.setup.timeout.max.ms"?: int

  // This is the maximum number of bytes in the log between the latest snapshot and the high-watermark needed before generating a new snapshot.
  "metadata.log.max.record.bytes.between.snapshots"?: int & >=1

  // The length of time in milliseconds between broker heartbeats. Used when running in KRaft mode.
  "broker.heartbeat.interval.ms"?: int

  // The length of time in milliseconds that a broker lease lasts if no heartbeats are made. Used when running in KRaft mode.
  "broker.session.timeout.ms"?: int

  // SASL mechanism used for communication with controllers. Default is GSSAPI.
  "sasl.mechanism.controller.protocol"?: string

  // The maximum size of a single metadata log file.
  "metadata.log.segment.bytes"?: int & >=12

  // The maximum time before a new metadata log file is rolled out (in milliseconds).
  "metadata.log.segment.ms"?: int

  // The maximum combined size of the metadata log and snapshots before deleting old snapshots and log files.
  "metadata.max.retention.bytes"?: int

  // The number of milliseconds to keep a metadata log file or snapshot before deleting it. Since at least one snapshot must exist before any logs can be deleted, this is a soft limit.
  "metadata.max.retention.ms"?: int

  // This configuration controls how often the active controller should write no-op records to the metadata partition.
  // If the value is 0, no-op records are not appended to the metadata partition. The default value is 500
  "metadata.max.idle.interval.ms"?: int & >=0

  // The fully qualified name of a class that implements org.apache.kafka.server.authorizer.Authorizer interface, which is used by the broker for authorization.
  "authorizer.class.name"?: string

  // A comma-separated list of listener names which may be started before the authorizer has finished initialization.
  "early.start.listeners"?: string

  // Name of listener used for communication between controller and brokers.
  "control.plane.listener.name"?: string

  // The SO_SNDBUF buffer of the socket server sockets. If the value is -1, the OS default will be used.
  "socket.send.buffer.bytes"?: int

  // The SO_RCVBUF buffer of the socket server sockets. If the value is -1, the OS default will be used.
  "socket.receive.buffer.bytes"?: int

  // The maximum number of bytes in a socket request
  "socket.request.max.bytes"?: int & >=1

  // The maximum number of pending connections on the socket.
  // In Linux, you may also need to configure `somaxconn` and `tcp_max_syn_backlog` kernel parameters accordingly to make the configuration takes effect.
  "socket.listen.backlog.size"?: int & >=1

  // The maximum number of connections we allow from each ip address.
  "max.connections.per.ip"?: int & >=0

  // A comma-separated list of per-ip or hostname overrides to the default maximum number of connections. An example value is "hostName:100,127.0.0.1:200"
  "max.connections.per.ip.overrides"?: string

  // The maximum number of connections we allow in the broker at any time.
  "max.connections"?: int & >=0

  // The maximum connection creation rate we allow in the broker at any time.
  "max.connection.creation.rate"?: int & >=0

  // Close idle connections after the number of milliseconds specified by this config.
  "connections.max.idle.ms"?: int

  // Connection close delay on failed authentication: this is the time (in milliseconds) by which connection close will be delayed on authentication failure.
  // This must be configured to be less than connections.max.idle.ms to prevent connection timeout.
  "connection.failed.authentication.delay.ms"?: int & >=0

  // Rack of the broker. This will be used in rack aware replication assignment for fault tolerance.
  "broker.rack"?: string

  // The default number of log partitions per topic
  "num.partitions"?: int & >=1

  // The maximum size of a single log file
  "log.segment.bytes"?: int & >=14

  // The maximum time before a new log segment is rolled out (in milliseconds). If not set, the value in log.roll.hours is used
  "log.roll.ms"?: int

  // The maximum time before a new log segment is rolled out (in hours), secondary to log.roll.ms property
  "log.roll.hours"?: int & >=1

  // The maximum jitter to subtract from logRollTimeMillis (in milliseconds). If not set, the value in log.roll.jitter.hours is used
  "log.roll.jitter.ms"?: int

  // The maximum jitter to subtract from logRollTimeMillis (in hours), secondary to log.roll.jitter.ms property
  "log.roll.jitter.hours"?: int & >=0

  // The number of milliseconds to keep a log file before deleting it (in milliseconds), If not set, the value in log.retention.minutes is used. If set to -1, no time limit is applied.
  "log.retention.ms"?: int

  // The number of minutes to keep a log file before deleting it (in minutes), secondary to log.retention.ms property. If not set, the value in log.retention.hours is used
  "log.retention.minutes"?: int

  // The number of hours to keep a log file before deleting it (in hours), tertiary to log.retention.ms property
  "log.retention.hours"?: int

  // The maximum size of the log before deleting it
  "log.retention.bytes"?: int

  // The frequency in milliseconds that the log cleaner checks whether any log is eligible for deletion
  "log.retention.check.interval.ms"?: int & >=1

  // The default cleanup policy for segments beyond the retention window. A comma separated list of valid policies.
  "log.cleanup.policy"?: string & "compact" | "delete"

  // The number of background threads to use for log cleaning
  "log.cleaner.threads"?: int & >=0

  // The log cleaner will be throttled so that the sum of its read and write i/o will be less than this value on average
  "log.cleaner.io.max.bytes.per.second"?: number

  // The total memory used for log deduplication across all cleaner threads
  "log.cleaner.dedupe.buffer.size"?: int

  // The total memory used for log cleaner I/O buffers across all cleaner threads
  "log.cleaner.io.buffer.size"?: int & >=0

  // Log cleaner dedupe buffer load factor. The percentage full the dedupe buffer can become. A higher value will allow more log to be cleaned at once but will lead to more hash collisions
  "log.cleaner.io.buffer.load.factor"?: number

  // The amount of time to sleep when there are no logs to clean
  "log.cleaner.backoff.ms"?: int & >=0

  // The minimum ratio of dirty log to total log for a log to eligible for cleaning.
  "log.cleaner.min.cleanable.ratio"?: number & >=0 & <=1

  // Enable the log cleaner process to run on the server.
  "log.cleaner.enable"?: bool

  // The amount of time to retain delete tombstone markers for log compacted topics.
  "log.cleaner.delete.retention.ms"?: int & >=0

  // The minimum time a message will remain uncompacted in the log. Only applicable for logs that are being compacted.
  "log.cleaner.min.compaction.lag.ms"?: int & >=0

  // The maximum time a message will remain ineligible for compaction in the log. Only applicable for logs that are being compacted.
  "log.cleaner.max.compaction.lag.ms"?: int & >=1

  // The maximum size in bytes of the offset index
  "log.index.size.max.bytes"?: int & >=4

  // The interval with which we add an entry to the offset index
  "log.index.interval.bytes"?: int & >=0

  // The amount of time to wait before deleting a file from the filesystem
  "log.segment.delete.delay.ms"?: int & >=0

  // The frequency in ms that the log flusher checks whether any log needs to be flushed to disk
  "log.flush.scheduler.interval.ms"?: int

  // The frequency with which we update the persistent record of the last flush which acts as the log recovery point
  "log.flush.offset.checkpoint.interval.ms"?: int & >=0

  // The frequency with which we update the persistent record of log start offset
  "log.flush.start.offset.checkpoint.interval.ms"?: int & >=0

  // Should pre allocate file when create new segment? If you are using Kafka on Windows, you probably need to set it to true.
  "log.preallocate"?: bool

  // The number of threads per data directory to be used for log recovery at startup and flushing at shutdown
  "num.recovery.threads.per.data.dir"?: int & >=1

  // Enable auto creation of topic on the server
  "auto.create.topics.enable"?: bool

  // When a producer sets acks to "all" (or "-1"), min.insync.replicas specifies the minimum number of replicas that must acknowledge a write for the write to be considered successful.
  // If this minimum cannot be met, then the producer will raise an exception (either NotEnoughReplicas or NotEnoughReplicasAfterAppend).
  "min.insync.replicas"?: int & >=1

  // Specify the message format version the broker will use to append messages to the logs.
  "log.message.format.version"?: string & "0.8.0" | "0.8.1" | "0.8.2" | "0.9.0" | "0.10.0-IV0" | "0.10.0-IV1" | "0.10.1-IV0" | "0.10.1-IV1" | "0.10.1-IV2" | "0.10.2-IV0" | "0.11.0-IV0" | "0.11.0-IV1" | "0.11.0-IV2" | "1.0-IV0" | "1.1-IV0" | "2.0-IV0" | "2.0-IV1" | "2.1-IV0" | "2.1-IV1" | "2.1-IV2" | "2.2-IV0" | "2.2-IV1" | "2.3-IV0" | "2.3-IV1" | "2.4-IV0" | "2.4-IV1" | "2.5-IV0" | "2.6-IV0" | "2.7-IV0" | "2.7-IV1" | "2.7-IV2" | "2.8-IV0" | "2.8-IV1" | "3.0-IV0" | "3.0-IV1" | "3.1-IV0" | "3.2-IV0" | "3.3-IV0" | "3.3-IV1" | "3.3-IV2" | "3.3-IV3" | "3.4-IV0"

  // Define whether the timestamp in the message is message create time or log append time.
  "log.message.timestamp.type"?: string & "CreateTime" | "LogAppendTime"

  // The maximum difference allowed between the timestamp when a broker receives a message and the timestamp specified in the message.
  "log.message.timestamp.difference.max.ms"?: int & >=0

  // The create topic policy class that should be used for validation.
  "create.topic.policy.class.name"?: string

  // The alter configs policy class that should be used for validation.
  "alter.config.policy.class.name"?: string

  // This configuration controls whether down-conversion of message formats is enabled to satisfy consume requests.
  "log.message.downconversion.enable"?: bool

  // The socket timeout for controller-to-broker channels
  "controller.socket.timeout.ms"?: int

  // The default replication factors for automatically created topics
  "default.replication.factor"?: int

  // If a follower hasn't sent any fetch requests or hasn't consumed up to the leaders log end offset for at least this time, the leader will remove the follower from isr
  "replica.lag.time.max.ms"?: int

  // The socket timeout for network requests. Its value should be at least replica.fetch.wait.max.ms
  "replica.socket.timeout.ms"?: int

  // The socket receive buffer for network requests
  "replica.socket.receive.buffer.bytes"?: int

  // The number of bytes of messages to attempt to fetch for each partition.
  "replica.fetch.max.bytes"?: int & >=0

  // The maximum wait time for each fetcher request issued by follower replicas.
  "replica.fetch.wait.max.ms"?: int

  // The amount of time to sleep when fetch partition error occurs.
  "replica.fetch.backoff.ms"?: int & >=0

  // Minimum bytes expected for each fetch response. If not enough bytes, wait up to replica.fetch.wait.max.ms (broker config).
  "replica.fetch.min.bytes"?: int

  // Maximum bytes expected for the entire fetch response.
  "replica.fetch.response.max.bytes"?: int & >=0

  // Number of fetcher threads used to replicate records from each source broker.
  "num.replica.fetchers"?: int

  // The frequency with which the high watermark is saved out to disk
  "replica.high.watermark.checkpoint.interval.ms"?: int

  // The purge interval (in number of requests) of the fetch request purgatory
  "fetch.purgatory.purge.interval.requests"?: int

  // The purge interval (in number of requests) of the producer request purgatory
  "producer.purgatory.purge.interval.requests"?: int

  // The purge interval (in number of requests) of the delete records request purgatory
  "delete.records.purgatory.purge.interval.requests"?: int

  // Enables auto leader balancing.
  "auto.leader.rebalance.enable"?: bool

  // The ratio of leader imbalance allowed per broker.
  "leader.imbalance.per.broker.percentage"?: int

  // The frequency with which the partition rebalance check is triggered by the controller
  "leader.imbalance.check.interval.seconds"?: int & >=1

  // Indicates whether to enable replicas not in the ISR set to be elected as leader as a last resort, even though doing so may result in data loss
  "unclean.leader.election.enable"?: bool

  // Security protocol used to communicate between brokers.
  "security.inter.broker.protocol"?: string & "PLAINTEXT" | "SSL" | "SASL_PLAINTEXT" | "SASL_SSL"

  // The fully qualified class name that implements ReplicaSelector.
  "replica.selector.class"?: string

  // Controlled shutdown can fail for multiple reasons. This determines the number of retries when such failure happens
  "controlled.shutdown.max.retries"?: int

  // Before each retry, the system needs time to recover from the state that caused the previous failure (Controller fail over, replica lag etc)
  "controlled.shutdown.retry.backoff.ms"?: int

  // Enable controlled shutdown of the server
  "controlled.shutdown.enable"?: bool

  // The minimum allowed session timeout for registered consumers.
  "group.min.session.timeout.ms"?: int

  // The maximum allowed session timeout for registered consumers.
  "group.max.session.timeout.ms"?: int

  // The amount of time the group coordinator will wait for more consumers to join a new group before performing the first rebalance.
  "group.initial.rebalance.delay.ms"?: int

  // The maximum number of consumers that a single consumer group can accommodate.
  "group.max.size"?: int & >=1

  // The maximum size for a metadata entry associated with an offset commit
  "offset.metadata.max.bytes"?: int

  // Batch size for reading from the offsets segments when loading offsets into the cache (soft-limit, overridden if records are too large).
  "offsets.load.buffer.size"?: int & >=1

  // The number of partitions for the offset commit topic (should not change after deployment)
  "offsets.topic.num.partitions"?: int & >=1

  // The offsets topic segment bytes should be kept relatively small in order to facilitate faster log compaction and cache loads
  "offsets.topic.segment.bytes"?: int & >=1

  // Compression codec for the offsets topic - compression may be used to achieve "atomic" commits
  "offsets.topic.compression.codec"?: int

  // For subscribed consumers, committed offset of a specific partition will be expired and discarded when 1) this retention
  // period has elapsed after the consumer group loses all its consumers (i.e. becomes empty); 2) this retention period has elapsed
  // since the last time an offset is committed for the partition and the group is no longer subscribed to the corresponding topic.
  "offsets.retention.minutes"?: int & >=1

  // Frequency at which to check for stale offsets
  "offsets.retention.check.interval.ms"?: int & >=1

  // Offset commit will be delayed until all replicas for the offsets topic receive the commit or this timeout is reached. This is similar to the producer request timeout.
  "offsets.commit.timeout.ms"?: int & >=1

  // The required acks before the commit can be accepted. In general, the default (-1) should not be overridden
  "offsets.commit.required.acks"?: int

  // Specify the final compression type for a given topic.
  "compression.type"?: string & "uncompressed" | "zstd" | "lz4" | "snappy" | "gzip" | "producer"

  // The time in ms that the transaction coordinator will wait without receiving any transaction status updates for the current transaction before expiring its transactional id.
  "transactional.id.expiration.ms"?: int & >=1

  // The maximum allowed timeout for transactions.
  "transaction.max.timeout.ms"?: int & >=1

  // Batch size for reading from the transaction log segments when loading producer ids and transactions into the cache (soft-limit, overridden if records are too large).
  "transaction.state.log.load.buffer.size"?: int & >=1

  // The number of partitions for the transaction topic (should not change after deployment).
  "transaction.state.log.num.partitions"?: int & >=1

  // The transaction topic segment bytes should be kept relatively small in order to facilitate faster log compaction and cache loads
  "transaction.state.log.segment.bytes"?: int & >=1

  // The interval at which to rollback transactions that have timed out
  "transaction.abort.timed.out.transaction.cleanup.interval.ms"?: int & >=1

  // The interval at which to remove transactions that have expired due to transactional.id.expiration.ms passing
  "transaction.remove.expired.transaction.cleanup.interval.ms"?: int & >=1

  // The time in ms that a topic partition leader will wait before expiring producer IDs.
  // "producer.id.expiration.ms"?: int & >=1

  // The maximum number of incremental fetch sessions that we will maintain.
  "max.incremental.fetch.session.cache.slots"?: int & >=0

  // The maximum amount of data the server should return for a fetch request.
  "fetch.max.bytes"?: int & >=0

  // The number of samples maintained to compute metrics.
  "metrics.num.samples"?: int & >=1

  // The window of time a metrics sample is computed over.
  "metrics.sample.window.ms"?: int & >=0

  // The highest recording level for metrics.
  "metrics.recording.level"?: string & "INFO" | "DEBUG" | "TRACE"

  // A list of classes to use as metrics reporters. Implementing the org.apache.kafka.common.metrics.MetricsReporter interface allows plugging in classes that will be notified of new metric creation. The JmxReporter is always included to register JMX statistics.
  "metric.reporters"?: string

  // A list of classes to use as Yammer metrics custom reporters.
  "kafka.metrics.reporters"?: string

  // The metrics polling interval (in seconds) which can be used in kafka.metrics.reporters implementations.
  "kafka.metrics.polling.interval.secs"?: int & >=1

  // The number of samples to retain in memory for client quotas
  "quota.window.num"?: int & >=1

  // The number of samples to retain in memory for replication quotas
  "replication.quota.window.num"?: int & >=1

  // The number of samples to retain in memory for alter log dirs replication quotas
  "alter.log.dirs.replication.quota.window.num"?: int & >=1

  // The number of samples to retain in memory for controller mutation quotas
  "controller.quota.window.num"?: int & >=1

  // The time span of each sample for client quotas
  "quota.window.size.seconds"?: int & >=1

  // The time span of each sample for replication quotas
  "replication.quota.window.size.seconds"?: int & >=1

  // The time span of each sample for alter log dirs replication quotas
  "alter.log.dirs.replication.quota.window.size.seconds"?: int & >=1

  // The time span of each sample for controller mutations quotas
  "controller.quota.window.size.seconds"?: int & >=1

  // The fully qualified name of a class that implements the ClientQuotaCallback interface, which is used to determine quota limits applied to client requests.
  "client.quota.callback.class"?: string

  // When explicitly set to a positive number (the default is 0, not a positive number), a session lifetime that will not exceed the configured value will be communicated to v2.2.0 or later clients when they authenticate.
  "connections.max.reauth.ms"?: int

  // The maximum receive size allowed before and during initial SASL authentication.
  "sasl.server.max.receive.size"?: int

  // A list of configurable creator classes each returning a provider implementing security algorithms.
  "security.providers"?: string

  // The SSL protocol used to generate the SSLContext.
  "ssl.protocol"?: string & "TLSv1.2" | "TLSv1.3" | "TLS" | "TLSv1.1" | "SSL" | "SSLv2" | "SSLv3"

  // The name of the security provider used for SSL connections. Default value is the default security provider of the JVM.
  "ssl.provider"?: string

  // The list of protocols enabled for SSL connections.
  "ssl.enabled.protocols"?: string

  // The file format of the key store file. This is optional for client. The values currently supported by the default `ssl.engine.factory.class` are [JKS, PKCS12, PEM].
  "ssl.keystore.type"?: string

  // The location of the key store file. This is optional for client and can be used for two-way authentication for client.
  "ssl.keystore.location"?: string

  // The store password for the key store file. This is optional for client and only needed if 'ssl.keystore.location' is configured. Key store password is not supported for PEM format.
  "ssl.keystore.password"?: string

  // The password of the private key in the key store file or the PEM key specified in 'ssl.keystore.key'.
  "ssl.key.password"?: string

  // Private key in the format specified by 'ssl.keystore.type'.
  "ssl.keystore.key"?: string

  // Certificate chain in the format specified by 'ssl.keystore.type'. Default SSL engine factory supports only PEM format with a list of X.509 certificates
  "ssl.keystore.certificate.chain"?: string

  // The file format of the trust store file. The values currently supported by the default `ssl.engine.factory.class` are [JKS, PKCS12, PEM].
  "ssl.truststore.type"?: string

  // The location of the trust store file.
  "ssl.truststore.location"?: string

  // The password for the trust store file. If a password is not set, trust store file configured will still be used, but integrity checking is disabled. Trust store password is not supported for PEM format.
  "ssl.truststore.password"?: string

  // Trusted certificates in the format specified by 'ssl.truststore.type'. Default SSL engine factory supports only PEM format with X.509 certificates.
  "ssl.truststore.certificates"?: string

  // The algorithm used by key manager factory for SSL connections. Default value is the key manager factory algorithm configured for the Java Virtual Machine.
  "ssl.keymanager.algorithm"?: string

  // The algorithm used by trust manager factory for SSL connections. Default value is the trust manager factory algorithm configured for the Java Virtual Machine.
  "ssl.trustmanager.algorithm"?: string

  // The endpoint identification algorithm to validate server hostname using server certificate.
  "ssl.endpoint.identification.algorithm"?: string

  // The SecureRandom PRNG implementation to use for SSL cryptography operations.
  "ssl.secure.random.implementation"?: string

  // Configures kafka broker to request client authentication.
  "ssl.client.auth"?: string & "required" | "requested" | "none"

  // A list of cipher suites. This is a named combination of authentication, encryption,
  "ssl.cipher.suites"?: string

  // A list of rules for mapping from distinguished name from the client certificate to short name.
  "ssl.principal.mapping.rules"?: string

  // The class of type org.apache.kafka.common.security.auth.SslEngineFactory to provide SSLEngine objects. Default value is org.apache.kafka.common.security.ssl.DefaultSslEngineFactory
  "ssl.engine.factory.class"?: string

  // SASL mechanism used for inter-broker communication. Default is GSSAPI.
  "sasl.mechanism.inter.broker.protocol"?: string

  // JAAS login context parameters for SASL connections in the format used by JAAS configuration files.
  "sasl.jaas.config"?: string

  // The list of SASL mechanisms enabled in the Kafka server. The list may contain any mechanism for which a security provider is available. Only GSSAPI is enabled by default.
  "sasl.enabled.mechanisms"?: string

  // The fully qualified name of a SASL server callback handler class that implements the AuthenticateCallbackHandler interface.
  "sasl.server.callback.handler.class"?: string

  // The fully qualified name of a SASL client callback handler class that implements the AuthenticateCallbackHandler interface.
  "sasl.client.callback.handler.class"?: string

  // The fully qualified name of a class that implements the Login interface. For brokers, login config must be prefixed with listener prefix and SASL mechanism name in lower-case. For example, listener.name.sasl_ssl.scram-sha-256.sasl.login.class=com.example.CustomScramLogin
  "sasl.login.class"?: string

  // The fully qualified name of a SASL login callback handler class that implements the AuthenticateCallbackHandler interface.
  "sasl.login.callback.handler.class"?: string

  // The Kerberos principal name that Kafka runs as. This can be defined either in Kafka's JAAS config or in Kafka's config.
  "sasl.kerberos.service.name"?: string

  // Kerberos kinit command path.
  "sasl.kerberos.kinit.cmd"?: string

  // Login thread will sleep until the specified window factor of time from last refresh to ticket's expiry has been reached, at which time it will try to renew the ticket.
  "sasl.kerberos.ticket.renew.window.factor"?: number

  // Percentage of random jitter added to the renewal time.
  "sasl.kerberos.ticket.renew.jitter"?: number

  // Login thread sleep time between refresh attempts.
  "sasl.kerberos.min.time.before.relogin"?: int

  // A list of rules for mapping from principal names to short names (typically operating system usernames).
  "sasl.kerberos.principal.to.local.rules"?: string

  // Login refresh thread will sleep until the specified window factor relative to the credential's lifetime has been reached, at which time it will try to refresh the credential. Legal values are between 0.5 (50%) and 1.0 (100%) inclusive; a default value of 0.8 (80%) is used if no value is specified. Currently applies only to OAUTHBEARER.
  "sasl.login.refresh.window.factor"?: number & >=0.5 & <=1.0

  // The maximum amount of random jitter relative to the credential's lifetime that is added to the login refresh thread's sleep time.
  "sasl.login.refresh.window.jitter"?: number & >=0.0 & <=0.25

  // The desired minimum time for the login refresh thread to wait before refreshing a credential, in seconds.
  "sasl.login.refresh.min.period.seconds"?: int & >=0 & <=900

  // The amount of buffer time before credential expiration to maintain when refreshing a credential, in seconds.
  "sasl.login.refresh.buffer.seconds"?: int & >=0 & <=3600

  // The (optional) value in milliseconds for the external authentication provider connection timeout. Currently applies only to OAUTHBEARER.
  "sasl.login.connect.timeout.ms"?: int

  // The (optional) value in milliseconds for the external authentication provider read timeout. Currently applies only to OAUTHBEARER.
  "sasl.login.read.timeout.ms"?: int

  // The (optional) value in milliseconds for the maximum wait between login attempts to the external authentication provider.
  "sasl.login.retry.backoff.max.ms"?: int

  // The (optional) value in milliseconds for the initial wait between login attempts to the external authentication provider.
  "sasl.login.retry.backoff.ms"?: int

  // The OAuth claim for the scope is often named "scope", but this (optional) setting can provide a different name to use for the scope included in the JWT payload's claims if the OAuth/OIDC provider uses a different name for that claim.
  "sasl.oauthbearer.scope.claim.name"?: string

  // The OAuth claim for the subject is often named "sub", but this (optional) setting can provide a different name to use for the subject included in the JWT payload's claims if the OAuth/OIDC provider uses a different name for that claim.
  "sasl.oauthbearer.sub.claim.name"?: string

  // The URL for the OAuth/OIDC identity provider. If the URL is HTTP(S)-based, it is the issuer's token endpoint URL to which requests will be made to login based on the configuration in sasl.jaas.config.
  "sasl.oauthbearer.token.endpoint.url"?: string

  // The OAuth/OIDC provider URL from which the provider's JWKS (JSON Web Key Set) can be retrieved.
  "sasl.oauthbearer.jwks.endpoint.url"?: string

  // The (optional) value in milliseconds for the broker to wait between refreshing its JWKS (JSON Web Key Set) cache that contains the keys to verify the signature of the JWT.
  "sasl.oauthbearer.jwks.endpoint.refresh.ms"?: int

  // The (optional) value in milliseconds for the initial wait between JWKS (JSON Web Key Set) retrieval attempts from the external authentication provider.
  "sasl.oauthbearer.jwks.endpoint.retry.backoff.ms"?: int

  // The (optional) value in milliseconds for the maximum wait between attempts to retrieve the JWKS (JSON Web Key Set) from the external authentication provider.
  "sasl.oauthbearer.jwks.endpoint.retry.backoff.max.ms"?: int

  // The (optional) value in seconds to allow for differences between the time of the OAuth/OIDC identity provider and the broker.
  "sasl.oauthbearer.clock.skew.seconds"?: int

  // The (optional) comma-delimited setting for the broker to use to verify that the JWT was issued for one of the expected audiences.
  "sasl.oauthbearer.expected.audience"?: string

  // The (optional) setting for the broker to use to verify that the JWT was created by the expected issuer.
  "sasl.oauthbearer.expected.issuer"?: string

  // Secret key to generate and verify delegation tokens. The same key must be configured across all the brokers. If the key is not set or set to empty string, brokers will disable the delegation token support.
  "delegation.token.secret.key"?: string

  // The token has a maximum lifetime beyond which it cannot be renewed anymore. Default value 7 days.
  "delegation.token.max.lifetime.ms"?: int & >=1

  // The token validity time in miliseconds before the token needs to be renewed. Default value 1 day.
  "delegation.token.expiry.time.ms"?: int & >=1

  // Scan interval to remove expired delegation tokens.
  "delegation.token.expiry.check.interval.ms"?: int & >=1

  // The secret used for encoding dynamically configured passwords for this broker.
  "password.encoder.secret"?: string

  // The old secret that was used for encoding dynamically configured passwords.
  "password.encoder.old.secret"?: string

  // The SecretKeyFactory algorithm used for encoding dynamically configured passwords.
  "password.encoder.keyfactory.algorithm"?: string

  // The Cipher algorithm used for encoding dynamically configured passwords.
  "password.encoder.cipher.algorithm"?: string

  // The key length used for encoding dynamically configured passwords.
  "password.encoder.key.length"?: int & >=8

  // The iteration count used for encoding dynamically configured passwords.
  "password.encoder.iterations"?: int & >=1024

  // Maximum time in milliseconds to wait without being able to fetch from the leader before triggering a new election
  "controller.quorum.election.timeout.ms"?: int

  // Maximum time without a successful fetch from the current leader before becoming a candidate and triggering an election for voters; Maximum time without receiving fetch from a majority of the quorum before asking around to see if there's a new epoch for leader
  "controller.quorum.fetch.timeout.ms"?: int

  // Maximum time in milliseconds before starting new elections. This is used in the binary exponential backoff mechanism that helps prevent gridlocked elections
  "controller.quorum.election.backoff.max.ms"?: int

  // The duration in milliseconds that the leader will wait for writes to accumulate before flushing them to disk.
  "controller.quorum.append.linger.ms"?: int

  // The configuration controls the maximum amount of time the client will wait for the response of a request.
  "controller.quorum.request.timeout.ms"?: int

  // The amount of time to wait before attempting to retry a failed request to a given topic partition.
  "controller.quorum.retry.backoff.ms"?: int

  // other parameters
  ...
}
