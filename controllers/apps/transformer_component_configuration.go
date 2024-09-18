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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	configurationv1alpha1 "github.com/apecloud/kubeblocks/apis/configuration/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/configuration"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// clusterServiceTransformer handles cluster services.
type componentConfigurationTransformer struct {
	client.Client
}

var _ graph.Transformer = &componentConfigurationTransformer{}

func (c *componentConfigurationTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)

	if model.IsObjectDeleting(transCtx.ComponentOrig) {
		return nil
	}
	if common.IsCompactMode(transCtx.ComponentOrig.Annotations) {
		transCtx.V(1).Info("Component is in compact mode, no need to create configuration related objects",
			"component", client.ObjectKeyFromObject(transCtx.ComponentOrig))
		return nil
	}

	return c.reconcile(transCtx, transCtx.SynthesizeComponent, dag)
}

func (c *componentConfigurationTransformer) reconcile(transCtx *componentTransformContext, component *component.SynthesizedComponent, dag *graph.DAG) error {
	existingConfig, err := c.runningComponentConfiguration(transCtx, transCtx.GetClient(), component)
	if err != nil {
		return err
	}

	cluster := transCtx.Cluster
	config, err := buildConfiguration(transCtx, cluster, component)
	if err != nil {
		return err
	}
	if _, err = configuration.UpdateConfigPayload(&config.Spec, component); err != nil {
		return err
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	if existingConfig != nil {
		graphCli.Update(dag, existingConfig, config, inDataContext4G())
	} else {
		graphCli.Create(dag, config, inDataContext4G())
	}
	return nil
}

func (c *componentConfigurationTransformer) runningComponentConfiguration(ctx context.Context, cli client.Reader, component *component.SynthesizedComponent) (*configurationv1alpha1.ComponentParameter, error) {
	key := client.ObjectKey{
		Name:      core.GenerateComponentConfigurationName(component.ClusterName, component.Name),
		Namespace: component.Namespace,
	}
	existingConfig := &configurationv1alpha1.ComponentParameter{}
	err := cli.Get(ctx, key, existingConfig, inDataContext4C())
	if err == nil {
		return existingConfig, nil
	}
	if !apierrors.IsNotFound(err) {
		return nil, err
	}
	return nil, nil
}

func buildConfiguration(transCtx *componentTransformContext, cluster *appsv1.Cluster, component *component.SynthesizedComponent) (*configurationv1alpha1.ComponentParameter, error) {
	items, err := configuration.ClassifyParamsFromConfigTemplate(transCtx, transCtx.GetClient(), transCtx.Component, transCtx.CompDef, component)
	if err != nil {
		return nil, err
	}

	return builder.NewConfigurationBuilder(cluster.Namespace,
		core.GenerateComponentConfigurationName(cluster.Name, component.Name)).
		AddLabelsInMap(constant.GetComponentWellKnownLabels(cluster.Name, component.Name)).
		ClusterRef(cluster.Name).
		Component(component.Name).
		SetConfigurationItem(items).
		GetObject(), nil
}
