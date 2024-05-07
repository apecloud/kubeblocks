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

package constant

import (
	"fmt"
)

func EnvPlaceHolder(env string) string {
	return fmt.Sprintf("$(%s)", env)
}

// Global
const (
	KBEnvNamespace = "KB_NAMESPACE"
)

// Cluster
const (
	KBEnvClusterName                  = "KB_CLUSTER_NAME"
	KBEnvClusterUID                   = "KB_CLUSTER_UID"
	KBEnvClusterCompName              = "KB_CLUSTER_COMP_NAME"
	KBEnvClusterUIDPostfix8Deprecated = "KB_CLUSTER_UID_POSTFIX_8"
)

// Component
const (
	KBEnvCompName           = "KB_COMP_NAME"
	KBEnvCompReplicas       = "KB_COMP_REPLICAS"
	KBEnvCompServiceVersion = "KB_COMP_SERVICE_VERSION"
)

// Pod
const (
	KBEnvPodName          = "KB_POD_NAME"
	KBEnvPodUID           = "KB_POD_UID"
	KBEnvPodIP            = "KB_POD_IP"
	KBEnvPodIPs           = "KB_POD_IPS"
	KBEnvPodFQDN          = "KB_POD_FQDN"
	KBEnvPodOrdinal       = "KB_POD_ORDINAL"
	KBEnvPodIPDeprecated  = "KB_PODIP"
	KBEnvPodIPsDeprecated = "KB_PODIPS"
)

// Host
const (
	KBEnvHostIP           = "KB_HOST_IP"
	KBEnvNodeName         = "KB_NODENAME"
	KBEnvHostIPDeprecated = "KB_HOSTIP"
)

// ServiceAccount
const (
	KBEnvServiceAccountName = "KB_SA_NAME"
)

// TLS
const (
	KBEnvTLSCertPath = "KB_TLS_CERT_PATH"
	KBEnvTLSCertFile = "KB_TLS_CERT_FILE"
	KBEnvTLSCAFile   = "KB_TLS_CA_FILE"
	KBEnvTLSKeyFile  = "KB_TLS_KEY_FILE"
)

// Lorry
const (
	KBEnvWorkloadType    = "KB_WORKLOAD_TYPE"
	KBEnvBuiltinHandler  = "KB_BUILTIN_HANDLER"
	KBEnvActionCommands  = "KB_ACTION_COMMANDS"
	KBEnvCronJobs        = "KB_CRON_JOBS"
	KBEnvCharacterType   = "KB_SERVICE_CHARACTER_TYPE"
	KBEnvServiceUser     = "KB_SERVICE_USER"
	KBEnvServicePassword = "KB_SERVICE_PASSWORD"
	KBEnvLorryHTTPPort   = "LORRY_HTTP_PORT"
	KBEnvLorryGRPCPort   = "LORRY_GRPC_PORT"
	KBEnvLorryLogLevel   = "LORRY_LOG_LEVEL"
	// KBEnvServiceRoles defines the Roles configured in the cluster definition that are visible to users.
	KBEnvServiceRoles = "KB_SERVICE_ROLES"

	// KBEnvServicePort defines the port of the DB service
	KBEnvServicePort = "KB_SERVICE_PORT"

	// KBEnvDataPath defines the data volume path of the DB service.
	KBEnvDataPath = "KB_DATA_PATH"

	// KBEnvTTL controls the lease expiration time in DCS. If the leader fails to renew its lease within the TTL duration, it will lose the leader role, allowing other replicas to take over.
	KBEnvTTL = "KB_TTL"

	// KBEnvMaxLag defines maximum replication lag permitted when performing a switchover.
	KBEnvMaxLag = "KB_MAX_LAG"

	// KBEnvEnableHA Whether to enable high availability, true by default.
	KBEnvEnableHA = "KB_ENABLE_HA"

	// KBEnvRsmRoleUpdateMechanism defines the method to send events: DirectAPIServerEventUpdate(through lorry service), ReadinessProbeEventUpdate(through kubelet service)
	KBEnvRsmRoleUpdateMechanism = "KB_RSM_ROLE_UPDATE_MECHANISM"
	KBEnvRoleProbeTimeout       = "KB_RSM_ROLE_PROBE_TIMEOUT"
	KBEnvRoleProbePeriod        = "KB_RSM_ROLE_PROBE_PERIOD"

	KBEnvVolumeProtectionSpec = "KB_VOLUME_PROTECTION_SPEC"
)
