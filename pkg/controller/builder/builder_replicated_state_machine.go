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

package builder

import (
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
)

type ReplicatedStateMachineBuilder struct {
	BaseBuilder[workloads.ReplicatedStateMachine, *workloads.ReplicatedStateMachine, ReplicatedStateMachineBuilder]
}

func NewReplicatedStateMachineBuilder(namespace, name string) *ReplicatedStateMachineBuilder {
	builder := &ReplicatedStateMachineBuilder{}
	replicas := int32(1)
	builder.init(namespace, name,
		&workloads.ReplicatedStateMachine{
			Spec: workloads.ReplicatedStateMachineSpec{
				Replicas: &replicas,
			},
		}, builder)
	return builder
}

func (builder *ReplicatedStateMachineBuilder) SetReplicas(replicas int32) *ReplicatedStateMachineBuilder {
	builder.get().Spec.Replicas = &replicas
	return builder
}

func (builder *ReplicatedStateMachineBuilder) SetRsmTransformPolicy(transformPolicy workloads.RsmTransformPolicy) *ReplicatedStateMachineBuilder {
	builder.get().Spec.RsmTransformPolicy = transformPolicy
	return builder
}

func (builder *ReplicatedStateMachineBuilder) SetNodeAssignment(nodeAssignment []workloads.NodeAssignment) *ReplicatedStateMachineBuilder {
	builder.get().Spec.NodeAssignment = nodeAssignment
	return builder
}

func (builder *ReplicatedStateMachineBuilder) AddMatchLabel(key, value string) *ReplicatedStateMachineBuilder {
	labels := make(map[string]string, 1)
	labels[key] = value
	return builder.AddMatchLabelsInMap(labels)
}

func (builder *ReplicatedStateMachineBuilder) AddMatchLabels(keyValues ...string) *ReplicatedStateMachineBuilder {
	return builder.AddMatchLabelsInMap(WithMap(keyValues...))
}

func (builder *ReplicatedStateMachineBuilder) AddMatchLabelsInMap(labels map[string]string) *ReplicatedStateMachineBuilder {
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

func (builder *ReplicatedStateMachineBuilder) SetServiceName(serviceName string) *ReplicatedStateMachineBuilder {
	builder.get().Spec.ServiceName = serviceName
	return builder
}

func (builder *ReplicatedStateMachineBuilder) SetRoles(roles []workloads.ReplicaRole) *ReplicatedStateMachineBuilder {
	builder.get().Spec.Roles = roles
	return builder
}

func (builder *ReplicatedStateMachineBuilder) SetTemplate(template corev1.PodTemplateSpec) *ReplicatedStateMachineBuilder {
	builder.get().Spec.Template = template
	return builder
}

func (builder *ReplicatedStateMachineBuilder) AddVolumeClaimTemplates(templates ...corev1.PersistentVolumeClaim) *ReplicatedStateMachineBuilder {
	templateList := builder.get().Spec.VolumeClaimTemplates
	templateList = append(templateList, templates...)
	builder.get().Spec.VolumeClaimTemplates = templateList
	return builder
}

func (builder *ReplicatedStateMachineBuilder) SetVolumeClaimTemplates(templates ...corev1.PersistentVolumeClaim) *ReplicatedStateMachineBuilder {
	builder.get().Spec.VolumeClaimTemplates = templates
	return builder
}

func (builder *ReplicatedStateMachineBuilder) SetPodManagementPolicy(policy apps.PodManagementPolicyType) *ReplicatedStateMachineBuilder {
	builder.get().Spec.PodManagementPolicy = policy
	return builder
}

func (builder *ReplicatedStateMachineBuilder) SetUpdateStrategy(strategy apps.StatefulSetUpdateStrategy) *ReplicatedStateMachineBuilder {
	builder.get().Spec.UpdateStrategy = strategy
	return builder
}

func (builder *ReplicatedStateMachineBuilder) SetUpdateStrategyType(strategyType apps.StatefulSetUpdateStrategyType) *ReplicatedStateMachineBuilder {
	builder.get().Spec.UpdateStrategy.Type = strategyType
	return builder
}

func (builder *ReplicatedStateMachineBuilder) SetCustomHandler(handler []workloads.Action) *ReplicatedStateMachineBuilder {
	roleProbe := builder.get().Spec.RoleProbe
	if roleProbe == nil {
		roleProbe = &workloads.RoleProbe{}
	}
	roleProbe.CustomHandler = handler
	builder.get().Spec.RoleProbe = roleProbe
	return builder
}

func (builder *ReplicatedStateMachineBuilder) AddCustomHandler(handler workloads.Action) *ReplicatedStateMachineBuilder {
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

func (builder *ReplicatedStateMachineBuilder) SetRoleProbe(roleProbe *workloads.RoleProbe) *ReplicatedStateMachineBuilder {
	builder.get().Spec.RoleProbe = roleProbe
	return builder
}

func (builder *ReplicatedStateMachineBuilder) SetService(service *corev1.Service) *ReplicatedStateMachineBuilder {
	builder.get().Spec.Service = service
	return builder
}

func (builder *ReplicatedStateMachineBuilder) SetAlternativeServices(services []corev1.Service) *ReplicatedStateMachineBuilder {
	builder.get().Spec.AlternativeServices = services
	return builder
}

func (builder *ReplicatedStateMachineBuilder) SetMembershipReconfiguration(reconfiguration *workloads.MembershipReconfiguration) *ReplicatedStateMachineBuilder {
	builder.get().Spec.MembershipReconfiguration = reconfiguration
	return builder
}

func (builder *ReplicatedStateMachineBuilder) SetMemberUpdateStrategy(strategy *workloads.MemberUpdateStrategy) *ReplicatedStateMachineBuilder {
	builder.get().Spec.MemberUpdateStrategy = strategy
	if strategy != nil {
		builder.SetUpdateStrategyType(apps.OnDeleteStatefulSetStrategyType)
	}
	return builder
}

func (builder *ReplicatedStateMachineBuilder) SetPaused(paused bool) *ReplicatedStateMachineBuilder {
	builder.get().Spec.Paused = paused
	return builder
}

func (builder *ReplicatedStateMachineBuilder) SetCredential(credential workloads.Credential) *ReplicatedStateMachineBuilder {
	builder.get().Spec.Credential = &credential
	return builder
}
