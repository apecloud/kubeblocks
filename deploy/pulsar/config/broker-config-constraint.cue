// Copyright (C) 2022-2023 ApeCloud Co., Ltd
//
// This file is part of KubeBlocks project
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

#PulsarBrokersParameter: {

	// Default number of message dispatching throttling-limit for every replicator in replication. Using a value of 0, is disabling replication message dispatch-throttling.
	dispatchThrottlingRatePerReplicatorInMsg: int & >=0

	// Default number of message-bytes dispatching throttling-limit for a subscription. Using a value of 0, is disabling default message-byte dispatch-throttling.
	dispatchThrottlingRatePerSubscriptionInByte: int & >=0

	...
}

configuration: #PulsarBrokersParameter & {
}
