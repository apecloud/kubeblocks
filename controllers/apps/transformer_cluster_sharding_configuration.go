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
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/configuration"
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
	return c.reconcile(transCtx, cluster, dag)
}

func (c *clusterShardingConfigurationTransformer) reconcile(transCtx *clusterTransformContext, cluster *appsv1alpha1.Cluster, dag *graph.DAG) error {
	existingConfigMap, err := c.runningShardingConfigurations(transCtx)
	if err != nil {
		return err
	}
	expectedConfigMap, err := createShardingConfigurations(transCtx, cluster)
	if err != nil {
		return err
	}

	existingConfigSet := sets.KeySet(existingConfigMap)
	expectedConfigSet := sets.KeySet(expectedConfigMap)
	createSet := expectedConfigSet.Difference(existingConfigSet)
	updateSet := expectedConfigSet.Intersection(existingConfigSet)
	deleteSet := existingConfigSet.Difference(expectedConfigSet)

	graphCli, _ := transCtx.Client.(model.GraphClient)
	for configName := range createSet {
		graphCli.Create(dag, expectedConfigMap[configName], inDataContext4G())
	}

	for configName := range updateSet {
		graphCli.Patch(dag, existingConfigMap[configName],
			c.mergeConfiguration(expectedConfigMap[configName], existingConfigMap[configName]), inDataContext4G())
	}

	// Clean configurations that are not being used by the sharding component, e.g: shards -> 100 and shards --> 20
	for configName := range deleteSet {
		graphCli.Delete(dag, existingConfigMap[configName], inDataContext4G())
	}
	return nil
}

func (c *clusterShardingConfigurationTransformer) mergeConfiguration(expected *appsv1alpha1.Configuration, existing *appsv1alpha1.Configuration) *appsv1alpha1.Configuration {
	return configuration.MergeConfiguration(expected, existing, func(dest, expected *appsv1alpha1.ConfigurationItemDetail) {
		if len(expected.ConfigFileParams) != 0 {
			dest.ConfigFileParams = expected.ConfigFileParams
		}
	})
}

func createShardingConfigurations(transCtx *clusterTransformContext, cluster *appsv1alpha1.Cluster) (map[string]*appsv1alpha1.Configuration, error) {
	expectedObjects := make(map[string]*appsv1alpha1.Configuration)

	for shardingName, shardingComponent := range transCtx.ShardingComponentSpecs {
		if len(shardingComponent) != 0 {
			configs, err := buildShardingConfigurations(transCtx, cluster.GetName(), cluster.GetNamespace(), shardingComponent, shardingName)
			if err != nil {
				return nil, err
			}
			for _, config := range configs {
				expectedObjects[config.Name] = config
			}
		}
	}
	return expectedObjects, nil
}

func (c *clusterShardingConfigurationTransformer) runningShardingConfigurations(ctx *clusterTransformContext) (map[string]*appsv1alpha1.Configuration, error) {
	ns := ctx.Cluster.Namespace
	clusterName := ctx.Cluster.Name
	configMaps := make(map[string]*appsv1alpha1.Configuration)

	for shardingName := range ctx.ShardingComponentSpecs {
		objects, err := listShardingConfigurations(ctx, ctx.GetClient(), ns, clusterName, shardingName)
		if err != nil {
			return nil, err
		}
		for _, object := range objects {
			configMaps[object.Name] = object.DeepCopy()
		}
	}
	return configMaps, nil
}

func listShardingConfigurations(ctx context.Context, cli client.Reader, ns, clusterName, shardingName string) ([]appsv1alpha1.Configuration, error) {
	compList := &appsv1alpha1.ConfigurationList{}
	ml := constant.GetClusterShardingNameLabel(clusterName, shardingName)
	if err := cli.List(ctx, compList, client.InNamespace(ns), client.MatchingLabels(ml)); err != nil {
		return nil, err
	}
	return compList.Items, nil
}

func buildShardingConfigurations(transCtx *clusterTransformContext, clusterName, ns string, shardingComponents []*appsv1alpha1.ClusterComponentSpec, shardingName string) ([]*appsv1alpha1.Configuration, error) {
	var configs []*appsv1alpha1.Configuration
	for i := 0; i < len(shardingComponents); i++ {
		config, err := buildConfiguration(transCtx, clusterName, ns, shardingComponents[i], shardingName)
		if err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}
	return configs, nil
}

func buildConfiguration(transCtx *clusterTransformContext, clusterName, ns string, shardingComponent *appsv1alpha1.ClusterComponentSpec, shardingName string) (*appsv1alpha1.Configuration, error) {
	config := builder.NewConfigurationBuilder(ns,
		core.GenerateComponentConfigurationName(clusterName,
			shardingComponent.Name)).
		AddLabelsInMap(constant.GetComponentWellKnownLabels(clusterName, shardingComponent.Name)).
		AddLabelsInMap(constant.GetShardingNameLabel(shardingName)).
		ClusterRef(clusterName).
		Component(shardingComponent.Name).
		GetObject()

	for _, item := range shardingComponent.ComponentParameters {
		configInFile, configSpec, err := fromUserParameters(transCtx, item.Parameters, shardingComponent.ComponentDef, item.Name)
		if err != nil {
			return nil, err
		}
		config.Spec.ConfigItemDetails = append(config.Spec.ConfigItemDetails, appsv1alpha1.ConfigurationItemDetail{
			Name:             item.Name,
			ConfigSpec:       configSpec,
			ConfigFileParams: configInFile,
		})
	}
	return config, nil
}

func fromUserParameters(transCtx *clusterTransformContext, parameters map[string]*string, componentDefName string, configSpecName string) (map[string]appsv1alpha1.ParametersInFile, *appsv1alpha1.ComponentConfigSpec, error) {
	foundConfigSpec := func(component *appsv1alpha1.ComponentDefinition, name string) *appsv1alpha1.ComponentConfigSpec {
		for _, config := range component.Spec.Configs {
			if config.Name == name {
				return config.DeepCopy()
			}
		}
		return nil
	}

	if len(parameters) == 0 {
		return nil, nil, nil
	}
	componentDef, ok := transCtx.ComponentDefs[componentDefName]
	if !ok {
		return nil, nil, fmt.Errorf("not fount componentDef[%s]", componentDefName)
	}
	configSpec := foundConfigSpec(componentDef, configSpecName)
	if configSpec == nil {
		return nil, nil, fmt.Errorf("not fount component config: [%s] componentDefName: %s", configSpecName, componentDefName)
	}

	returnSucceed := func(fileName string) (map[string]appsv1alpha1.ParametersInFile, *appsv1alpha1.ComponentConfigSpec, error) {
		return map[string]appsv1alpha1.ParametersInFile{
			fileName: {
				Parameters: parameters,
			},
		}, configSpec, nil
	}

	if len(configSpec.Keys) == 1 {
		return returnSucceed(configSpec.Keys[0])
	}

	cmKey := client.ObjectKey{
		Name:      configSpec.TemplateRef,
		Namespace: configSpec.Namespace,
	}
	cmObj := corev1.ConfigMap{}
	if err := transCtx.GetClient().Get(transCtx.GetContext(), cmKey, &cmObj); err != nil {
		return nil, nil, err
	}
	files := sets.StringKeySet(cmObj.Data)
	if len(files) == 1 {
		return returnSucceed(files.List()[0])
	}
	return nil, nil, fmt.Errorf("not fount configSpec file:[%v]", files)
}
