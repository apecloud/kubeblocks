allow.everyone.if.no.acl.found=true
default.replication.factor=1
auto.create.topics.enable=true
delete.topic.enable=false
socket.send.buffer.bytes=102400
socket.receive.buffer.bytes=102400
socket.request.max.bytes=104857600
num.network.threads=3
num.io.threads=8
num.partitions=1
num.recovery.threads.per.data.dir=1
offsets.topic.replication.factor=1
transaction.state.log.replication.factor=1
transaction.state.log.min.isr=1
log.flush.interval.messages=10000
log.flush.interval.ms=1000
log.retention.hours=168
log.retention.bytes=1073741824
log.segment.bytes=1073741824
log.retention.check.interval.ms=300000
group.initial.rebalance.delay.ms=0
message.max.bytes=1000012
{{- $component := fromJson "{}" }}
{{- if ne "broker" ( getEnvByName ( index $.component.podSpec.containers 0 ) "KAFKA_CFG_PROCESS_ROLES" ) }}
  {{- $component = $.component }}
{{- else }} 
{{- /* find kafka-controller component */}}
  {{- range $i, $e := $.cluster.spec.componentSpecs }}
    {{- if eq $e.componentDefRef "kafka-controller" }}
      {{- $component = $e }}
    {{- end }}
  {{- end }}
{{- end }}
{{- /* build controller.quorum.voters value string */}}
{{- $clusterName := $.cluster.metadata.name }}
{{- $namespace := $.cluster.metadata.namespace }}
{{- $replicas := $component.replicas | int }}
{{- $voters := "" }}
{{- range $i, $e := until $replicas }}
  {{- $podFQDN := printf "%s-%s-%d.%s-%s-headless.%s.svc" $clusterName $component.name $i $clusterName $component.name $namespace }}
  {{- $voter := printf "%d@%s:9093" $i $podFQDN }}
  {{- $voters = printf "%s,%s" $voters $voter }}
{{- end }}
{{- $voters = trimPrefix "," $voters }}
controller.quorum.voters={{ $voters }}
authorizer.class.name=
# end (DON'T REMOVE THIS LINE)