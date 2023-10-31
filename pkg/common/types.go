/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package common

// PodRoleNamePair defines a pod name and role name pair.
type PodRoleNamePair struct {
	PodName  string `json:"podName,omitempty"`
	RoleName string `json:"roleName,omitempty"`
}

// GlobalRoleSnapshot defines a global(leader) perspective of all pods role.
// KB provides two role probe methods: per-pod level role probe and retrieving all node roles from the leader node.
// The latter is referred to as the global role snapshot. This data structure is used to represent a snapshot of global role information.
// The snapshot contains two types of information: the mapping relationship between all node names and role names,
// and the version of the snapshot. The purpose of the snapshot version is to ensure that only role information
// that is more up-to-date than the current role information on the Pod Label will be updated. This resolves the issue of
// role information disorder in scenarios such as KB upgrades or exceptions causing restarts,
// network partitioning leading to split-brain situations, node crashes, and similar occurrences.
type GlobalRoleSnapshot struct {
	Version          string            `json:"term,omitempty"`
	PodRoleNamePairs []PodRoleNamePair `json:"PodRoleNamePairs,omitempty"`
}

// BuiltinHandler defines builtin role probe handler name.
type BuiltinHandler string

const (
	MySQLHandler    BuiltinHandler = "mysql"
	PostgresHandler BuiltinHandler = "postgres"
	MongoDBHandler  BuiltinHandler = "mongodb"
	RedisHandler    BuiltinHandler = "redis"
	ETCDHandler     BuiltinHandler = "etcd"
	KafkaHandler    BuiltinHandler = "kafka"
	WeSQLHandler    BuiltinHandler = "wesql"
)
