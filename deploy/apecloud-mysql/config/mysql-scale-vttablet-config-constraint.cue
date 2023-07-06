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

  // Connection timeout to mysqld in milliseconds. (0 for no timeout)
  db_connect_timeout_ms: int

  // Interval between health checks. (default 20s)
  health_check_interval: string

  // Time to wait for a remote operation. (default 15s)
  remote_operation_timeout: string

  // Delay between retries of updates to keep the tablet and its shard record in sync. (default 30s)
  shard_sync_retry_delay: string
}