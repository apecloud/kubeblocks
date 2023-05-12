{{- $clusterName := $.cluster.metadata.name }}
{{- $namespace := $.cluster.metadata.namespace }}
{{- /* find job manager */}}
{{- $jm_component := fromJson "{}" }}
{{- range $i, $e := $.cluster.spec.componentSpecs }}
  {{- if eq $e.componentDefRef "jobmanager" }}
    {{- $jm_component = $e }}
  {{- end }}
{{- end }}
# get port
{{- $blob_port_info := getPortByName ( index $.podSpec.containers 0 ) "tcp-blob" }}
{{- $rpc_port_info := getPortByName ( index $.podSpec.containers 0 ) "tcp-rpc" }}
{{- $rest_port_info := getPortByName ( index $.podSpec.containers 0 ) "tcp-http" }}
# get mem
{{- $phy_memory := getContainerMemory ( index $.podSpec.containers 0 ) }}
{{- $default_rpc_port := 6123 }}
{{- $default_blob_port := 6124 }}
{{- $default_rest_port := 8081 }}
{{- if $rpc_port_info }}
{{- $default_rpc_port = $rpc_port_info.containerPort }}
{{- end }}
{{- if $blob_port_info }}
{{- $default_blob_port = $blob_port_info.containerPort }}
{{- end }}
{{- if $rest_port_info }}
{{- $default_rest_port = $rest_port_info.containerPort }}
{{- end }}
blob.server.port: {{ $default_blob_port }}
jobmanager.bind-host: 0.0.0.0
jobmanager.execution.failover-strategy: region
jobmanager.memory.process.size: {{ $phy_memory }}
jobmanager.rpc.address: {{ $clusterName }}-{{ $jm_component.name }}
jobmanager.rpc.bind-port: {{ $default_rpc_port }}
jobmanager.rpc.port:  {{ $default_rpc_port }}
parallelism.default: 1
rest.address: {{ $clusterName }}-{{ $jm_component.name }}
rest.bind-address: 0.0.0.0
rest.port: {{ $default_rest_port }}