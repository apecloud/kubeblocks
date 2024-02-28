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

// GetKBConfigMapWellKnownLabels returns the well-known labels for KB ConfigMap
func GetKBConfigMapWellKnownLabels(cmTplName, clusterDefName, clusterName, componentName string) map[string]string {
	return map[string]string{
		CMTemplateNameLabelKey: cmTplName,
		AppNameLabelKey:        clusterDefName,
		AppInstanceLabelKey:    clusterName,
		KBAppComponentLabelKey: componentName,
	}
}

// GetKBWellKnownLabels returns the well-known labels for KB resources with ClusterDefinition API
func GetKBWellKnownLabels(clusterDefName, clusterName, componentName string) map[string]string {
	return map[string]string{
		AppManagedByLabelKey:   AppName,
		AppNameLabelKey:        clusterDefName,
		AppInstanceLabelKey:    clusterName,
		KBAppComponentLabelKey: componentName,
	}
}

// GetKBWellKnownLabelsWithCompDef returns the well-known labels for KB resources with ComponentDefinition API
func GetKBWellKnownLabelsWithCompDef(compDefName, clusterName, componentName string) map[string]string {
	return map[string]string{
		AppManagedByLabelKey:   AppName,
		AppNameLabelKey:        compDefName, // TODO: reusing AppNameLabelKey for compDefName ?
		AppInstanceLabelKey:    clusterName,
		KBAppComponentLabelKey: componentName,
	}
}

// GetClusterWellKnownLabels returns the well-known labels for a cluster
func GetClusterWellKnownLabels(clusterName string) map[string]string {
	return map[string]string{
		AppManagedByLabelKey: AppName,
		AppInstanceLabelKey:  clusterName,
	}
}

// GetComponentWellKnownLabels returns the well-known labels for Component API
func GetComponentWellKnownLabels(clusterName, componentName string) map[string]string {
	return map[string]string{
		AppManagedByLabelKey:   AppName,
		AppInstanceLabelKey:    clusterName,
		KBAppComponentLabelKey: componentName,
	}
}

// GetAppVersionLabel returns the label for AppVersion
func GetAppVersionLabel(appVersion string) map[string]string {
	return map[string]string{
		AppVersionLabelKey: appVersion,
	}
}

// GetComponentDefLabel returns the label for ComponentDefinition (refer ComponentDefinition.Name)
func GetComponentDefLabel(compDefName string) map[string]string {
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

// GetClusterCompDefLabel returns the label for ClusterComponentDefinition (refer clusterDefinition.Spec.ComponentDefs[*].Name)
// TODO:ClusterCompDef will be deprecated in the future
func GetClusterCompDefLabel(clusterCompDefName string) map[string]string {
	return map[string]string{
		AppComponentLabelKey: clusterCompDefName,
	}
}

// GetClusterDefTypeLabel returns the label for ClusterDefinition type (refer clusterDefinition.Spec.Type)
// TODO:clusterDefType will be deprecated in the future
func GetClusterDefTypeLabel(clusterDefType string) map[string]string {
	return map[string]string{
		KBAppClusterDefTypeLabelKey: clusterDefType,
	}
}

// GetKBReservedLabelKeys returns the reserved label keys for KubeBlocks
func GetKBReservedLabelKeys() []string {
	return []string{
		AppManagedByLabelKey,
		AppNameLabelKey,
		AppInstanceLabelKey,
		AppComponentLabelKey,
		AppVersionLabelKey,
		KBAppComponentLabelKey,
		KBAppShardingNameLabelKey,
		KBManagedByKey,
		RoleLabelKey,
	}
}
