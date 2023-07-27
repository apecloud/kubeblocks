[vtgate]
gateway_initial_tablet_timeout=30s
healthcheck_timeout=2s
srv_topo_timeout=1s
grpc_keepalive_time=10s
grpc_keepalive_timeout=10s
tablet_refresh_interval=1m
read_write_splitting_policy=disable
read_write_splitting_ratio=100
read_after_write_consistency=EVENTUAL
read_after_write_timeout=30.0
enable_buffer=false
buffer_size=10000
buffer_window=30s
buffer_max_failover_duration=60s
buffer_min_time_between_failovers=60s
mysql_auth_server_impl=none
mysql_server_require_secure_transport=false
mysql_auth_server_static_file=
mysql_server_ssl_key=
mysql_server_ssl_cert=

{{ block "logsBlock" . }}
{{- if hasKey $.component "enabledLogs" }}
enable_logs=true
{{- if mustHas "queryLog" $.component.enabledLogs }}
enable_query_log=true
{{- end }}
{{- end }}
{{ end }}