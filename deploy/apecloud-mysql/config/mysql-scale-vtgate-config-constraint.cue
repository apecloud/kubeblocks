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

#VtGateParameter: {

	// Stop buffering completely if a failover takes longer than this duration. (default 20s)
	buffer_max_failover_duration: =~ "[-+]?([0-9]*(\\.[0-9]*)?[a-z]+)+$"

	// Minimum time between the end of a failover and the start of the next one (tracked per shard). Faster consecutive failovers will not trigger buffering. (default 1m0s)
	buffer_min_time_between_failovers: =~ "[-+]?([0-9]*(\\.[0-9]*)?[a-z]+)+$"

	// Maximum number of buffered requests in flight (across all ongoing failovers). (default 10000)
	buffer_size: int & >=1

	// Duration for how long a request should be buffered at most. (default 10s)
	buffer_window: =~ "[-+]?([0-9]*(\\.[0-9]*)?[a-z]+)+$"

	// Enable buffering (stalling) of primary traffic during failovers.
	enable_buffer: bool

	// At startup, the tabletGateway will wait up to this duration to get at least one tablet per keyspace/shard/tablet type. (default 30s)
	gateway_initial_tablet_timeout: =~ "[-+]?([0-9]*(\\.[0-9]*)?[a-z]+)+$"

	// After a duration of this time, if the client doesn't see any activity, it pings the server to see if the transport is still alive. (default 10s)
	grpc_keepalive_time: =~ "[-+]?([0-9]*(\\.[0-9]*)?[a-z]+)+$"

	// After having pinged for keepalive check, the client waits for a duration of Timeout and if no activity is seen even after that the connection is closed. (default 10s)
	grpc_keepalive_timeout: =~ "[-+]?([0-9]*(\\.[0-9]*)?[a-z]+)+$"

	// The health check timeout period. (default 2s)
	healthcheck_timeout: =~ "[-+]?([0-9]*(\\.[0-9]*)?[a-z]+)+$"

	// Read After Write Consistency Level. Valid values are: EVENTUAL, SESSION, INSTANCE, GLOBAL. (default EVENTUAL)
	read_after_write_consistency: string & "EVENTUAL" | "SESSION" | "INSTANCE" | "GLOBAL"

	// The default timeout for read after write. (default 30.0)
	read_after_write_timeout: number & >=0

	// Enable read write splitting. Valid values are: disable, random, least_global_qps, least_qps, least_rt, least_behind_primary. (default disable)
	read_write_splitting_policy: string & "disable" | "random" | "least_global_qps" | "least_qps" | "least_rt" | "least_behind_primary"

	// Topo server timeout. (default 1s)
	srv_topo_timeout: =~ "[-+]?([0-9]*(\\.[0-9]*)?[a-z]+)+$"

	// Tablet refresh interval. (default 1m0s)
	tablet_refresh_interval: =~ "[-+]?([0-9]*(\\.[0-9]*)?[a-z]+)+$"

	...
}

// SectionName is section name
[SectionName=_]: #VtGateParameter
