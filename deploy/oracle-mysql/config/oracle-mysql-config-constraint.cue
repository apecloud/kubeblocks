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

	// Sets the autocommit mode
	autocommit?: string & "0" | "1" | "OFF" | "ON"

	open_files_limit: int | *5000

	// Enables or disables the Performance Schema
	performance_schema: string & "0" | "1" | "OFF" | "ON" | *"0"

	// Prevents execution of statements that cannot be logged in a transactionally safe manner
	enforce_gtid_consistency?: string & "OFF" | "WARN" | "ON"

	// The size in bytes of the memory buffer innodb uses to cache data and indexes of its tables
	innodb_buffer_pool_size?: int & >=5242880 & <=18446744073709551615 @k8sResource(quantity)

	// The number of simultaneous client connections allowed.
	max_connections?: int & >=1 & <=100000

	// GTID Mode
	gtid_mode?: string & "0" | "OFF" | "ON" | "1"

	// Each thread that does a sequential scan allocates this buffer. Increased value may help perf if performing many sequential scans.
	read_buffer_size: int & >=8200 & <=2147479552 | *262144

	// When it is enabled, the server permits no updates except from updates performed by slave threads.
	read_only?: string & "0" | "1" | "{TrueIfReplica}"

	// Avoids disk reads when reading rows in sorted order following a key-sort operation. Large values can improve ORDER BY perf.
	read_rnd_buffer_size: int & >=8200 & <=2147479552 | *524288

	// Increase the value of join_buffer_size to get a faster full join when adding indexes is not possible.
	join_buffer_size?: int & >=128 & <=18446744073709547520

	// Larger value improves perf for ORDER BY or GROUP BY operations.
	sort_buffer_size?: int & >=32768 & <=18446744073709551615

	// Determines Innodb transaction durability
	innodb_flush_log_at_trx_commit?: int & >=0 & <=2

	// Sync binlog (MySQL flush to disk or rely on OS)
	sync_binlog: int & >=0 & <=18446744073709547520 | *1

	// Write a core file if mysqld dies.
	"core-file"?: string & "0" | "1" | "OFF" | "ON"

	// MySQL data directory
	datadir?: string

	// The number of the port on which the server listens for TCP/IP connections.
	port?: int

	// The MySQL installation base directory.
	basedir?: string

	// (UNIX) socket file and (WINDOWS) named pipe used for local connections.
	socket?: string

	// The path name of the process ID file. This file is used by other programs such as MySQLd_safe to determine the server's process ID.
	pid_file?: string

	// other parameters
	// reference mysql parameters
	...
}

// SectionName is section name
[SectionName=_]: #MysqlParameter
