########## basics ##########
# Whether to run as a daemon process
--daemonize=true
# The file to host the process id
--pid_file=pids/nebula-graphd.pid
# Whether to enable optimizer
--enable_optimizer=true
# The default charset when a space is created
--default_charset=utf8
# The default collate when a space is created
--default_collate=utf8_bin
# Whether to use the configuration obtained from the configuration file
--local_config=true

########## logging ##########
# The directory to host logging files
--log_dir=logs
# Log level, 0, 1, 2, 3 for INFO, WARNING, ERROR, FATAL respectively
--minloglevel=0
# Verbose log level, 1, 2, 3, 4, the higher of the level, the more verbose of the logging
--v=0
# Maximum seconds to buffer the log messages
--logbufsecs=0
# Whether to redirect stdout and stderr to separate output files
--redirect_stdout=true
# Destination filename of stdout and stderr, which will also reside in log_dir.
--stdout_log_file=graphd-stdout.log
--stderr_log_file=graphd-stderr.log
# Copy log messages at or above this level to stderr in addition to logfiles. The numbers of severity levels INFO, WARNING, ERROR, and FATAL are 0, 1, 2, and 3, respectively.
--stderrthreshold=3
# wether logging files' name contain timestamp.
--timestamp_in_logfile_name=true

########## query ##########
# Whether to treat partial success as an error.
# This flag is only used for Read-only access, and Modify access always treats partial success as an error.
--accept_partial_success=false
# Maximum sentence length, unit byte
--max_allowed_query_size=4194304

########## networking ##########
# Comma separated Meta Server Addresses
--meta_server_addrs=127.0.0.1:9559
# Local IP used to identify the nebula-graphd process.
# Change it to an address other than loopback if the service is distributed or
# will be accessed remotely.
--local_ip=127.0.0.1
# Network device to listen on
--listen_netdev=any
# Port to listen on
--port=9669
# To turn on SO_REUSEPORT or not
--reuse_port=false
# Backlog of the listen socket, adjust this together with net.core.somaxconn
--listen_backlog=1024
# The number of seconds Nebula service waits before closing the idle connections
--client_idle_timeout_secs=28800
# The number of seconds before idle sessions expire
# The range should be in [1, 604800]
--session_idle_timeout_secs=28800
# The number of threads to accept incoming connections
--num_accept_threads=1
# The number of networking IO threads, 0 for # of CPU cores
--num_netio_threads=0
# The number of threads to execute user queries, 0 for # of CPU cores
--num_worker_threads=0
# HTTP service ip
--ws_ip=0.0.0.0
# HTTP service port
--ws_http_port=19669
# storage client timeout
--storage_client_timeout_ms=60000
# Enable slow query records
--enable_record_slow_query=true
# The number of slow query records
--slow_query_limit=100
# slow query threshold in us
--slow_query_threshold_us=200000
# Port to listen on Meta with HTTP protocol, it corresponds to ws_http_port in metad's configuration file
--ws_meta_http_port=19559

########## authentication ##########
# Enable authorization
--enable_authorize=false
# User login authentication type, password for nebula authentication, ldap for ldap authentication, cloud for cloud authentication
--auth_type=password

########## memory ##########
# System memory high watermark ratio, cancel the memory checking when the ratio greater than 1.0
--system_memory_high_watermark_ratio=0.8

########## audit ##########
# This variable is used to enable audit. The value can be 'true' or 'false'.
--enable_audit=false
# This variable is used to configure where the audit log will be written. Optional：[ file | es ]
# If it is set to 'file', the log will be written into a file specified by audit_log_file variable.
# If it is set to 'es', the audit log will be written to Elasticsearch.
--audit_log_handler=file
# This variable is used to specify the filename that’s going to store the audit log.
# It can contain the path relative to the install dir or absolute path.
# This variable has effect only when audit_log_handler is set to 'file'.
--audit_log_file=./logs/audit/audit.log
# This variable is used to specify the audit log strategy, Optional：[ asynchronous｜ synchronous ]
# asynchronous: log using memory buffer, do not block the main thread
# synchronous: log directly to file, flush and sync every event
# Caution: For performance reasons, when the buffer is full and has not been flushed to the disk,
# the 'asynchronous' mode will discard subsequent requests.
# This variable has effect only when audit_log_handler is set to 'file'.
--audit_log_strategy=synchronous
# This variable can be used to specify the size of memory buffer used for logging,
# used when audit_log_strategy variable is set to 'asynchronous' values.
# This variable has effect only when audit_log_handler is set to 'file'. Uint: B
--audit_log_max_buffer_size=1048576
# This variable is used to specify the audit log format. Supports three log formats [ xml | json | csv ]
# This variable has effect only when audit_log_handler is set to 'file'.
--audit_log_format=xml
# This variable can be used to specify the comma-seperated list of Elasticsearch addresses,
# eg, '192.168.0.1:7001, 192.168.0.2:7001'.
# This variable has effect only when audit_log_handler is set to 'es'.
--audit_log_es_address=
# This variable can be used to specify the user name of the Elasticsearch.
# This variable has effect only when audit_log_handler is set to 'es'.
--audit_log_es_user=
# This variable can be used to specify the user password of the Elasticsearch.
# This variable has effect only when audit_log_handler is set to 'es'.
--audit_log_es_password=
# This variable can be used to specify the number of logs which are sent to Elasticsearch at one time.
# This variable has effect only when audit_log_handler is set to 'es'.
--audit_log_es_batch_size=1000
# This variable is used to specify the list of spaces for not tracking.
# The value can be comma separated list of spaces, ie, 'nba, basketball'.
--audit_log_exclude_spaces=
# This variable is used to specify the list of log categories for tracking, eg, 'login, ddl'.
# There are eight categories for tracking. There are: [ login ｜ exit | ddl | dql | dml | dcl | util | unknown ].
--audit_log_categories=login,exit

########## metrics ##########
--enable_space_level_metrics=false

########## experimental feature ##########
# if use experimental features
--enable_experimental_feature=false

########## Black box ########
# Enable black box
--ng_black_box_switch=true
# Black box log folder
--ng_black_box_home=black_box
# Black box dump metrics log period
--ng_black_box_dump_period_seconds=5
# Black box log files expire time
--ng_black_box_file_lifetime_seconds=1800

########## session ##########
# Maximum number of sessions that can be created per IP and per user
--max_sessions_per_ip_per_user=300

########## memory tracker ##########
# trackable memory ratio (trackable_memory / (total_memory - untracked_reserved_memory) )
--memory_tracker_limit_ratio=0.8
# untracked reserved memory in Mib
--memory_tracker_untracked_reserved_memory_mb=50

# enable log memory tracker stats periodically
--memory_tracker_detail_log=false
# log memory tacker stats interval in milliseconds
--memory_tracker_detail_log_interval_ms=60000

# enable memory background purge (if jemalloc is used)
--memory_purge_enabled=true
# memory background purge interval in seconds
--memory_purge_interval_seconds=10