[databases]
* = host=127.0.0.1 port=5432

[pgbouncer]
listen_addr = 0.0.0.0
listen_port = 6432
unix_socket_dir =
user = postgres
auth_file = /etc/pgbouncer/userlist.txt
auth_type = md5
pool_mode = transaction
max_client_conn = 100
ignore_startup_parameters = extra_float_digits

# Log settings
admin_users = postgres