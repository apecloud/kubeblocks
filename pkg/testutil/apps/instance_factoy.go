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

package apps

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

type MockInstanceFactory struct {
	BaseFactory[workloads.Instance, *workloads.Instance, MockInstanceFactory]
}

func NewInstanceFactory(namespace, name string) *MockInstanceFactory {
	f := &MockInstanceFactory{}
	f.Init(namespace, name,
		&workloads.Instance{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					constant.AppManagedByLabelKey: constant.AppName,
				},
			},
			Spec: workloads.InstanceSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						constant.AppManagedByLabelKey:      constant.AppName,
						constant.KBAppInstanceNameLabelKey: name,
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							constant.AppManagedByLabelKey:      constant.AppName,
							constant.KBAppInstanceNameLabelKey: name,
						},
					},
				},
			},
		}, f)
	return f
}

func (factory *MockInstanceFactory) AddContainer(container corev1.Container) *MockInstanceFactory {
	containers := &factory.Get().Spec.Template.Spec.Containers
	*containers = append(*containers, container)
	return factory
}

func (factory *MockInstanceFactory) AddVolume(volume corev1.Volume) *MockInstanceFactory {
	volumes := &factory.Get().Spec.Template.Spec.Volumes
	*volumes = append(*volumes, volume)
	return factory
}

func (factory *MockInstanceFactory) SetMinReadySeconds(seconds int32) *MockInstanceFactory {
	factory.Get().Spec.MinReadySeconds = seconds
	return factory
}

func (factory *MockInstanceFactory) AddVolumeClaimTemplate(pvc corev1.PersistentVolumeClaim) *MockInstanceFactory {
	templates := &factory.Get().Spec.VolumeClaimTemplates
	*templates = append(*templates, corev1.PersistentVolumeClaimTemplate{
		ObjectMeta: pvc.ObjectMeta,
		Spec:       pvc.Spec,
	})
	return factory
}

func (factory *MockInstanceFactory) SetPVCRetentionPolicy(retentionPolicy *workloads.PersistentVolumeClaimRetentionPolicy) *MockInstanceFactory {
	factory.Get().Spec.PersistentVolumeClaimRetentionPolicy = retentionPolicy
	return factory
}

func (factory *MockInstanceFactory) SetInstanceSetName(itsName string) *MockInstanceFactory {
	factory.Get().Spec.InstanceSetName = itsName
	return factory
}

func (factory *MockInstanceFactory) SetInstanceUpdateStrategyType(instanceUpdateStrategyType *kbappsv1.InstanceUpdateStrategyType) *MockInstanceFactory {
	factory.Get().Spec.InstanceUpdateStrategyType = instanceUpdateStrategyType
	return factory
}

func (factory *MockInstanceFactory) SetPodUpdatePolicy(podUpdatePolicyType workloads.PodUpdatePolicyType) *MockInstanceFactory {
	factory.Get().Spec.PodUpdatePolicy = podUpdatePolicyType
	return factory
}

func (factory *MockInstanceFactory) SetRoles(roles []workloads.ReplicaRole) *MockInstanceFactory {
	factory.Get().Spec.Roles = roles
	return factory
}

func (factory *MockInstanceFactory) SetMembershipActions(actions *workloads.MembershipReconfiguration) *MockInstanceFactory {
	factory.Get().Spec.MembershipReconfiguration = actions
	return factory
}
