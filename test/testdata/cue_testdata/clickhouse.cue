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
