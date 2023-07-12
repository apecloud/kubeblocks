# Enable Key_Shared subscription (default is enabled)
# @deprecated since 2.8.0 subscriptionTypesEnabled is preferred over subscriptionKeySharedEnable.
subscriptionKeySharedEnable=true

# KoP config
# ref: 
kafkaListeners=kafka_internal://0.0.0.0:9094,kafka_external://0.0.0.0:9092
kafkaProtocolMap=kafka_internal:PLAINTEXT,kafka_external:PLAINTEXT
{{- $clusterName := $.cluster.metadata.name }}
{{- $namespace := $.cluster.metadata.namespace }}
{{- $pulsar_zk_component := fromJson "{}" }}
{{- range $i, $e := $.cluster.spec.componentSpecs }}
  {{- if eq $e.componentDefRef "broker" }}
    {{- $pulsar_broker_component = $e }}
  {{- end }}
{{- end }}
{{- $brokerSvcFDQN := printf "%s-%s.%s.svc.cluster.local" $clusterName $pulsar_broker_component.name $i $clusterName $component.name $namespace }}
kafkaAdvertisedListeners=kafka_internal://{{ $brokerSvcFDQN }}:9094,kafka_external://{{ $brokerSvcFDQN }}:9092

