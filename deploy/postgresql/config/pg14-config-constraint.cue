// Copyright ApeCloud, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// PostgreSQL parameters: https://postgresqlco.nf/doc/en/param/
#PGParameter: {
	// Allows tablespaces directly inside pg_tblspc, for testing, pg version: 15
	allow_in_place_tablespaces?: bool
	// Allows modification of the structure of system tables as well as certain other risky actions on system tables. This is otherwise not allowed even for superusers. Ill-advised use of this setting can cause irretrievable data loss or seriously corrupt the database system. 
	allow_system_table_mods?: bool
	// Sets the application name to be reported in statistics and logs.
	application_name?: string
	// Sets the shell command that will be called to archive a WAL file.
	archive_command?: string
	// The library to use for archiving completed WAL file segments. If set to an empty string (the default), archiving via shell is enabled, and archive_command is used. Otherwise, the specified shared library is used for archiving. The WAL archiver process is restarted by the postmaster when this parameter changes. For more information, see backup-archiving-wal and archive-modules.
	archive_library?: string
	// When archive_mode is enabled, completed WAL segments are sent to archive storage by setting archive_command or guc-archive-library. In addition to off, to disable, there are two modes: on, and always. During normal operation, there is no difference between the two modes, but when set to always the WAL archiver is enabled also during archive recovery or standby mode. In always mode, all files restored from the archive or streamed with streaming replication will be archived (again). See continuous-archiving-in-standby for details.
	archive_mode: string & "always" | "on" | "off"
	// (s) Forces a switch to the next xlog file if a new file has not been started within N seconds.
	archive_timeout: int & >=0 & <=2147483647 | *300 @timeDurationResource(1s)
	// Enable input of NULL elements in arrays.
	array_nulls?: bool
	// (s) Sets the maximum allowed time to complete client authentication.
	authentication_timeout?: int & >=1 & <=600 @timeDurationResource(1s)
	// Use EXPLAIN ANALYZE for plan logging.
	"auto_explain.log_analyze"?: bool
	// Log buffers usage.
	"auto_explain.log_buffers"?: bool & false | true
	// EXPLAIN format to be used for plan logging.
	"auto_explain.log_format"?: string & "text" | "xml" | "json" | "yaml"

	// (ms) Sets the minimum execution time above which plans will be logged.
	"auto_explain.log_min_duration"?: int & >=-1 & <=2147483647 @timeDurationResource()

	// Log nested statements.
	"auto_explain.log_nested_statements"?: bool & false | true

	// Collect timing data, not just row counts.
	"auto_explain.log_timing"?: bool & false | true

	// Include trigger statistics in plans.
	"auto_explain.log_triggers"?: bool & false | true

	// Use EXPLAIN VERBOSE for plan logging.
	"auto_explain.log_verbose"?: bool & false | true

	// Fraction of queries to process.
	"auto_explain.sample_rate"?: float & >=0 & <=1

	// Starts the autovacuum subprocess.
	autovacuum?: bool

	// Number of tuple inserts, updates or deletes prior to analyze as a fraction of reltuples.
	autovacuum_analyze_scale_factor: float & >=0 & <=100 | *0.05

	// Minimum number of tuple inserts, updates or deletes prior to analyze.
	autovacuum_analyze_threshold?: int & >=0 & <=2147483647

	// Age at which to autovacuum a table to prevent transaction ID wraparound.
	autovacuum_freeze_max_age?: int & >=100000 & <=2000000000

	// Sets the maximum number of simultaneously running autovacuum worker processes.
	autovacuum_max_workers?: int & >=1 & <=8388607

	// Multixact age at which to autovacuum a table to prevent multixact wraparound.
	autovacuum_multixact_freeze_max_age?: int & >=10000000 & <=2000000000

	// (s) Time to sleep between autovacuum runs.
	autovacuum_naptime: int & >=1 & <=2147483 | *15 @timeDurationResource(1s)

	// (ms) Vacuum cost delay in milliseconds, for autovacuum.
	autovacuum_vacuum_cost_delay?: int & >=-1 & <=100 @timeDurationResource()

	// Vacuum cost amount available before napping, for autovacuum.
	autovacuum_vacuum_cost_limit?: int & >=-1 & <=10000

	// Number of tuple inserts prior to vacuum as a fraction of reltuples.
	autovacuum_vacuum_insert_scale_factor?: float & >=0 & <=100

	// Minimum number of tuple inserts prior to vacuum, or -1 to disable insert vacuums.
	autovacuum_vacuum_insert_threshold?: int & >=-1 & <=2147483647

	// Number of tuple updates or deletes prior to vacuum as a fraction of reltuples.
	autovacuum_vacuum_scale_factor: float & >=0 & <=100 | *0.1

	// Minimum number of tuple updates or deletes prior to vacuum.
	autovacuum_vacuum_threshold?: int & >=0 & <=2147483647

	// (kB) Sets the maximum memory to be used by each autovacuum worker process.
	autovacuum_work_mem?: int & >=-1 & <=2147483647 @storeResource(1KB)

	// (8Kb) Number of pages after which previously performed writes are flushed to disk.
	backend_flush_after?: int & >=0 & <=256 @storeResource(8KB)

	// Sets whether "\" is allowed in string literals.
	backslash_quote?: string & "safe_encoding" | "on" | "off"

	// Log backtrace for errors in these functions.
	backtrace_functions?: string

	// (ms) Background writer sleep time between rounds.
	bgwriter_delay?: int & >=10 & <=10000 @timeDurationResource()

	// (8Kb) Number of pages after which previously performed writes are flushed to disk.
	bgwriter_flush_after?: int & >=0 & <=256 @storeResource(8KB)

	// Background writer maximum number of LRU pages to flush per round.
	bgwriter_lru_maxpages?: int & >=0 & <=1000

	// Multiple of the average buffer usage to free per round.
	bgwriter_lru_multiplier?: float & >=0 & <=10

	// Sets the output format for bytea.
	bytea_output?: string & "escape" | "hex"

	// Check function bodies during CREATE FUNCTION.
	check_function_bodies?: bool & false | true

	// Time spent flushing dirty buffers during checkpoint, as fraction of checkpoint interval.
	checkpoint_completion_target: float & >=0 & <=1 | *0.9

	// (8kB) Number of pages after which previously performed writes are flushed to disk.
	checkpoint_flush_after?: int & >=0 & <=256 @storeResource(8KB)

	// (s) Sets the maximum time between automatic WAL checkpoints.
	checkpoint_timeout?: int & >=30 & <=3600 @timeDurationResource(1s)

	// (s) Enables warnings if checkpoint segments are filled more frequently than this.
	checkpoint_warning?: int & >=0 & <=2147483647 @timeDurationResource(1s)

	// time between checks for client disconnection while running queries
	client_connection_check_interval?: int & >=0 & <=2147483647 @timeDurationResource()

	// Sets the clients character set encoding.
	client_encoding?: string

	// Sets the message levels that are sent to the client.
	client_min_messages?: string & "debug5" | "debug4" | "debug3" | "debug2" | "debug1" | "log" | "notice" | "warning" | "error"

	// Sets the delay in microseconds between transaction commit and flushing WAL to disk.
	commit_delay?: int & >=0 & <=100000

	// Sets the minimum concurrent open transactions before performing commit_delay.
	commit_siblings?: int & >=0 & <=1000

	// Enables in-core computation of a query identifier
	compute_query_id?: string & "on" | "auto"

	// Sets the servers main configuration file.
	config_file?: string

	// Enables the planner to use constraints to optimize queries.
	constraint_exclusion?: string & "partition" | "on" | "off"

	// Sets the planners estimate of the cost of processing each index entry during an index scan.
	cpu_index_tuple_cost?: float & >=0 & <=1.79769e+308

	// Sets the planners estimate of the cost of processing each operator or function call.
	cpu_operator_cost?: float & >=0 & <=1.79769e+308

	// Sets the planners estimate of the cost of processing each tuple (row).
	cpu_tuple_cost?: float & >=0 & <=1.79769e+308

	// Sets the database to store pg_cron metadata tables
	"cron.database_name"?: string

	// Log all jobs runs into the job_run_details table
	"cron.log_run"?: string & "on" | "off"

	// Log all cron statements prior to execution.
	"cron.log_statement"?: string & "on" | "off"

	// Maximum number of jobs that can run concurrently.
	"cron.max_running_jobs": int & >=0 & <=100 | *5

	// Enables background workers for pg_cron
	"cron.use_background_workers"?: string

	// Sets the planners estimate of the fraction of a cursors rows that will be retrieved.
	cursor_tuple_fraction?: float & >=0 & <=1

	// Sets the servers data directory.
	data_directory?: string

	// Sets the display format for date and time values.
	datestyle?: string

	// Enables per-database user names.
	db_user_namespace?: bool & false | true

	// (ms) Sets the time to wait on a lock before checking for deadlock.
	deadlock_timeout?: int & >=1 & <=2147483647 @timeDurationResource()

	// Indents parse and plan tree displays.
	debug_pretty_print?: bool & false | true

	// Logs each querys parse tree.
	debug_print_parse?: bool & false | true

	// Logs each querys execution plan.
	debug_print_plan?: bool & false | true

	// Logs each querys rewritten parse tree.
	debug_print_rewritten?: bool & false | true

	// Sets the default statistics target.
	default_statistics_target?: int & >=1 & <=10000

	// Sets the default tablespace to create tables and indexes in.
	default_tablespace?: string

	// Sets the default TOAST compression method for columns of newly-created tables
	default_toast_compression?: string & "pglz" | "lz4"

	// Sets the default deferrable status of new transactions.
	default_transaction_deferrable?: bool & false | true

	// Sets the transaction isolation level of each new transaction.
	default_transaction_isolation?: string & "serializable" | "repeatable read" | "read committed" | "read uncommitted"

	// Sets the default read-only status of new transactions.
	default_transaction_read_only?: bool & false | true

	// (8kB) Sets the planners assumption about the size of the disk cache.
	effective_cache_size?: int & >=1 & <=2147483647 @storeResource(8KB)

	// Number of simultaneous requests that can be handled efficiently by the disk subsystem.
	effective_io_concurrency?: int & >=0 & <=1000

	// Enables or disables the query planner's use of async-aware append plan types
	enable_async_append?: bool & false | true

	// Enables the planners use of bitmap-scan plans.
	enable_bitmapscan?: bool & false | true

	// Enables the planner's use of gather merge plans.
	enable_gathermerge?: bool & false | true

	// Enables the planners use of hashed aggregation plans.
	enable_hashagg?: bool & false | true

	// Enables the planners use of hash join plans.
	enable_hashjoin?: bool & false | true

	// Enables the planner's use of incremental sort steps.
	enable_incremental_sort?: bool & false | true

	// Enables the planner's use of index-only-scan plans.
	enable_indexonlyscan?: bool & false | true

	// Enables the planners use of index-scan plans.
	enable_indexscan?: bool & false | true

	// Enables the planners use of materialization.
	enable_material?: bool & false | true

	// Enables the planner's use of memoization
	enable_memoize?: bool & false | true

	// Enables the planners use of merge join plans.
	enable_mergejoin?: bool & false | true

	// Enables the planners use of nested-loop join plans.
	enable_nestloop?: bool & false | true

	// Enables the planner's use of parallel append plans.
	enable_parallel_append?: bool & false | true

	// Enables the planner's user of parallel hash plans.
	enable_parallel_hash?: bool & false | true

	// Enable plan-time and run-time partition pruning.
	enable_partition_pruning?: bool & false | true

	// Enables partitionwise aggregation and grouping.
	enable_partitionwise_aggregate?: bool & false | true

	// Enables partitionwise join.
	enable_partitionwise_join?: bool & false | true

	// Enables the planners use of sequential-scan plans.
	enable_seqscan?: bool & false | true

	// Enables the planners use of explicit sort steps.
	enable_sort?: bool & false | true

	// Enables the planners use of TID scan plans.
	enable_tidscan?: bool & false | true

	// Warn about backslash escapes in ordinary string literals.
	escape_string_warning?: bool & false | true

	// Terminate session on any error.
	exit_on_error?: bool & false | true

	// Sets the number of digits displayed for floating-point values.
	extra_float_digits?: int & >=-15 & <=3

	// Forces use of parallel query facilities.
	force_parallel_mode?: bool & false | true

	// Sets the FROM-list size beyond which subqueries are not collapsed.
	from_collapse_limit?: int & >=1 & <=2147483647

	// Forces synchronization of updates to disk.
	fsync: bool & false | true | *true

	// Writes full pages to WAL when first modified after a checkpoint.
	full_page_writes: bool & false | true | *true

	// Enables genetic query optimization.
	geqo?: bool & false | true

	// GEQO: effort is used to set the default for other GEQO parameters.
	geqo_effort?: int & >=1 & <=10

	// GEQO: number of iterations of the algorithm.
	geqo_generations?: int & >=0 & <=2147483647

	// GEQO: number of individuals in the population.
	geqo_pool_size?: int & >=0 & <=2147483647 @storeResource()

	// GEQO: seed for random path selection.
	geqo_seed?: float & >=0 & <=1

	// GEQO: selective pressure within the population.
	geqo_selection_bias?: float & >=1.5 & <=2

	// Sets the threshold of FROM items beyond which GEQO is used.
	geqo_threshold?: int & >=2 & <=2147483647

	// Sets the maximum allowed result for exact search by GIN.
	gin_fuzzy_search_limit?: int & >=0 & <=2147483647

	// (kB) Sets the maximum size of the pending list for GIN index.
	gin_pending_list_limit?: int & >=64 & <=2147483647 @storeResource(1KB)

	// Multiple of work_mem to use for hash tables.
	hash_mem_multiplier?: float & >=1 & <=1000

	// Sets the servers hba configuration file.
	hba_file?: string

	// Force group aggregation for hll
	"hll.force_groupagg"?: bool & false | true

	// Allows feedback from a hot standby to the primary that will avoid query conflicts.
	hot_standby_feedback?: bool & false | true

	// Use of huge pages on Linux.
	huge_pages?: string & "on" | "off" | "try"

	// The size of huge page that should be requested. Controls the size of huge pages, when they are enabled with huge_pages. The default is zero (0). When set to 0, the default huge page size on the system will be used. This parameter can only be set at server start. 
	huge_page_size?: int & >=0 & <=2147483647 @storeResource(1KB)

	// Sets the servers ident configuration file.
	ident_file?: string

	// (ms) Sets the maximum allowed duration of any idling transaction.
	idle_in_transaction_session_timeout: int & >=0 & <=2147483647 | *86400000 @timeDurationResource()

	// Terminate any session that has been idle (that is, waiting for a client query), but not within an open transaction, for longer than the specified amount of time
	idle_session_timeout?: int & >=0 & <=2147483647 @timeDurationResource()

	// Continues recovery after an invalid pages failure.
	ignore_invalid_pages: bool & false | true | *false

	// Sets the display format for interval values.
	intervalstyle?: string & "postgres" | "postgres_verbose" | "sql_standard" | "iso_8601"

	// Allow JIT compilation.
	jit: bool

	// Perform JIT compilation if query is more expensive.
	jit_above_cost?: float & >=-1 & <=1.79769e+308

	// Perform JIT inlining if query is more expensive.
	jit_inline_above_cost?: float & >=-1 & <=1.79769e+308

	// Optimize JITed functions if query is more expensive.
	jit_optimize_above_cost?: float & >=-1 & <=1.79769e+308

	// Sets the FROM-list size beyond which JOIN constructs are not flattened.
	join_collapse_limit?: int & >=1 & <=2147483647

	// Sets the language in which messages are displayed.
	lc_messages?: string

	// Sets the locale for formatting monetary amounts.
	lc_monetary?: string

	// Sets the locale for formatting numbers.
	lc_numeric?: string

	// Sets the locale for formatting date and time values.
	lc_time?: string

	// Sets the host name or IP address(es) to listen to.
	listen_addresses?: string

	// Enables backward compatibility mode for privilege checks on large objects.
	lo_compat_privileges: bool & false | true | *false

	// (ms) Sets the minimum execution time above which autovacuum actions will be logged.
	log_autovacuum_min_duration: int & >=-1 & <=2147483647 | *10000 @timeDurationResource()

	// Logs each checkpoint.
	log_checkpoints: bool & false | true | *true

	// Logs each successful connection.
	log_connections?: bool & false | true

	// Sets the destination for server log output.
	log_destination?: string & "stderr" | "csvlog"

	// Sets the destination directory for log files.
	log_directory?: string

	// Logs end of a session, including duration.
	log_disconnections?: bool & false | true

	// Logs the duration of each completed SQL statement.
	log_duration?: bool & false | true

	// Sets the verbosity of logged messages.
	log_error_verbosity?: string & "terse" | "default" | "verbose"

	// Writes executor performance statistics to the server log.
	log_executor_stats?: bool & false | true

	// Sets the file permissions for log files.
	log_file_mode?: string

	// Sets the file name pattern for log files.
	log_filename?: string

	// Start a subprocess to capture stderr output and/or csvlogs into log files.
	logging_collector: bool & false | true | *true

	// Logs the host name in the connection logs.
	log_hostname?: bool & false | true

	// (kB) Sets the maximum memory to be used for logical decoding.
	logical_decoding_work_mem?: int & >=64 & <=2147483647 @storeResource(1KB)

	// Controls information prefixed to each log line.
	log_line_prefix?: string

	// Logs long lock waits.
	log_lock_waits?: bool & false | true

	// (ms) Sets the minimum execution time above which a sample of statements will be logged. Sampling is determined by log_statement_sample_rate.
	log_min_duration_sample?: int & >=-1 & <=2147483647 @timeDurationResource()

	// (ms) Sets the minimum execution time above which statements will be logged.
	log_min_duration_statement?: int & >=-1 & <=2147483647 @timeDurationResource()

	// Causes all statements generating error at or above this level to be logged.
	log_min_error_statement?: string & "debug5" | "debug4" | "debug3" | "debug2" | "debug1" | "info" | "notice" | "warning" | "error" | "log" | "fatal" | "panic"

	// Sets the message levels that are logged.
	log_min_messages?: string & "debug5" | "debug4" | "debug3" | "debug2" | "debug1" | "info" | "notice" | "warning" | "error" | "log" | "fatal"

	// When logging statements, limit logged parameter values to first N bytes.
	log_parameter_max_length?: int & >=-1 & <=1073741823

	// When reporting an error, limit logged parameter values to first N bytes.
	log_parameter_max_length_on_error?: int & >=-1 & <=1073741823

	// Writes parser performance statistics to the server log.
	log_parser_stats?: bool & false | true

	// Writes planner performance statistics to the server log.
	log_planner_stats?: bool & false | true

	// Controls whether a log message is produced when the startup process waits longer than deadlock_timeout for recovery conflicts
	log_recovery_conflict_waits?: bool & false | true

	// Logs each replication command.
	log_replication_commands?: bool & false | true

	// (min) Automatic log file rotation will occur after N minutes.
	log_rotation_age: int & >=1 & <=1440 | *60 @timeDurationResource(1min)

	// (kB) Automatic log file rotation will occur after N kilobytes.
	log_rotation_size?: int & >=0 & <=2097151 @storeResource(1KB)

	// Time between progress updates for long-running startup operations. Sets the amount of time after which the startup process will log a message about a long-running operation that is still in progress, as well as the interval between further progress messages for that operation. The default is 10 seconds. A setting of 0 disables the feature. If this value is specified without units, it is taken as milliseconds. This setting is applied separately to each operation. This parameter can only be set in the postgresql.conf file or on the server command line.  
	log_startup_progress_interval: int & >=0 & <=2147483647 @timeDurationResource()

	// Sets the type of statements logged.
	log_statement?: string & "none" | "ddl" | "mod" | "all"

	// Fraction of statements exceeding log_min_duration_sample to be logged.
	log_statement_sample_rate?: float & >=0 & <=1

	// Writes cumulative performance statistics to the server log.
	log_statement_stats?: bool

	// (kB) Log the use of temporary files larger than this number of kilobytes.
	log_temp_files?: int & >=-1 & <=2147483647 @storeResource(1KB)

	// Sets the time zone to use in log messages.
	log_timezone?: string

	// Set the fraction of transactions to log for new transactions.
	log_transaction_sample_rate?: float & >=0 & <=1

	// Truncate existing log files of same name during log rotation.
	log_truncate_on_rotation: bool & false | true | *false

	// A variant of effective_io_concurrency that is used for maintenance work.
	maintenance_io_concurrency?: int & >=0 & <=1000

	// (kB) Sets the maximum memory to be used for maintenance operations.
	maintenance_work_mem?: int & >=1024 & <=2147483647 @storeResource(1KB)

	// Sets the maximum number of concurrent connections.
	max_connections?: int & >=6 & <=8388607

	// Sets the maximum number of simultaneously open files for each server process.
	max_files_per_process?: int & >=64 & <=2147483647

	// Sets the maximum number of locks per transaction.
	max_locks_per_transaction: int & >=10 & <=2147483647 | *64

	// Maximum number of logical replication worker processes.
	max_logical_replication_workers?: int & >=0 & <=262143

	// Sets the maximum number of parallel processes per maintenance operation.
	max_parallel_maintenance_workers?: int & >=0 & <=1024

	// Sets the maximum number of parallel workers than can be active at one time.
	max_parallel_workers?: int & >=0 & <=1024

	// Sets the maximum number of parallel processes per executor node.
	max_parallel_workers_per_gather?: int & >=0 & <=1024

	// Sets the maximum number of predicate-locked tuples per page.
	max_pred_locks_per_page?: int & >=0 & <=2147483647

	// Sets the maximum number of predicate-locked pages and tuples per relation.
	max_pred_locks_per_relation?: int & >=-2147483648 & <=2147483647

	// Sets the maximum number of predicate locks per transaction.
	max_pred_locks_per_transaction?: int & >=10 & <=2147483647

	// Sets the maximum number of simultaneously prepared transactions.
	max_prepared_transactions: int & >=0 & <=8388607 | *0

	// Sets the maximum number of replication slots that the server can support.
	max_replication_slots: int & >=5 & <=8388607 | *20

	// (MB) Sets the maximum WAL size that can be reserved by replication slots.
	max_slot_wal_keep_size?: int & >=-1 & <=2147483647 @storeResource(1MB)

	// (kB) Sets the maximum stack depth, in kilobytes.
	max_stack_depth: int & >=100 & <=2147483647 | *6144 @storeResource(1KB)

	// (ms) Sets the maximum delay before canceling queries when a hot standby server is processing archived WAL data.
	max_standby_archive_delay?: int & >=-1 & <=2147483647 @timeDurationResource()

	// (ms) Sets the maximum delay before canceling queries when a hot standby server is processing streamed WAL data.
	max_standby_streaming_delay?: int & >=-1 & <=2147483647 @timeDurationResource()

	// Maximum number of synchronization workers per subscription
	max_sync_workers_per_subscription?: int & >=0 & <=262143

	// Sets the maximum number of simultaneously running WAL sender processes.
	max_wal_senders: int & >=5 & <=8388607 | *20

	// (MB) Sets the WAL size that triggers a checkpoint.
	max_wal_size: int & >=128 & <=201326592 | *2048 @storeResource(1MB)

	// Sets the maximum number of concurrent worker processes.
	max_worker_processes?: int & >=0 & <=262143

	// Specifies the amount of memory that should be allocated at server startup for use by parallel queries
	min_dynamic_shared_memory?: int & >=0 & <=715827882 @storeResource(1MB)

	// (8kB) Sets the minimum amount of index data for a parallel scan.
	min_parallel_index_scan_size?: int & >=0 & <=715827882 @storeResource(8KB)

	// Sets the minimum size of relations to be considered for parallel scan. Sets the minimum size of relations to be considered for parallel scan. 
	min_parallel_relation_size?: int & >=0 & <=715827882 @storeResource(8KB)

	// (8kB) Sets the minimum amount of table data for a parallel scan.
	min_parallel_table_scan_size?: int & >=0 & <=715827882 @storeResource(8KB)

	// (MB) Sets the minimum size to shrink the WAL to.
	min_wal_size: int & >=128 & <=201326592 | *192 @storeResource(1MB)

	// (min) Time before a snapshot is too old to read pages changed after the snapshot was taken.
	old_snapshot_threshold?: int & >=-1 & <=86400 @timeDurationResource(1min)

	// Emulate oracle's date output behaviour.
	"orafce.nls_date_format"?: string

	// Specify timezone used for sysdate function.
	"orafce.timezone"?: string

	// Controls whether Gather and Gather Merge also run subplans.
	parallel_leader_participation?: bool & false | true

	// Sets the planner's estimate of the cost of starting up worker processes for parallel query.
	parallel_setup_cost?: float & >=0 & <=1.79769e+308

	// Sets the planner's estimate of the cost of passing each tuple (row) from worker to master backend.
	parallel_tuple_cost?: float & >=0 & <=1.79769e+308

	// Encrypt passwords.
	password_encryption?: string & "md5" | "scram-sha-256"

	// Specifies which classes of statements will be logged by session audit logging.
	"pgaudit.log"?: string & "ddl" | "function" | "misc" | "read" | "role" | "write" | "none" | "all" | "-ddl" | "-function" | "-misc" | "-read" | "-role" | "-write"

	// Specifies that session logging should be enabled in the case where all relations in a statement are in pg_catalog.
	"pgaudit.log_catalog"?: bool & false | true

	// Specifies the log level that will be used for log entries.
	"pgaudit.log_level"?: string & "debug5" | "debug4" | "debug3" | "debug2" | "debug1" | "info" | "notice" | "warning" | "log"

	// Specifies that audit logging should include the parameters that were passed with the statement.
	"pgaudit.log_parameter"?: bool & false | true

	// Specifies whether session audit logging should create a separate log entry for each relation (TABLE, VIEW, etc.) referenced in a SELECT or DML statement.
	"pgaudit.log_relation"?: bool & false | true

	// Specifies that audit logging should include the rows retrieved or affected by a statement.
	"pgaudit.log_rows": bool & false | true | *false

	// Specifies whether logging will include the statement text and parameters (if enabled).
	"pgaudit.log_statement": bool & false | true | *true

	// Specifies whether logging will include the statement text and parameters with the first log entry for a statement/substatement combination or with every entry.
	"pgaudit.log_statement_once"?: bool & false | true

	// Specifies the master role to use for object audit logging.
	"pgaudit.role"?: string

	// It specifies whether to perform Recheck which is an internal process of full text search.
	"pg_bigm.enable_recheck"?: string & "on" | "off"

	// It specifies the maximum number of 2-grams of the search keyword to be used for full text search.
	"pg_bigm.gin_key_limit": int & >=0 & <=2147483647 | *0

	// It specifies the minimum threshold used by the similarity search.
	"pg_bigm.similarity_limit": float & >=0 & <=1 | *0.3

	// Logs results of hint parsing.
	"pg_hint_plan.debug_print"?: string & "off" | "on" | "detailed" | "verbose"

	// Force planner to use plans specified in the hint comment preceding to the query.
	"pg_hint_plan.enable_hint"?: bool & false | true

	// Force planner to not get hint by using table lookups.
	"pg_hint_plan.enable_hint_table"?: bool & false | true

	// Message level of debug messages.
	"pg_hint_plan.message_level"?: string & "debug5" | "debug4" | "debug3" | "debug2" | "debug1" | "log" | "info" | "notice" | "warning" | "error"

	// Message level of parse errors.
	"pg_hint_plan.parse_messages"?: string & "debug5" | "debug4" | "debug3" | "debug2" | "debug1" | "log" | "info" | "notice" | "warning" | "error"

	// Batch inserts if possible
	"pglogical.batch_inserts"?: bool & false | true

	// Sets log level used for logging resolved conflicts.
	"pglogical.conflict_log_level"?: string & "debug5" | "debug4" | "debug3" | "debug2" | "debug1" | "info" | "notice" | "warning" | "error" | "log" | "fatal" | "panic"

	// Sets method used for conflict resolution for resolvable conflicts.
	"pglogical.conflict_resolution"?: string & "error" | "apply_remote" | "keep_local" | "last_update_wins" | "first_update_wins"

	// connection options to add to all peer node connections
	"pglogical.extra_connection_options"?: string

	// pglogical specific synchronous commit value
	"pglogical.synchronous_commit"?: bool & false | true

	// Use SPI instead of low-level API for applying changes
	"pglogical.use_spi"?: bool & false | true

	// Starts the autoprewarm worker.
	"pg_prewarm.autoprewarm"?: bool & false | true

	// Sets the interval between dumps of shared buffers
	"pg_prewarm.autoprewarm_interval"?: int & >=0 & <=2147483

	// Sets if the result value is normalized or not.
	"pg_similarity.block_is_normalized"?: bool & false | true

	// Sets the threshold used by the Block similarity function.
	"pg_similarity.block_threshold"?: float & >=0 & <=1

	// Sets the tokenizer for Block similarity function.
	"pg_similarity.block_tokenizer"?: string & "alnum" | "gram" | "word" | "camelcase"

	// Sets if the result value is normalized or not.
	"pg_similarity.cosine_is_normalized"?: bool & false | true

	// Sets the threshold used by the Cosine similarity function.
	"pg_similarity.cosine_threshold"?: float & >=0 & <=1

	// Sets the tokenizer for Cosine similarity function.
	"pg_similarity.cosine_tokenizer"?: string & "alnum" | "gram" | "word" | "camelcase"

	// Sets if the result value is normalized or not.
	"pg_similarity.dice_is_normalized"?: bool & false | true

	// Sets the threshold used by the Dice similarity measure.
	"pg_similarity.dice_threshold"?: float & >=0 & <=1

	// Sets the tokenizer for Dice similarity measure.
	"pg_similarity.dice_tokenizer"?: string & "alnum" | "gram" | "word" | "camelcase"

	// Sets if the result value is normalized or not.
	"pg_similarity.euclidean_is_normalized"?: bool & false | true

	// Sets the threshold used by the Euclidean similarity measure.
	"pg_similarity.euclidean_threshold"?: float & >=0 & <=1

	// Sets the tokenizer for Euclidean similarity measure.
	"pg_similarity.euclidean_tokenizer"?: string & "alnum" | "gram" | "word" | "camelcase"

	// Sets if the result value is normalized or not.
	"pg_similarity.hamming_is_normalized"?: bool & false | true

	// Sets the threshold used by the Block similarity metric.
	"pg_similarity.hamming_threshold"?: float & >=0 & <=1

	// Sets if the result value is normalized or not.
	"pg_similarity.jaccard_is_normalized"?: bool & false | true

	// Sets the threshold used by the Jaccard similarity measure.
	"pg_similarity.jaccard_threshold"?: float & >=0 & <=1

	// Sets the tokenizer for Jaccard similarity measure.
	"pg_similarity.jaccard_tokenizer"?: string & "alnum" | "gram" | "word" | "camelcase"

	// Sets if the result value is normalized or not.
	"pg_similarity.jaro_is_normalized"?: bool & false | true

	// Sets the threshold used by the Jaro similarity measure.
	"pg_similarity.jaro_threshold"?: float & >=0 & <=1

	// Sets if the result value is normalized or not.
	"pg_similarity.jarowinkler_is_normalized"?: bool & false | true

	// Sets the threshold used by the Jarowinkler similarity measure.
	"pg_similarity.jarowinkler_threshold"?: float & >=0 & <=1

	// Sets if the result value is normalized or not.
	"pg_similarity.levenshtein_is_normalized"?: bool & false | true

	// Sets the threshold used by the Levenshtein similarity measure.
	"pg_similarity.levenshtein_threshold"?: float & >=0 & <=1

	// Sets if the result value is normalized or not.
	"pg_similarity.matching_is_normalized"?: bool & false | true

	// Sets the threshold used by the Matching Coefficient similarity measure.
	"pg_similarity.matching_threshold"?: float & >=0 & <=1

	// Sets the tokenizer for Matching Coefficient similarity measure.
	"pg_similarity.matching_tokenizer"?: string & "alnum" | "gram" | "word" | "camelcase"

	// Sets if the result value is normalized or not.
	"pg_similarity.mongeelkan_is_normalized"?: bool & false | true

	// Sets the threshold used by the Monge-Elkan similarity measure.
	"pg_similarity.mongeelkan_threshold"?: float & >=0 & <=1

	// Sets the tokenizer for Monge-Elkan similarity measure.
	"pg_similarity.mongeelkan_tokenizer"?: string & "alnum" | "gram" | "word" | "camelcase"

	// Sets the gap penalty used by the Needleman-Wunsch similarity measure.
	"pg_similarity.nw_gap_penalty"?: float & >=-9.22337e+18 & <=9.22337e+18

	// Sets if the result value is normalized or not.
	"pg_similarity.nw_is_normalized"?: bool & false | true

	// Sets the threshold used by the Needleman-Wunsch similarity measure.
	"pg_similarity.nw_threshold"?: float & >=0 & <=1

	// Sets if the result value is normalized or not.
	"pg_similarity.overlap_is_normalized"?: bool & false | true

	// Sets the threshold used by the Overlap Coefficient similarity measure.
	"pg_similarity.overlap_threshold"?: float & >=0 & <=1

	// Sets the tokenizer for Overlap Coefficientsimilarity measure.
	"pg_similarity.overlap_tokenizer"?: string & "alnum" | "gram" | "word" | "camelcase"

	// Sets if the result value is normalized or not.
	"pg_similarity.qgram_is_normalized"?: bool & false | true

	// Sets the threshold used by the Q-Gram similarity measure.
	"pg_similarity.qgram_threshold"?: float & >=0 & <=1

	// Sets the tokenizer for Q-Gram measure.
	"pg_similarity.qgram_tokenizer"?: string & "alnum" | "gram" | "word" | "camelcase"

	// Sets if the result value is normalized or not.
	"pg_similarity.swg_is_normalized"?: bool & false | true

	// Sets the threshold used by the Smith-Waterman-Gotoh similarity measure.
	"pg_similarity.swg_threshold"?: float & >=0 & <=1

	// Sets if the result value is normalized or not.
	"pg_similarity.sw_is_normalized"?: bool & false | true

	// Sets the threshold used by the Smith-Waterman similarity measure.
	"pg_similarity.sw_threshold"?: float & >=0 & <=1

	// Sets the maximum number of statements tracked by pg_stat_statements.
	"pg_stat_statements.max"?: int & >=100 & <=2147483647

	// Save pg_stat_statements statistics across server shutdowns.
	"pg_stat_statements.save"?: bool & false | true

	// Selects which statements are tracked by pg_stat_statements.
	"pg_stat_statements.track"?: string & "none" | "top" | "all"

	// Selects whether planning duration is tracked by pg_stat_statements.
	"pg_stat_statements.track_planning"?: bool & false | true

	// Selects whether utility commands are tracked by pg_stat_statements.
	"pg_stat_statements.track_utility"?: bool & false | true

	// Sets the behavior for interacting with passcheck feature.
	"pgtle.enable_password_check"?: string & "on" | "off" | "require"

	// Number of workers to use for a physical transport.
	"pg_transport.num_workers"?: int & >=1 & <=32

	// Specifies whether to report timing information during transport.
	"pg_transport.timing"?: bool & false | true

	// (kB) Amount of memory each worker can allocate for a physical transport.
	"pg_transport.work_mem"?: int & >=65536 & <=2147483647 @storeResource(1KB)

	// Controls the planner selection of custom or generic plan.
	plan_cache_mode?: string & "auto" | "force_generic_plan" | "force_custom_plan"

	// Sets the TCP port the server listens on.
	port?: int & >=1 & <=65535

	// Sets the amount of time to wait after authentication on connection startup. The amount of time to delay when a new server process is started, after it conducts the authentication procedure. This is intended to give developers an opportunity to attach to the server process with a debugger. If this value is specified without units, it is taken as seconds. A value of zero (the default) disables the delay. This parameter cannot be changed after session start.
	post_auth_delay?: int & >=0 & <=2147 @timeDurationResource(1s)

	// Sets the amount of time to wait before authentication on connection startup. The amount of time to delay just after a new server process is forked, before it conducts the authentication procedure. This is intended to give developers an opportunity to attach to the server process with a debugger to trace down misbehavior in authentication. If this value is specified without units, it is taken as seconds. A value of zero (the default) disables the delay. This parameter can only be set in the postgresql.conf file or on the server command line.
	pre_auth_delay?: int & >=0 & <=60 @timeDurationResource(1s)

	// Enable for disable GDAL drivers used with PostGIS in Postgres 9.3.5 and above.
	"postgis.gdal_enabled_drivers"?: string & "ENABLE_ALL" | "DISABLE_ALL"

	// When generating SQL fragments, quote all identifiers.
	quote_all_identifiers?: bool & false | true

	// Sets the planners estimate of the cost of a nonsequentially fetched disk page.
	random_page_cost?: float & >=0 & <=1.79769e+308

	// Lower threshold of Dice similarity. Molecules with similarity lower than threshold are not similar by # operation.
	"rdkit.dice_threshold"?: float & >=0 & <=1

	// Should stereochemistry be taken into account in substructure matching. If false, no stereochemistry information is used in substructure matches.
	"rdkit.do_chiral_sss"?: bool & false | true

	// Should enhanced stereochemistry be taken into account in substructure matching.
	"rdkit.do_enhanced_stereo_sss"?: bool & false | true

	// Lower threshold of Tanimoto similarity. Molecules with similarity lower than threshold are not similar by % operation.
	"rdkit.tanimoto_threshold"?: float & >=0 & <=1

	// When set to fsync, PostgreSQL will recursively open and synchronize all files in the data directory before crash recovery begins
	recovery_init_sync_method?: string & "fsync" | "syncfs"

	// When set to on, which is the default, PostgreSQL will automatically remove temporary files after a backend crash
	remove_temp_files_after_crash: float & >=0 & <=1 | *0

	// Reinitialize server after backend crash.
	restart_after_crash?: bool & false | true

	// Enable row security.
	row_security?: bool & false | true

	// Sets the schema search order for names that are not schema-qualified.
	search_path?: string

	// Sets the planners estimate of the cost of a sequentially fetched disk page.
	seq_page_cost?: float & >=0 & <=1.79769e+308

	// Lists shared libraries to preload into each backend.
	session_preload_libraries?: string & "auto_explain" | "orafce" | "pg_bigm" | "pg_hint_plan" | "pg_prewarm" | "pg_similarity" | "pg_stat_statements" | "pg_transport" | "plprofiler"

	// Sets the sessions behavior for triggers and rewrite rules.
	session_replication_role?: string & "origin" | "replica" | "local"

	// (8kB) Sets the number of shared memory buffers used by the server.
	shared_buffers?: int & >=16 & <=1073741823 @storeResource(8KB)

	// Lists shared libraries to preload into server.
	// TODO support enum list, e.g. shared_preload_libraries = 'pg_stat_statements, auto_explain'
	// shared_preload_libraries?: string & "auto_explain" | "orafce" | "pgaudit" | "pglogical" | "pg_bigm" | "pg_cron" | "pg_hint_plan" | "pg_prewarm" | "pg_similarity" | "pg_stat_statements" | "pg_tle" | "pg_transport" | "plprofiler"

	// Enables SSL connections.
	ssl: bool & false | true | *true

	// Location of the SSL server authority file.
	ssl_ca_file?: string

	// Location of the SSL server certificate file.
	ssl_cert_file?: string

	// Sets the list of allowed SSL ciphers.
	ssl_ciphers?: string

	// Location of the SSL server private key file
	ssl_key_file?: string

	// Sets the maximum SSL/TLS protocol version to use.
	ssl_max_protocol_version?: string & "TLSv1" | "TLSv1.1" | "TLSv1.2"

	// Sets the minimum SSL/TLS protocol version to use.
	ssl_min_protocol_version?: string & "TLSv1" | "TLSv1.1" | "TLSv1.2"

	// Causes ... strings to treat backslashes literally.
	standard_conforming_strings?: bool & false | true

	// (ms) Sets the maximum allowed duration of any statement.
	statement_timeout?: int & >=0 & <=2147483647 @timeDurationResource()

	// Writes temporary statistics files to the specified directory.
	stats_temp_directory?: string

	// Sets the number of connection slots reserved for superusers.
	superuser_reserved_connections: int & >=0 & <=8388607 | *3

	// Enable synchronized sequential scans.
	synchronize_seqscans?: bool & false | true

	// Sets the current transactions synchronization level.
	synchronous_commit?: string & "local" | "on" | "off"

	// Maximum number of TCP keepalive retransmits.
	tcp_keepalives_count?: int & >=0 & <=2147483647

	// (s) Time between issuing TCP keepalives.
	tcp_keepalives_idle?: int & >=0 & <=2147483647 @timeDurationResource(1s)

	// (s) Time between TCP keepalive retransmits.
	tcp_keepalives_interval?: int & >=0 & <=2147483647 @timeDurationResource(1s)

	// TCP user timeout. Specifies the amount of time that transmitted data may remain unacknowledged before the TCP connection is forcibly closed. If this value is specified without units, it is taken as milliseconds. A value of 0 (the default) selects the operating system's default. This parameter is supported only on systems that support TCP_USER_TIMEOUT; on other systems, it must be zero. In sessions connected via a Unix-domain socket, this parameter is ignored and always reads as zero.
	tcp_user_timeout?: int & >=0 & <=2147483647 @timeDurationResource()

	// (8kB) Sets the maximum number of temporary buffers used by each session.
	temp_buffers?: int & >=100 & <=1073741823 @storeResource(8KB)

	// (kB) Limits the total size of all temporary files used by each process.
	temp_file_limit?: int & >=-1 & <=2147483647 @storeResource(1KB)

	// Sets the tablespace(s) to use for temporary tables and sort files.
	temp_tablespaces?: string

	// Sets the time zone for displaying and interpreting time stamps.
	timezone?: string

	// Collects information about executing commands.
	track_activities?: bool & false | true

	// Sets the size reserved for pg_stat_activity.current_query, in bytes.
	track_activity_query_size: int & >=100 & <=1048576 | *4096 @storeResource()

	// Collects transaction commit time.
	track_commit_timestamp?: bool & false | true

	// Collects statistics on database activity.
	track_counts?: bool & false | true

	// Collects function-level statistics on database activity.
	track_functions?: string & "none" | "pl" | "all"

	// Collects timing statistics on database IO activity.
	track_io_timing: bool & false | true | *true

	// Enables timing of WAL I/O calls.
	track_wal_io_timing?: bool & false | true

	// Treats expr=NULL as expr IS NULL.
	transform_null_equals?: bool & false | true

	// Sets the directory where the Unix-domain socket will be created.
	unix_socket_directories?: string

	// Sets the owning group of the Unix-domain socket.
	unix_socket_group?: string

	// Sets the access permissions of the Unix-domain socket.
	unix_socket_permissions?: int & >=0 & <=511

	// Updates the process title to show the active SQL command.
	update_process_title: bool & false | true | *true

	// (ms) Vacuum cost delay in milliseconds.
	vacuum_cost_delay?: int & >=0 & <=100 @timeDurationResource()

	// Vacuum cost amount available before napping.
	vacuum_cost_limit?: int & >=1 & <=10000

	// Vacuum cost for a page dirtied by vacuum.
	vacuum_cost_page_dirty?: int & >=0 & <=10000

	// Vacuum cost for a page found in the buffer cache.
	vacuum_cost_page_hit?: int & >=0 & <=10000

	// Vacuum cost for a page not found in the buffer cache.
	vacuum_cost_page_miss: int & >=0 & <=10000 | *5

	// Number of transactions by which VACUUM and HOT cleanup should be deferred, if any.
	vacuum_defer_cleanup_age?: int & >=0 & <=1000000

	// Specifies the maximum age (in transactions) that a table's pg_class.relfrozenxid field can attain before VACUUM takes extraordinary measures to avoid system-wide transaction ID wraparound failure
	vacuum_failsafe_age: int & >=0 & <=1200000000 | *1200000000

	// Minimum age at which VACUUM should freeze a table row.
	vacuum_freeze_min_age?: int & >=0 & <=1000000000

	// Age at which VACUUM should scan whole table to freeze tuples.
	vacuum_freeze_table_age?: int & >=0 & <=2000000000

	// Specifies the maximum age (in transactions) that a table's pg_class.relminmxid field can attain before VACUUM takes extraordinary measures to avoid system-wide multixact ID wraparound failure
	vacuum_multixact_failsafe_age: int & >=0 & <=1200000000 | *1200000000

	// Minimum age at which VACUUM should freeze a MultiXactId in a table row.
	vacuum_multixact_freeze_min_age?: int & >=0 & <=1000000000

	// Multixact age at which VACUUM should scan whole table to freeze tuples.
	vacuum_multixact_freeze_table_age?: int & >=0 & <=2000000000

	// (8kB) Sets the number of disk-page buffers in shared memory for WAL.
	wal_buffers?: int & >=-1 & <=262143 @storeResource(8KB)

	// Compresses full-page writes written in WAL file.
	wal_compression: bool & false | true | *true

	// Sets the WAL resource managers for which WAL consistency checks are done.
	wal_consistency_checking?: string

	// Buffer size for reading ahead in the WAL during recovery. 
	wal_decode_buffer_size: int & >=65536 & <=1073741823 | *524288 @storeResource()

	// (MB) Sets the size of WAL files held for standby servers.
	wal_keep_size: int & >=0 & <=2147483647 | *2048 @storeResource(1MB)

	// Sets whether a WAL receiver should create a temporary replication slot if no permanent slot is configured.
	wal_receiver_create_temp_slot: bool & false | true | *false

	// (s) Sets the maximum interval between WAL receiver status reports to the primary.
	wal_receiver_status_interval?: int & >=0 & <=2147483 @timeDurationResource(1s)

	// (ms) Sets the maximum wait time to receive data from the primary.
	wal_receiver_timeout: int & >=0 & <=3600000 | *30000 @timeDurationResource()

	// Recycles WAL files by renaming them. If set to on (the default), this option causes WAL files to be recycled by renaming them, avoiding the need to create new ones. On COW file systems, it may be faster to create new ones, so the option is given to disable this behavior.
	wal_recycle?: bool

	// Sets the time to wait before retrying to retrieve WAL after a failed attempt. Specifies how long the standby server should wait when WAL data is not available from any sources (streaming replication, local pg_wal or WAL archive) before trying again to retrieve WAL data. If this value is specified without units, it is taken as milliseconds. The default value is 5 seconds. This parameter can only be set in the postgresql.conf file or on the server command line.
	wal_retrieve_retry_interval: int & >=1 & <=2147483647 | *5000 @timeDurationResource()

	// (ms) Sets the maximum time to wait for WAL replication.
	wal_sender_timeout: int & >=0 & <=3600000 | *30000 @timeDurationResource()

	// (kB) Size of new file to fsync instead of writing WAL.
	wal_skip_threshold?: int & >=0 & <=2147483647 @storeResource(1KB)

	// Selects the method used for forcing WAL updates to disk.
	wal_sync_method?: string & "fsync" | "fdatasync" | "open_sync" | "open_datasync"

	// (ms) WAL writer sleep time between WAL flushes.
	wal_writer_delay?: int & >=1 & <=10000 @timeDurationResource()

	// (8Kb) Amount of WAL written out by WAL writer triggering a flush.
	wal_writer_flush_after?: int & >=0 & <=2147483647 @storeResource(8KB)

	// (kB) Sets the maximum memory to be used for query workspaces.
	work_mem?: int & >=64 & <=2147483647 @storeResource(1KB)

	// Sets how binary values are to be encoded in XML.
	xmlbinary?: string & "base64" | "hex"

	// Sets whether XML data in implicit parsing and serialization operations is to be considered as documents or content fragments.
	xmloption?: string & "content" | "document"

	...
}

configuration: #PGParameter & {
}
