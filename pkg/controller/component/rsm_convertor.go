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
func BuildRSMFrom(synthesizeComp *SynthesizedComponent, protoRSM *workloads.InstanceSet) (*workloads.InstanceSet, error) {
	if synthesizeComp == nil {
		return nil, nil
	}
	if protoRSM == nil {
		protoRSM = &workloads.InstanceSet{}
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
		"instances":                 &rsmInstancesConvertor{},
		"offlineinstances":          &rsmOfflineInstancesConvertor{},
	}
	if err := covertObject(convertors, &protoRSM.Spec, synthesizeComp); err != nil {
		return nil, err
	}
	return protoRSM, nil
}

// rsmServiceConvertor is an implementation of the convertor interface, used to convert the given object into InstanceSet.Spec.Service.
type rsmServiceConvertor struct{}

// rsmAlternativeServicesConvertor is an implementation of the convertor interface, used to convert the given object into InstanceSet.Spec.AlternativeServices.
type rsmAlternativeServicesConvertor struct{}

// rsmRolesConvertor is an implementation of the convertor interface, used to convert the given object into InstanceSet.Spec.Roles.
type rsmRolesConvertor struct{}

// rsmRoleProbeConvertor is an implementation of the convertor interface, used to convert the given object into InstanceSet.Spec.RoleProbe.
type rsmRoleProbeConvertor struct{}

// rsmCredentialConvertor is an implementation of the convertor interface, used to convert the given object into InstanceSet.Spec.Credential.
type rsmCredentialConvertor struct{}

// rsmMembershipReconfigurationConvertor is an implementation of the convertor interface, used to convert the given object into InstanceSet.Spec.MembershipReconfiguration.
type rsmMembershipReconfigurationConvertor struct{}

// rsmMemberUpdateStrategyConvertor is an implementation of the convertor interface, used to convert the given object into InstanceSet.Spec.MemberUpdateStrategy.
type rsmMemberUpdateStrategyConvertor struct{}

func (c *rsmMemberUpdateStrategyConvertor) convert(args ...any) (any, error) {
	synthesizeComp, err := parseRSMConvertorArgs(args...)
	if err != nil {
		return nil, err
	}
	return getMemberUpdateStrategy(synthesizeComp), nil
}

// rsmPodManagementPolicyConvertor is an implementation of the convertor interface, used to convert the given object into InstanceSet.Spec.PodManagementPolicy.
type rsmPodManagementPolicyConvertor struct{}

func (c *rsmPodManagementPolicyConvertor) convert(args ...any) (any, error) {
	synthesizedComp, err := parseRSMConvertorArgs(args...)
	if err != nil {
		return nil, err
	}
	if synthesizedComp.PodManagementPolicy != nil {
		return *synthesizedComp.PodManagementPolicy, nil
	}
	memberUpdateStrategy := getMemberUpdateStrategy(synthesizedComp)
	if memberUpdateStrategy == nil || *memberUpdateStrategy == workloads.SerialUpdateStrategy {
		return appsv1.OrderedReadyPodManagement, nil
	}
	return appsv1.ParallelPodManagement, nil
}

// rsmUpdateStrategyConvertor is an implementation of the convertor interface, used to convert the given object into InstanceSet.Spec.Instances.
type rsmUpdateStrategyConvertor struct{}

func (c *rsmUpdateStrategyConvertor) convert(args ...any) (any, error) {
	synthesizedComp, err := parseRSMConvertorArgs(args...)
	if err != nil {
		return nil, err
	}
	if getMemberUpdateStrategy(synthesizedComp) != nil {
		// appsv1.OnDeleteStatefulSetStrategyType is the default value if member update strategy is set.
		return appsv1.StatefulSetUpdateStrategy{}, nil
	}
	return nil, nil
}

// rsmInstancesConvertor converts component instanceTemplate to rsm instanceTemplate
type rsmInstancesConvertor struct{}

func (c *rsmInstancesConvertor) convert(args ...any) (any, error) {
	synthesizedComp, err := parseRSMConvertorArgs(args...)
	if err != nil {
		return nil, err
	}

	var instances []workloads.InstanceTemplate
	for _, instance := range synthesizedComp.Instances {
		instances = append(instances, *AppsInstanceToWorkloadInstance(&instance))
	}
	return instances, nil
}

// rsmOfflineInstancesConvertor converts component offlineInstances to rsm offlineInstances
type rsmOfflineInstancesConvertor struct{}

func (c *rsmOfflineInstancesConvertor) convert(args ...any) (any, error) {
	synthesizedComp, err := parseRSMConvertorArgs(args...)
	if err != nil {
		return nil, err
	}

	var offlineInstances []string
	offlineInstances = append(offlineInstances, synthesizedComp.OfflineInstances...)
	return offlineInstances, nil
}

func AppsInstanceToWorkloadInstance(instance *appsv1alpha1.InstanceTemplate) *workloads.InstanceTemplate {
	if instance == nil {
		return nil
	}
	return &workloads.InstanceTemplate{
		Name:                 instance.Name,
		Replicas:             instance.Replicas,
		Annotations:          instance.Annotations,
		Labels:               instance.Labels,
		Image:                instance.Image,
		NodeName:             instance.NodeName,
		NodeSelector:         instance.NodeSelector,
		Tolerations:          instance.Tolerations,
		Resources:            instance.Resources,
		Env:                  instance.Env,
		Volumes:              instance.Volumes,
		VolumeMounts:         instance.VolumeMounts,
		VolumeClaimTemplates: instance.VolumeClaimTemplates,
	}
}

// parseRSMConvertorArgs parses the args of rsm convertor.
func parseRSMConvertorArgs(args ...any) (*SynthesizedComponent, error) {
	synthesizeComp, ok := args[0].(*SynthesizedComponent)
	if !ok {
		return nil, errors.New("args[0] not a SynthesizedComponent object")
	}
	return synthesizeComp, nil
}

func getMemberUpdateStrategy(synthesizedComp *SynthesizedComponent) *workloads.MemberUpdateStrategy {
	if synthesizedComp.UpdateStrategy == nil {
		return nil
	}
	var (
		serial                   = workloads.SerialUpdateStrategy
		parallelUpdate           = workloads.ParallelUpdateStrategy
		bestEffortParallelUpdate = workloads.BestEffortParallelUpdateStrategy
	)
	switch *synthesizedComp.UpdateStrategy {
	case appsv1alpha1.SerialStrategy:
		return &serial
	case appsv1alpha1.ParallelStrategy:
		return &parallelUpdate
	case appsv1alpha1.BestEffortParallelStrategy:
		return &bestEffortParallelUpdate
	default:
		return nil
	}
}

// rsmServiceConvertor converts the given object into InstanceSet.Spec.Service.
// TODO(xingran): ComponentServices are not consistent with InstanceSet.Spec.Service, If it is based on the new ComponentDefinition API,
// the services is temporarily handled in the component controller, and the corresponding InstanceSet.Spec.Service is temporarily set nil.
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

		// TODO(xingran): ComponentService.Name and ComponentService.RoleSelector are not used in InstanceSet.Spec.Service
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

// rsmAlternativeServicesConvertor converts the given object into InstanceSet.Spec.AlternativeServices.
// TODO: ComponentServices are not consistent with InstanceSet.Spec.AlternativeServices, If it is based on the new ComponentDefinition API,
// the services is temporarily handled in the component controller, and the corresponding InstanceSet.Spec.AlternativeServices is temporarily set nil.
func (c *rsmAlternativeServicesConvertor) convert(args ...any) (any, error) {
	return nil, nil
}

// rsmRolesConvertor converts the ComponentDefinition.Spec.Roles into InstanceSet.Spec.Roles.
func (c *rsmRolesConvertor) convert(args ...any) (any, error) {
	synthesizeComp, err := parseRSMConvertorArgs(args...)
	if err != nil {
		return nil, err
	}
	return ConvertSynthesizeCompRoleToRSMRole(synthesizeComp), nil
}

// rsmRoleProbeConvertor converts the ComponentDefinition.Spec.LifecycleActions.RoleProbe into InstanceSet.Spec.RoleProbe.
func (c *rsmRoleProbeConvertor) convert(args ...any) (any, error) {
	synthesizeComp, err := parseRSMConvertorArgs(args...)
	if err != nil {
		return nil, err
	}

	if synthesizeComp.LifecycleActions == nil || synthesizeComp.LifecycleActions.RoleProbe == nil {
		return nil, nil
	}

	rsmRoleProbe := &workloads.RoleProbe{
		TimeoutSeconds:      synthesizeComp.LifecycleActions.RoleProbe.TimeoutSeconds,
		PeriodSeconds:       synthesizeComp.LifecycleActions.RoleProbe.PeriodSeconds,
		SuccessThreshold:    1,
		FailureThreshold:    2,
		RoleUpdateMechanism: workloads.DirectAPIServerEventUpdate,
	}

	if synthesizeComp.LifecycleActions.RoleProbe.BuiltinHandler != nil {
		builtinHandler := string(*synthesizeComp.LifecycleActions.RoleProbe.BuiltinHandler)
		rsmRoleProbe.BuiltinHandler = &builtinHandler
		return rsmRoleProbe, nil
	}

	// TODO(xingran): RSM Action does not support args[] yet
	if synthesizeComp.LifecycleActions.RoleProbe.CustomHandler != nil {
		rsmRoleProbeCmdAction := workloads.Action{
			Image:   synthesizeComp.LifecycleActions.RoleProbe.CustomHandler.Image,
			Command: synthesizeComp.LifecycleActions.RoleProbe.CustomHandler.Exec.Command,
			Args:    synthesizeComp.LifecycleActions.RoleProbe.CustomHandler.Exec.Args,
		}
		rsmRoleProbe.CustomHandler = []workloads.Action{rsmRoleProbeCmdAction}
	}

	return rsmRoleProbe, nil
}

func (c *rsmCredentialConvertor) convert(args ...any) (any, error) {
	synthesizeComp, err := parseRSMConvertorArgs(args...)
	if err != nil {
		return nil, err
	}

	// use the system init account as the default credential
	var sysInitAccount *appsv1alpha1.SystemAccount
	for index, sysAccount := range synthesizeComp.SystemAccounts {
		if sysAccount.InitAccount {
			sysInitAccount = &synthesizeComp.SystemAccounts[index]
			break
		}
	}
	if sysInitAccount == nil && len(synthesizeComp.CompDefName) != 0 {
		return nil, nil
	}

	var secretName string
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
