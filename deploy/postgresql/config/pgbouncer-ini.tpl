[databases]
* = host=127.0.0.1 port=5432

[pgbouncer]
listen_addr = 0.0.0.0
listen_port = 6432
unix_socket_dir =
user = postgres
auth_file = /etc/pgbouncer/userlist.txt
auth_type = md5
pool_mode = session
ignore_startup_parameters = extra_float_digits
{{- $max_client_conn := 10000 }}
{{- $phy_memory := getContainerMemory ( index $.podSpec.containers 0 ) }}
{{- if gt $phy_memory 0 }}
{{- $max_client_conn = min ( div $phy_memory 9531392 ) 5000 }}
{{- end }}
max_client_conn = {{ $max_client_conn }}

# Log settings
admin_users = postgres