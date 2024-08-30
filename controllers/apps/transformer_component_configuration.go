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
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/controller/configuration"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
)

type componentConfigurationTransformer struct {
	client.Client
}

var _ = componentConfigurationTransformer{}

func (c *componentConfigurationTransformer) Transform(ctx graph.TransformContext, _ *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	synthesizedComp := transCtx.SynthesizeComponent

	config := appsv1alpha1.ComponentConfiguration{}
	configKey := client.ObjectKey{Namespace: synthesizedComp.Namespace,
		Name: cfgcore.GenerateComponentConfigurationName(synthesizedComp.ClusterName, synthesizedComp.Name)}
	if err := c.Get(ctx.GetContext(), configKey, &config); err != nil {
		return client.IgnoreNotFound(err)
	}

	configNew := config.DeepCopy()
	updated, err := configuration.UpdateConfigPayload(&configNew.Spec, synthesizedComp)
	if err != nil {
		return err
	}
	if !updated {
		return nil
	}
	return c.Patch(ctx.GetContext(), configNew, client.MergeFrom(config.DeepCopy()))
}
