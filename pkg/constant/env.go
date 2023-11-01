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

package constant

const (
	KBEnvNamespace            = "KB_NAMESPACE"
	KBEnvHostIP               = "KB_HOST_IP"
	KBEnvNodeName             = "KB_NODENAME"
	KBEnvPodName              = "KB_POD_NAME"
	KBEnvPodUID               = "KB_POD_UID"
	KBEnvVolumeProtectionSpec = "KB_VOLUME_PROTECTION_SPEC"
)

const (
	// Lorry env names
	KBEnvClusterName     = "KB_CLUSTER_NAME"
	KBEnvComponentName   = "KB_COMP_NAME"
	KBEnvClusterCompName = "KB_CLUSTER_COMP_NAME"
	KBEnvWorkloadType    = "KB_WORKLOAD_TYPE"
	KBEnvBuiltinHandler  = "KB_BUILTIN_HANDLER"
	KBEnvCharacterType   = "KB_SERVICE_CHARACTER_TYPE"
	KBEnvServiceUser     = "KB_SERVICE_USER"
	KBEnvServicePassword = "KB_SERVICE_PASSWORD"

	// The Roles configured in the cluster definition that are visible to users.
	KBEnvServiceRoles = "KB_SERVICE_ROLES"

	// The port of the DB service
	KBEnvServicePort = "KB_SERVICE_PORT"

	// The data volume path of the DB service.
	KBEnvDataPath = "KB_DATA_PATH"

	// TTL controls the lease expiration time in DCS. If the leader fails to renew
	// its lease within the TTL duration, it will lose the leader role,
	// allowing other replicas to take over.
	KBEnvTTL = "KB_TTL"

	// Maximum replication lag permitted when performing a switchover.
	KBEnvMaxLag = "KB_MAX_LAG"

	// Whether to enable high availability, true by default.
	KBEnvEnableHA = "KB_ENABLE_HA"

	// The method to send events: DirectAPIServerEventUpdate(through lorry servier),
	// ReadinessProbeEventUpdate(through kubelet service)
	KBEnvRsmRoleUpdateMechanism = "KB_RSM_ROLE_UPDATE_MECHANISM"
	KBEnvRoleProbeTimeout       = "KB_RSM_ROLE_PROBE_TIMEOUT"
)
