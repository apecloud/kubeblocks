# - Connection Settings -

{{- $buffer_unit := "B" }}
{{- $shared_buffers := 1073741824 }}
{{- $max_connections := 10000 }}
{{- $autovacuum_max_workers := 3 }}
{{- $phy_memory := getContainerMemory ( index $.podSpec.containers 0 ) }}
{{- $phy_cpu := getContainerCPU ( index $.podSpec.containers 0 ) }}
{{- if gt $phy_memory 0 }}
{{- $shared_buffers = div $phy_memory 4 }}
{{- $max_connections = min ( div $phy_memory 9531392 ) 5000 }}
{{- $autovacuum_max_workers = min ( max ( div $phy_memory 17179869184 ) 3 ) 10 }}
{{- end }}

{{- if ge $shared_buffers 1024 }}
{{- $shared_buffers = div $shared_buffers 1024 }}
{{- $buffer_unit = "kB" }}
{{- end }}

{{- if ge $shared_buffers 1024 }}
{{- $shared_buffers = div $shared_buffers 1024 }}
{{- $buffer_unit = "MB" }}
{{- end }}

{{- if ge $shared_buffers 1024 }}
{{- $shared_buffers = div $shared_buffers 1024 }}
{{ $buffer_unit = "GB" }}
{{- end }}

listen_addresses = '*'
port = '5432'
# archive_command = 'wal_dir=/home/postgres/pgdata/pgroot/arcwal; wal_dir_today=${wal_dir}/$(date +%Y%m%d); [[ $(date +%H%M) == 1200 ]] && rm -rf ${wal_dir}/$(date -d"yesterday" +%Y%m%d); mkdir -p ${wal_dir_today} && gzip -kqc %p > ${wal_dir_today}/%f.gz'
archive_command = 'if [ $(date +%H%M) -eq 1200 ]; then rm -rf /home/postgres/pgdata/pgroot/arcwal/$(date -d"yesterday" +%Y%m%d); fi; mkdir -p /home/postgres/pgdata/pgroot/arcwal/$(date +%Y%m%d) && gzip -kqc %p > /home/postgres/pgdata/pgroot/arcwal/$(date +%Y%m%d)/%f.gz && sync /home/postgres/pgdata/pgroot/arcwal/$(date +%Y%m%d)/%f.gz'
archive_mode = 'on'
auto_explain.log_analyze = 'False'
auto_explain.log_buffers = 'False'
auto_explain.log_format = 'text'
auto_explain.log_min_duration = '-1'
auto_explain.log_nested_statements = 'False'
auto_explain.log_timing = 'True'
auto_explain.log_triggers = 'False'
auto_explain.log_verbose = 'False'
auto_explain.sample_rate = '1'
autovacuum_analyze_scale_factor = '0.1'
autovacuum_analyze_threshold = '50'
autovacuum_freeze_max_age = '200000000'
autovacuum_max_workers = '{{ $autovacuum_max_workers }}'
autovacuum_multixact_freeze_max_age = '400000000'
autovacuum_naptime = '15s'
autovacuum_vacuum_cost_delay = '2'
autovacuum_vacuum_cost_limit = '200'
autovacuum_vacuum_scale_factor = '0.05'
autovacuum_vacuum_threshold = '50'
{{- if gt $phy_memory 0 }}
autovacuum_work_mem = '{{ printf "%dkB" ( max ( div $phy_memory 65536 ) 131072 ) }}'
{{- end }}
backend_flush_after = '0'
backslash_quote = 'safe_encoding'
bgwriter_delay = '200ms'
bgwriter_flush_after = '64'
bgwriter_lru_maxpages = '1000'
bgwriter_lru_multiplier = '10.0'
bytea_output = 'hex'
check_function_bodies = 'True'
checkpoint_completion_target = '0.4'
checkpoint_flush_after = '32'
checkpoint_timeout = '25min'
checkpoint_warning = '30s'
client_min_messages = 'notice'
# commit_delay = '20'
commit_siblings = '5'
constraint_exclusion = 'partition'

#extension: pg_cron
cron.database_name = 'postgres'
cron.log_statement = 'on'
cron.max_running_jobs = '32'

cursor_tuple_fraction = '0.1'
datestyle = 'ISO,YMD'
deadlock_timeout = '1000ms'
debug_pretty_print = 'True'
debug_print_parse = 'False'
debug_print_plan = 'False'
debug_print_rewritten = 'False'
default_statistics_target = '100'
default_transaction_deferrable = 'False'
default_transaction_isolation = 'read committed'
# unit 8KB
{{- if gt $phy_memory 0 }}
effective_cache_size = '{{ printf "%dMB" ( div ( div $phy_memory 16384 ) 128 ) }}'
{{- end }}
effective_io_concurrency = '1'
enable_bitmapscan = 'True'
enable_gathermerge = 'True'
enable_hashagg = 'True'
enable_hashjoin = 'True'
enable_indexonlyscan = 'True'
enable_indexscan = 'True'
enable_material = 'True'
enable_mergejoin = 'True'
enable_nestloop = 'True'
enable_parallel_append = 'True'
enable_parallel_hash = 'True'
enable_partition_pruning = 'True'
# patroni off
enable_partitionwise_aggregate = 'True'
# patroni off
enable_partitionwise_join = 'True'
enable_seqscan = 'True'
enable_sort = 'True'
enable_tidscan = 'True'
escape_string_warning = 'True'
extra_float_digits = '1'
force_parallel_mode = '0'
from_collapse_limit = '8'
#fsync=off # patroni for Extreme Performance
#full_page_writes=off # patroni for Extreme Performance
geqo = 'True'
geqo_effort = '5'
geqo_generations = '0'
geqo_pool_size = '0'
geqo_seed = '0'
geqo_selection_bias = '2'
geqo_threshold = '12'
gin_fuzzy_search_limit = '0'
gin_pending_list_limit = '4096kB'
# patroni on
hot_standby_feedback = 'False'
# rds huge_pages=on, patroni try
huge_pages = 'try'
#patroni 10min
idle_in_transaction_session_timeout = '3600000ms'
index_adviser.enable_log = 'on'
index_adviser.max_aggregation_column_count = '10'
index_adviser.max_candidate_index_count = '500'
intervalstyle = 'postgres'
join_collapse_limit = '8'
lc_monetary = 'C'
lc_numeric = 'C'
lc_time = 'C'
lock_timeout = '0'
# patroni 1s
log_autovacuum_min_duration = '10000'
log_checkpoints = 'True'
log_connections = 'False'
log_disconnections = 'False'
log_duration = 'False'
log_executor_stats = 'False'

{{- block "logsBlock" . }}
{{- if hasKey $.component "enabledLogs" }}
{{- if mustHas "running" $.component.enabledLogs }}
logging_collector = 'True'
log_destination = 'csvlog'
log_directory = 'log'
log_filename = 'postgresql-%Y-%m-%d.log'
{{ end -}}
{{ end -}}
{{ end }}

# log_lock_waits = 'True'
log_min_duration_statement = '1000'
log_parser_stats = 'False'
log_planner_stats = 'False'
log_replication_commands = 'False'
log_statement = 'ddl'
log_statement_stats = 'False'
log_temp_files = '128kB'
log_transaction_sample_rate = '0'
#maintenance_work_mem = '3952MB'
max_connections = '{{ $max_connections }}'
max_files_per_process = '1000'
max_logical_replication_workers = '32'
max_locks_per_transaction = '64'
max_parallel_maintenance_workers = '{{ max ( div $phy_cpu 2 ) 2 }}'
max_parallel_workers = '{{ max ( div ( mul $phy_cpu 3 ) 4 ) 8 }}'
max_parallel_workers_per_gather = '{{ max ( div $phy_cpu 2 ) 2 }}'
max_pred_locks_per_page = '2'
max_pred_locks_per_relation = '-2'
max_pred_locks_per_transaction = '64'
max_prepared_transactions = '100'
max_replication_slots = '16'
max_stack_depth = '2MB'

max_standby_archive_delay = '300000ms'
max_standby_streaming_delay = '300000ms'
max_sync_workers_per_subscription = '2'
max_wal_senders = '64'
# {LEAST(GREATEST(DBInstanceClassMemory/2097152, 2048), 16384)}
max_wal_size = '{{ printf "%dMB" ( min ( max ( div $phy_memory 2097152 ) 2048 ) 16384 ) }}'
max_worker_processes = '{{ max $phy_cpu 8 }}'
# min_parallel_index_scan_size unit is 8KB, 64 = 512KB
min_parallel_index_scan_size = '512kB'
# min_parallel_table_scan_size unit is 8KB, 1024 = 8MB
min_parallel_table_scan_size = '8MB'
{{- if gt $phy_memory 0 }}
# min_wal_size={LEAST(GREATEST(DBInstanceClassMemory/8388608, 256), 8192)} # patroni 1/20 disk size
min_wal_size = '{{ printf "%dMB" ( min ( max ( div $phy_memory 8388608 ) 256 ) 8192 ) }}'
{{- end }}

old_snapshot_threshold = '-1'
operator_precedence_warning = 'off'
parallel_leader_participation = 'True'

password_encryption = 'md5'
pg_stat_statements.max = '5000'
pg_stat_statements.save = 'False'

# patroni all
pg_stat_statements.track = 'top'
# pg_stat_statements.track_planning = 'False'
pg_stat_statements.track_utility = 'False'

#extension: pgaudit
pgaudit.log_catalog = 'True'
pgaudit.log_level = 'log'
pgaudit.log_parameter = 'False'
pgaudit.log_relation = 'False'
pgaudit.log_statement_once = 'False'
# TODO
# pgaudit.role = ''

#extension: pglogical
pglogical.batch_inserts = 'True'
pglogical.conflict_log_level = 'log'
pglogical.conflict_resolution = 'apply_remote'
# TODO
# pglogical.extra_connection_options = ''
pglogical.synchronous_commit = 'False'
pglogical.use_spi = 'False'
plan_cache_mode = 'auto'
quote_all_identifiers = 'False'

random_page_cost = '1.1'
row_security = 'True'
session_replication_role = 'origin'

#extension: sql_firewall
sql_firewall.firewall = 'disable'

#auto generated
shared_buffers = '{{ printf "%d%s" $shared_buffers $buffer_unit }}'
# shared_preload_libraries = 'pg_stat_statements,auto_explain,bg_mon,pgextwlist,pg_auth_mon,set_user,pg_cron,pg_stat_kcache'

{{- if $.component.tls }}
{{- $ca_file := getCAFile }}
{{- $cert_file := getCertFile }}
{{- $key_file := getKeyFile }}
# tls
ssl = 'True'
ssl_ca_file = '{{ $ca_file }}'
ssl_cert_file = '{{ $cert_file }}'
ssl_key_file = '{{ $key_file }}'
{{- end }}

# ssl_max_protocol_version=''
ssl_min_protocol_version = 'TLSv1'
standard_conforming_strings = 'True'
statement_timeout = '0'
#patroni 10
superuser_reserved_connections = '20'
synchronize_seqscans = 'True'

# rds off ，patroni off for Extreme Performance
synchronous_commit = 'off'
# synchronous_standby_names=''
tcp_keepalives_count = '10'
tcp_keepalives_idle = '45s'
tcp_keepalives_interval = '10s'
temp_buffers = '8MB'

# {DBInstanceClassMemory/1024}
{{- if gt $phy_memory 0 }}
temp_file_limit = '{{ printf "%dkB" ( div $phy_memory 1024 ) }}'
{{- end }}

#extension: timescaledb
#timescaledb.max_background_workers = '6'
#timescaledb.telemetry_level = 'off'
# TODO timezone
# timezone=Asia/Shanghai
track_activity_query_size = '4096'
track_commit_timestamp = 'False'
track_functions = 'pl'
track_io_timing = 'True'
transform_null_equals = 'False'


vacuum_cleanup_index_scale_factor = '0.1'
# patroni 20ms
vacuum_cost_delay = '0'
# patroni 2000
vacuum_cost_limit = '10000'
vacuum_cost_page_dirty = '20'
vacuum_cost_page_hit = '1'
vacuum_cost_page_miss = '2'
# patroni 50000
vacuum_defer_cleanup_age = '0'
vacuum_freeze_min_age = '50000000'
vacuum_freeze_table_age = '200000000'
vacuum_multixact_freeze_min_age = '5000000'
vacuum_multixact_freeze_table_age = '200000000'
# wal_buffers ={LEAST(GREATEST(DBInstanceClassMemory/2097152, 2048), 16384)} # patroni 16M
# unit 8KB
wal_buffers = '{{ printf "%dMB" ( div ( min ( max ( div $phy_memory 2097152 ) 2048) 16384 ) 128 ) }}'
wal_compression = 'True'
wal_keep_segments = '128'
# patroni minimal for Extreme Performance
wal_level = 'replica'
# patroni on , off for Extreme Performance
wal_log_hints = 'False'
wal_receiver_status_interval = '1s'
wal_receiver_timeout = '60000'
wal_sender_timeout = '60000'
# patroni 20ms
wal_writer_delay = '200ms'
# rds unit 8KB, so 1M, patroni 1M
wal_writer_flush_after = '1MB'
# {GREATEST(DBInstanceClassMemory/4194304, 4096)}
work_mem = '{{ printf "%dkB" ( max ( div $phy_memory 4194304 ) 4096 ) }}'
xmlbinary = 'base64'
xmloption = 'content'