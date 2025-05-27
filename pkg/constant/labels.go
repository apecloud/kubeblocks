/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	ShardingDefLabelKey           = "shardingdefinition.kubeblocks.io/name"
	ComponentDefinitionLabelKey   = "componentdefinition.kubeblocks.io/name"
	ComponentVersionLabelKey      = "componentversion.kubeblocks.io/name"
	SidecarDefLabelKey            = "sidecardefinition.kubeblocks.io/name"
	ServiceDescriptorNameLabelKey = "servicedescriptor.kubeblocks.io/name"
	AddonNameLabelKey             = "extensions.kubeblocks.io/addon-name"

	KBAppComponentLabelKey    = "apps.kubeblocks.io/component-name"
	KBAppShardingNameLabelKey = "apps.kubeblocks.io/sharding-name"

	KBAppComponentInstanceTemplateLabelKey = "apps.kubeblocks.io/instance-template"
	PVCNameLabelKey                        = "apps.kubeblocks.io/pvc-name"
	VolumeClaimTemplateNameLabelKey        = "apps.kubeblocks.io/vct-name"
	KBAppPodNameLabelKey                   = "apps.kubeblocks.io/pod-name"

	RoleLabelKey           = "kubeblocks.io/role" // RoleLabelKey consensusSet and replicationSet role label key
	KBAppServiceVersionKey = "apps.kubeblocks.io/service-version"
	KBAppReleasePhaseKey   = "apps.kubeblocks.io/release-phase" // TODO: release or service phase?
)

func GetClusterLabels(clusterName string, labels ...map[string]string) map[string]string {
	return withShardingNameLabel(map[string]string{
		AppManagedByLabelKey: AppName,
		AppInstanceLabelKey:  clusterName,
	}, labels...)
}

func GetCompLabels(clusterName, compName string, labels ...map[string]string) map[string]string {
	return withShardingNameLabel(map[string]string{
		AppManagedByLabelKey:   AppName,
		AppInstanceLabelKey:    clusterName,
		KBAppComponentLabelKey: compName,
	}, labels...)
}

func GetCompLabelsWithDef(clusterName, compName, compDef string, labels ...map[string]string) map[string]string {
	m := map[string]string{
		AppManagedByLabelKey:   AppName,
		AppInstanceLabelKey:    clusterName,
		KBAppComponentLabelKey: compName,
	}
	if len(compDef) > 0 {
		m[AppComponentLabelKey] = compDef
	}
	return withShardingNameLabel(m, labels...)
}

func withShardingNameLabel(labels map[string]string, extraLabels ...map[string]string) map[string]string {
	for _, m := range extraLabels {
		if m != nil {
			if v, ok := m[KBAppShardingNameLabelKey]; ok {
				labels[KBAppShardingNameLabelKey] = v
				break
			}
		}
	}
	return labels
}

func GetConfigurationLabels(clusterName, compName, cmTplName string) map[string]string {
	return map[string]string{
		AppManagedByLabelKey:   AppName,
		AppInstanceLabelKey:    clusterName,
		KBAppComponentLabelKey: compName,
		CMTemplateNameLabelKey: cmTplName,
	}
}
