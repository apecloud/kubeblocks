/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

/*
Package instanceset is a general-purpose workload API designed to manage role-based stateful workloads, such as databases.
Think of InstanceSet as an enhanced version of StatefulSet.

While the native StatefulSet in Kubernetes handles stateful workloads effectively,
additional work is required when the workload pods have specific roles (e.g., leader/follower in etcd, primary/secondary in PostgreSQL, etc.).

InstanceSet provides the following features:

1. Role-based Update Strategy (Serial/Parallel/BestEffortParallel)
2. Role-based Access Mode (ReadWrite/Readonly/None)
3. Automatic Switchover
4. Membership Reconfiguration
5. Multiple Instance Templates
6. Specified Instance Scale In
7. In-place Instance Update
*/
package instanceset
