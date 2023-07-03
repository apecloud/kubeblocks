[pgbouncer]
listen_addr = 0.0.0.0
listen_port = 6432
unix_socket_dir = /tmp/
unix_socket_mode = 0777
auth_file = /opt/bitnami/pgbouncer/conf/userlist.txt
auth_user = postgres
auth_query = SELECT usename, passwd FROM pg_shadow WHERE usename=$1
pidfile =/opt/bitnami/pgbouncer/tmp/pgbouncer.pid
logfile =/opt/bitnami/pgbouncer/logs/pgbouncer.log
auth_type = md5
pool_mode = session
ignore_startup_parameters = extra_float_digits
{{- $max_client_conn := 10000 }}
{{- $phy_memory := getContainerMemory ( index $.podSpec.containers 0 ) }}
{{- if gt $phy_memory 0 }}
{{- $max_client_conn = min ( div $phy_memory 9531392 ) 5000 }}
{{- end }}
max_client_conn = {{ $max_client_conn }}
admin_users = postgres
;;; [database]
;;; config default database in pgbouncer_setup.sh