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

	// At startup, the tabletGateway will wait up to this duration to get at least one tablet per keyspace/shard/tablet type. (default 30s)
	gateway_initial_tablet_timeout: string

	// After a duration of this time, if the client doesn't see any activity, it pings the server to see if the transport is still alive. (default 10s)
	grpc_keepalive_time: string

	// After having pinged for keepalive check, the client waits for a duration of Timeout and if no activity is seen even after that the connection is closed. (default 10s)
	grpc_keepalive_timeout: string

	// The health check timeout period. (default 1m0s)
	healthcheck_timeout: string

	// Topo server timeout. (default 5s)
	srv_topo_timeout: string

	// Tablet refresh interval. (default 1m0s)
	tablet_refresh_interval: string
}
