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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

// BuildRSMFrom builds a new Component object based on SynthesizedComponent.
func BuildRSMFrom(synthesizeComp *SynthesizedComponent, protoRSM *workloads.ReplicatedStateMachine) (*workloads.ReplicatedStateMachine, error) {
	if synthesizeComp == nil {
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
		"podmanagementpolicy":       &rsmPodManagementPolicyConvertor{},
		"updatestrategy":            &rsmUpdateStrategyConvertor{},
	}
	if err := covertObject(convertors, &protoRSM.Spec, synthesizeComp); err != nil {
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

// rsmPodManagementPolicyConvertor is an implementation of the convertor interface, used to convert the given object into ReplicatedStateMachine.Spec.PodManagementPolicy.
type rsmPodManagementPolicyConvertor struct{}

// rsmUpdateStrategyConvertor is an implementation of the convertor interface, used to convert the given object into ReplicatedStateMachine.Spec.UpdateStrategy.
type rsmUpdateStrategyConvertor struct{}

// parseRSMConvertorArgs parses the args of rsm convertor.
func parseRSMConvertorArgs(args ...any) (*SynthesizedComponent, error) {
	synthesizeComp, ok := args[0].(*SynthesizedComponent)
	if !ok {
		return nil, errors.New("args[0] not a SynthesizedComponent object")
	}
	return synthesizeComp, nil
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
	synthesizeComp, err := parseRSMConvertorArgs(args...)
	if err != nil {
		return nil, err
	}
	return ConvertSynthesizeCompRoleToRSMRole(synthesizeComp), nil
}

// rsmRoleProbeConvertor converts the ComponentDefinition.Spec.LifecycleActions.RoleProbe into ReplicatedStateMachine.Spec.RoleProbe.
func (c *rsmRoleProbeConvertor) convert(args ...any) (any, error) {
	synthesizeComp, err := parseRSMConvertorArgs(args...)
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
		RoleUpdateMechanism: workloads.DirectAPIServerEventUpdate,
	}

	if synthesizeComp.LifecycleActions.RoleProbe.BuiltinHandler != nil {
		builtinHandler := string(*synthesizeComp.LifecycleActions.RoleProbe.BuiltinHandler)
		rsmRoleProbe.BuiltinHandler = &builtinHandler
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
	var (
		secretName     string
		sysInitAccount *appsv1alpha1.SystemAccount
	)

	synthesizeComp, err := parseRSMConvertorArgs(args...)
	if err != nil {
		return nil, err
	}

	// use the system init account as the default credential
	for index, sysAccount := range synthesizeComp.SystemAccounts {
		if sysAccount.InitAccount {
			sysInitAccount = &synthesizeComp.SystemAccounts[index]
			break
		}
	}
	if sysInitAccount != nil {
		secretName = constant.GenerateAccountSecretName(synthesizeComp.ClusterName, synthesizeComp.Name, sysInitAccount.Name)
	} else {
		secretName = constant.GenerateDefaultConnCredential(synthesizeComp.ClusterName)
	}
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
	// synthesizeComp, err := parseRSMConvertorArgs(args...)
	return "", nil // TODO
}

func (c *rsmMemberUpdateStrategyConvertor) convert(args ...any) (any, error) {
	synthesizeComp, err := parseRSMConvertorArgs(args...)
	if err != nil {
		return nil, err
	}
	var memberUpdateStrategy *workloads.MemberUpdateStrategy
	switch *synthesizeComp.UpdateStrategy {
	case appsv1alpha1.SerialStrategy:
		memberSerialUpdateStrategy := workloads.SerialUpdateStrategy
		memberUpdateStrategy = &memberSerialUpdateStrategy
	case appsv1alpha1.ParallelStrategy:
		memberParallelUpdateStrategy := workloads.ParallelUpdateStrategy
		memberUpdateStrategy = &memberParallelUpdateStrategy
	case appsv1alpha1.BestEffortParallelStrategy:
		memberBestEffortParallelUpdateStrategy := workloads.BestEffortParallelUpdateStrategy
		memberUpdateStrategy = &memberBestEffortParallelUpdateStrategy
	default:
		return nil, err
	}
	return memberUpdateStrategy, err
}

func (c *rsmPodManagementPolicyConvertor) convert(args ...any) (any, error) {
	// componentDefinition does not define PodManagementPolicy and StatefulSetUpdateStrategy.
	// The Parallel strategy is used by default here. If necessary, the componentDefinition API can be expanded later
	return appsv1.ParallelPodManagement, nil
}

func (c *rsmUpdateStrategyConvertor) convert(args ...any) (any, error) {
	// synthesizeComp, err := parseRSMConvertorArgs(args...)
	return "", nil // TODO
}

// ConvertSynthesizeCompRoleToRSMRole converts the component.SynthesizedComponent.Roles to workloads.ReplicaRole.
func ConvertSynthesizeCompRoleToRSMRole(synthesizedComp *SynthesizedComponent) []workloads.ReplicaRole {
	if synthesizedComp.Roles == nil {
		return nil
	}

	accessMode := func(role appsv1alpha1.ReplicaRole) workloads.AccessMode {
		switch {
		case role.Serviceable && role.Writable:
			return workloads.ReadWriteMode
		case role.Serviceable:
			return workloads.ReadonlyMode
		default:
			return workloads.NoneMode
		}
	}
	rsmReplicaRoles := make([]workloads.ReplicaRole, 0)
	for _, role := range synthesizedComp.Roles {
		rsmReplicaRole := workloads.ReplicaRole{
			Name:       role.Name,
			AccessMode: accessMode(role),
			CanVote:    role.Votable,
			// HACK: Since the RSM relies on IsLeader field to determine whether a workload is available, we are using
			// such a workaround to combine these two fields to provide the information.
			// However, the condition will be broken if a service with multiple different roles that can be writable
			// at the same time, such as Zookeeper.
			// TODO: We need to discuss further whether we should rely on the concept of "Leader" in the case
			//  where the KB controller does not provide HA functionality.
			IsLeader: role.Serviceable && role.Writable,
		}
		rsmReplicaRoles = append(rsmReplicaRoles, rsmReplicaRole)
	}
	return rsmReplicaRoles
}
