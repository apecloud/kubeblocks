PULSAR_EXTRA_OPTS: -Dpulsar.allocator.exit_on_oom=true -Dio.netty.recycler.maxCapacity.default=1000 -Dio.netty.recycler.linkCapacity=1024 -Dnetworkaddress.cache.ttl=60 -XX:ActiveProcessorCount=1 -XshowSettings:vm -Ddepth=64
PULSAR_GC: -XX:+UseG1GC -XX:MaxGCPauseMillis=10 -XX:+ParallelRefProcEnabled -XX:+UnlockExperimentalVMOptions -XX:+DoEscapeAnalysis -XX:G1NewSizePercent=50 -XX:+DisableExplicitGC -XX:-ResizePLAB
{{- $MaxDirectMemorySize := "" }}
{{- $phy_memory := getContainerMemory ( index $.podSpec.containers 0 ) }}
{{- if gt $phy_memory 0 }}
  {{- $MaxDirectMemorySize = printf "-XX:MaxDirectMemorySize=%dm" (mul (div $phy_memory ( mul 1024 1024 10)) 6) }}
{{- end }}
PULSAR_MEM: -XX:MinRAMPercentage=30 -XX:MaxRAMPercentage=30 {{ $MaxDirectMemorySize }}
statusFilePath: /pulsar/status

# KoP config
# ref:
# - https://github.com/streamnative/kop/blob/master/docs/kop.md
# - https://github.com/streamnative/kop/blob/master/docs/configuration.md
PULSAR_PREFIX_messagingProtocols: kafka
#PULSAR_PREFIX_protocolHandlerDirectory: ./protocols
#PULSAR_PREFIX_narExtractionDirectory: /tmp/pulsar-nar
PULSAR_PREFIX_allowAutoTopicCreationType: partitioned
PULSAR_PREFIX_kafkaListeners: PLAINTEXT://0.0.0.0:9092
#PULSAR_PREFIX_kafkaListeners: kafka_external://0.0.0.0:9092
#PULSAR_PREFIX_kafkaProtocolMap: kafka_external:PLAINTEXT
{{- $clusterName := $.cluster.metadata.name }}
{{- $namespace := $.cluster.metadata.namespace }}
{{- $pulsar_broker_component := fromJson "{}" }}
{{- range $i, $e := $.cluster.spec.componentSpecs }}
  {{- if eq $e.componentDefRef "pulsar-broker" }}
    {{- $pulsar_broker_component = $e }}
  {{- end }}
{{- end }}
{{- $brokerSvcFDQN := printf "%s-%s.%s.svc" $clusterName $pulsar_broker_component.name $namespace }}
#PULSAR_PREFIX_kafkaAdvertisedListeners: kafka_external://{{ $brokerSvcFDQN }}:9092

# Set offset management as below, since offset management for KoP depends on Pulsar "Broker Entry Metadata".
# It’s required for KoP 2.8.0 or higher version.
PULSAR_PREFIX_brokerEntryMetadataInterceptors: org.apache.pulsar.common.intercept.AppendIndexMetadataInterceptor
# Disable the deletion of inactive topics. It’s not required but very important in KoP. Currently,
# Pulsar deletes inactive partitions of a partitioned topic while the metadata of the partitioned topic is not deleted.
# KoP cannot create missed partitions in this case.
PULSAR_PREFIX_brokerDeleteInactiveTopicsEnabled: false