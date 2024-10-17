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

// config keys used in viper, DON'T refactor the value without careful inspections
const (
	CfgKeyServerInfo                    = "_KUBE_SERVER_INFO"
	CfgKeyCtrlrMgrNS                    = "CM_NAMESPACE"
	CfgKeyCtrlrMgrAffinity              = "CM_AFFINITY"
	CfgKeyCtrlrMgrNodeSelector          = "CM_NODE_SELECTOR"
	CfgKeyCtrlrMgrTolerations           = "CM_TOLERATIONS"
	CfgKeyCtrlrReconcileRetryDurationMS = "CM_RECON_RETRY_DURATION_MS"       // accept time
	CfgRecoverVolumeExpansionFailure    = "RECOVER_VOLUME_EXPANSION_FAILURE" // refer to feature gates RecoverVolumeExpansionFailure of k8s.
	CfgKeyProvider                      = "KUBE_PROVIDER"
	CfgHostPortConfigMapName            = "HOST_PORT_CM_NAME"
	CfgHostPortIncludeRanges            = "HOST_PORT_INCLUDE_RANGES"
	CfgHostPortExcludeRanges            = "HOST_PORT_EXCLUDE_RANGES"

	// addon config keys
	CfgKeyAddonJobTTL        = "ADDON_JOB_TTL"
	CfgAddonJobImgPullPolicy = "ADDON_JOB_IMAGE_PULL_POLICY"

	// data plane config key
	CfgKeyDataPlaneTolerations = "DATA_PLANE_TOLERATIONS"
	CfgKeyDataPlaneAffinity    = "DATA_PLANE_AFFINITY"

	// storage config keys
	CfgKeyDefaultStorageClass = "DEFAULT_STORAGE_CLASS"

	// customized encryption key for encrypting the password of connection credential.
	CfgKeyDPEncryptionKey                = "DP_ENCRYPTION_KEY"
	CfgKeyDPBackupEncryptionSecretKeyRef = "DP_BACKUP_ENCRYPTION_SECRET_KEY_REF"
	CfgKeyDPBackupEncryptionAlgorithm    = "DP_BACKUP_ENCRYPTION_ALGORITHM"

	CfgKBReconcileWorkers = "KUBEBLOCKS_RECONCILE_WORKERS"
	CfgClientQPS          = "CLIENT_QPS"
	CfgClientBurst        = "CLIENT_BURST"

	CfgRegistries = "registries"
)
