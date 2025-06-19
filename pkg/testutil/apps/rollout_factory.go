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
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type MockRolloutFactory struct {
	BaseFactory[appsv1alpha1.Rollout, *appsv1alpha1.Rollout, MockRolloutFactory]
}

func NewRolloutFactory(namespace, name string) *MockRolloutFactory {
	f := &MockRolloutFactory{}
	f.Init(namespace, name, &appsv1alpha1.Rollout{}, f)
	return f
}

func (factory *MockRolloutFactory) SetClusterName(clusterName string) *MockRolloutFactory {
	factory.Get().Spec.ClusterName = clusterName
	return factory
}

func (factory *MockRolloutFactory) AddComponent(compName string) *MockRolloutFactory {
	comp := appsv1alpha1.RolloutComponent{
		Name: compName,
	}
	factory.Get().Spec.Components = append(factory.Get().Spec.Components, comp)
	return factory
}

type updateRollComponentFn func(*appsv1alpha1.RolloutComponent)

func (factory *MockRolloutFactory) updateLastComponent(update updateRollComponentFn) *MockRolloutFactory {
	comps := factory.Get().Spec.Components
	if len(comps) > 0 {
		update(&comps[len(comps)-1])
	}
	factory.Get().Spec.Components = comps
	return factory
}

func (factory *MockRolloutFactory) SetServiceVersion(serviceVersion string) *MockRolloutFactory {
	return factory.updateLastComponent(func(comp *appsv1alpha1.RolloutComponent) {
		comp.ServiceVersion = serviceVersion
	})
}

func (factory *MockRolloutFactory) SetCompDef(compDef string) *MockRolloutFactory {
	return factory.updateLastComponent(func(comp *appsv1alpha1.RolloutComponent) {
		comp.CompDef = compDef
	})
}

func (factory *MockRolloutFactory) SetStrategy(strategy appsv1alpha1.RolloutStrategy) *MockRolloutFactory {
	return factory.updateLastComponent(func(comp *appsv1alpha1.RolloutComponent) {
		comp.Strategy = strategy
	})
}

func (factory *MockRolloutFactory) SetInplaceStrategy() *MockRolloutFactory {
	return factory.updateLastComponent(func(comp *appsv1alpha1.RolloutComponent) {
		comp.Strategy.Inplace = &appsv1alpha1.RolloutStrategyInplace{}
	})
}

func (factory *MockRolloutFactory) SetReplaceStrategy() *MockRolloutFactory {
	return factory.updateLastComponent(func(comp *appsv1alpha1.RolloutComponent) {
		comp.Strategy.Replace = &appsv1alpha1.RolloutStrategyReplace{}
	})
}

func (factory *MockRolloutFactory) SetCreateStrategy() *MockRolloutFactory {
	return factory.updateLastComponent(func(comp *appsv1alpha1.RolloutComponent) {
		comp.Strategy.Create = &appsv1alpha1.RolloutStrategyCreate{}
	})
}

func (factory *MockRolloutFactory) SetReplicas(replicas int32) *MockRolloutFactory {
	return factory.updateLastComponent(func(comp *appsv1alpha1.RolloutComponent) {
		comp.Replicas = ptr.To(intstr.FromInt32(replicas))
	})
}
