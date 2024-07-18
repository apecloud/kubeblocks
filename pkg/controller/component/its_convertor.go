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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

// BuildWorkloadFrom builds a new Component object based on SynthesizedComponent.
func BuildWorkloadFrom(synthesizeComp *SynthesizedComponent, protoITS *workloads.InstanceSet) (*workloads.InstanceSet, error) {
	if synthesizeComp == nil {
		return nil, nil
	}
	if protoITS == nil {
		protoITS = &workloads.InstanceSet{}
	}
	convertors := map[string]convertor{
		"service":                   &itsServiceConvertor{},
		"alternativeservices":       &itsAlternativeServicesConvertor{},
		"roles":                     &itsRolesConvertor{},
		"roleprobe":                 &itsRoleProbeConvertor{},
		"credential":                &itsCredentialConvertor{},
		"membershipreconfiguration": &itsMembershipReconfigurationConvertor{},
		"memberupdatestrategy":      &itsMemberUpdateStrategyConvertor{},
		"podmanagementpolicy":       &itsPodManagementPolicyConvertor{},
		"podupdatepolicy":           &itsPodUpdatePolicyConvertor{},
		"updatestrategy":            &itsUpdateStrategyConvertor{},
		"instances":                 &itsInstancesConvertor{},
		"offlineinstances":          &itsOfflineInstancesConvertor{},
	}
	if err := covertObject(convertors, &protoITS.Spec, synthesizeComp); err != nil {
		return nil, err
	}
	return protoITS, nil
}

// itsServiceConvertor is an implementation of the convertor interface, used to convert the given object into InstanceSet.Spec.Service.
type itsServiceConvertor struct{}

// itsAlternativeServicesConvertor is an implementation of the convertor interface, used to convert the given object into InstanceSet.Spec.AlternativeServices.
type itsAlternativeServicesConvertor struct{}

// itsRolesConvertor is an implementation of the convertor interface, used to convert the given object into InstanceSet.Spec.Roles.
type itsRolesConvertor struct{}

// itsRoleProbeConvertor is an implementation of the convertor interface, used to convert the given object into InstanceSet.Spec.RoleProbe.
type itsRoleProbeConvertor struct{}

// itsCredentialConvertor is an implementation of the convertor interface, used to convert the given object into InstanceSet.Spec.Credential.
type itsCredentialConvertor struct{}

// itsMembershipReconfigurationConvertor is an implementation of the convertor interface, used to convert the given object into InstanceSet.Spec.MembershipReconfiguration.
type itsMembershipReconfigurationConvertor struct{}

// itsMemberUpdateStrategyConvertor is an implementation of the convertor interface, used to convert the given object into InstanceSet.Spec.MemberUpdateStrategy.
type itsMemberUpdateStrategyConvertor struct{}

func (c *itsMemberUpdateStrategyConvertor) convert(args ...any) (any, error) {
	synthesizeComp, err := parseITSConvertorArgs(args...)
	if err != nil {
		return nil, err
	}
	return getMemberUpdateStrategy(synthesizeComp), nil
}

// itsPodManagementPolicyConvertor is an implementation of the convertor interface, used to convert the given object into InstanceSet.Spec.PodManagementPolicy.
type itsPodManagementPolicyConvertor struct{}

func (c *itsPodManagementPolicyConvertor) convert(args ...any) (any, error) {
	synthesizedComp, err := parseITSConvertorArgs(args...)
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

// itsPodUpdatePolicyConvertor is an implementation of the convertor interface, used to convert the given object into InstanceSet.Spec.PodUpdatePolicy.
type itsPodUpdatePolicyConvertor struct{}

func (c *itsPodUpdatePolicyConvertor) convert(args ...any) (any, error) {
	synthesizedComp, err := parseITSConvertorArgs(args...)
	if err != nil {
		return nil, err
	}
	if synthesizedComp.PodUpdatePolicy != nil {
		return *synthesizedComp.PodUpdatePolicy, nil
	}
	return workloads.PreferInPlacePodUpdatePolicyType, nil
}

// itsUpdateStrategyConvertor is an implementation of the convertor interface, used to convert the given object into InstanceSet.Spec.Instances.
type itsUpdateStrategyConvertor struct{}

func (c *itsUpdateStrategyConvertor) convert(args ...any) (any, error) {
	synthesizedComp, err := parseITSConvertorArgs(args...)
	if err != nil {
		return nil, err
	}
	if getMemberUpdateStrategy(synthesizedComp) != nil {
		// appsv1.OnDeleteStatefulSetStrategyType is the default value if member update strategy is set.
		return appsv1.StatefulSetUpdateStrategy{}, nil
	}
	return nil, nil
}

// itsInstancesConvertor converts component instanceTemplate to ITS instanceTemplate
type itsInstancesConvertor struct{}

func (c *itsInstancesConvertor) convert(args ...any) (any, error) {
	synthesizedComp, err := parseITSConvertorArgs(args...)
	if err != nil {
		return nil, err
	}

	var instances []workloads.InstanceTemplate
	for _, instance := range synthesizedComp.Instances {
		instances = append(instances, *AppsInstanceToWorkloadInstance(&instance))
	}
	return instances, nil
}

// itsOfflineInstancesConvertor converts component offlineInstances to ITS offlineInstances
type itsOfflineInstancesConvertor struct{}

func (c *itsOfflineInstancesConvertor) convert(args ...any) (any, error) {
	synthesizedComp, err := parseITSConvertorArgs(args...)
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
	var schedulingPolicy *workloads.SchedulingPolicy
	if instance.SchedulingPolicy != nil {
		schedulingPolicy = &workloads.SchedulingPolicy{
			SchedulerName:             instance.SchedulingPolicy.SchedulerName,
			NodeSelector:              instance.SchedulingPolicy.NodeSelector,
			NodeName:                  instance.SchedulingPolicy.NodeName,
			Affinity:                  instance.SchedulingPolicy.Affinity,
			Tolerations:               instance.SchedulingPolicy.Tolerations,
			TopologySpreadConstraints: instance.SchedulingPolicy.TopologySpreadConstraints,
		}
	}

	return &workloads.InstanceTemplate{
		Name:                 instance.Name,
		Replicas:             instance.Replicas,
		Annotations:          instance.Annotations,
		Labels:               instance.Labels,
		Image:                instance.Image,
		SchedulingPolicy:     schedulingPolicy,
		Resources:            instance.Resources,
		Env:                  instance.Env,
		Volumes:              instance.Volumes,
		VolumeMounts:         instance.VolumeMounts,
		VolumeClaimTemplates: toPersistentVolumeClaims(instance.VolumeClaimTemplates),
	}
}

func toPersistentVolumeClaims(vcts []appsv1alpha1.ClusterComponentVolumeClaimTemplate) []corev1.PersistentVolumeClaim {
	var pvcs []corev1.PersistentVolumeClaim
	for _, v := range vcts {
		pvcs = append(pvcs, corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: v.Name,
			},
			Spec: v.Spec.ToV1PersistentVolumeClaimSpec(),
		})
	}
	return pvcs
}

// parseITSConvertorArgs parses the args of ITS convertor.
func parseITSConvertorArgs(args ...any) (*SynthesizedComponent, error) {
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

// itsServiceConvertor converts the given object into InstanceSet.Spec.Service.
func (c *itsServiceConvertor) convert(args ...any) (any, error) {
	return nil, nil
}

// itsAlternativeServicesConvertor converts the given object into InstanceSet.Spec.AlternativeServices.
// TODO: ComponentServices are not consistent with InstanceSet.Spec.AlternativeServices, If it is based on the new ComponentDefinition API,
// the services is temporarily handled in the component controller, and the corresponding InstanceSet.Spec.AlternativeServices is temporarily set nil.
func (c *itsAlternativeServicesConvertor) convert(args ...any) (any, error) {
	return nil, nil
}

// itsRolesConvertor converts the ComponentDefinition.Spec.Roles into InstanceSet.Spec.Roles.
func (c *itsRolesConvertor) convert(args ...any) (any, error) {
	synthesizeComp, err := parseITSConvertorArgs(args...)
	if err != nil {
		return nil, err
	}
	return ConvertSynthesizeCompRoleToInstanceSetRole(synthesizeComp), nil
}

// itsRoleProbeConvertor converts the ComponentDefinition.Spec.LifecycleActions.RoleProbe into InstanceSet.Spec.RoleProbe.
func (c *itsRoleProbeConvertor) convert(args ...any) (any, error) {
	return nil, nil
	// synthesizeComp, err := parseITSConvertorArgs(args...)
	// if err != nil {
	//	return nil, err
	//}
	//
	// if synthesizeComp.LifecycleActions == nil || synthesizeComp.LifecycleActions.RoleProbe == nil {
	//	return nil, nil
	// }
	//
	// itsRoleProbe := &workloads.RoleProbe{
	//	TimeoutSeconds:      synthesizeComp.LifecycleActions.RoleProbe.TimeoutSeconds,
	//	PeriodSeconds:       synthesizeComp.LifecycleActions.RoleProbe.PeriodSeconds,
	//	SuccessThreshold:    1,
	//	FailureThreshold:    2,
	//	RoleUpdateMechanism: workloads.DirectAPIServerEventUpdate,
	// }
	//
	// if synthesizeComp.LifecycleActions.RoleProbe.BuiltinHandler != nil {
	//	builtinHandler := string(*synthesizeComp.LifecycleActions.RoleProbe.BuiltinHandler)
	//	itsRoleProbe.BuiltinHandler = &builtinHandler
	//	return itsRoleProbe, nil
	// }
	//
	//// TODO(xingran): ITS Action does not support args[] yet
	// if synthesizeComp.LifecycleActions.RoleProbe.Exec != nil {
	//	itsRoleProbeCmdAction := workloads.Action{
	//		Image:   synthesizeComp.LifecycleActions.RoleProbe.Exec.Image,
	//		Command: synthesizeComp.LifecycleActions.RoleProbe.Exec.Command,
	//		Args:    synthesizeComp.LifecycleActions.RoleProbe.Exec.Args,
	//	}
	//	itsRoleProbe.CustomHandler = []workloads.Action{itsRoleProbeCmdAction}
	// }
	//
	// return itsRoleProbe, nil
}

func (c *itsCredentialConvertor) convert(args ...any) (any, error) {
	synthesizeComp, err := parseITSConvertorArgs(args...)
	if err != nil {
		return nil, err
	}

	// use first init account as the default credential
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

func (c *itsMembershipReconfigurationConvertor) convert(args ...any) (any, error) {
	// synthesizeComp, err := parseITSConvertorArgs(args...)
	return "", nil // TODO
}

// ConvertSynthesizeCompRoleToInstanceSetRole converts the component.SynthesizedComponent.Roles to workloads.ReplicaRole.
func ConvertSynthesizeCompRoleToInstanceSetRole(synthesizedComp *SynthesizedComponent) []workloads.ReplicaRole {
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
	itsReplicaRoles := make([]workloads.ReplicaRole, 0)
	for _, role := range synthesizedComp.Roles {
		itsReplicaRole := workloads.ReplicaRole{
			Name:       role.Name,
			AccessMode: accessMode(role),
			CanVote:    role.Votable,
			// HACK: Since the InstanceSet relies on IsLeader field to determine whether a workload is available, we are using
			// such a workaround to combine these two fields to provide the information.
			// However, the condition will be broken if a service with multiple different roles that can be writable
			// at the same time, such as Zookeeper.
			// TODO: We need to discuss further whether we should rely on the concept of "Leader" in the case
			//  where the KB controller does not provide HA functionality.
			IsLeader: role.Serviceable && role.Writable,
		}
		itsReplicaRoles = append(itsReplicaRoles, itsReplicaRole)
	}
	return itsReplicaRoles
}
