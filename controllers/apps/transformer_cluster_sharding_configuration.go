/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package apps

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// clusterServiceTransformer handles cluster services.
type clusterShardingConfigurationTransformer struct {
	client.Client
}

var _ graph.Transformer = &clusterShardingConfigurationTransformer{}

func (c *clusterShardingConfigurationTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	cluster := transCtx.Cluster
	if model.IsObjectDeleting(transCtx.OrigCluster) {
		return nil
	}

	if len(cluster.Spec.ShardingSpecs) == 0 {
		return nil
	}
	return c.createOrUpdateConfiguration(cluster, cluster.Spec.ShardingSpecs)
}

func (c *clusterShardingConfigurationTransformer) createOrUpdateConfiguration(cluster *appsv1alpha1.Cluster, shardingComponents []appsv1alpha1.ShardingSpec) error {
	var configs []appsv1alpha1.Configuration
	for _, shardingComponent := range shardingComponents {
		if len(shardingComponent.Template.ComponentConfigItems) != 0 {
			configs = append(configs, buildShardingConfigurations(cluster.GetName(), cluster.GetNamespace(), shardingComponent)...)
		}
	}

}

func buildShardingConfigurations(name, ns string, shardingComponent appsv1alpha1.ShardingSpec) []appsv1alpha1.Configuration {
	var configs []appsv1alpha1.Configuration
	for i := int32(0); i < shardingComponent.Shards; i++ {
		config := builder.NewConfigurationBuilder(ns,
			core.GenerateComponentConfigurationName(name,
				p.ComponentName))
		configs = append(configs, buildConfiguration(shardingComponent, i, ns))
	}
	return configs
}

func buildConfiguration(shardingComponent appsv1alpha1.ShardingSpec, index int32, ns string) appsv1alpha1.Configuration {
}
