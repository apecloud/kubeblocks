[mysqld]
{{- $phy_memory := getContainerMemory ( index $.podSpec.containers 0 ) }}
{{- $pool_buffer_size := ( callBufferSizeByResource ( index $.podSpec.containers 0 ) ) }}
{{- if $pool_buffer_size }}
innodb_buffer_pool_size={{ $pool_buffer_size }}
{{- end }}

{{- $thread_stack := 262144 }}
{{- $binlog_cache_size := 32768 }}
{{- $join_buffer_size := 262144 }}
{{- $sort_buffer_size := 262144 }}
{{- $read_buffer_size := 262144 }}
{{- $read_rnd_buffer_size := 524288 }}
{{- $single_thread_memory := add $thread_stack $binlog_cache_size $join_buffer_size $sort_buffer_size $read_buffer_size $read_rnd_buffer_size }}

{{- if gt $phy_memory 0 }}
# Global_Buffer = innodb_buffer_pool_size = PhysicalMemory *3/4
# max_connections = (PhysicalMemory  - Global_Buffer) / single_thread_memory
max_connections={{ div ( div $phy_memory 4 ) $single_thread_memory }}
{{- end}}

# if memory less than 8Gi, disable performance_schema
{{- if lt $phy_memory 8589934592 }}
performance_schema=OFF
{{- end }}

read_buffer_size={{ $read_buffer_size }}
read_rnd_buffer_size={{ $read_rnd_buffer_size }}
join_buffer_size={{ $join_buffer_size }}
sort_buffer_size={{ $sort_buffer_size }}

# gtid
gtid_mode=ON
enforce_gtid_consistency=ON

port=3306
mysqlx-port=33060
mysqlx=0

pid-file=/var/run/mysqld/mysqld.pid
socket=/var/run/mysqld/mysqld.sock

innodb_flush_log_at_trx_commit = 2
sync_binlog = 1000

[client]
port=3306
socket=/var/run/mysqld/mysqld.sock