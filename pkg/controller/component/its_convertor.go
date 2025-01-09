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

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
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
		"roles":                &itsRolesConvertor{},
		"credential":           &itsCredentialConvertor{},
		"memberupdatestrategy": &itsMemberUpdateStrategyConvertor{},
		"podmanagementpolicy":  &itsPodManagementPolicyConvertor{},
		"updatestrategy":       &itsUpdateStrategyConvertor{},
	}
	if err := covertObject(convertors, &protoITS.Spec, synthesizeComp); err != nil {
		return nil, err
	}
	return protoITS, nil
}

// itsRolesConvertor is an implementation of the convertor interface, used to convert the given object into InstanceSet.Spec.Roles.
type itsRolesConvertor struct{}

// itsRolesConvertor converts the ComponentDefinition.Spec.Roles into InstanceSet.Spec.Roles.
func (c *itsRolesConvertor) convert(args ...any) (any, error) {
	synthesizeComp, err := parseITSConvertorArgs(args...)
	if err != nil {
		return nil, err
	}
	return synthesizeComp.Roles, nil
}

// itsCredentialConvertor is an implementation of the convertor interface, used to convert the given object into InstanceSet.Spec.Credential.
type itsCredentialConvertor struct{}

func (c *itsCredentialConvertor) convert(args ...any) (any, error) {
	synthesizeComp, err := parseITSConvertorArgs(args...)
	if err != nil {
		return nil, err
	}

	credential := func(sysAccount kbappsv1.SystemAccount) *workloads.Credential {
		secretName := constant.GenerateAccountSecretName(synthesizeComp.ClusterName, synthesizeComp.Name, sysAccount.Name)
		return &workloads.Credential{
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
	}

	// use first init account as the default credential
	for index, sysAccount := range synthesizeComp.SystemAccounts {
		if sysAccount.InitAccount {
			return credential(synthesizeComp.SystemAccounts[index]), nil
		}
	}
	return nil, nil
}

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
	case kbappsv1.SerialStrategy:
		return &serial
	case kbappsv1.ParallelStrategy:
		return &parallelUpdate
	case kbappsv1.BestEffortParallelStrategy:
		return &bestEffortParallelUpdate
	default:
		return nil
	}
}
