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

// GetKBConfigMapWellKnownLabels returns the well-known labels for KB ConfigMap
func GetKBConfigMapWellKnownLabels(cmTplName, clusterDefName, clusterName, componentName string) map[string]string {
	return map[string]string{
		CMTemplateNameLabelKey: cmTplName,
		AppNameLabelKey:        clusterDefName,
		AppInstanceLabelKey:    clusterName,
		KBAppComponentLabelKey: componentName,
	}
}

// GetKBWellKnownLabels returns the well-known labels for KB resources
func GetKBWellKnownLabels(clusterDefName, clusterName, componentName string) map[string]string {
	return map[string]string{
		AppManagedByLabelKey:   AppName,
		AppNameLabelKey:        clusterDefName,
		AppInstanceLabelKey:    clusterName,
		KBAppComponentLabelKey: componentName,
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

// GetClusterCompDefLabel returns the label for ClusterComponentDefinition (refer clusterDefinition.Spec.ComponentDefs[*].Name)
// TODO:ClusterCompDef will be deprecated in the future
func GetClusterCompDefLabel(clusterCompDefName string) map[string]string {
	return map[string]string{
		AppComponentLabelKey: clusterCompDefName,
	}
}

// GetWorkloadTypeLabel returns the label for WorkloadType (refer clusterDefinition.Spec.ComponentDefs[*].WorkloadType)
// TODO:workloadType will be deprecated in the future
func GetWorkloadTypeLabel(workloadType string) map[string]string {
	return map[string]string{
		WorkloadTypeLabelKey: workloadType,
	}
}

// GetClusterVersionLabel returns the label for ClusterVersion
// TODO:clusterVersion will be deprecated in the future
func GetClusterVersionLabel(clusterVersion string) map[string]string {
	return map[string]string{
		AppVersionLabelKey: clusterVersion,
	}
}

// GetClusterDefTypeLabel returns the label for ClusterDefinition type (refer clusterDefinition.Spec.Type)
// TODO:clusterDefType will be deprecated in the future
func GetClusterDefTypeLabel(clusterDefType string) map[string]string {
	return map[string]string{
		KBAppClusterDefTypeLabelKey: clusterDefType,
	}
}
