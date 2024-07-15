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

package builder

import (
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
)

type InstanceSetBuilder struct {
	BaseBuilder[workloads.InstanceSet, *workloads.InstanceSet, InstanceSetBuilder]
}

func NewInstanceSetBuilder(namespace, name string) *InstanceSetBuilder {
	builder := &InstanceSetBuilder{}
	replicas := int32(1)
	builder.init(namespace, name,
		&workloads.InstanceSet{
			Spec: workloads.InstanceSetSpec{
				Replicas: &replicas,
			},
		}, builder)
	return builder
}

func (builder *InstanceSetBuilder) SetReplicas(replicas int32) *InstanceSetBuilder {
	builder.get().Spec.Replicas = &replicas
	return builder
}

func (builder *InstanceSetBuilder) SetMinReadySeconds(minReadySeconds int32) *InstanceSetBuilder {
	builder.get().Spec.MinReadySeconds = minReadySeconds
	return builder
}

func (builder *InstanceSetBuilder) AddMatchLabel(key, value string) *InstanceSetBuilder {
	labels := make(map[string]string, 1)
	labels[key] = value
	return builder.AddMatchLabelsInMap(labels)
}

func (builder *InstanceSetBuilder) AddMatchLabels(keyValues ...string) *InstanceSetBuilder {
	return builder.AddMatchLabelsInMap(WithMap(keyValues...))
}

func (builder *InstanceSetBuilder) AddMatchLabelsInMap(labels map[string]string) *InstanceSetBuilder {
	selector := builder.get().Spec.Selector
	if selector == nil {
		selector = &metav1.LabelSelector{}
		builder.get().Spec.Selector = selector
	}
	matchLabels := builder.get().Spec.Selector.MatchLabels
	if matchLabels == nil {
		matchLabels = make(map[string]string, len(labels))
	}
	for k, v := range labels {
		matchLabels[k] = v
	}
	builder.get().Spec.Selector.MatchLabels = matchLabels
	return builder
}

func (builder *InstanceSetBuilder) SetRoles(roles []workloads.ReplicaRole) *InstanceSetBuilder {
	builder.get().Spec.Roles = roles
	return builder
}

func (builder *InstanceSetBuilder) SetTemplate(template corev1.PodTemplateSpec) *InstanceSetBuilder {
	builder.get().Spec.Template = template
	return builder
}

func (builder *InstanceSetBuilder) AddVolumeClaimTemplates(templates ...corev1.PersistentVolumeClaim) *InstanceSetBuilder {
	templateList := builder.get().Spec.VolumeClaimTemplates
	templateList = append(templateList, templates...)
	builder.get().Spec.VolumeClaimTemplates = templateList
	return builder
}

func (builder *InstanceSetBuilder) SetVolumeClaimTemplates(templates ...corev1.PersistentVolumeClaim) *InstanceSetBuilder {
	builder.get().Spec.VolumeClaimTemplates = templates
	return builder
}

func (builder *InstanceSetBuilder) SetPodManagementPolicy(policy apps.PodManagementPolicyType) *InstanceSetBuilder {
	builder.get().Spec.PodManagementPolicy = policy
	return builder
}

func (builder *InstanceSetBuilder) SetPodUpdatePolicy(policy workloads.PodUpdatePolicyType) *InstanceSetBuilder {
	builder.get().Spec.PodUpdatePolicy = policy
	return builder
}

func (builder *InstanceSetBuilder) SetUpdateStrategy(strategy apps.StatefulSetUpdateStrategy) *InstanceSetBuilder {
	builder.get().Spec.UpdateStrategy = strategy
	return builder
}

func (builder *InstanceSetBuilder) SetUpdateStrategyType(strategyType apps.StatefulSetUpdateStrategyType) *InstanceSetBuilder {
	builder.get().Spec.UpdateStrategy.Type = strategyType
	return builder
}

func (builder *InstanceSetBuilder) SetCustomHandler(handler []workloads.Action) *InstanceSetBuilder {
	roleProbe := builder.get().Spec.RoleProbe
	if roleProbe == nil {
		roleProbe = &workloads.RoleProbe{}
	}
	roleProbe.CustomHandler = handler
	builder.get().Spec.RoleProbe = roleProbe
	return builder
}

func (builder *InstanceSetBuilder) AddCustomHandler(handler workloads.Action) *InstanceSetBuilder {
	roleProbe := builder.get().Spec.RoleProbe
	if roleProbe == nil {
		roleProbe = &workloads.RoleProbe{}
	}
	handlers := roleProbe.CustomHandler
	handlers = append(handlers, handler)
	roleProbe.CustomHandler = handlers
	builder.get().Spec.RoleProbe = roleProbe
	return builder
}

func (builder *InstanceSetBuilder) SetRoleProbe(roleProbe *workloads.RoleProbe) *InstanceSetBuilder {
	builder.get().Spec.RoleProbe = roleProbe
	return builder
}

func (builder *InstanceSetBuilder) SetService(service *corev1.Service) *InstanceSetBuilder {
	builder.get().Spec.Service = service
	return builder
}

func (builder *InstanceSetBuilder) SetMembershipReconfiguration(reconfiguration *workloads.MembershipReconfiguration) *InstanceSetBuilder {
	builder.get().Spec.MembershipReconfiguration = reconfiguration
	return builder
}

func (builder *InstanceSetBuilder) SetMemberUpdateStrategy(strategy *workloads.MemberUpdateStrategy) *InstanceSetBuilder {
	builder.get().Spec.MemberUpdateStrategy = strategy
	if strategy != nil {
		builder.SetUpdateStrategyType(apps.OnDeleteStatefulSetStrategyType)
	}
	return builder
}

func (builder *InstanceSetBuilder) SetPaused(paused bool) *InstanceSetBuilder {
	builder.get().Spec.Paused = paused
	return builder
}

func (builder *InstanceSetBuilder) SetCredential(credential workloads.Credential) *InstanceSetBuilder {
	builder.get().Spec.Credential = &credential
	return builder
}

func (builder *InstanceSetBuilder) SetInstances(instances []workloads.InstanceTemplate) *InstanceSetBuilder {
	builder.get().Spec.Instances = instances
	return builder
}
