# - Connection Settings -

{{- $buffer_unit := "B" }}
{{- $shared_buffers := 1073741824 }}
{{- $max_connections := 10000 }}
{{- $phy_memory := getContainerMemory ( index $.podSpec.containers 0 ) }}
{{- if gt $phy_memory 0 }}
{{- $shared_buffers = div $phy_memory 4 }}
{{- $max_connections = min ( div $phy_memory 9531392 ) 5000 }}
{{- end -}}

{{- if ge $shared_buffers 1024 }}
{{- $shared_buffers = div $shared_buffers 1024 }}
{{- $buffer_unit = "KB" }}
{{- end -}}

{{- if ge $shared_buffers 1024 }}
{{- $shared_buffers = div $shared_buffers 1024 }}
{{- $buffer_unit = "MB" }}
{{- end -}}

{{- if ge $shared_buffers 1024 }}
{{- $shared_buffers = div $shared_buffers 1024 }}
{{ $buffer_unit = "GB" }}
{{- end -}}

listen_addresses = '*'
port = '5432'
#archive_command = 'wal_dir=/pg/arcwal; [[ $(date +%H%M) == 1200 ]] && rm -rf ${wal_dir}/$(date -d"yesterday" +%Y%m%d); /bin/mkdir -p ${wal_dir}/$(date +%Y%m%d) && /usr/bin/lz4 -q -z %p > ${wal_dir}/$(date +%Y%m%d)/%f.lz4'
#archive_mode = 'True'
auto_explain.log_analyze = 'True'
auto_explain.log_min_duration = '1s'
auto_explain.log_nested_statements = 'True'
auto_explain.log_timing = 'True'
auto_explain.log_verbose = 'True'
autovacuum_analyze_scale_factor = '0.05'
autovacuum_freeze_max_age = '100000000'
autovacuum_max_workers = '1'
autovacuum_naptime = '1min'
autovacuum_vacuum_cost_delay = '-1'
autovacuum_vacuum_cost_limit = '-1'
autovacuum_vacuum_scale_factor = '0.1'
bgwriter_delay = '10ms'
bgwriter_lru_maxpages = '800'
bgwriter_lru_multiplier = '5.0'
checkpoint_completion_target = '0.95'
checkpoint_timeout = '10min'
commit_delay = '20'
commit_siblings = '10'
deadlock_timeout = '50ms'
default_statistics_target = '500'
effective_cache_size = '12GB'
hot_standby = 'on'
hot_standby_feedback = 'True'
huge_pages = 'try'
idle_in_transaction_session_timeout = '1h'
listen_addresses = '0.0.0.0'
log_autovacuum_min_duration = '1s'
log_checkpoints = 'True'
log_destination = 'csvlog'
log_directory = 'log'
log_filename = 'postgresql-%Y-%m-%d.log'
log_lock_waits = 'True'
log_min_duration_statement = '100'
log_replication_commands = 'True'
log_statement = 'ddl'
logging_collector = 'True'
#maintenance_work_mem = '3952MB'
max_connections = '{{ $max_connections }}'
max_locks_per_transaction = '128'
max_logical_replication_workers = '8'
max_parallel_maintenance_workers = '2'
max_parallel_workers = '8'
max_parallel_workers_per_gather = '0'
max_prepared_transactions = '0'
max_replication_slots = '16'
max_standby_archive_delay = '10min'
max_standby_streaming_delay = '3min'
max_sync_workers_per_subscription = '6'
max_wal_senders = '24'
max_wal_size = '100GB'
max_worker_processes = '8'
min_wal_size = '20GB'
password_encryption = 'md5'
pg_stat_statements.max = '5000'
pg_stat_statements.track = 'all'
pg_stat_statements.track_planning = 'False'
pg_stat_statements.track_utility = 'False'
random_page_cost = '1.1'
#auto generated
shared_buffers = '{{ printf "%d%s" $shared_buffers $buffer_unit }}'
shared_preload_libraries = 'pg_stat_statements, auto_explain'
superuser_reserved_connections = '10'
temp_file_limit = '100GB'
#timescaledb.max_background_workers = '6'
#timescaledb.telemetry_level = 'off'
track_activity_query_size = '8192'
track_commit_timestamp = 'True'
track_functions = 'all'
track_io_timing = 'True'
vacuum_cost_delay = '2ms'
vacuum_cost_limit = '10000'
vacuum_defer_cleanup_age = '50000'
wal_buffers = '16MB'
wal_keep_size = '20GB'
wal_level = 'replica'
wal_log_hints = 'on'
wal_receiver_status_interval = '1s'
wal_receiver_timeout = '60s'
wal_writer_delay = '20ms'
wal_writer_flush_after = '1MB'
work_mem = '32MB'