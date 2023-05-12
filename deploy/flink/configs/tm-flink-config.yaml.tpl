{{- $clusterName := $.cluster.metadata.name }}
{{- $namespace := $.cluster.metadata.namespace }}
{{- /* find job manager */}}
{{- $jm_component := fromJson "{}" }}
{{- $tm_component := fromJson "{}" }}
{{- range $i, $e := $.cluster.spec.componentSpecs }}
  {{- if eq $e.componentDefRef "jobmanager" }}
    {{- $jm_component = $e }}
  {{- else if eq $e.componentDefRef "taskmanager" }}
    {{- $tm_component = $e }}
  {{- end }}
{{- end }}
# get port
{{- $default_rpc_port := 6122 }}
{{- $rpc_port_info := getPortByName ( index $.podSpec.containers 0 ) "rpc" }}
{{- if $rpc_port_info }}
{{- $default_rpc_port = $rpc_port_info.containerPort }}
{{- end }}
# get mem
{{- $phy_memory := getContainerMemory ( index $.podSpec.containers 0 ) }}
jobmanager.rpc.address: {{ $clusterName }}-{{ $jm_component.name }}
jobmanager.rpc.port: 6123
parallelism.default: 1
taskmanager.bind-host: 0.0.0.0
taskmanager.host: {{ $clusterName }}-{{ $tm_component.name }}
taskmanager.memory.process.size: {{ $phy_memory }}
taskmanager.numberOfTaskSlots: 6
taskmanager.rpc.port: {{ $default_rpc_port }}