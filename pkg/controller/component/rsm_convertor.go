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

package component

import (
	"errors"

	"github.com/apecloud/kubeblocks/pkg/constant"
	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
)

// BuildRSMFrom builds a new Component object based on Cluster, SynthesizedComponent.
func BuildRSMFrom(cluster *appsv1alpha1.Cluster, synthesizeComp *SynthesizedComponent, protoRSM *workloads.ReplicatedStateMachine) (*workloads.ReplicatedStateMachine, error) {
	if cluster == nil || synthesizeComp == nil {
		return nil, nil
	}
	if protoRSM == nil {
		protoRSM = &workloads.ReplicatedStateMachine{}
	}
	convertors := map[string]convertor{
		"service":                   &rsmServiceConvertor{},
		"alternativeservices":       &rsmAlternativeServicesConvertor{},
		"roles":                     &rsmRolesConvertor{},
		"roleprobe":                 &rsmRoleProbeConvertor{},
		"credential":                &rsmCredentialConvertor{},
		"membershipreconfiguration": &rsmMembershipReconfigurationConvertor{},
		"memberupdatestrategy":      &rsmMemberUpdateStrategyConvertor{},
	}
	if err := covertObject(convertors, &protoRSM.Spec, cluster, synthesizeComp); err != nil {
		return nil, err
	}
	return protoRSM, nil
}

// rsmServiceConvertor is an implementation of the convertor interface, used to convert the given object into ReplicatedStateMachine.Spec.Service.
type rsmServiceConvertor struct{}

// rsmAlternativeServicesConvertor is an implementation of the convertor interface, used to convert the given object into ReplicatedStateMachine.Spec.AlternativeServices.
type rsmAlternativeServicesConvertor struct{}

// rsmRolesConvertor is an implementation of the convertor interface, used to convert the given object into ReplicatedStateMachine.Spec.Roles.
type rsmRolesConvertor struct{}

// rsmRoleProbeConvertor is an implementation of the convertor interface, used to convert the given object into ReplicatedStateMachine.Spec.RoleProbe.
type rsmRoleProbeConvertor struct{}

// rsmCredentialConvertor is an implementation of the convertor interface, used to convert the given object into ReplicatedStateMachine.Spec.Credential.
type rsmCredentialConvertor struct{}

// rsmMembershipReconfigurationConvertor is an implementation of the convertor interface, used to convert the given object into ReplicatedStateMachine.Spec.MembershipReconfiguration.
type rsmMembershipReconfigurationConvertor struct{}

// rsmMemberUpdateStrategyConvertor is an implementation of the convertor interface, used to convert the given object into ReplicatedStateMachine.Spec.MemberUpdateStrategy.
type rsmMemberUpdateStrategyConvertor struct{}

// parseRSMConvertorArgs parses the args of rsm convertor.
func parseRSMConvertorArgs(args ...any) (*appsv1alpha1.Cluster, *SynthesizedComponent, error) {
	cluster, ok := args[0].(*appsv1alpha1.Cluster)
	if !ok {
		return nil, nil, errors.New("args[0] is not a cluster object")
	}
	synthesizeComp, ok := args[1].(*SynthesizedComponent)
	if !ok {
		return nil, nil, errors.New("args[1] not a SynthesizedComponent object")
	}
	return cluster, synthesizeComp, nil
}

// rsmServiceConvertor converts the given object into ReplicatedStateMachine.Spec.Service.
// TODO(xingran): ComponentServices are not consistent with ReplicatedStateMachine.Spec.Service, If it is based on the new ComponentDefinition API,
// the services is temporarily handled in the component controller, and the corresponding ReplicatedStateMachine.Spec.Service is temporarily set nil.
func (c *rsmServiceConvertor) convert(args ...any) (any, error) {
	/*
		var compService appsv1alpha1.ComponentService
		_, synthesizeComp, err := parseRSMConvertorArgs(args...)
		if err != nil {
			return nil, err
		}
		compServices := synthesizeComp.ComponentServices
		if len(compServices) == 0 {
			return nil, nil
		}
		// get the first component service as the rsm service
		if len(compServices) > 0 {
			compService = compServices[0]
		}

		// TODO(xingran): ComponentService.Name and ComponentService.RoleSelector are not used in ReplicatedStateMachine.Spec.Service
		rsmService := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: string(compService.ServiceName),
			},
			Spec: compService.ServiceSpec,
		}
		return rsmService, nil
	*/
	return nil, nil
}

// rsmAlternativeServicesConvertor converts the given object into ReplicatedStateMachine.Spec.AlternativeServices.
// TODO: ComponentServices are not consistent with ReplicatedStateMachine.Spec.AlternativeServices, If it is based on the new ComponentDefinition API,
// the services is temporarily handled in the component controller, and the corresponding ReplicatedStateMachine.Spec.AlternativeServices is temporarily set nil.
func (c *rsmAlternativeServicesConvertor) convert(args ...any) (any, error) {
	return nil, nil
}

// rsmRolesConvertor converts the ComponentDefinition.Spec.Roles into ReplicatedStateMachine.Spec.Roles.
func (c *rsmRolesConvertor) convert(args ...any) (any, error) {
	_, synthesizeComp, err := parseRSMConvertorArgs(args...)
	if err != nil {
		return nil, err
	}
	rsmReplicaRoles := make([]workloads.ReplicaRole, len(synthesizeComp.Roles))
	compReplicaRoles := synthesizeComp.Roles
	for _, compReplicaRole := range compReplicaRoles {
		rsmReplicaRole := workloads.ReplicaRole{
			Name:     compReplicaRole.Name,
			IsLeader: false,
			CanVote:  false,
		}

		if compReplicaRole.Writable {
			rsmReplicaRole.IsLeader = true
			rsmReplicaRole.CanVote = true
		}

		// TODO(xingran): Serviceable equals to CanVote ?
		if compReplicaRole.Serviceable {
			rsmReplicaRole.CanVote = true
			if compReplicaRole.Writable {
				rsmReplicaRole.AccessMode = workloads.ReadWriteMode
			} else {
				rsmReplicaRole.AccessMode = workloads.ReadonlyMode
			}
		} else {
			rsmReplicaRole.AccessMode = workloads.NoneMode
		}
		rsmReplicaRoles = append(rsmReplicaRoles, rsmReplicaRole)
	}
	return rsmReplicaRoles, nil
}

// rsmRoleProbeConvertor converts the ComponentDefinition.Spec.LifecycleActions.RoleProbe into ReplicatedStateMachine.Spec.RoleProbe.
func (c *rsmRoleProbeConvertor) convert(args ...any) (any, error) {
	_, synthesizeComp, err := parseRSMConvertorArgs(args...)
	if err != nil {
		return nil, err
	}

	if synthesizeComp.LifecycleActions == nil || synthesizeComp.LifecycleActions.RoleProbe == nil {
		return nil, nil
	}

	rsmRoleProbe := &workloads.RoleProbe{
		InitialDelaySeconds: synthesizeComp.LifecycleActions.RoleProbe.InitialDelaySeconds,
		TimeoutSeconds:      synthesizeComp.LifecycleActions.RoleProbe.TimeoutSeconds,
		PeriodSeconds:       synthesizeComp.LifecycleActions.RoleProbe.PeriodSeconds,
		SuccessThreshold:    synthesizeComp.LifecycleActions.RoleProbe.SuccessThreshold,
		FailureThreshold:    synthesizeComp.LifecycleActions.RoleProbe.FailureThreshold,
	}

	if synthesizeComp.LifecycleActions.RoleProbe.BuiltinHandler != nil {
		rsmRoleProbe.BuiltinHandler = synthesizeComp.LifecycleActions.RoleProbe.BuiltinHandler
	}

	// TODO(xingran): RSM Action does not support args[] yet
	if synthesizeComp.LifecycleActions.RoleProbe.CustomHandler != nil {
		rsmRoleProbeCmdAction := workloads.Action{
			Image:   synthesizeComp.LifecycleActions.RoleProbe.CustomHandler.Image,
			Command: synthesizeComp.LifecycleActions.RoleProbe.CustomHandler.Exec.Command,
		}
		rsmRoleProbe.CustomHandler = []workloads.Action{rsmRoleProbeCmdAction}
	}

	return rsmRoleProbe, nil
}

func (c *rsmCredentialConvertor) convert(args ...any) (any, error) {
	cluster, _, err := parseRSMConvertorArgs(args...)
	if err != nil {
		return nil, err
	}

	secretName := constant.GenerateDefaultConnCredential(cluster.Name)
	credential := &workloads.Credential{
		Username: workloads.CredentialVar{
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
					Key: constant.AccountNameForSecret,
				},
			},
		},
		Password: workloads.CredentialVar{
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
					Key: constant.AccountPasswdForSecret,
				},
			},
		},
	}

	return credential, nil
}

func (c *rsmMembershipReconfigurationConvertor) convert(args ...any) (any, error) {
	// cluster, synthesizeComp, err := parseRSMConvertorArgs(args...)
	return "", nil // TODO
}

func (c *rsmMemberUpdateStrategyConvertor) convert(args ...any) (any, error) {
	// cluster, synthesizeComp, err := parseRSMConvertorArgs(args...)
	return "", nil // TODO
}
