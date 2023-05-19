# broker config. generate from https://github.com/apache/kafka/blob/3.4/core/src/main/scala/kafka/server/KafkaConfig.scala#L1127

# Default Topic Configuration
log.segment.bytes=1073741824
# log.roll.ms=
log.roll.hours=168
log.index.size.max.bytes=10485760
log.flush.interval.ms=1000
log.flush.interval.messages=10000
# log.roll.jitter.ms=
log.roll.jitter.hours=0
# log.retention.ms=
# log.retention.minutes=
log.retention.hours=168
log.retention.bytes=-1
log.retention.check.interval.ms=300000
log.cleanup.policy=delete
log.cleaner.min.cleanable.ratio=0.5
log.cleaner.delete.retention.ms=86400000
log.cleaner.min.compaction.lag.ms=0
log.cleaner.max.compaction.lag.ms=9223372036854775807
log.index.interval.bytes=4096
log.segment.delete.delay.ms=60000
log.preallocate=false
min.insync.replicas=1
compression.type=producer
unclean.leader.election.enable=false

# Log Cleaner Configs
log.cleaner.threads=1
log.cleaner.io.max.bytes.per.second=1.7976931348623157E308
log.cleaner.dedupe.buffer.size=134217728
log.cleaner.io.buffer.size=524288
log.cleaner.io.buffer.load.factor=0.9
log.cleaner.backoff.ms=15000

log.cleaner.enable=true
log.flush.offset.checkpoint.interval.ms=60000
log.flush.start.offset.checkpoint.interval.ms=60000
log.flush.scheduler.interval.ms=9223372036854775807

# Thread Configs
num.network.threads=3
num.io.threads=8
num.replica.fetchers=1
num.recovery.threads.per.data.dir=1
background.threads=10

# Connection Quota Configs
max.connections.per.ip=2147483647
# max.connections.per.ip.overrides=

# according to the content of server.properties, changes were made based on the source code
offsets.topic.replication.factor=1
transaction.state.log.replication.factor=1
transaction.state.log.min.isr=1
delete.topic.enable=false

auto.create.topics.enable=true

# Controller
controller.quorum.election.timeout.ms=1000
controller.quorum.fetch.timeout.ms=2000
controller.quorum.election.backoff.max.ms=1000
controller.quorum.append.linger.ms=25
controller.quorum.request.timeout.ms=2000
controller.quorum.retry.backoff.ms=20

message.max.bytes=1048588
# num.replica.alter.log.dirs.threads=
queued.max.requests=500
queued.max.request.bytes=-1
request.timeout.ms=30000
socket.connection.setup.timeout.ms=10000
socket.connection.setup.timeout.max.ms=30000
metadata.log.max.record.bytes.between.snapshots=20971520
metadata.log.max.snapshot.interval.ms=3600000
broker.heartbeat.interval.ms=2000
broker.session.timeout.ms=9000
sasl.mechanism.controller.protocol=GSSAPI
metadata.log.segment.bytes=1073741824
# metadata.log.segment.min.bytes=8388608
metadata.log.segment.ms=604800000
metadata.max.retention.bytes=104857600
metadata.max.retention.ms=604800000
metadata.max.idle.interval.ms=500
# authorizer.class.name=
# early.start.listeners=
# control.plane.listener.name=
socket.send.buffer.bytes=102400
socket.receive.buffer.bytes=102400
socket.request.max.bytes=104857600
socket.listen.backlog.size=50
max.connections=2147483647
max.connection.creation.rate=2147483647
connections.max.idle.ms=600000
connection.failed.authentication.delay.ms=100
# broker.rack=
num.partitions=1

log.message.format.version=3.0-IV1
log.message.timestamp.type=CreateTime
log.message.timestamp.difference.max.ms=9223372036854775807
# create.topic.policy.class.name=
# alter.config.policy.class.name=
log.message.downconversion.enable=true
controller.socket.timeout.ms=30000
default.replication.factor=1
replica.lag.time.max.ms=30000
replica.socket.timeout.ms=30000
replica.socket.receive.buffer.bytes=65536
replica.fetch.max.bytes=1048576
replica.fetch.wait.max.ms=500
replica.fetch.backoff.ms=1000
replica.fetch.min.bytes=1
replica.fetch.response.max.bytes=10485760
replica.high.watermark.checkpoint.interval.ms=5000
fetch.purgatory.purge.interval.requests=1000
producer.purgatory.purge.interval.requests=1000
delete.records.purgatory.purge.interval.requests=1
auto.leader.rebalance.enable=true
leader.imbalance.per.broker.percentage=10
leader.imbalance.check.interval.seconds=300
security.inter.broker.protocol=PLAINTEXT
# replica.selector.class=
controlled.shutdown.max.retries=3
controlled.shutdown.retry.backoff.ms=5000
controlled.shutdown.enable=true
group.min.session.timeout.ms=6000
group.max.session.timeout.ms=1800000
group.initial.rebalance.delay.ms=3000
group.max.size=2147483647
offset.metadata.max.bytes=4096
offsets.load.buffer.size=5242880
offsets.topic.num.partitions=50
offsets.topic.segment.bytes=104857600
offsets.topic.compression.codec=0
offsets.retention.minutes=10080
offsets.retention.check.interval.ms=600000
offsets.commit.timeout.ms=5000
offsets.commit.required.acks=-1
transactional.id.expiration.ms=604800000
transaction.max.timeout.ms=900000
transaction.state.log.load.buffer.size=5242880
transaction.state.log.num.partitions=50
transaction.state.log.segment.bytes=104857600
transaction.abort.timed.out.transaction.cleanup.interval.ms=10000
transaction.remove.expired.transaction.cleanup.interval.ms=3600000
producer.id.expiration.ms=86400000
# producer.id.expiration.check.interval.ms=600000
max.incremental.fetch.session.cache.slots=1000
fetch.max.bytes=57671680
metrics.num.samples=2
metrics.sample.window.ms=30000
# metric.reporters=
metrics.recording.level=INFO
# auto.include.jmx.reporter=true ## will deprecated in Kafka4.0, use metric.reporters instead
# kafka.metrics.reporters=
kafka.metrics.polling.interval.secs=10
quota.window.num=11
replication.quota.window.num=11
alter.log.dirs.replication.quota.window.num=11
controller.quota.window.num=11
quota.window.size.seconds=1
replication.quota.window.size.seconds=1
alter.log.dirs.replication.quota.window.size.seconds=1
controller.quota.window.size.seconds=1
# client.quota.callback.class=
connections.max.reauth.ms=0

sasl.server.max.receive.size=524288
security.providers=
#principal.builder.class=class org.apache.kafka.common.security.authenticator.DefaultKafkaPrincipalBuilder
ssl.protocol=TLSv1.2
# ssl.provider=
ssl.enabled.protocols=TLSv1.2
ssl.keymanager.algorithm=SunX509
ssl.trustmanager.algorithm=PKIX
# ssl.endpoint.identification.algorithm=https
ssl.endpoint.identification.algorithm=
# ssl.secure.random.implementation=
ssl.client.auth=none
# ssl.cipher.suites=
ssl.principal.mapping.rules=DEFAULT
# ssl.engine.factory.class=
sasl.mechanism.inter.broker.protocol=GSSAPI
# sasl.jaas.config=
sasl.enabled.mechanisms=GSSAPI
# sasl.server.callback.handler.class=
# sasl.client.callback.handler.class=
# sasl.login.class=
# sasl.login.callback.handler.class=
# sasl.kerberos.service.name=
sasl.kerberos.kinit.cmd=/usr/bin/kinit
sasl.kerberos.ticket.renew.window.factor=0.8
sasl.kerberos.ticket.renew.jitter=0.05
sasl.kerberos.min.time.before.relogin=60000
sasl.kerberos.principal.to.local.rules=DEFAULT
sasl.login.refresh.window.factor=0.8
sasl.login.refresh.window.jitter=0.05
sasl.login.refresh.min.period.seconds=60
sasl.login.refresh.buffer.seconds=300
# sasl.login.connect.timeout.ms=
# sasl.login.read.timeout.ms=
sasl.login.retry.backoff.max.ms=10000
sasl.login.retry.backoff.ms=100
sasl.oauthbearer.scope.claim.name=scope
sasl.oauthbearer.sub.claim.name=sub
# sasl.oauthbearer.token.endpoint.url=
# sasl.oauthbearer.jwks.endpoint.url=
sasl.oauthbearer.jwks.endpoint.refresh.ms=3600000
sasl.oauthbearer.jwks.endpoint.retry.backoff.ms=100
sasl.oauthbearer.jwks.endpoint.retry.backoff.max.ms=10000
sasl.oauthbearer.clock.skew.seconds=30
# sasl.oauthbearer.expected.audience=
# sasl.oauthbearer.expected.issuer=
# delegation.token.master.key=  ## Deprecated

# delegation.token.secret.key=
delegation.token.max.lifetime.ms=604800000
delegation.token.expiry.time.ms=86400000
delegation.token.expiry.check.interval.ms=3600000
# password.encoder.secret=
# password.encoder.old.secret=
# password.encoder.keyfactory.algorithm=
password.encoder.cipher.algorithm=AES/CBC/PKCS5Padding
password.encoder.key.length=128
password.encoder.iterations=4096

# SSL Keystore of an Existing Listener
ssl.keystore.type=JKS
# ssl.keystore.location=
# ssl.keystore.password=
# ssl.key.password=
# ssl.keystore.key=
# ssl.keystore.certificate.chain=

# SSL Truststore of an Existing Listener
ssl.truststore.type=JKS
# ssl.truststore.location=
# ssl.truststore.password=
# ssl.truststore.certificates=

# override by image env
# process.roles=
# controller.listener.names=
# inter.broker.listener.name=
# listeners=PLAINTEXT://:9092
# listener.security.protocol.map=PLAINTEXT:PLAINTEXT,SSL:SSL,SASL_PLAINTEXT:SASL_PLAINTEXT,SASL_SSL:SASL_SSL
# advertised.listeners=
# initial.broker.registration.timeout.ms=60000
# node.id=-1
# metadata.log.dir=
# log.dirs=
# log.dir=/tmp/kafka-logs

# acl
allow.everyone.if.no.acl.found=true

# deprecated with kraft version
# inter.broker.protocol.version=3.4-IV0
# broker.id.generation.enable=true
# reserved.broker.max.id=1000
# broker.id=-1
# zookeeper.connect=
# zookeeper.session.timeout.ms=18000
# zookeeper.connection.timeout.ms=
# zookeeper.set.acl=false
# zookeeper.max.in.flight.requests=10
# zookeeper.ssl.client.enable=false
# zookeeper.clientCnxnSocket=
# zookeeper.ssl.keystore.location=
# zookeeper.ssl.keystore.password=
# zookeeper.ssl.keystore.type=
# zookeeper.ssl.truststore.location=
# zookeeper.ssl.truststore.password=
# zookeeper.ssl.truststore.type=
# zookeeper.ssl.protocol=TLSv1.2
# zookeeper.ssl.enabled.protocols=
# zookeeper.ssl.cipher.suites=
# zookeeper.ssl.endpoint.identification.algorithm=HTTPS
# zookeeper.ssl.crl.enable=false
# zookeeper.ssl.ocsp.enable=false
# zookeeper.metadata.migration.enable=false

# end (DON'T REMOVE THIS LINE)