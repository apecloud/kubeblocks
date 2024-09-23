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

// k8s recommended well-known label keys
const (
	AppVersionLabelKey = "app.kubernetes.io/version"

	AppManagedByLabelKey = "app.kubernetes.io/managed-by"

	AppNameLabelKey      = "app.kubernetes.io/name"
	AppComponentLabelKey = "app.kubernetes.io/component"

	AppInstanceLabelKey = "app.kubernetes.io/instance"
)

// labels defined by KubeBlocks
const (
	ClusterDefLabelKey            = "clusterdefinition.kubeblocks.io/name"
	ComponentDefinitionLabelKey   = "componentdefinition.kubeblocks.io/name"
	ComponentVersionLabelKey      = "componentversion.kubeblocks.io/name"
	ServiceDescriptorNameLabelKey = "servicedescriptor.kubeblocks.io/name"
	AddonNameLabelKey             = "extensions.kubeblocks.io/addon-name"

	KBAppComponentLabelKey    = "apps.kubeblocks.io/component-name"
	KBAppShardingNameLabelKey = "apps.kubeblocks.io/sharding-name"

	KBAppComponentInstanceTemplateLabelKey = "apps.kubeblocks.io/instance-template"
	PVCNameLabelKey                        = "apps.kubeblocks.io/pvc-name"
	VolumeClaimTemplateNameLabelKey        = "apps.kubeblocks.io/vct-name"
	KBAppPodNameLabelKey                   = "apps.kubeblocks.io/pod-name"

	RoleLabelKey             = "kubeblocks.io/role" // RoleLabelKey consensusSet and replicationSet role label key
	KBAppServiceVersionKey   = "apps.kubeblocks.io/service-version"
	BackupProtectionLabelKey = "kubeblocks.io/backup-protection" // BackupProtectionLabelKey Backup delete protection policy label
	AccessModeLabelKey       = "workloads.kubeblocks.io/access-mode"
	ReadyWithoutPrimaryKey   = "kubeblocks.io/ready-without-primary"

	KBManagedByKey = "apps.kubeblocks.io/managed-by" // KBManagedByKey marks resources that auto created
)

func GetClusterLabels(clusterName string) map[string]string {
	return map[string]string{
		AppManagedByLabelKey: AppName,
		AppInstanceLabelKey:  clusterName,
	}
}

// func GetClusterLabelsWithDef(clusterName, clusterDef string) map[string]string {
//	labels := map[string]string{
//		AppManagedByLabelKey: AppName,
//		AppInstanceLabelKey:  clusterName,
//	}
//	if len(clusterDef) > 0 {
//		labels[AppNameLabelKey] = clusterDef
//	}
//	return labels
// }

func GetCompLabels(clusterName, compName string) map[string]string {
	return map[string]string{
		AppManagedByLabelKey:   AppName,
		AppInstanceLabelKey:    clusterName,
		KBAppComponentLabelKey: compName,
	}
}

func GetCompLabelsWithDef(clusterName, compName, compDef string) map[string]string {
	labels := map[string]string{
		AppManagedByLabelKey:   AppName,
		AppInstanceLabelKey:    clusterName,
		KBAppComponentLabelKey: compName,
	}
	if len(compDef) > 0 {
		labels[AppComponentLabelKey] = compDef
	}
	return labels
}

func GetShardingLabels(clusterName, shardingName string) map[string]string {
	return map[string]string{
		AppManagedByLabelKey:      AppName,
		AppInstanceLabelKey:       clusterName,
		KBAppShardingNameLabelKey: shardingName,
	}
}

func GetConfigurationLabels(clusterName, compName, cmTplName string) map[string]string {
	return map[string]string{
		AppManagedByLabelKey:   AppName,
		AppInstanceLabelKey:    clusterName,
		KBAppComponentLabelKey: compName,
		CMTemplateNameLabelKey: cmTplName,
	}
}
