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
	AppManagedByLabelKey = "app.kubernetes.io/managed-by"
	AppNameLabelKey      = "app.kubernetes.io/name"
	AppInstanceLabelKey  = "app.kubernetes.io/instance"
	AppComponentLabelKey = "app.kubernetes.io/component"
)

// labels for KubeBlocks
const (
	BackupProtectionLabelKey               = "kubeblocks.io/backup-protection" // BackupProtectionLabelKey Backup delete protection policy label
	RoleLabelKey                           = "kubeblocks.io/role"              // RoleLabelKey consensusSet and replicationSet role label key
	AccessModeLabelKey                     = "workloads.kubeblocks.io/access-mode"
	ReadyWithoutPrimaryKey                 = "kubeblocks.io/ready-without-primary"
	ClusterAccountLabelKey                 = "account.kubeblocks.io/name"
	KBAppComponentLabelKey                 = "apps.kubeblocks.io/component-name"
	KBAppShardingNameLabelKey              = "apps.kubeblocks.io/sharding-name"
	KBManagedByKey                         = "apps.kubeblocks.io/managed-by" // KBManagedByKey marks resources that auto created
	PVCNameLabelKey                        = "apps.kubeblocks.io/pvc-name"
	VolumeClaimTemplateNameLabelKey        = "apps.kubeblocks.io/vct-name"
	KBAppComponentInstanceTemplateLabelKey = "apps.kubeblocks.io/instance-template"
	KBAppServiceVersionKey                 = "apps.kubeblocks.io/service-version"
	KBAppPodNameLabelKey                   = "apps.kubeblocks.io/pod-name"
	ClusterDefLabelKey                     = "clusterdefinition.kubeblocks.io/name"
	ComponentDefinitionLabelKey            = "componentdefinition.kubeblocks.io/name"
	ComponentVersionLabelKey               = "componentversion.kubeblocks.io/name"
	ConsensusSetAccessModeLabelKey         = "cs.apps.kubeblocks.io/access-mode"
	AddonNameLabelKey                      = "extensions.kubeblocks.io/addon-name"
	OpsRequestTypeLabelKey                 = "ops.kubeblocks.io/ops-type"
	OpsRequestNameLabelKey                 = "ops.kubeblocks.io/ops-name"
	OpsRequestNamespaceLabelKey            = "ops.kubeblocks.io/ops-namespace"
	ServiceDescriptorNameLabelKey          = "servicedescriptor.kubeblocks.io/name"
)

func GetClusterLabels(clusterName string) map[string]string {
	return map[string]string{
		AppManagedByLabelKey: AppName,
		AppInstanceLabelKey:  clusterName,
	}
}

func GetClusterLabelsWithDef(clusterName, clusterDef string) map[string]string {
	labels := map[string]string{
		AppManagedByLabelKey: AppName,
		AppInstanceLabelKey:  clusterName,
	}
	if len(clusterDef) > 0 {
		labels[AppNameLabelKey] = clusterDef
	}
	return labels
}

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

// GetCompDefLabel returns the label for ComponentDefinition (refer ComponentDefinition.Name)
func GetCompDefLabel(compDefName string) map[string]string {
	return map[string]string{
		AppComponentLabelKey: compDefName,
	}
}

// GetShardingNameLabel returns the shard template name label for component generated from shardSpec
func GetShardingNameLabel(shardingName string) map[string]string {
	return map[string]string{
		KBAppShardingNameLabelKey: shardingName,
	}
}

// GetKBConfigMapWellKnownLabels returns the well-known labels for KB ConfigMap
func GetKBConfigMapWellKnownLabels(cmTplName, componentDefName, clusterName, componentName string) map[string]string {
	return map[string]string{
		CMTemplateNameLabelKey: cmTplName,
		AppNameLabelKey:        componentDefName,
		AppInstanceLabelKey:    clusterName,
		KBAppComponentLabelKey: componentName,
	}
}
