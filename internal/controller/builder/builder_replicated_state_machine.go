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
	corev1 "k8s.io/api/core/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
)

type ReplicatedStateMachineBuilder struct {
	BaseBuilder[workloads.ReplicatedStateMachine, *workloads.ReplicatedStateMachine, ReplicatedStateMachineBuilder]
}

func NewReplicatedStateMachineBuilder(namespace, name string) *ReplicatedStateMachineBuilder {
	builder := &ReplicatedStateMachineBuilder{}
	builder.init(namespace, name,
		&workloads.ReplicatedStateMachine{
			Spec: workloads.ReplicatedStateMachineSpec{
				Replicas: 1,
				Roles: []workloads.ReplicaRole{
					{
						Name:       "leader",
						AccessMode: workloads.ReadWriteMode,
						IsLeader:   true,
						CanVote:    true,
					},
				},
				UpdateStrategy: workloads.SerialUpdateStrategy,
			},
		}, builder)
	return builder
}

func (builder *ReplicatedStateMachineBuilder) SetReplicas(replicas int32) *ReplicatedStateMachineBuilder {
	builder.get().Spec.Replicas = replicas
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

func (builder *ReplicatedStateMachineBuilder) SetObservationActions(actions []workloads.Action) *ReplicatedStateMachineBuilder {
	builder.get().Spec.RoleObservation.ObservationActions = actions
	return builder
}

func (builder *ReplicatedStateMachineBuilder) AddObservationAction(action workloads.Action) *ReplicatedStateMachineBuilder {
	actions := builder.get().Spec.RoleObservation.ObservationActions
	actions = append(actions, action)
	builder.get().Spec.RoleObservation.ObservationActions = actions
	return builder
}

func (builder *ReplicatedStateMachineBuilder) SetService(service corev1.ServiceSpec) *ReplicatedStateMachineBuilder {
	builder.get().Spec.Service = service
	return builder
}

func (builder *ReplicatedStateMachineBuilder) SetMembershipReconfiguration(reconfiguration workloads.MembershipReconfiguration) *ReplicatedStateMachineBuilder {
	builder.get().Spec.MembershipReconfiguration = &reconfiguration
	return builder
}
