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

#VtTabletParameter: {

	// Connection timeout to mysqld in milliseconds. (0 for no timeout, default 500)
	db_connect_timeout_ms: int & >=0

	// Enable or disable logs. (default true)
	enable_logs: bool

	// Enable or disable query log. (default true)
	enable_query_log: bool

	// Interval between health checks. (default 20s)
	health_check_interval: =~"[-+]?([0-9]*(\\.[0-9]*)?[a-z]+)+$"

	// Time to wait for a remote operation. (default 15s)
	remote_operation_timeout: =~"[-+]?([0-9]*(\\.[0-9]*)?[a-z]+)+$"

	// Delay between retries of updates to keep the tablet and its shard record in sync. (default 30s)
	shard_sync_retry_delay: =~"[-+]?([0-9]*(\\.[0-9]*)?[a-z]+)+$"

	// Table acl config mode. Valid values are: simple, mysqlbased. (default simple)
	table_acl_config_mode: string & "simple" | "mysqlbased"

	// path to table access checker config file (json file);
	table_acl_config: string

	// Ticker to reload ACLs. Duration flag, format e.g.: 30s. Default: 30s
	table_acl_config_reload_interval: =~"[-+]?([0-9]*(\\.[0-9]*)?[a-z]+)+$"

	// only allow queries that pass table acl checks if true
	queryserver_config_strict_table_acl: bool

	// if this flag is true, vttablet will fail to start if a valid tableacl config does not exist
	enforce_tableacl_config: bool

	// query server read pool size, connection pool is used by regular queries (non streaming, not in a transaction)
	queryserver_config_pool_size: int & >=0

	// query server stream connection pool size, stream pool is used by stream queries: queries that return results to client in a streaming fashion
	queryserver_config_stream_pool_size: int & >=0

	// query server transaction cap is the maximum number of transactions allowed to happen at any given point of a time for a single vttablet. E.g. by setting transaction cap to 100, there are at most 100 transactions will be processed by a vttablet and the 101th transaction will be blocked (and fail if it cannot get connection within specified timeout)
	queryserver_config_transaction_cap: int & >=0

	...
}

// SectionName is section name
[SectionName=_]: #VtTabletParameter
