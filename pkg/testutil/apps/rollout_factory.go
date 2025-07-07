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

func (factory *MockRolloutFactory) AddSharding(shardingName string) *MockRolloutFactory {
	sharding := appsv1alpha1.RolloutSharding{
		Name: shardingName,
	}
	factory.Get().Spec.Shardings = append(factory.Get().Spec.Shardings, sharding)
	return factory
}

type updateRolloutComponentFn func(*appsv1alpha1.RolloutComponent)

func (factory *MockRolloutFactory) updateLastComponent(update updateRolloutComponentFn) *MockRolloutFactory {
	comps := factory.Get().Spec.Components
	if len(comps) > 0 {
		update(&comps[len(comps)-1])
	}
	factory.Get().Spec.Components = comps
	return factory
}

func (factory *MockRolloutFactory) SetCompServiceVersion(serviceVersion string) *MockRolloutFactory {
	return factory.updateLastComponent(func(comp *appsv1alpha1.RolloutComponent) {
		comp.ServiceVersion = ptr.To(serviceVersion)
	})
}

func (factory *MockRolloutFactory) SetCompCompDef(compDef string) *MockRolloutFactory {
	return factory.updateLastComponent(func(comp *appsv1alpha1.RolloutComponent) {
		comp.CompDef = ptr.To(compDef)
	})
}

func (factory *MockRolloutFactory) SetCompStrategy(strategy appsv1alpha1.RolloutStrategy) *MockRolloutFactory {
	return factory.updateLastComponent(func(comp *appsv1alpha1.RolloutComponent) {
		comp.Strategy = strategy
	})
}

func (factory *MockRolloutFactory) SetCompReplicas(replicas int32) *MockRolloutFactory {
	return factory.updateLastComponent(func(comp *appsv1alpha1.RolloutComponent) {
		comp.Replicas = ptr.To(intstr.FromInt32(replicas))
	})
}

type updateRolloutShardingFn func(*appsv1alpha1.RolloutSharding)

func (factory *MockRolloutFactory) updateLastSharding(update updateRolloutShardingFn) *MockRolloutFactory {
	shardings := factory.Get().Spec.Shardings
	if len(shardings) > 0 {
		update(&shardings[len(shardings)-1])
	}
	factory.Get().Spec.Shardings = shardings
	return factory
}

func (factory *MockRolloutFactory) SetShardingDef(shardingDef string) *MockRolloutFactory {
	return factory.updateLastSharding(func(sharding *appsv1alpha1.RolloutSharding) {
		sharding.ShardingDef = ptr.To(shardingDef)
	})
}

func (factory *MockRolloutFactory) SetShardingServiceVersion(serviceVersion string) *MockRolloutFactory {
	return factory.updateLastSharding(func(sharding *appsv1alpha1.RolloutSharding) {
		sharding.ServiceVersion = ptr.To(serviceVersion)
	})
}

func (factory *MockRolloutFactory) SetShardingCompDef(compDef string) *MockRolloutFactory {
	return factory.updateLastSharding(func(sharding *appsv1alpha1.RolloutSharding) {
		sharding.CompDef = ptr.To(compDef)
	})
}

func (factory *MockRolloutFactory) SetShardingStrategy(strategy appsv1alpha1.RolloutStrategy) *MockRolloutFactory {
	return factory.updateLastSharding(func(sharding *appsv1alpha1.RolloutSharding) {
		sharding.Strategy = strategy
	})
}
