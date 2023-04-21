//Copyright (C) 2022 ApeCloud Co., Ltd
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

#ProfilesParameter: {
	profiles: [string]: #ClickhouseParameter
	// ignore other configure
}

#ClickhouseParameter: {
	// [0|1|2] default 0
	readonly: int & 0 | 1 | 2 | *0

	// [0|1] default 1
	allow_ddl: int & 0 | 1 | *1

	// [deny|local|global|allow] default : deny
	distributed_product_mode: string & "deny" | "local" | "global" | "allow" | *"deny"

	// [0|1] default 0
	prefer_global_in_and_join: int & 0 | 1 | *0
	...

	// other parameter
	// Clickhouse all parameter define: clickhouse settings define
}

configuration: #ProfilesParameter & {
}
