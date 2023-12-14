//Copyright (C) 2022-2023 ApeCloud Co., Ltd
//
//This file is part of KubeBlocks project
//
//This program is free software: you can redistribute it and/or modify
//it under the terms of the GNU Affero General Public License as published by
//the Free Software Foundation, either version 3 of the License, or
//(at your option) any later version.
//
//This program is distributed in the hope that it will be useful
//but WITHOUT ANY WARRANTY; without even the implied warranty of
//MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//GNU Affero General Public License for more details.
//
//You should have received a copy of the GNU Affero General Public License
//along with this program.  If not, see <http://www.gnu.org/licenses/>.

#MysqlParameter: {

	// reference aws rds params: https://console.amazonaws.cn/rds/home?region=cn-north-1#parameter-groups-detail:ids=default.mysql8.0;type=DbParameterGroup;editing=false
	// auto generate by cue_generate.go

	// Automatically set all granted roles as active after the user has authenticated successfully.
	activate_all_roles_on_login: string & "0" | "1" | "OFF" | "ON" | *"0"

	// Controls whether user-defined functions that have only an xxx symbol for the main function can be loaded
	"allow-suspicious-udfs"?: string & "0" | "1" | "OFF" | "ON"

	// Sets the autocommit mode
	autocommit?: string & "0" | "1" | "OFF" | "ON"

	// Controls whether the server autogenerates SSL key and certificate files in the data directory, if they do not already exist.
	auto_generate_certs?: string & "0" | "1" | "OFF" | "ON"

	// Intended for use with master-to-master replication, and can be used to control the operation of AUTO_INCREMENT columns
	auto_increment_increment?: int & >=1 & <=65535

	// Determines the starting point for the AUTO_INCREMENT column value
	auto_increment_offset?: int & >=1 & <=65535

	// When this variable has a value of 1 (the default), the server automatically grants the EXECUTE and ALTER ROUTINE privileges to the creator of a stored routine, if the user cannot already execute and alter or drop the routine.
	automatic_sp_privileges?: string & "0" | "1" | "OFF" | "ON"

	// This variable controls whether ALTER TABLE implicitly upgrades temporal columns found to be in pre-5.6.4 format.
	avoid_temporal_upgrade?: string & "0" | "1" | "OFF" | "ON"

	// The number of outstanding connection requests MySQL can have
	back_log?: int & >=1 & <=65535

	// The MySQL installation base directory.
	basedir?: string

	big_tables: string & "0" | "1" | "OFF" | "ON" | *"0"

	bind_address?: string

	// The size of the cache to hold the SQL statements for the binary log during a transaction.
	binlog_cache_size: int & >=4096 & <=18446744073709547520 | *32768

	// When enabled, this variable causes the master to write a checksum for each event in the binary log.
	binlog_checksum?: string & "NONE" | "CRC32"

	binlog_direct_non_transactional_updates: string & "0" | "1" | "OFF" | "ON" | *"0"

	// Controls what happens when the server cannot write to the binary log.
	binlog_error_action?: string & "IGNORE_ERROR" | "ABORT_SERVER"

	// If non-zero, binary logs will be purged after expire_logs_days days; If this option alone is set on the command line or in a configuration file, it overrides the default value for binlog-expire-logs-seconds. If both options are set to nonzero values, binlog-expire-logs-seconds takes priority. Possible purges happen at startup and at binary log rotation.
	binlog_expire_logs_seconds: int & >=0 & <=4294967295 | *2592000

	// Row-based, Statement-based or Mixed replication
	binlog_format?: string & "ROW" | "STATEMENT" | "MIXED" | "row" | "statement" | "mixed"

	// Controls how many microseconds the binary log commit waits before synchronizing the binary log file to disk.
	binlog_group_commit_sync_delay?: int & >=0 & <=1000000

	// The maximum number of transactions to wait for before aborting the current delay as specified by binlog-group-commit-sync-delay.
	binlog_group_commit_sync_no_delay_count?: int & >=0 & <=1000000

	// Controls how binary logs are iterated during GTID recovery
	binlog_gtid_simple_recovery?: string & "0" | "1" | "OFF" | "ON"

	// How long in microseconds to keep reading transactions from the flush queue before proceeding with the group commit (and syncing the log to disk, if sync_binlog is greater than 0). If the value is 0 (the default), there is no timeout and the server keeps reading new transactions until the queue is empty.
	binlog_max_flush_queue_time?: int & >=0 & <=100000

	// If this variable is enabled (the default), transactions are committed in the same order they are written to the binary log. If disabled, transactions may be committed in parallel.
	binlog_order_commits?: string & "0" | "1" | "OFF" | "ON"

	// Whether the server logs full or minimal rows with row-based replication.
	binlog_row_image?: string & "FULL" | "MINIMAL" | "NOBLOB" | "full" | "minimal" | "noblob"

	// Controls whether metadata is logged using FULL or MINIMAL format. FULL causes all metadata to be logged; MINIMAL means that only metadata actually required by slave is logged. Default: MINIMAL.
	binlog_row_metadata?: string & "FULL" | "MINIMAL" | "full" | "minimal"

	// When enabled, it causes a MySQL 5.6.2 or later server to write informational log events such as row query log events into its binary log.
	binlog_rows_query_log_events?: string & "0" | "1" | "OFF" | "ON"

	// When set to PARTIAL_JSON, this option enables a space-efficient row-based binary log format for UPDATE statements that modify a JSON value using only the functions JSON_SET, JSON_REPLACE, and JSON_REMOVE. For such updates, only the modified parts of the JSON document are included in the binary log, so small changes of big documents may need significantly less space.
	binlog_row_value_options?: string & "PARTIAL_JSON"

	// This variable determines the size of the cache for the binary log to hold nontransactional statements issued during a transaction.
	binlog_stmt_cache_size?: int & >=4096 & <=18446744073709547520

	// Maximum number of rows to keep in the writeset history.
	binlog_transaction_dependency_history_size?: int & >=1 & <=1000000

	// Selects the source of dependency information from which to assess which transactions can be executed in parallel by the slave's multi-threaded applier. Possible values are COMMIT_ORDER, WRITESET and WRITESET_SESSION.
	binlog_transaction_dependency_tracking?: string & "COMMIT_ORDER" | "WRITESET" | "WRITESET_SESSION"

	// This variable controls the block encryption mode for block-based algorithms such as AES. It affects encryption for AES_ENCRYPT() and AES_DECRYPT().
	block_encryption_mode?: string & "aes-128-ecb" | "aes-192-ecb" | "aes-256-ecb" | "aes-128-cbc" | "aes-192-cbc" | "aes-256-cbc"

	// Limits the size of the MyISAM cache tree in bytes per thread.
	bulk_insert_buffer_size?: int & >=0 & <=18446744073709547520

	// Auto generate RSA keys at server startup if corresponding system variables are not specified and key files are not present at the default location.
	caching_sha2_password_auto_generate_rsa_keys: string & "0" | "1" | "OFF" | "ON" | *"1"

	// A fully qualified path to the private RSA key used for authentication.
	caching_sha2_password_private_key_path?: string

	// A fully qualified path to the public RSA key used for authentication.
	caching_sha2_password_public_key_path?: string

	// The character set for statements that arrive from the client.
	character_set_client?: string & "big5" | "dec8" | "cp850" | "hp8" | "koi8r" | "latin1" | "latin2" | "swe7" | "ascii" | "ujis" | "sjis" | "hebrew" | "tis620" | "euckr" | "koi8u" | "gb2312" | "greek" | "cp1250" | "gbk" | "latin5" | "armscii8" | "utf8" | "cp866" | "keybcs2" | "macce" | "macroman" | "cp852" | "latin7" | "utf8mb4" | "cp1251" | "cp1256" | "cp1257" | "binary" | "geostd8" | "cp932" | "eucjpms"

	// Don't ignore character set information sent by the client.
	"character-set-client-handshake"?: string & "0" | "1" | "OFF" | "ON"

	// The character set used for literals that do not have a character set introducer and for number-to-string conversion.
	character_set_connection?: string & "big5" | "dec8" | "cp850" | "hp8" | "koi8r" | "latin1" | "latin2" | "swe7" | "ascii" | "ujis" | "sjis" | "hebrew" | "tis620" | "euckr" | "koi8u" | "gb2312" | "greek" | "cp1250" | "gbk" | "latin5" | "armscii8" | "utf8" | "ucs2" | "cp866" | "keybcs2" | "macce" | "macroman" | "cp852" | "latin7" | "utf8mb4" | "cp1251" | "utf16" | "cp1256" | "cp1257" | "utf32" | "binary" | "geostd8" | "cp932" | "eucjpms"

	// The character set used by the default database.
	character_set_database?: string & "big5" | "dec8" | "cp850" | "hp8" | "koi8r" | "latin1" | "latin2" | "swe7" | "ascii" | "ujis" | "sjis" | "hebrew" | "tis620" | "euckr" | "koi8u" | "gb2312" | "greek" | "cp1250" | "gbk" | "latin5" | "armscii8" | "utf8" | "ucs2" | "cp866" | "keybcs2" | "macce" | "macroman" | "cp852" | "latin7" | "utf8mb4" | "cp1251" | "utf16" | "cp1256" | "cp1257" | "utf32" | "binary" | "geostd8" | "cp932" | "eucjpms"

	// The file system character set.
	character_set_filesystem?: string & "big5" | "dec8" | "cp850" | "hp8" | "koi8r" | "latin1" | "latin2" | "swe7" | "ascii" | "ujis" | "sjis" | "hebrew" | "tis620" | "euckr" | "koi8u" | "gb2312" | "greek" | "cp1250" | "gbk" | "latin5" | "armscii8" | "utf8" | "ucs2" | "cp866" | "keybcs2" | "macce" | "macroman" | "cp852" | "latin7" | "utf8mb4" | "cp1251" | "utf16" | "cp1256" | "cp1257" | "utf32" | "binary" | "geostd8" | "cp932" | "eucjpms"

	// The character set used for returning query results to the client.
	character_set_results?: string & "big5" | "dec8" | "cp850" | "hp8" | "koi8r" | "latin1" | "latin2" | "swe7" | "ascii" | "ujis" | "sjis" | "hebrew" | "tis620" | "euckr" | "koi8u" | "gb2312" | "greek" | "cp1250" | "gbk" | "latin5" | "armscii8" | "utf8" | "ucs2" | "cp866" | "keybcs2" | "macce" | "macroman" | "cp852" | "latin7" | "utf8mb4" | "cp1251" | "utf16" | "cp1256" | "cp1257" | "utf32" | "binary" | "geostd8" | "cp932" | "eucjpms"

	character_sets_dir?: string

	// The server's default character set.
	character_set_server?: string & "big5" | "dec8" | "cp850" | "hp8" | "koi8r" | "latin1" | "latin2" | "swe7" | "ascii" | "ujis" | "sjis" | "hebrew" | "tis620" | "euckr" | "koi8u" | "gb2312" | "greek" | "cp1250" | "gbk" | "latin5" | "armscii8" | "utf8" | "ucs2" | "cp866" | "keybcs2" | "macce" | "macroman" | "cp852" | "latin7" | "utf8mb4" | "cp1251" | "utf16" | "cp1256" | "cp1257" | "utf32" | "binary" | "geostd8" | "cp932" | "eucjpms"

	// Controls whether the mysql_native_password and sha256_password built-in authentication plugins support proxy users.
	check_proxy_users?: string & "0" | "1" | "OFF" | "ON"

	// The collation of the connection character set.
	collation_connection?: string & "big5_chinese_ci" | "big5_bin" | "dec8_swedish_ci" | "dec8_bin" | "cp850_general_ci" | "cp850_bin" | "hp8_english_ci" | "hp8_bin" | "koi8r_general_ci" | "koi8r_bin" | "latin1_german1_ci" | "latin1_swedish_ci" | "latin1_danish_ci" | "latin1_german2_ci" | "latin1_bin" | "latin1_general_ci" | "latin1_general_cs" | "latin1_spanish_ci" | "latin2_czech_cs" | "latin2_general_ci" | "latin2_hungarian_ci" | "latin2_croatian_ci" | "latin2_bin" | "swe7_swedish_ci" | "swe7_bin" | "ascii_general_ci" | "ascii_bin" | "ujis_japanese_ci" | "ujis_bin" | "sjis_japanese_ci" | "sjis_bin" | "hebrew_general_ci" | "hebrew_bin" | "tis620_thai_ci" | "tis620_bin" | "euckr_korean_ci" | "euckr_bin" | "koi8u_general_ci" | "koi8u_bin" | "gb2312_chinese_ci" | "gb2312_bin" | "greek_general_ci" | "greek_bin" | "cp1250_general_ci" | "cp1250_czech_cs" | "cp1250_croatian_ci" | "cp1250_bin" | "cp1250_polish_ci" | "gbk_chinese_ci" | "gbk_bin" | "latin5_turkish_ci" | "latin5_bin" | "armscii8_general_ci" | "armscii8_bin" | "utf8_general_ci" | "utf8_bin" | "utf8_unicode_ci" | "utf8_icelandic_ci" | "utf8_latvian_ci" | "utf8_romanian_ci" | "utf8_slovenian_ci" | "utf8_polish_ci" | "utf8_estonian_ci" | "utf8_spanish_ci" | "utf8_swedish_ci" | "utf8_turkish_ci" | "utf8_czech_ci" | "utf8_danish_ci" | "utf8_lithuanian_ci" | "utf8_slovak_ci" | "utf8_spanish2_ci" | "utf8_roman_ci" | "utf8_persian_ci" | "utf8_esperanto_ci" | "utf8_hungarian_ci" | "utf8_sinhala_ci" | "ucs2_general_ci" | "ucs2_bin" | "ucs2_unicode_ci" | "ucs2_icelandic_ci" | "ucs2_latvian_ci" | "ucs2_romanian_ci" | "ucs2_slovenian_ci" | "ucs2_polish_ci" | "ucs2_estonian_ci" | "ucs2_spanish_ci" | "ucs2_swedish_ci" | "ucs2_turkish_ci" | "ucs2_czech_ci" | "ucs2_danish_ci" | "ucs2_lithuanian_ci" | "ucs2_slovak_ci" | "ucs2_spanish2_ci" | "ucs2_roman_ci" | "ucs2_persian_ci" | "ucs2_esperanto_ci" | "ucs2_hungarian_ci" | "ucs2_sinhala_ci" | "cp866_general_ci" | "cp866_bin" | "keybcs2_general_ci" | "keybcs2_bin" | "macce_general_ci" | "macce_bin" | "macroman_general_ci" | "macroman_bin" | "cp852_general_ci" | "cp852_bin" | "latin7_estonian_cs" | "latin7_general_ci" | "latin7_general_cs" | "latin7_bin" | "utf8mb4_general_ci" | "utf8mb4_bin" | "utf8mb4_unicode_ci" | "utf8mb4_icelandic_ci" | "utf8mb4_latvian_ci" | "utf8mb4_romanian_ci" | "utf8mb4_slovenian_ci" | "utf8mb4_polish_ci" | "utf8mb4_estonian_ci" | "utf8mb4_spanish_ci" | "utf8mb4_swedish_ci" | "utf8mb4_turkish_ci" | "utf8mb4_czech_ci" | "utf8mb4_danish_ci" | "utf8mb4_lithuanian_ci" | "utf8mb4_slovak_ci" | "utf8mb4_spanish2_ci" | "utf8mb4_roman_ci" | "utf8mb4_persian_ci" | "utf8mb4_esperanto_ci" | "utf8mb4_hungarian_ci" | "utf8mb4_sinhala_ci" | "cp1251_bulgarian_ci" | "cp1251_ukrainian_ci" | "cp1251_bin" | "cp1251_general_ci" | "cp1251_general_cs" | "utf16_general_ci" | "utf16_bin" | "utf16_unicode_ci" | "utf16_icelandic_ci" | "utf16_latvian_ci" | "utf16_romanian_ci" | "utf16_slovenian_ci" | "utf16_polish_ci" | "utf16_estonian_ci" | "utf16_spanish_ci" | "utf16_swedish_ci" | "utf16_turkish_ci" | "utf16_czech_ci" | "utf16_danish_ci" | "utf16_lithuanian_ci" | "utf16_slovak_ci" | "utf16_spanish2_ci" | "utf16_roman_ci" | "utf16_persian_ci" | "utf16_esperanto_ci" | "utf16_hungarian_ci" | "utf16_sinhala_ci" | "cp1256_general_ci" | "cp1256_bin" | "cp1257_lithuanian_ci" | "cp1257_bin" | "cp1257_general_ci" | "utf32_general_ci" | "utf32_bin" | "utf32_unicode_ci" | "utf32_icelandic_ci" | "utf32_latvian_ci" | "utf32_romanian_ci" | "utf32_slovenian_ci" | "utf32_polish_ci" | "utf32_estonian_ci" | "utf32_spanish_ci" | "utf32_swedish_ci" | "utf32_turkish_ci" | "utf32_czech_ci" | "utf32_danish_ci" | "utf32_lithuanian_ci" | "utf32_slovak_ci" | "utf32_spanish2_ci" | "utf32_roman_ci" | "utf32_persian_ci" | "utf32_esperanto_ci" | "utf32_hungarian_ci" | "utf32_sinhala_ci" | "binary" | "geostd8_general_ci" | "geostd8_bin" | "cp932_japanese_ci" | "cp932_bin" | "eucjpms_japanese_ci" | "eucjpms_bin"

	collation_database?: string

	// The server's default collation.
	collation_server?: string & "big5_chinese_ci" | "big5_bin" | "dec8_swedish_ci" | "dec8_bin" | "cp850_general_ci" | "cp850_bin" | "hp8_english_ci" | "hp8_bin" | "koi8r_general_ci" | "koi8r_bin" | "latin1_german1_ci" | "latin1_swedish_ci" | "latin1_danish_ci" | "latin1_german2_ci" | "latin1_bin" | "latin1_general_ci" | "latin1_general_cs" | "latin1_spanish_ci" | "latin2_czech_cs" | "latin2_general_ci" | "latin2_hungarian_ci" | "latin2_croatian_ci" | "latin2_bin" | "swe7_swedish_ci" | "swe7_bin" | "ascii_general_ci" | "ascii_bin" | "ujis_japanese_ci" | "ujis_bin" | "sjis_japanese_ci" | "sjis_bin" | "hebrew_general_ci" | "hebrew_bin" | "tis620_thai_ci" | "tis620_bin" | "euckr_korean_ci" | "euckr_bin" | "koi8u_general_ci" | "koi8u_bin" | "gb2312_chinese_ci" | "gb2312_bin" | "greek_general_ci" | "greek_bin" | "cp1250_general_ci" | "cp1250_czech_cs" | "cp1250_croatian_ci" | "cp1250_bin" | "cp1250_polish_ci" | "gbk_chinese_ci" | "gbk_bin" | "latin5_turkish_ci" | "latin5_bin" | "armscii8_general_ci" | "armscii8_bin" | "utf8_general_ci" | "utf8_bin" | "utf8_unicode_ci" | "utf8_icelandic_ci" | "utf8_latvian_ci" | "utf8_romanian_ci" | "utf8_slovenian_ci" | "utf8_polish_ci" | "utf8_estonian_ci" | "utf8_spanish_ci" | "utf8_swedish_ci" | "utf8_turkish_ci" | "utf8_czech_ci" | "utf8_danish_ci" | "utf8_lithuanian_ci" | "utf8_slovak_ci" | "utf8_spanish2_ci" | "utf8_roman_ci" | "utf8_persian_ci" | "utf8_esperanto_ci" | "utf8_hungarian_ci" | "utf8_sinhala_ci" | "ucs2_general_ci" | "ucs2_bin" | "ucs2_unicode_ci" | "ucs2_icelandic_ci" | "ucs2_latvian_ci" | "ucs2_romanian_ci" | "ucs2_slovenian_ci" | "ucs2_polish_ci" | "ucs2_estonian_ci" | "ucs2_spanish_ci" | "ucs2_swedish_ci" | "ucs2_turkish_ci" | "ucs2_czech_ci" | "ucs2_danish_ci" | "ucs2_lithuanian_ci" | "ucs2_slovak_ci" | "ucs2_spanish2_ci" | "ucs2_roman_ci" | "ucs2_persian_ci" | "ucs2_esperanto_ci" | "ucs2_hungarian_ci" | "ucs2_sinhala_ci" | "cp866_general_ci" | "cp866_bin" | "keybcs2_general_ci" | "keybcs2_bin" | "macce_general_ci" | "macce_bin" | "macroman_general_ci" | "macroman_bin" | "cp852_general_ci" | "cp852_bin" | "latin7_estonian_cs" | "latin7_general_ci" | "latin7_general_cs" | "latin7_bin" | "utf8mb4_0900_ai_ci" | "utf8mb4_general_ci" | "utf8mb4_bin" | "utf8mb4_unicode_ci" | "utf8mb4_icelandic_ci" | "utf8mb4_latvian_ci" | "utf8mb4_romanian_ci" | "utf8mb4_slovenian_ci" | "utf8mb4_polish_ci" | "utf8mb4_estonian_ci" | "utf8mb4_spanish_ci" | "utf8mb4_swedish_ci" | "utf8mb4_turkish_ci" | "utf8mb4_czech_ci" | "utf8mb4_danish_ci" | "utf8mb4_lithuanian_ci" | "utf8mb4_slovak_ci" | "utf8mb4_spanish2_ci" | "utf8mb4_roman_ci" | "utf8mb4_persian_ci" | "utf8mb4_esperanto_ci" | "utf8mb4_hungarian_ci" | "utf8mb4_sinhala_ci" | "cp1251_bulgarian_ci" | "cp1251_ukrainian_ci" | "cp1251_bin" | "cp1251_general_ci" | "cp1251_general_cs" | "utf16_general_ci" | "utf16_bin" | "utf16_unicode_ci" | "utf16_icelandic_ci" | "utf16_latvian_ci" | "utf16_romanian_ci" | "utf16_slovenian_ci" | "utf16_polish_ci" | "utf16_estonian_ci" | "utf16_spanish_ci" | "utf16_swedish_ci" | "utf16_turkish_ci" | "utf16_czech_ci" | "utf16_danish_ci" | "utf16_lithuanian_ci" | "utf16_slovak_ci" | "utf16_spanish2_ci" | "utf16_roman_ci" | "utf16_persian_ci" | "utf16_esperanto_ci" | "utf16_hungarian_ci" | "utf16_sinhala_ci" | "cp1256_general_ci" | "cp1256_bin" | "cp1257_lithuanian_ci" | "cp1257_bin" | "cp1257_general_ci" | "utf32_general_ci" | "utf32_bin" | "utf32_unicode_ci" | "utf32_icelandic_ci" | "utf32_latvian_ci" | "utf32_romanian_ci" | "utf32_slovenian_ci" | "utf32_polish_ci" | "utf32_estonian_ci" | "utf32_spanish_ci" | "utf32_swedish_ci" | "utf32_turkish_ci" | "utf32_czech_ci" | "utf32_danish_ci" | "utf32_lithuanian_ci" | "utf32_slovak_ci" | "utf32_spanish2_ci" | "utf32_roman_ci" | "utf32_persian_ci" | "utf32_esperanto_ci" | "utf32_hungarian_ci" | "utf32_sinhala_ci" | "binary" | "geostd8_general_ci" | "geostd8_bin" | "cp932_japanese_ci" | "cp932_bin" | "eucjpms_japanese_ci" | "eucjpms_bin" | "utf8mb4_unicode_520_ci"

	// The transaction completion type (0 - default, 1 - chain, 2 - release)
	completion_type?: int & >=0 & <=2

	// Allows INSERT and SELECT statements to run concurrently for MyISAM tables that have no free blocks in the middle of the data file.
	concurrent_insert?: int & >=0 & <=2

	// The number of seconds that the MySQLd server waits for a connect packet before responding with Bad handshake.
	connect_timeout?: int & >=2 & <=31536000

	// Write a core file if mysqld dies.
	"core-file"?: string & "0" | "1" | "OFF" | "ON"

	// smartengine enable
	"smartengine"?: string & "0" | "1" | "OFF" | "ON"

	// smartengine enable
	"loose_smartengine"?: string & "0" | "1" | "OFF" | "ON"

	// Abort a recursive common table expression if it does more than this number of iterations.
	cte_max_recursion_depth: int & >=0 & <=4294967295 | *1000

	// MySQL data directory
	datadir?: string

	// The default authentication plugin
	default_authentication_plugin?: string & "mysql_native_password" | "sha256_password" | "caching_sha2_password"

	// Controls default collation for utf8mb4 while replicating implicit utf8mb4 collations.
	default_collation_for_utf8mb4?: string & "utf8mb4_0900_ai_ci" | "utf8mb4_general_ci"

	// Defines the global automatic password expiration policy.
	default_password_lifetime: int & >=0 & <=65535 | *0

	// The default storage engine (table type).
	default_storage_engine?: string & "InnoDB" | "MRG_MYISAM" | "BLACKHOLE" | "CSV" | "MEMORY" | "FEDERATED" | "ARCHIVE" | "MyISAM" | "xengine" | "XENGINE" | "smartengine" | "SMARTENGINE" | "INNODB" | "innodb"

	// Server current time zone
	default_time_zone?: string

	// The default storage engine for TEMPORARY tables.
	default_tmp_storage_engine?: string

	// The default mode value to use for the WEEK() function.
	default_week_format?: int & >=0 & <=7

	// After inserting delayed_insert_limit delayed rows, the INSERT DELAYED handler thread checks whether there are any SELECT statements pending. If so, it allows them to execute before continuing to insert delayed rows.
	delayed_insert_limit?: int & >=1 & <=9223372036854775807

	// How many seconds an INSERT DELAYED handler thread should wait for INSERT statements before terminating.
	delayed_insert_timeout?: int & >=1 & <=31536000

	// If the queue becomes full, any client that issues an INSERT DELAYED statement waits until there is room in the queue again.
	delayed_queue_size?: int & >=1 & <=9223372036854775807

	// Determines when keys are flushed for MyISAM tables
	delay_key_write?: string & "OFF" | "ON" | "ALL"

	// This variable indicates which storage engines cannot be used to create tables or tablespaces.
	disabled_storage_engines?: string

	// Controls how the server handles clients with expired passwords:
	disconnect_on_expired_password?: string & "0" | "1" | "OFF" | "ON"

	// Number of digits by which to increase the scale of the result of division operations.
	div_precision_increment?: int & >=0 & <=30

	// Whether optimizer JSON output should add end markers.
	end_markers_in_json?: string & "0" | "1" | "OFF" | "ON"

	// Prevents execution of statements that cannot be logged in a transactionally safe manner
	enforce_gtid_consistency?: string & "OFF" | "WARN" | "ON"

	// Number of equality ranges when the optimizer should switch from using index dives to index statistics.
	eq_range_index_dive_limit?: int & >=0 & <=4294967295

	// Indicates the status of the Event Scheduler
	event_scheduler?: string & "ON" | "OFF"

	expire_logs_days: int & >=0 & <=4294967295 | *2592000

	// Needed for 5.6.7
	explicit_defaults_for_timestamp: string & "0" | "1" | "OFF" | "ON" | *"1"

	// If ON, the server flushes all changes to disk after each SQL statement.
	flush?: string & "0" | "1" | "OFF" | "ON"

	// Frees up resources and synchronize unflushed data to disk. Recommended only on systems with minimal resources.
	flush_time?: int & >=0 & <=31536000

	foreign_key_checks: string & "0" | "1" | "OFF" | "ON" | *"1"

	// The list of operators supported by boolean full-text searches
	ft_boolean_syntax?: string

	// Maximum length of the word to be included in a FULLTEXT index.
	ft_max_word_len?: int & >=10 & <=84

	// Minimum length of the word to be included in a FULLTEXT index.
	ft_min_word_len?: int & >=1 & <=84

	// Number of top matches to use for full-text searches performed using WITH QUERY EXPANSION.
	ft_query_expansion_limit?: int & >=0 & <=1000

	// File for Full Search Stop Words. NULL uses Default, /dev/null to disable Stop Words
	ft_stopword_file?: string & "/dev/null"

	// Whether the general query log is enabled
	general_log?: string & "0" | "1" | "OFF" | "ON"

	// Location of mysql general log.
	general_log_file?: string

	// Maximum allowed result length in bytes for the GROUP_CONCAT().
	group_concat_max_len?: int & >=4 & <=18446744073709547520

	// Compress the mysql.gtid_executed table each time this many transactions have taken place.
	gtid_executed_compression_period?: int & >=0 & <=4294967295

	gtid_mode?: string & "0" | "OFF" | "ON" | "1"

	// Controls whether GTID based logging is enabled and what type of transactions the logs can contain
	"gtid-mode"?: string & "OFF" | "OFF_PERMISSIVE" | "ON_PERMISSIVE" | "ON"

	gtid_owned?: string

	// The set of all GTIDs that have been purged from the binary log
	gtid_purged?: string

	// Controls default collation for utf8mb4 while replicating implicit utf8mb4 collations.
	histogram_generation_max_mem_size: int & >=1000000 & <=18446744073709551615 | *20000000

	// The size of the internal host cache.
	host_cache_size?: int & >=0 & <=65536

	// The number of seconds after which mysqld server will fetch data from storage engine and replace the data in cache.
	information_schema_stats_expiry: int & >=0 & <=31536000 | *86400

	// String to be executed by the server for each client that connects.
	init_connect?: string

	init_file?: string

	init_slave?: string

	// Enables InnoDB Adaptive Flushing (default=on for RDS)
	innodb_adaptive_flushing?: string & "0" | "1" | "OFF" | "ON"

	// Low water mark representing percentage of redo log capacity at which adaptive flushing is enabled.
	innodb_adaptive_flushing_lwm?: int & >=0 & <=70

	// Whether innodb adaptive hash indexes are enabled or disabled
	innodb_adaptive_hash_index?: string & "0" | "1" | "OFF" | "ON"

	// Partitions the adaptive hash index search system.
	innodb_adaptive_hash_index_parts?: int & >=1 & <=512

	// Allows InnoDB to automatically adjust the value of innodb_thread_sleep_delay up or down according to the current workload.
	innodb_adaptive_max_sleep_delay?: int & >=0 & <=1000000

	// The increment size (in MB) for extending the size of an auto-extending tablespace file when it becomes full
	innodb_autoextend_increment?: int & >=1 & <=1000

	// The locking mode to use for generating auto-increment values
	innodb_autoinc_lock_mode?: int & >=0 & <=2

	// Defines the chunk size for online InnoDB buffer pool resizing operations.
	innodb_buffer_pool_chunk_size?: int & >=1048576 & <=4294967295

	// Specifies whether to record the pages cached in the InnoDB buffer pool when the MySQL server is shut down.
	innodb_buffer_pool_dump_at_shutdown?: string & "0" | "1" | "OFF" | "ON"

	// Immediately records the pages cached in the InnoDB buffer pool.
	innodb_buffer_pool_dump_now?: string & "0" | "1" | "OFF" | "ON"

	// Specifies the percentage of the most recently used pages for each buffer pool to read out and dump.
	innodb_buffer_pool_dump_pct?: int & >=1 & <=100

	// Specifies the file that holds the list of page numbers produced by innodb_buffer_pool_dump_at_shutdown or innodb_buffer_pool_dump_now.
	innodb_buffer_pool_filename?: string

	// The number of regions that the InnoDB buffer pool is divided into
	innodb_buffer_pool_instances?: int & >=1 & <=64

	// Interrupts the process of restoring InnoDB buffer pool contents triggered by innodb_buffer_pool_load_at_startup or innodb_buffer_pool_load_now.
	innodb_buffer_pool_load_abort?: string & "0" | "1" | "OFF" | "ON"

	// Specifies that, on MySQL server startup, the InnoDB buffer pool is automatically warmed up by loading the same pages it held at an earlier time.
	innodb_buffer_pool_load_at_startup?: string & "0" | "1" | "OFF" | "ON"

	// Immediately warms up the InnoDB buffer pool by loading a set of data pages, without waiting for a server restart.
	innodb_buffer_pool_load_now?: string & "0" | "1" | "OFF" | "ON"

	// The size in bytes of the memory buffer innodb uses to cache data and indexes of its tables
	innodb_buffer_pool_size?: int & >=5242880 & <=18446744073709551615 @k8sResource(quantity)

	// Controls InnoDB change buffering
	innodb_change_buffering?: string & "inserts" | "deletes" | "purges" | "changes" | "all" | "none"

	// Maximum size for the InnoDB change buffer, as a percentage of the total size of the buffer pool.
	innodb_change_buffer_max_size?: int & >=0 & <=50

	// This is a debug option that is only intended for expert debugging use. It disables checkpoints so that a deliberate server exit always initiates InnoDB recovery.
	innodb_checkpoint_disabled?: string & "0" | "1" | "OFF" | "ON"

	// Specifies how to generate and verify the checksum stored in each disk block of each InnoDB tablespace.
	innodb_checksum_algorithm?: string & "crc32" | "innodb" | "none" | "strict_crc32" | "strict_innodb" | "strict_none"

	// Enables per-index compression-related statistics in the INFORMATION_SCHEMA.INNODB_CMP_PER_INDEX table.
	innodb_cmp_per_index_enabled?: string & "0" | "1" | "OFF" | "ON"

	// The number of threads that can commit at the same time.
	innodb_commit_concurrency?: int & >=0 & <=1000

	// Sets the cutoff point at which MySQL begins adding padding within compressed pages to avoid expensive compression failures.
	innodb_compression_failure_threshold_pct?: int & >=0 & <=100

	// Sets the cutoff point at which MySQL begins adding padding within compressed pages to avoid expensive compression failures.
	innodb_compression_level?: int & >=0 & <=9

	// Specifies the maximum percentage that can be reserved as free space within each compressed page, allowing room to reorganize the data and modification log within the page when a compressed table or index is updated and the data might be recompressed.
	innodb_compression_pad_pct_max?: int & >=0 & <=75

	// Number of times a thread can enter and leave Innodb before it is subject to innodb-thread-concurrency
	innodb_concurrency_tickets?: int & >=1 & <=4294967295

	innodb_data_file_path?: string

	// Directory where innodb files are stored
	innodb_data_home_dir?: string

	// Enable this debug option to reset DDL log crash injection counters to 1.
	innodb_ddl_log_crash_reset_debug?: string & "0" | "1" | "OFF" | "ON"

	// This option is used to disable deadlock detection.
	innodb_deadlock_detect?: string & "0" | "1" | "OFF" | "ON"

	// Automatically scale innodb_buffer_pool_size and innodb_log_file_size based on system memory. Also set innodb_flush_method=O_DIRECT_NO_FSYNC, if supported.
	innodb_dedicated_server?: string & "0" | "1" | "OFF" | "ON"

	// Defines the default row format for InnoDB tables (including user-created InnoDB temporary tables).
	innodb_default_row_format?: string & "DYNAMIC" | "COMPACT" | "REDUNDANT"

	// Defines directories to scan at startup for tablespace files.
	innodb_directories?: string

	// If enabled, this variable disables the operating system file system cache for merge-sort temporary files.
	innodb_disable_sort_file_cache?: string & "0" | "1" | "OFF" | "ON"

	innodb_doublewrite: string & "0" | "1" | "OFF" | "ON" | *"1"

	// Defines the number of doublewrite pages to write in a batch.
	innodb_doublewrite_batch_size: int & >=0 & <=256 | *16

	// Defines the number of doublewrite files.
	innodb_doublewrite_files?: int & >=2 & <=256

	// Defines the maximum number of doublewrite pages per thread for a batch write. If no value is specified, innodb_doublewrite_pages is set to the innodb_write_io_threads value.
	innodb_doublewrite_pages: int & >=1 & <=512 | *32

	// The InnoDB shutdown mode
	innodb_fast_shutdown?: int & 0 | 1 | 2

	// Use tablespaces or files for Innodb.
	innodb_file_per_table: string & "0" | "1" | "OFF" | "ON" | *"1"

	// Defines the percentage of space on each B-tree page that is filled during a sorted index build, with the remaining space reserved for future index growth.
	innodb_fill_factor?: int & >=10 & <=100

	// Number of iterations for which InnoDB keeps the previously calculated snapshot of the flushing state, controlling how quickly adaptive flushing responds to changing workloads.
	innodb_flushing_avg_loops?: int & >=1 & <=1000

	// Write and flush the logs every N seconds. This setting has an effect only when innodb_flush_log_at_trx_commit has a value of 2.
	innodb_flush_log_at_timeout?: int & >=0 & <=2700

	// Determines Innodb transaction durability
	innodb_flush_log_at_trx_commit?: int & >=0 & <=2

	// Determines Innodb flush method
	innodb_flush_method?: string & "O_DIRECT"

	// Specifies whether flushing a page from the InnoDB buffer pool also flushes other dirty pages in the same extent.
	innodb_flush_neighbors?: int & >=0 & <=2

	// Ignore the innodb_io_capacity setting to be ignored for bursts of I/O activity that occur at checkpoints.
	innodb_flush_sync?: string & "0" | "1" | "OFF" | "ON"

	// Lets InnoDB load tables at startup that are marked as corrupted
	innodb_force_load_corrupted?: string & "0" | "1" | "OFF" | "ON"

	innodb_force_recovery: int & >=0 & <=6 | *0

	// Specifies the qualified name of an InnoDB table containing a FULLTEXT index.
	innodb_ft_aux_table?: string

	// Size of the cache that holds a parsed document in memory while creating an InnoDB FULLTEXT index.
	innodb_ft_cache_size?: int & >=0 & <=4294967295

	// Whether to enable additional full-text search (FTS) diagnostic output.
	innodb_ft_enable_diag_print?: string & "0" | "1" | "OFF" | "ON"

	// Specifies that a set of stopwords is associated with an InnoDB FULLTEXT index at the time the index is created.
	innodb_ft_enable_stopword?: string & "0" | "1" | "OFF" | "ON"

	// Maximum length of words that are stored in an InnoDB FULLTEXT index.
	innodb_ft_max_token_size?: int & >=10 & <=252

	// Minimum length of words that are stored in an InnoDB FULLTEXT index.
	innodb_ft_min_token_size?: int & >=0 & <=16

	// Number of words to process during each OPTIMIZE TABLE operation on an InnoDB FULLTEXT index.
	innodb_ft_num_word_optimize?: int & >=1 & <=4294967295

	// The InnoDB FULLTEXT search (FTS) query result cache limit (defined in bytes) per FTS query or per thread.
	innodb_ft_result_cache_limit?: int & >=1000000 & <=4294967295

	// Name of the table containing a list of words to ignore when creating an InnoDB FULLTEXT index, in the format db_name/table_name.
	innodb_ft_server_stopword_table?: string

	// Number of threads used in parallel to index and tokenize text in an InnoDB FULLTEXT index, when building a search index for a large table.
	innodb_ft_sort_pll_degree?: int & >=1 & <=32

	// The total memory allocated, in bytes, for the InnoDB FULLTEXT search index cache for all tables.
	innodb_ft_total_cache_size?: int & >=32000000 & <=1600000000

	// Name of the table containing a list of words to ignore when creating an InnoDB FULLTEXT index, in the format db_name/table_name.
	innodb_ft_user_stopword_table?: string

	// The maximum number of I/O operations per second that InnoDB will perform.
	innodb_io_capacity?: int & >=100 & <=18446744073709551615

	// The limit up to which InnoDB is allowed to extend the innodb_io_capacity setting in case of emergency.
	innodb_io_capacity_max?: int & >=2000 & <=18446744073709547520

	// Timeout in seconds an innodb transaction may wait for a row lock before giving up
	innodb_lock_wait_timeout?: int & >=1 & <=1073741824

	// The size in bytes of the buffer that innodb uses to write to the log files on disk
	innodb_log_buffer_size: int & >=262144 & <=4294967295 | *8388608

	// Enables or disables checksums for redo log pages.
	innodb_log_checksums?: string & "0" | "1" | "OFF" | "ON"

	// Specifies whether images of re-compressed pages are stored in InnoDB redo logs.
	innodb_log_compressed_pages?: string & "0" | "1" | "OFF" | "ON"

	innodb_log_files_in_group: int & >=2 & <=100 | *2

	// The size in bytes of each log file in a log group
	innodb_log_file_size: int & >=4194304 & <=273804165120 | *134217728

	// The directory path to the innodb log files
	innodb_log_group_home_dir?: string

	// Defines the minimum amount of CPU usage below which user threads no longer spin while waiting for flushed redo.
	innodb_log_spin_cpu_abs_lwm: int & >=0 & <=4294967295 | *80

	// Defines the maximum amount of CPU usage above which user threads no longer spin while waiting for flushed redo.
	innodb_log_spin_cpu_pct_hwm: int & >=0 & <=100 | *50

	// Defines the maximum average log flush time beyond which user threads no longer spin while waiting for flushed redo.
	innodb_log_wait_for_flush_spin_hwm: int & >=0 & <=18446744073709551615 | *400

	// The write-ahead block size for the redo log, in bytes.
	innodb_log_write_ahead_size?: int & >=512 & <=16384

	// A parameter that influences the algorithms and heuristics for the flush operation for the InnoDB buffer pool.
	innodb_lru_scan_depth?: int & >=100 & <=18446744073709551615

	// Maximum percentage of dirty pages in the buffer pool
	innodb_max_dirty_pages_pct?: int & >=0 & <=99

	// Low water mark representing percentage of dirty pages where preflushing is enabled to control the dirty page ratio.
	innodb_max_dirty_pages_pct_lwm?: float & >=0 & <=99.99

	// Controls how to delay INSERT, UPDATE, and DELETE operations when purge operations are lagging
	innodb_max_purge_lag?: int & >=0 & <=4294967295

	// Specifies the maximum delay in milliseconds for the delay imposed by the innodb_max_purge_lag configuration option
	innodb_max_purge_lag_delay?: int & >=0 & <=18446744073709551615

	// Defines a threshold size for undo tablespaces.
	innodb_max_undo_log_size?: int & >=10485760 & <=18446744073709551615

	// Turns off one or more counters in the information_schema.innodb_metrics table.
	innodb_monitor_disable?: string

	// Turns on one or more counters in the information_schema.innodb_metrics table.
	innodb_monitor_enable?: string

	// Resets to zero the count value for one or more counters in the information_schema.innodb_metrics table.
	innodb_monitor_reset?: string

	// Resets all values (minimum, maximum, and so on) for one or more counters in the information_schema.innodb_metrics table.
	innodb_monitor_reset_all?: string

	// Specifies the approximate percentage of the InnoDB buffer pool used for the old block sublist.
	innodb_old_blocks_pct?: int & >=5 & <=95

	// Specifies how long in milliseconds (ms) a block inserted into the old sublist must stay there after its first access before it can be moved to the new sublist.
	innodb_old_blocks_time?: int & >=0 & <=4294967295

	// Specifies an upper limit on the size of the temporary log files used during online DDL operations for InnoDB tables.
	innodb_online_alter_log_max_size?: int & >=65536 & <=18446744073709551615

	// Relevant only if you use multiple tablespaces in innodb. It specifies the maximum number of .ibd files that innodb can keep open at one time
	innodb_open_files?: int & >=10 & <=4294967295

	// Changes the way the OPTIMIZE TABLE statement operates on InnoDB tables.
	innodb_optimize_fulltext_only?: string & "0" | "1" | "OFF" | "ON"

	// The number of page cleaner threads that flush dirty pages from buffer pool instances.
	innodb_page_cleaners?: int & >=1 & <=64

	// Specifies the page size for all InnoDB tablespaces in a MySQL instance.
	innodb_page_size?: string

	// Defines the number of threads that can be used for parallel clustered index reads.
	innodb_parallel_read_threads?: int & >=1 & <=256

	// When this option is enabled, information about all deadlocks in InnoDB user transactions is recorded in the mysqld error log.
	innodb_print_all_deadlocks?: string & "0" | "1" | "OFF" | "ON"

	// Enabling this option causes MySQL to write DDL logs to stderr.
	innodb_print_ddl_logs: string & "0" | "1" | "OFF" | "ON" | *"0"

	// The granularity of changes, expressed in units of redo log records, that trigger a purge operation, flushing the changed buffer pool blocks to disk
	innodb_purge_batch_size?: int & >=1 & <=5000

	// Defines the frequency with which the purge system frees rollback segments.
	innodb_purge_rseg_truncate_frequency?: int & >=1 & <=128

	// The number of background threads devoted to the InnoDB purge operation
	innodb_purge_threads: int & >=1 & <=32 | *1

	// Enables or disables Innodb Random Read Ahead
	innodb_random_read_ahead?: string & "0" | "1" | "OFF" | "ON"

	// Controls the sensitivity of linear read-ahead that InnoDB uses to prefetch pages into the buffer cache.
	innodb_read_ahead_threshold?: int & >=0 & <=64

	// The number of I/O threads for read operations in InnoDB.
	innodb_read_io_threads?: int & >=1 & <=64

	// Starts the server in read-only mode.
	innodb_read_only?: string & "0" | "1" | "OFF" | "ON"

	// Controls encryption of redo log data for tables encrypted using the InnoDB tablespace encryption feature.
	innodb_redo_log_encrypt: string & "0" | "1" | "OFF" | "ON" | *"0"

	// The replication thread delay (in ms) on a slave server if innodb_thread_concurrency is reached.
	innodb_replication_delay?: int & >=0 & <=4294967295

	// Controls whether timeouts rollback the last statement or the entire transaction
	innodb_rollback_on_timeout?: string & "0" | "1" | "OFF" | "ON"

	// Defines how many of the rollback segments in the system tablespace that InnoDB uses within a transaction.
	innodb_rollback_segments?: int & >=1 & <=128

	// If tablespace map files are lost or corrupted, the innodb_scan_directories startup option can be used to specify tablespace file directories. This option causes InnoDB to read the first page of each tablespace file in the specified directories and recreate tablespace map files so that the recovery process can apply redo logs to affected tablespaces.
	innodb_scan_directories?: string

	// Defines the percentage of tablespace file segment pages reserved as empty pages
	innodb_segment_reserve_factor: float & >=0.03 & <=40 | *12.5

	// Specifies the sizes of several buffers used for sorting data during creation of an InnoDB index.
	innodb_sort_buffer_size?: int & >=65536 & <=67108864

	// The maximum delay between polls for a spin lock.
	innodb_spin_wait_delay?: int & >=0 & <=4294967295

	// Causes InnoDB to automatically recalculate persistent statistics after the data in a table is changed substantially.
	innodb_stats_auto_recalc?: string & "0" | "1" | "OFF" | "ON"

	// When innodb_stats_include_delete_marked is enabled, ANALYZE TABLE considers delete-marked records when recalculating statistics.
	innodb_stats_include_delete_marked: string & "0" | "1" | "OFF" | "ON" | *"0"

	// How the server treats NULL values when collecting statistics about the distribution of index values for InnoDB tables.
	innodb_stats_method?: string & "nulls_equal" | "nulls_unequal" | "nulls_ignored"

	// Controls whether table and index stats are updated when getting status information via SHOW STATUS or the INFORMATION_SCHEMA
	innodb_stats_on_metadata?: string & "0" | "1" | "OFF" | "ON"

	// Specifies whether the InnoDB index statistics produced by the ANALYZE TABLE command are stored on disk.
	innodb_stats_persistent?: string & "OFF" | "ON" | "0" | "1"

	// The number of index pages to sample when estimating cardinality and other statistics for an indexed column, such as those calculated by ANALYZE TABLE.
	innodb_stats_persistent_sample_pages?: int & >=0 & <=18446744073709551615

	// The number of index pages to sample when estimating cardinality and other statistics for an indexed column, such as those calculated by ANALYZE TABLE.
	innodb_stats_transient_sample_pages?: int & >=0 & <=18446744073709551615

	// Enables or disables periodic output for the standard InnoDB Monitor.
	innodb_status_output?: string & "0" | "1" | "OFF" | "ON"

	// Enables or disables the InnoDB Lock Monitor.
	innodb_status_output_locks?: string & "0" | "1" | "OFF" | "ON"

	// Whether InnoDB returns errors rather than warnings for exceptional conditions.
	innodb_strict_mode?: string & "0" | "1" | "OFF" | "ON"

	// Splits an internal data structure used to coordinate threads, for higher concurrency in workloads with large numbers of waiting threads.
	innodb_sync_array_size?: int & >=1 & <=1024

	// The number of times a thread waits for an innodb mutex to be freed before the thread is suspended
	innodb_sync_spin_loops?: int & >=0 & <=9223372036854775807

	// If autocommit = 0, innodb honors LOCK TABLES
	innodb_table_locks?: string & "0" | "1" | "OFF" | "ON"

	// Specifies the path, file name, and file size for InnoDB temporary table tablespace data files.
	innodb_temp_data_file_path?: string

	// The number of threads that can enter innodb concurrently
	innodb_thread_concurrency?: int & >=0 & <=1000

	// How long innodb threads sleep before joining the innodb queue, in microseconds.
	innodb_thread_sleep_delay?: int & >=0 & <=9223372036854775807

	innodb_tmpdir?: string

	// The relative or absolute directory path where InnoDB creates separate tablespaces for the undo logs
	innodb_undo_directory?: string

	// Controls encryption of undo log data for tables encrypted using the InnoDB tablespace encryption feature.
	innodb_undo_log_encrypt: string & "0" | "1" | "OFF" | "ON" | *"0"

	// Undo tablespaces that exceed the threshold value defined by innodb_max_undo_log_size are marked for truncation.
	innodb_undo_log_truncate?: string & "0" | "1" | "OFF" | "ON"

	// The number of tablespace files that the undo logs are divided between, when you use a non-zero innodb_undo_logs setting.
	innodb_undo_tablespaces?: int & >=0 & <=126

	// Controls whether or not MySQL uses Linux native asynchronous IO.
	innodb_use_native_aio?: string & "0" | "1" | "OFF" | "ON"

	// The number of I/O threads for write operations in InnoDB.
	innodb_write_io_threads?: int & >=1 & <=64

	// Number of seconds the server waits for activity on an interactive connection before closing it.
	interactive_timeout?: int & >=1 & <=31536000

	// The default storage engine for in-memory internal temporary tables.
	internal_tmp_mem_storage_engine?: string & "TempTable" | "MEMORY"

	// Increase the value of join_buffer_size to get a faster full join when adding indexes is not possible.
	join_buffer_size?: int & >=128 & <=18446744073709547520

	// Suppress behavior to overwrite MyISAM file created in DATA DIRECTORY or INDEX DIRECTORY.
	keep_files_on_create?: string & "0" | "1" | "OFF" | "ON"

	// Increase the buffer size to get better index handling used for index blocks (for all reads and multiple writes).
	key_buffer_size: int & >=8 & <=9223372036854771712 | *16777216

	// Controls the demotion of buffers from the hot sub-chain of a key cache to the warm sub-chain. Lower values cause demotion to happen more quickly.
	key_cache_age_threshold?: int & >=100 & <=18446744073709547520

	// Size in bytes of blocks in the key cache.
	key_cache_block_size?: int & >=512 & <=16384

	// The division point between the hot and warm sub-chains of the key cache buffer chain. The value is the percentage of the buffer chain to use for the warm sub-chain.
	key_cache_division_limit?: int & >=1 & <=100

	keyring_operations: string & "0" | "1" | "OFF" | "ON" | *"1"

	large_pages: string & "0" | "1" | "OFF" | "ON" | *"0"

	lc_messages?: string

	lc_messages_dir?: string

	// This variable specifies the locale that controls the language used to display day and month names and abbreviations.
	lc_time_names?: string & "ar_AE" | "ar_BH" | "ar_DZ" | "ar_EG" | "ar_IN" | "ar_IQ" | "ar_JO" | "ar_KW" | "ar_LB" | "ar_LY" | "ar_MA" | "ar_OM" | "ar_QA" | "ar_SA" | "ar_SD" | "ar_SY" | "ar_TN" | "ar_YE" | "be_BY" | "bg_BG" | "ca_ES" | "cs_CZ" | "da_DK" | "de_AT" | "de_BE" | "de_CH" | "de_DE" | "de_LU" | "el_GR" | "en_AU" | "en_CA" | "en_GB" | "en_IN" | "en_NZ" | "en_PH" | "en_US" | "en_ZA" | "en_ZW" | "es_AR" | "es_BO" | "es_CL" | "es_CO" | "es_CR" | "es_DO" | "es_EC" | "es_ES" | "es_GT" | "es_HN" | "es_MX" | "es_NI" | "es_PA" | "es_PE" | "es_PR" | "es_PY" | "es_SV" | "es_US" | "es_UY" | "es_VE" | "et_EE" | "eu_ES" | "fi_FI" | "fo_FO" | "fr_BE" | "fr_CA" | "fr_CH" | "fr_FR" | "fr_LU" | "gl_ES" | "gu_IN" | "he_IL" | "hi_IN" | "hr_HR" | "hu_HU" | "id_ID" | "is_IS" | "it_CH" | "it_IT" | "ja_JP" | "ko_KR" | "lt_LT" | "lv_LV" | "mk_MK" | "mn_MN" | "ms_MY" | "nb_NO" | "nl_BE" | "nl_NL" | "no_NO" | "pl_PL" | "pt_BR" | "pt_PT" | "ro_RO" | "ru_RU" | "ru_UA" | "sk_SK" | "sl_SI" | "sq_AL" | "sr_RS" | "sv_FI" | "sv_SE" | "ta_IN" | "te_IN" | "th_TH" | "tr_TR" | "uk_UA" | "ur_PK" | "vi_VN" | "zh_CN" | "zh_HK" | "zh_TW"

	// Controls whetther LOCAL is supported for LOAD DATA INFILE
	local_infile: string & "0" | "1" | "OFF" | "ON" | *"1"

	// Specifies the timeout in seconds for attempts to acquire metadata locks
	lock_wait_timeout?: int & >=1 & <=31536000

	log_bin?: string

	// Controls binary logging.
	"log-bin"?: string

	log_bin_basename?: string

	log_bin_index?: string

	// Enforces restrictions on stored functions / triggers - logging for replication.
	log_bin_trust_function_creators?: string & "0" | "1" | "OFF" | "ON"

	// Whether MySQL writes binary log events using a Version 1 or Version 2 logging events
	log_bin_use_v1_row_events?: string & "0" | "1" | "OFF" | "ON"

	// Location of error log.
	log_error?: string

	log_error_services?: string

	// Controls verbosity of the server in writing error, warning, and note messages to the error log.
	log_error_verbosity?: int & >=1 & <=3

	// Controls where to store query logs
	log_output?: string & "TABLE" | "FILE" | "NONE"

	// Logs queries that are expected to retrieve all rows to slow query log
	log_queries_not_using_indexes?: string & "0" | "1" | "OFF" | "ON"

	// Allow for chain replication - ingression
	log_slave_updates: string & "0" | "1" | "OFF" | "ON" | *"1"

	// Include slow administrative statements in the statements written to the slow query log.
	log_slow_admin_statements?: string & "0" | "1" | "OFF" | "ON"

	// When the slow query log is enabled and the output destination as FILE, additional information related to the slow query is written to the slow query log file. TABLE output is unaffected.
	log_slow_extra?: string & "ON" | "OFF"

	// When the slow query log is enabled, this variable enables logging for queries that have taken more than long_query_time seconds to execute on the slave.
	log_slow_slave_statements?: string & "0" | "1" | "OFF" | "ON"

	// If error 1592 is encountered, controls whether the generated warnings are added to the error log or not.
	log_statements_unsafe_for_binlog: string & "0" | "1" | "OFF" | "ON" | *"0"

	// Whether to write error log output to syslog.
	log_syslog?: string & "0" | "1" | "OFF" | "ON"

	// Limits the number of such queries per minute that can be written to the slow query log.
	log_throttle_queries_not_using_indexes?: int & >=0 & <=4294967295

	// This variable controls the timestamp time zone of error log messages, and of general query log and slow query log messages written to files.
	log_timestamps?: string & "UTC" | "SYSTEM"

	// Defines what MySQL considers long queries
	long_query_time?: float & >=0 & <=3.1536e+07

	lower_case_file_system?: string & "0" | "1" | "OFF" | "ON"

	// Affects how the server handles identifier case sensitivity
	lower_case_table_names?: int & >=0 & <=1

	// INSERT, UPDATE, DELETE, and LOCK TABLE WRITE wait until no pending SELECT. Affects only storage engines that use only table-level locking (MyISAM, MEMORY, MERGE).
	low_priority_updates?: string & "0" | "1" | "OFF" | "ON"

	// All the specified roles are always considered granted to every user and they can't be revoked. Mandatory roles still require activation unless they are made into default roles. The granted roles will not be visible in the mysql.role_edges table.
	mandatory_roles?: string

	master_info_repository?: string

	// This option causes the server to write its master info log to a file or a table.
	"master-info-repository"?: string & "FILE" | "TABLE"

	// Enabling this variable causes the master to examine checksums when reading from the binary log.
	master_verify_checksum?: string & "0" | "1" | "OFF" | "ON"

	// This value by default is small, to catch large (possibly incorrect) packets. Must be increased if using large BLOB columns or long strings. As big as largest BLOB.
	max_allowed_packet?: int & >=16777216 & <=1073741824

	// Maximum binlog cache size a transaction may use
	max_binlog_cache_size?: int & >=4096 & <=18446744073709547520

	// Server rotates the binlog once it reaches this size
	max_binlog_size: int & >=4096 & <=1073741824 | *134217728

	// If nontransactional statements within a transaction require more than this many bytes of memory, the server generates an error.
	max_binlog_stmt_cache_size?: int & >=4096 & <=18446744073709547520

	// A host is blocked from further connections if there are more than this number of interrupted connections
	max_connect_errors?: int & >=1 & <=9223372036854775807

	// The number of simultaneous client connections allowed.
	max_connections?: int & >=1 & <=100000

	// Do not start more than this number of threads to handle INSERT DELAYED statements.
	max_delayed_threads?: int & >=0 & <=16384

	// The maximum number of bytes available for computing statement digests.
	max_digest_length: int & >=0 & <=1048576 | *1024

	// The maximum number of error, warning, and note messages to be stored for display.
	max_error_count?: int & >=0 & <=65535

	// The execution timeout for SELECT statements, in milliseconds.
	max_execution_time?: int & >=0 & <=18446744073709551615

	// Maximum size to which MEMORY tables are allowed to grow.
	max_heap_table_size?: int & >=16384 & <=1844674407370954752

	// Synonym for max_delayed_threads
	max_insert_delayed_threads?: int & >=0 & <=16384

	// Catch SELECT statements where keys are not used properly and would probably take a long time.
	max_join_size?: int & >=1 & <=18446744073709551615

	// ORDER BY Optimization. The cutoff on the size of index values that determines which filesort algorithm to use.
	max_length_for_sort_data?: int & >=4 & <=8388608

	// The maximum value of the points_per_circle argument to the ST_Buffer_Strategy() function.
	max_points_in_geometry?: int & >=3 & <=3145728

	// Used if the potential for denial-of-service attacks based on running the server out of memory by preparing huge numbers of statements.
	max_prepared_stmt_count?: int & >=0 & <=1048576

	max_relay_log_size: int & >=0 & <=1073741824 | *0

	// A low value can force MySQL to prefer indexes instead of table scans.
	max_seeks_for_key?: int & >=1 & <=18446744073709547520

	// The number of bytes to use when sorting BLOB or TEXT values.
	max_sort_length?: int & >=4 & <=8388608

	// Limits the number of times a stored procedure can be called recursively minimizing the demand on thread stack space.
	max_sp_recursion_depth?: int & >=0 & <=255

	// Maximum number of simultaneous connections allowed to any given MySQL account.
	max_user_connections?: int & >=0 & <=4294967295

	// After this many write locks, allow some pending read lock requests to be processed in between.
	max_write_lock_count?: int & >=1 & <=18446744073709547520

	// Defines the path to the mecabrc configuration file.
	mecab_rc_file?: string & "0" | "1"

	// Can be used to cause queries which examine fewer than the stated number of rows not to be logged.
	min_examined_row_limit?: int & >=0 & <=18446744073709547520

	// The default pointer size in bytes, to be used by CREATE TABLE for MyISAM tables when no MAX_ROWS option is specified.
	myisam_data_pointer_size?: int & >=2 & <=7

	// The maximum size of the temporary file that MySQL is allowed to use while re-creating a MyISAM index
	myisam_max_sort_file_size?: int & >=0 & <=9223372036853727232

	// Maximum amount of memory to use for memory mapping compressed MyISAM files
	myisam_mmap_size?: int & >=7 & <=922337203685477807

	myisam_repair_threads?: int & >=1 & <=18446744073709551615

	// Size of the buffer that is allocated when sorting MyISAM indexes during a REPAIR TABLE or when creating indexes with CREATE INDEX or ALTER TABLE.
	myisam_sort_buffer_size?: int & >=4 & <=9223372036854775807

	// How the server treats NULL values when collecting statistics about the distribution of index values for MyISAM tables
	myisam_stats_method?: string & "nulls_equal" | "nulls_unequal" | "nulls_ignored"

	// Memory mapping for reading and writing MyISAM tables.
	myisam_use_mmap?: string & "0" | "1" | "OFF" | "ON"

	// This variable controls whether the mysql_native_password built-in authentication plugin supports proxy users.
	mysql_native_password_proxy_users?: string & "0" | "1" | "OFF" | "ON"

	mysqlx_bind_address?: string

	mysqlx_connect_timeout: int & >=1 & <=1000000000 | *30

	mysqlx_document_id_unique_prefix: int & >=0 & <=65535 | *0

	mysqlx_idle_worker_thread_timeout: int & >=0 & <=3600 | *60

	mysqlx_interactive_timeout: int & >=1 & <=2147483 | *28800

	mysqlx_max_allowed_packet: int & >=512 & <=1073741824 | *1048576

	mysqlx_max_connections: int & >=1 & <=65535 | *100

	mysqlx_min_worker_threads: int & >=1 & <=100 | *2

	mysqlx_port: int & >=1 & <=65535 | *33060

	mysqlx_port_open_timeout: int & >=1 & <=120 | *0

	mysqlx_read_timeout: int & >=30 & <=2147483 | *28800

	mysqlx_socket?: string

	mysqlx_ssl_ca?: string

	mysqlx_ssl_capath?: string

	mysqlx_ssl_cert?: string

	mysqlx_ssl_crl?: string

	mysqlx_ssl_crlpath?: string

	mysqlx_ssl_key?: string

	mysqlx_wait_timeout: int & >=1 & <=2147483 | *28800

	mysqlx_write_timeout: int & >=1 & <=2147483 | *60

	// This variable should not normally be changed. For use when very little memory is available. Set it to the expected length of statements sent by clients.
	net_buffer_length?: int & >=1024 & <=1048576

	// The number of seconds to wait for more data from a TCP/IP connection before aborting the read.
	net_read_timeout?: int & >=1 & <=31536000

	// If a read on a communication port is interrupted, retry this many times before giving up. This value should be set quite high on freebsd because internal interrupts are sent to all threads.
	net_retry_count?: int & >=1 & <=9223372036854775807

	// The number of seconds to wait on TCP/IP connections for a block to be written before aborting the write.
	net_write_timeout?: int & >=1 & <=31536000

	new?: string & "NEVER" | "AUTO" | "ALWAYS" | "0" | "1" | "2"

	// Defines the n-gram token size for the n-gram full-text parser.
	ngram_token_size?: int & >=1 & <=10

	// Whether the server is in offline mode.
	offline_mode?: string & "0" | "1" | "OFF" | "ON"

	old_alter_table: string & "0" | "1" | "OFF" | "ON" | *"0"

	// Enable old-style user limits.
	"old-style-user-limits"?: string & "0" | "1" | "OFF" | "ON"

	open_files_limit: int | *5000

	// Controls the heuristics applied during query optimization to prune less-promising partial plans from the optimizer search space.
	optimizer_prune_level?: string & "0" | "1" | "OFF" | "ON"

	// The maximum depth of search performed by the query optimizer.
	optimizer_search_depth?: int & >=0 & <=62

	// Controls optimizer behavior.
	optimizer_switch?: string & "default" | "batched_key_access=on" | "batched_key_access=off" | "block_nested_loop=on" | "block_nested_loop=off" | "condition_fanout_filter=on" | "condition_fanout_filter=off" | "derived_merge=on" | "derived_merge=off" | "duplicateweedout=on" | "duplicateweedout=off" | "engine_condition_pushdown=on" | "engine_condition_pushdown=off" | "firstmatch=on" | "firstmatch=off" | "index_condition_pushdown=on" | "index_condition_pushdown=off" | "index_merge=on" | "index_merge=off" | "index_merge_intersection=on" | "index_merge_intersection=off" | "index_merge_sort_union=on" | "index_merge_sort_union=off" | "index_merge_union=on" | "index_merge_union=off" | "loosescan=on" | "loosescan=off" | "materialization=on" | "materialization=off" | "mrr=on" | "mrr=off" | "mrr_cost_based=on" | "mrr_cost_based=off" | "semijoin=on" | "semijoin=off" | "subquery_materialization_cost_based=on" | "subquery_materialization_cost_based=off" | "use_index_extensions=on" | "use_index_extensions=off"

	// Controls how statements are traced.
	optimizer_trace?: string & "default" | "enabled=on" | "enabled=off" | "one_line=on" | "one_line=off"

	// Controls optimizations during statement tracing.
	optimizer_trace_features?: string & "default" | "greedy_search=on" | "greedy_search=off" | "range_optimizer=on" | "range_optimizer=off" | "dynamic_range=on" | "dynamic_range=off" | "repeated_subselect=on" | "repeated_subselect=off"

	// Controls the limit on trace retention.
	optimizer_trace_limit?: int & >=1 & <=9223372036854775807

	// Maximum allowed cumulated size of stored optimizer traces
	optimizer_trace_max_mem_size?: int & >=0 & <=18446744073709551615

	// Controls the offset on trace retention.
	optimizer_trace_offset?: int & >=-9223372036854775807 & <=9223372036854775807

	// The time when the current transaction was committed on the originating replication master, measured in microseconds since the epoch.
	original_commit_timestamp?: int

	parser_max_mem_size: int & >=10000000 & <=18446744073709551615 | *18446744073709551615

	// The number of old passwords to check in the history. Set to 0 (the default) to turn the checks off.
	password_history: int & >=0 & <=4294967295 | *0

	// The minimum number of days that need to pass before a password can be reused. Set to 0 (the default) to turn the checks off.
	password_reuse_interval: int & >=0 & <=4294967295 | *0

	// Enables or disables the Performance Schema
	performance_schema: string & "0" | "1" | "OFF" | "ON" | *"0"

	// The number of rows in the Performance Schema accounts table.
	performance_schema_accounts_size?: int & >=-1 & <=1048576

	// The maximum number of rows in the events_statements_summary_by_digest table.
	performance_schema_digests_size?: int & >=-1 & <=1048576

	// Number of server errors instrumented.
	performance_schema_error_size: int & >=0 & <=1048576 | *0

	// The number of rows in the events_stages_history_long table.
	performance_schema_events_stages_history_long_size?: int & >=-1 & <=1048576

	// The number of rows per thread in the events_stages_history table.
	performance_schema_events_stages_history_size?: int & >=-1 & <=1024

	// The number of rows in the events_statements_history_long table.
	performance_schema_events_statements_history_long_size?: int & >=-1 & <=1048576

	// The number of rows per thread in the events_statements_history table.
	performance_schema_events_statements_history_size?: int & >=-1 & <=1024

	// The number of rows in the events_transactions_history_long table.
	performance_schema_events_transactions_history_long_size?: int & >=-1 & <=1048576

	// The number of rows per thread in the events_transactions_history table.
	performance_schema_events_transactions_history_size?: int & >=-1 & <=1024

	// The number of rows in the events_waits_history_long table.
	performance_schema_events_waits_history_long_size?: int & >=-1 & <=1048576

	// The number of rows per thread in the events_waits_history table.
	performance_schema_events_waits_history_size?: int & >=-1 & <=1024

	// The number of rows in the hosts table.
	performance_schema_hosts_size?: int & >=-1 & <=1048576

	// The maximum number of condition instruments.
	performance_schema_max_cond_classes?: int & >=0 & <=256

	// The maximum number of instrumented condition objects.
	performance_schema_max_cond_instances?: int & >=-1 & <=1048576

	// The maximum number of bytes available for computing statement digests.
	performance_schema_max_digest_length?: int & >=-1 & <=1048576

	// The time in seconds after which a previous query sample is considered old. When the value is 0, queries are sampled once. When the value is greater than zero, queries are re sampled if the last sample is more than performance_schema_max_digest_sample_age seconds old.
	performance_schema_max_digest_sample_age: int & >=0 & <=1048576 | *60

	// The maximum number of file instruments.
	performance_schema_max_file_classes?: int & >=0 & <=256

	// The maximum number of opened file objects.
	performance_schema_max_file_handles?: int & >=0 & <=1048576

	// The maximum number of instrumented file objects.
	performance_schema_max_file_instances?: int & >=-1 & <=1048576

	// The maximum number of indexes for which the Performance Schema maintains statistics.
	performance_schema_max_index_stat?: int & >=-1 & <=1048576

	// The maximum number of memory instruments.
	performance_schema_max_memory_classes?: int & >=-1 & <=1024

	// The maximum number of metadata lock instruments.
	performance_schema_max_metadata_locks?: int & >=-1 & <=104857600

	// The maximum number of mutex instruments.
	performance_schema_max_mutex_classes?: int & >=0 & <=256

	// The maximum number of instrumented mutex objects.
	performance_schema_max_mutex_instances?: int & >=-1 & <=104857600

	// The maximum number of rows in the prepared_statements_instances table.
	performance_schema_max_prepared_statements_instances?: int & >=-1 & <=1048576

	// The maximum number of stored programs for which the Performance Schema maintains statistics.
	performance_schema_max_program_instances?: int & >=-1 & <=1048576

	// The maximum number of rwlock instruments.
	performance_schema_max_rwlock_classes?: int & >=0 & <=256

	// The maximum number of instrumented rwlock objects.
	performance_schema_max_rwlock_instances?: int & >=-1 & <=104857600

	// The maximum number of socket instruments.
	performance_schema_max_socket_classes?: int & >=0 & <=256

	// The maximum number of instrumented socket objects.
	performance_schema_max_socket_instances?: int & >=-1 & <=1048576

	// The maximum number of bytes used to store SQL statements in the SQL_TEXT column of the events_statements_current, events_statements_history, and events_statements_history_long statement event tables.
	performance_schema_max_sql_text_length?: int & >=-1 & <=1048576

	// The maximum number of stage instruments.
	performance_schema_max_stage_classes?: int & >=0 & <=256

	// The maximum number of statement instruments.
	performance_schema_max_statement_classes?: int & >=0 & <=256

	// The maximum depth of nested stored program calls for which the Performance Schema maintains statistics.
	performance_schema_max_statement_stack?: int & >=1 & <=256

	// The maximum number of opened table objects.
	performance_schema_max_table_handles?: int & >=-1 & <=1048576

	// The maximum number of instrumented table objects.
	performance_schema_max_table_instances?: int & >=-1 & <=1048576

	// The maximum number of tables for which the Performance Schema maintains lock statistics.
	performance_schema_max_table_lock_stat?: int & >=-1 & <=1048576

	// The maximum number of thread instruments.
	performance_schema_max_thread_classes?: int & >=0 & <=256

	// The maximum number of instrumented thread objects.
	performance_schema_max_thread_instances?: int & >=-1 & <=1048576

	// The amount of preallocated memory per thread used to hold connection attribute strings.
	performance_schema_session_connect_attrs_size?: int & >=-1 & <=1048576

	// The number of rows in the setup_actors table.
	performance_schema_setup_actors_size?: int & >=0 & <=1024

	// The number of rows in the setup_objects table.
	performance_schema_setup_objects_size?: int & >=-1 & <=1048576

	// The number of rows in the users table.
	performance_schema_users_size?: int & >=-1 & <=1048576

	// Whether to load persisted configuration settings from the mysqld-auto.cnf file in the data directory.
	persisted_globals_load: string & "0" | "1" | "OFF" | "ON" | *"0"

	// The path name of the process ID file. This file is used by other programs such as MySQLd_safe to determine the server's process ID.
	pid_file?: string

	// Directory searched by systems dynamic linker for UDF object files. Otherwise, user-defined function object files must be located the default directory.
	plugin_dir?: string

	// The number of the port on which the server listens for TCP/IP connections.
	port?: int

	// The size of the buffer that is allocated when preloading indexes.
	preload_buffer_size?: int & >=1024 & <=1073741824

	// The number of statements for which to maintain profiling if profiling is enabled.
	profiling_history_size?: int & >=0 & <=100

	protocol_version?: int

	// The allocation size of memory blocks that are allocated for objects created during statement parsing and execution. May help resolve fragmentation problems.
	query_alloc_block_size?: int & >=1024 & <=4294967295

	// Increased size might help improve perf for complex queries (i.e. Reduces server memory allocation)
	query_prealloc_size?: int & >=8192 & <=18446744073709547520

	// The size of blocks that are allocated when doing range optimization.
	range_alloc_block_size?: int & >=1024 & <=4294967295

	// The limit on memory consumption for the range optimizer.
	range_optimizer_max_mem_size?: int & >=0 & <=18446744073709551615

	rbr_exec_mode?: string & "IDEMPOTENT" | "STRICT"

	// When the rds.optimized_writes parameter is set to AUTO, the DB instance uses RDS Optimized Writes for DB instance classes and engine versions that support it. When this parameter is set to OFF, the DB instance doesn't use RDS Optimized Writes.
	"rds.optimized_writes"?: string & "AUTO" | "OFF"

	// Each thread that does a sequential scan allocates this buffer. Increased value may help perf if performing many sequential scans.
	read_buffer_size: int & >=8200 & <=2147479552 | *262144

	// When it is enabled, the server permits no updates except from updates performed by slave threads.
	read_only?: string & "0" | "1" | "{TrueIfReplica}"

	// Avoids disk reads when reading rows in sorted order following a key-sort operation. Large values can improve ORDER BY perf.
	read_rnd_buffer_size: int & >=8200 & <=2147479552 | *524288

	// Stack size limit for regular expressions matches.
	regexp_stack_limit: int & >=0 & <=2147483647 | *8000000

	// Timeout for regular expressions matches, in steps of the match engine, typically on the order of milliseconds.
	regexp_time_limit: int & >=0 & <=2147483647 | *32

	relay_log?: string

	// The basename for the relay log.
	"relay-log"?: string

	relay_log_basename?: string

	relay_log_index?: string

	relay_log_info_file?: string

	// This option causes the server to log its relay log info to a file or a table.
	relay_log_info_repository?: string & "FILE" | "TABLE"

	relay_log_purge: string & "0" | "1" | "OFF" | "ON" | *"TRUE"

	// Enables automatic relay log recovery immediately following server startup.
	relay_log_recovery: string & "0" | "1" | "OFF" | "ON" | *"1"

	relay_log_space_limit: int & >=0 & <=18446744073709551615 | *0

	// Creates a replication filter using the name of a database
	"replicate-do-db"?: string

	// Creates a replication filter using the name of a table
	"replicate-do-table"?: string

	// Creates a replication ignore filter using the name of a database
	"replicate-ignore-db"?: string

	// Creates a replication ignore filter using the name of a table
	"replicate-ignore-table"?: string

	// Creates a replication filter using the pattern of a table name
	"replicate-wild-do-table"?: string

	// Creates a replication ignore filter using the pattern of a table name
	"replicate-wild-ignore-table"?: string

	report_host?: string

	report_password?: string

	report_port?: int & >=0 & <=65535

	report_user?: string

	// Whether client connections to the server are required to use some form of secure transport.
	require_secure_transport?: string & "0" | "1" | "OFF" | "ON"

	resultset_metadata?: string & "FULL" | "NONE"

	// The size for reads done from the binlog and relay log. It must be a multiple of 4kb. Making it larger might help with IO stalls while reading these files when they are not in the OS buffer cache.
	rpl_read_size: int & >=8192 & <=4294967295 | *8192

	// The number of slave acknowledgments the master must receive per transaction before proceeding.
	rpl_semi_sync_master_wait_for_slave_count?: int & >=1 & <=65535

	// Controls the point at which a semisynchronous replication master waits for slave acknowledgment of transaction receipt before returning a status to the client that committed the transaction.
	rpl_semi_sync_master_wait_point?: string & "AFTER_SYNC" | "AFTER_COMMIT"

	// Control the length of time (in seconds) that STOP SLAVE waits before timing out by setting this variable.
	rpl_stop_slave_timeout?: int & >=2 & <=31536000

	// If this option is enabled, a user cannot create new MySQL users by using the GRANT statement unless the user has the INSERT privilege for the mysql.user table or any column in the table.
	"safe-user-create"?: string & "0" | "1" | "OFF" | "ON"

	// Defines a limit for the number of schema definition objects, both used and unused, that can be kept in the dictionary object cache.
	schema_definition_cache: int & >=256 & <=524288 | *256

	// Limits the effect of LOAD_FILE(), LOAD_DATA, and SELECT ??? INTO OUTFILE to specified directory.
	secure_file_priv?: string

	// Integer value used to identify the instance in a replication group
	server_id?: int

	// Enables a tracker for capturing GTIDs and returning them in the OK packet.
	session_track_gtids?: string & "0" | "1" | "OFF" | "ON"

	// Whether the server tracks changes to the default schema and notifies the client when changes occur.
	session_track_schema?: string & "0" | "1" | "OFF" | "ON"

	// Whether the server tracks changes to the session state and notifies the client when changes occur.
	session_track_state_change?: string & "0" | "1" | "OFF" | "ON"

	// Whether the server tracks changes to the session system variables and notifies the client when changes occur.
	session_track_system_variables?: string & "time_zone" | "autocommit" | "character_set_client" | "character_set_results" | "character_set_connection"

	// Track changes to the transaction attributes.
	session_track_transaction_info?: string & "OFF" | "STATE" | "CHARACTERISTICS"

	// Controls whether the server autogenerates RSA private/public key-pair files in the data directory.
	sha256_password_auto_generate_rsa_keys?: string & "0" | "1" | "OFF" | "ON"

	sha256_password_private_key_path?: string

	// Controls whether the sha256_password built-in authentication plugin supports proxy users.
	sha256_password_proxy_users?: string & "0" | "1" | "OFF" | "ON"

	sha256_password_public_key_path?: string

	// When this option is enabled, it increases the verbosity of 'SHOW CREATE TABLE'.
	show_create_table_verbosity: string & "0" | "1" | "OFF" | "ON" | *"0"

	// Whether SHOW CREATE TABLE output includes comments to flag temporal columns found to be in pre-5.6.4 format.
	show_old_temporals?: string & "0" | "1" | "OFF" | "ON"

	// Ignore character set information sent by the client.
	"skip-character-set-client-handshake"?: string & "0" | "1" | "OFF" | "ON"

	// Uses OS locking instead of internal
	skip_external_locking?: string & "0" | "1" | "OFF" | "ON"

	// Host names are not resolved. All Host column values in the grant tables must be IP numbers or localhost.
	skip_name_resolve?: string & "0" | "1" | "OFF" | "ON"

	// SHOW DATABASES statement is allowed only to users who have the SHOW DATABASES privilege
	skip_show_database?: string & "0" | "1" | "OFF" | "ON"

	// Tells the slave server not to start the slave threads when the server starts.
	"skip-slave-start": string & "0" | "1" | "OFF" | "ON" | *"1"

	// Whether or not batched updates are enabled on replication slaves.
	slave_allow_batching?: string & "0" | "1" | "OFF" | "ON"

	// Sets the maximum number of transactions that can be processed by a multi-threaded slave before a checkpoint operation is called to update its status as shown by SHOW SLAVE STATUS.
	slave_checkpoint_group?: int & >=32 & <=524280

	// Sets the maximum time (in milliseconds) that is allowed to pass before a checkpoint operation is called to update the status of a multi-threaded slave as shown by SHOW SLAVE STATUS.
	slave_checkpoint_period?: int & >=1 & <=4294967295

	// Whether to use compression of the slave/master protocol if both the slave and the master support it.
	slave_compressed_protocol?: string & "0" | "1" | "OFF" | "ON"

	// slave_exec_mode controls how a replication thread resolves conflicts and errors during replication.
	slave_exec_mode?: string & "IDEMPOTENT" | "STRICT"

	slave_load_tmpdir?: string

	// The number of seconds to wait for more data from a master/slave connection before aborting the read.
	slave_net_timeout?: int & >=1 & <=31536000

	// Enable parallel execution on the slave of all uncommitted threads already in the prepare phase, without violating consistency.
	slave_parallel_type?: string & "DATABASE" | "LOGICAL_CLOCK"

	// Sets the number of slave worker threads for executing replication events (transactions) in parallel. Setting this variable to 0 (the default) disables parallel execution.
	slave_parallel_workers?: int & >=0 & <=1024

	// For multithreaded slaves, this option sets the maximum amount of memory (in bytes) available to slave worker queues holding events not yet applied.
	slave_pending_jobs_size_max?: int & >=1024 & <=18446744073709547520

	// Enable parallel execution on the slave of all uncommitted threads already in the prepare phase, without violating consistency.
	slave_preserve_commit_order?: string & "0" | "1" | "OFF" | "ON"

	// When preparing batches of rows for row-based logging and replication, this variable controls how the rows are searched for matches???that is, whether or not hashing is used for searches using a primary or unique key, using some other key, or using no key at all.
	slave_rows_search_algorithms?: string & "TABLE_SCAN" | "INDEX_SCAN" | "HASH_SCAN"

	// When this option is enabled, the slave examines checksums read from the relay log, in the event of a mismatch, the slave stops with an error.
	slave_sql_verify_checksum?: string & "0" | "1" | "OFF" | "ON"

	// If a replication slave SQL thread fails to execute a transaction because of an InnoDB deadlock or because the transaction's execution time exceeded InnoDB's innodb_lock_wait_timeout, it automatically retries slave_transaction_retries times before stopping with an error.
	slave_transaction_retries?: int & >=0 & <=18446744073709551615

	// Controls the type conversion mode in effect on the slave when using row-based replication
	slave_type_conversions?: string & "ALL_LOSSY" | "ALL_NON_LOSSY" | "ALL_SIGNED" | "ALL_UNSIGNED"

	// Increments Slow_launch_threads if creating thread takes longer than this many seconds.
	slow_launch_time?: int & >=0 & <=31536000

	// Enable or disable the slow query log
	slow_query_log?: string & "0" | "1" | "OFF" | "ON"

	// Location of the mysql slow query log file.
	slow_query_log_file?: string

	// (UNIX) socket file and (WINDOWS) named pipe used for local connections.
	socket?: string

	// Larger value improves perf for ORDER BY or GROUP BY operations.
	sort_buffer_size?: int & >=32768 & <=18446744073709551615

	sql_auto_is_null: string & "0" | "1" | "OFF" | "ON" | *"0"

	sql_big_selects: int & >=1 & <=18446744073709551615 | *18446744073709551615

	sql_buffer_result: string & "0" | "1" | "OFF" | "ON" | *"0"

	sql_log_off: string & "0" | "1" | "OFF" | "ON" | *"0"

	// Current SQL Server Mode.
	sql_mode?: string & "ALLOW_INVALID_DATES" | "ANSI_QUOTES" | "ERROR_FOR_DIVISION_BY_ZERO" | "HIGH_NOT_PRECEDENCE" | "IGNORE_SPACE" | "NO_AUTO_VALUE_ON_ZERO" | "NO_BACKSLASH_ESCAPES" | "NO_DIR_IN_CREATE" | "NO_ENGINE_SUBSTITUTION" | "NO_UNSIGNED_SUBTRACTION" | "NO_ZERO_DATE" | "NO_ZERO_IN_DATE" | "ONLY_FULL_GROUP_BY" | "PAD_CHAR_TO_FULL_LENGTH" | "PIPES_AS_CONCAT" | "REAL_AS_FLOAT" | "STRICT_ALL_TABLES" | "STRICT_TRANS_TABLES" | "ANSI" | "TRADITIONAL"

	sql_notes: string & "0" | "1" | "OFF" | "ON" | *"1"

	sql_quote_show_create: string & "0" | "1" | "OFF" | "ON" | *"1"

	// Whether statements that create new tables or alter the structure of existing tables enforce the requirement that tables have a primary key.
	sql_require_primary_key?: string & "0" | "1" | "OFF" | "ON"

	sql_safe_updates: string & "0" | "1" | "OFF" | "ON" | *"0"

	// The maximum number of rows to return from SELECT statements.
	sql_select_limit?: int & >=0 & <=18446744073709551615

	sql_slave_skip_counter?: int

	sql_warnings: string & "0" | "1" | "OFF" | "ON" | *"0"

	ssl_ca?: string

	ssl_capath?: string

	ssl_cert?: string

	// The list of permissible ciphers for encrypted connections that use TLS protocols up through TLSv1.2.
	ssl_cipher?: string & "ECDHE-RSA-AES256-GCM-SHA384" | "ECDHE-RSA-AES128-GCM-SHA256" | "ECDHE-RSA-AES256-SHA384" | "ECDHE-RSA-AES128-SHA256" | "ECDHE-RSA-AES256-SHA" | "ECDHE-RSA-AES128-SHA" | "AES256-GCM-SHA384" | "AES128-GCM-SHA256" | "AES256-SHA" | "AES128-SHA" | "DHE-RSA-AES128-SHA" | "DHE-RSA-AES256-SHA" | "DHE-DSS-AES128-SHA" | "DHE-DSS-AES256-SHA"

	ssl_crl?: string

	ssl_crlpath?: string

	// Controls whether to enable FIPS mode on the server side.
	ssl_fips_mode?: string & "0" | "1" | "STRICT"

	ssl_key?: string

	// Sets a soft upper limit for the number of cached stored routines per connection.
	stored_program_cache?: int & >=256 & <=524288

	// Defines a limit for the number of stored program definition objects, both used and unused, that can be kept in the dictionary object cache.
	stored_program_definition_cache: int & >=256 & <=524288 | *256

	// Whether client connections to the server are required to use some form of secure transport.
	super_read_only?: string & "0" | "1" | "OFF" | "ON"

	// Sync binlog (MySQL flush to disk or rely on OS)
	sync_binlog: int & >=0 & <=18446744073709547520 | *1

	// If the value of this variable is greater than 0, a replication slave synchronizes its master.info file to disk (using fdatasync()) after every sync_master_info events.
	sync_master_info?: int & >=0 & <=18446744073709547520

	// If the value of this variable is greater than 0, the MySQL server synchronizes its relay log to disk (using fdatasync()) after every sync_relay_log writes to the relay log.
	sync_relay_log?: int & >=0 & <=18446744073709547520

	// If the value of this variable is greater than 0, a replication slave synchronizes its relay-log.info file to disk (using fdatasync()) after every sync_relay_log_info transactions.
	sync_relay_log_info?: int & >=0 & <=18446744073709547520

	// Causes SYSYDATE() to be an alias for NOW(). Replication related
	"sysdate-is-now"?: string & "0" | "1" | "OFF" | "ON"

	system_time_zone?: string

	// The number of table definitions that can be stored in the definition cache
	table_definition_cache?: int & >=400 & <=524288

	// The number of open tables for all threads. Increasing this value increases the number of file descriptors.
	table_open_cache?: int & >=1 & <=524288

	// The number of open tables cache instances.
	table_open_cache_instances: int & >=1 & <=16 | *16

	// Defines a limit for the number of tablespace definition objects, both used and unused, that can be kept in the dictionary object cache.
	tablespace_definition_cache: int & >=256 & <=524288 | *256

	// Defines the maximum amount of memory (in bytes) the TempTable storage engine is permitted to allocate from memory-mapped temporary files before it starts storing data to InnoDB internal temporary tables on disk.
	temptable_max_mmap?: int & >=0 & <=18446744073709551615

	// Maximum amount of memory (in bytes) the TempTable storage engine is allowed to allocate from the main memory (RAM) before starting to store data on disk.
	temptable_max_ram: int & >=2097152 & <=18446744073709547520 | *1073741824

	// Number of threads to be cached. Doesn't improve perf for good thread implementations.
	thread_cache_size?: int & >=0 & <=16384

	thread_handling?: string & "no-threads" | "one-thread-per-connection" | "loaded-dynamically"

	// If the thread stack size is too small, it limits the complexity of the SQL statements that the server can handle, the recursion depth of stored procedures, and other memory-consuming actions.
	thread_stack: int & >=131072 & <=18446744073709547520 | *262144

	// The server time zone
	time_zone?: string & "Africa/Cairo" | "Africa/Casablanca" | "Africa/Harare" | "Africa/Monrovia" | "Africa/Nairobi" | "Africa/Tripoli" | "Africa/Windhoek" | "America/Araguaina" | "America/Asuncion" | "America/Bogota" | "America/Buenos_Aires" | "America/Caracas" | "America/Chihuahua" | "America/Cuiaba" | "America/Denver" | "America/Fortaleza" | "America/Guatemala" | "America/Halifax" | "America/Manaus" | "America/Matamoros" | "America/Monterrey" | "America/Montevideo" | "America/Phoenix" | "America/Santiago" | "America/Tijuana" | "Asia/Amman" | "Asia/Ashgabat" | "Asia/Baghdad" | "Asia/Baku" | "Asia/Bangkok" | "Asia/Beirut" | "Asia/Calcutta" | "Asia/Damascus" | "Asia/Dhaka" | "Asia/Irkutsk" | "Asia/Jerusalem" | "Asia/Kabul" | "Asia/Karachi" | "Asia/Kathmandu" | "Asia/Krasnoyarsk" | "Asia/Magadan" | "Asia/Muscat" | "Asia/Novosibirsk" | "Asia/Riyadh" | "Asia/Seoul" | "Asia/Shanghai" | "Asia/Singapore" | "Asia/Taipei" | "Asia/Tehran" | "Asia/Tokyo" | "Asia/Ulaanbaatar" | "Asia/Vladivostok" | "Asia/Yakutsk" | "Asia/Yerevan" | "Atlantic/Azores" | "Australia/Adelaide" | "Australia/Brisbane" | "Australia/Darwin" | "Australia/Hobart" | "Australia/Perth" | "Australia/Sydney" | "Canada/Newfoundland" | "Canada/Saskatchewan" | "Canada/Yukon" | "Brazil/East" | "Europe/Amsterdam" | "Europe/Athens" | "Europe/Dublin" | "Europe/Helsinki" | "Europe/Istanbul" | "Europe/Kaliningrad" | "Europe/Moscow" | "Europe/Paris" | "Europe/Prague" | "Europe/Sarajevo" | "Pacific/Auckland" | "Pacific/Fiji" | "Pacific/Guam" | "Pacific/Honolulu" | "Pacific/Samoa" | "US/Alaska" | "US/Central" | "US/Eastern" | "US/East-Indiana" | "US/Pacific" | "UTC"

	// The list of permissible ciphers for encrypted connections that use TLSv1.3.
	tls_ciphersuites?: string & "TLS_AES_128_GCM_SHA256" | "TLS_AES_256_GCM_SHA384" | "TLS_CHACHA20_POLY1305_SHA256" | "TLS_AES_128_CCM_SHA256" | "TLS_AES_128_CCM_8_SHA256"

	// The protocols permitted by the server for encrypted connections.
	tls_version?: string & "TLSv1" | "TLSv1.1" | "TLSv1.2" | "TLSv1.3"

	// The directory used for temporary files and temporary tables
	tmpdir?: string

	// If an in-memory temporary table exceeds the limit, MySQL automatically converts it to an on-disk MyISAM table. Increased value can improve perf for many advanced GROUP BY queries.
	tmp_table_size?: int & >=1024 & <=18446744073709551615

	// The amount in bytes by which to increase a per-transaction memory pool which needs memory.
	transaction_alloc_block_size?: int & >=1024 & <=18446744073709547520

	// Sets the default transaction isolation level.
	transaction_isolation?: string & "READ-UNCOMMITTED" | "READ-COMMITTED" | "REPEATABLE-READ" | "SERIALIZABLE"

	// There is a per-transaction memory pool from which various transaction-related allocations take memory. For every allocation that cannot be satisfied from the pool because it has insufficient memory available, the pool is incremented.
	transaction_prealloc_size?: int & >=1024 & <=18446744073709547520

	// Reserved for future use.
	transaction_write_set_extraction?: string & "OFF" | "MURMUR32"

	unique_checks: string & "0" | "1" | "OFF" | "ON" | *"1"

	// This variable controls whether updates to a view can be made when the view does not contain all columns of the primary key defined in the underlying table, if the update statement contains a LIMIT clause (Often generated by GUI tools).
	updatable_views_with_limit?: string & "0" | "1" | "OFF" | "ON"

	// This option controls how the server loads the validate_password plugin at startup.
	"validate-password"?: string & "ON" | "OFF" | "FORCE" | "FORCE_PLUS_PERMANENT"

	// The path name of the dictionary file used by the validate_password plugin for checking passwords.
	validate_password_dictionary_file?: string

	// The minimum number of characters that validate_password requires passwords to have.
	validate_password_length?: int & >=0 & <=2147483647

	// The minimum number of lowercase and uppercase characters that passwords checked by the validate_password plugin must have if the password policy is MEDIUM or stronger.
	validate_password_mixed_case_count?: int & >=0 & <=2147483647

	// The minimum number of numeric (digit) characters that passwords checked by the validate_password plugin must have if the password policy is MEDIUM or stronger.
	validate_password_number_count?: int & >=0 & <=2147483647

	// The password policy enforced by the validate_password plugin.
	validate_password_policy?: string & "LOW" | "MEDIUM" | "STRONG"

	// The minimum number of nonalphanumeric characters that passwords checked by the validate_password plugin must have if the password policy is MEDIUM or stronger.
	validate_password_special_char_count?: int & >=0 & <=2147483647

	version_comment?: string

	version_compile_machine?: string

	version_compile_os?: string

	version_compile_zlib?: string

	// The number of seconds the server waits for activity on a non-interactive TCP/IP or UNIX File connection before closing it.
	wait_timeout?: int & >=1 & <=31536000

	// For SQL window functions, determines whether to enable inversion optimization for moving window frames also for floating values.
	windowing_use_high_precision: string & "0" | "1" | "OFF" | "ON" | *"1"

	// Mysql audit log version.
	rds_audit_log_version?: string & "MYSQL_V1" | "MYSQL_V3"

	// To select the log format that the audit log plugin uses to write its log file, set the audit_log_format system variable at server startup
	rds_audit_log_format?: string & "JSON" | "PLAIN"

	// The option to enable or disable audit log.
	rds_audit_log_enabled?: string & "0" | "1" | "OFF" | "ON" | *"OFF"

	// The policy controlling how the audit log plugin writes connection events to its log file. Supported values are 'ALL' (default), 'ERRORS' and 'NONE'.
	rds_audit_log_connection_policy?: string & "ALL" | "ERRORS" | "NONE"

	// The policy controlling how the audit log plugin writes  query events to its log file. Supported values are 'ALL' (default), 'UPDATES',  'UPDATES_OR_ERRORS', 'ERRORS' and 'NONE'.
	rds_audit_log_statement_policy?: string & "ALL" | "UPDATES" | "NONE" | "ERRORS" | "UPDATES_OR_ERRORS"

	// Max number of rows in each audit log file. Log records will be discarded above this number.
	rds_audit_log_row_limit?: int & >=1 | *100000

	// other parameters
	// reference mysql parameters
	...
}

// SectionName is section name
[SectionName=_]: #MysqlParameter
