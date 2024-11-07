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

package apps

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/configuration"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type clusterParametersTransformer struct {
}

var _ = clusterParametersTransformer{}

func (c *clusterParametersTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	if model.IsObjectDeleting(transCtx.OrigCluster) {
		return nil
	}
	if common.IsCompactMode(transCtx.OrigCluster.Annotations) {
		transCtx.V(1).Info("Cluster is in compact mode, no need to create parameters objects",
			"cluster", client.ObjectKeyFromObject(transCtx.OrigCluster))
		return nil
	}
	return c.reconcile(transCtx, transCtx.Cluster, dag)
}

func (c *clusterParametersTransformer) reconcile(transCtx *clusterTransformContext, cluster *appsv1.Cluster, dag *graph.DAG) error {
	existingComponentParameters, err := runningComponentParameters(transCtx, cluster)
	if err != nil {
		return err
	}
	expectedComponentParameters, err := createComponentParameters(transCtx, cluster)
	if err != nil {
		return err
	}

	existingConfigSet := sets.KeySet(existingComponentParameters)
	expectedConfigSet := sets.KeySet(expectedComponentParameters)
	createSet := expectedConfigSet.Difference(existingConfigSet)
	updateSet := expectedConfigSet.Intersection(existingConfigSet)
	deleteSet := existingConfigSet.Difference(expectedConfigSet)
	graphCli, _ := transCtx.Client.(model.GraphClient)
	for parameter := range createSet {
		graphCli.Create(dag, expectedComponentParameters[parameter], inDataContext4G())
	}

	for parameter := range updateSet {
		graphCli.Patch(dag, existingComponentParameters[parameter],
			c.mergeComponentParameter(expectedComponentParameters[parameter], expectedComponentParameters[parameter]), inDataContext4G())
	}

	// Clean configurations that are not being used by the sharding component, e.g: shards -> 100 and shards --> 20
	for parameter := range deleteSet {
		graphCli.Delete(dag, existingComponentParameters[parameter], inDataContext4G())
	}
	return nil
}

func (c *clusterParametersTransformer) mergeComponentParameter(expected *parametersv1alpha1.ComponentParameter, existing *parametersv1alpha1.ComponentParameter) *parametersv1alpha1.ComponentParameter {
	return configuration.MergeComponentParameter(expected, existing, func(dest, expected *parametersv1alpha1.ConfigTemplateItemDetail) {
		if len(dest.ConfigFileParams) == 0 && len(expected.ConfigFileParams) != 0 {
			dest.ConfigFileParams = expected.ConfigFileParams
		}
	})
}

func createComponentParameters(transCtx *clusterTransformContext, cluster *appsv1.Cluster) (map[string]*parametersv1alpha1.ComponentParameter, error) {
	expectedObjects := make(map[string]*parametersv1alpha1.ComponentParameter, len(transCtx.shardings)+len(transCtx.components))

	for _, comp := range transCtx.components {
		parameter, err := buildComponentParameter(transCtx, cluster, comp.Name, comp)
		if err != nil {
			return nil, err
		}
		expectedObjects[parameter.Name] = parameter
	}

	for _, shardingComponents := range transCtx.shardingComps {
		for _, componentSpec := range shardingComponents {
			parameter, err := buildComponentParameter(transCtx, cluster, componentSpec.Name, componentSpec)
			if err != nil {
				return nil, err
			}
			expectedObjects[parameter.Name] = parameter
		}
	}
	return expectedObjects, nil
}

func runningComponentParameters(transCtx *clusterTransformContext, cluster *appsv1.Cluster) (map[string]*parametersv1alpha1.ComponentParameter, error) {
	var parameterList = &parametersv1alpha1.ComponentParameterList{}

	labels := client.MatchingLabels(constant.GetClusterLabels(cluster.Name))
	if err := transCtx.Client.List(transCtx.Context, parameterList, labels, client.InNamespace(cluster.Namespace)); err != nil {
		return nil, err
	}

	parameters := make(map[string]*parametersv1alpha1.ComponentParameter, len(parameterList.Items))
	for i := range parameterList.Items {
		ref := &parameterList.Items[i]
		if model.IsOwnerOf(cluster, ref) {
			parameters[ref.Name] = ref
		}
	}
	return parameters, nil
}

func buildComponentParameter(transCtx *clusterTransformContext, cluster *appsv1.Cluster, compName string, comp *appsv1.ClusterComponentSpec) (*parametersv1alpha1.ComponentParameter, error) {
	var cmpd *appsv1.ComponentDefinition

	if comp.ComponentDef == "" {
		return nil, nil
	}
	if cmpd = transCtx.componentDefs[comp.ComponentDef]; cmpd == nil || len(cmpd.Spec.Configs) == 0 {
		return nil, nil
	}
	_, paramsDefs, err := intctrlutil.ResolveCmpdParametersDefs(transCtx, transCtx.Client, cmpd)
	if err != nil {
		return nil, err
	}
	tpls, err := resolveComponentTemplate(transCtx, transCtx.Client, cmpd)
	if err != nil {
		return nil, err
	}
	return builder.NewComponentParameterBuilder(cluster.Namespace,
		core.GenerateComponentParameterName(cluster.Name, compName)).
		AddLabelsInMap(constant.GetCompLabelsWithDef(cluster.Name, compName, comp.ComponentDef)).
		ClusterRef(cluster.Name).
		Component(compName).
		SetConfigurationItem(configuration.ClassifyParamsFromConfigTemplate(comp.InitParameters, cmpd, paramsDefs, tpls)).
		GetObject(), nil
}

func resolveComponentTemplate(ctx context.Context, reader client.Reader, cmpd *appsv1.ComponentDefinition) (map[string]*corev1.ConfigMap, error) {
	tpls := make(map[string]*corev1.ConfigMap, len(cmpd.Spec.Configs))
	for _, config := range cmpd.Spec.Configs {
		cm := &corev1.ConfigMap{}
		if err := reader.Get(ctx, client.ObjectKey{Name: config.TemplateRef, Namespace: config.Namespace}, cm); err != nil {
			return nil, err
		}
		tpls[config.Name] = cm
	}
	return tpls, nil
}
