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

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/controller/configuration"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type componentRelatedParametersTransformer struct {
	client.Client
}

var _ = componentRelatedParametersTransformer{}

func (c *componentRelatedParametersTransformer) Transform(ctx graph.TransformContext, _ *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	synthesizedComp := transCtx.SynthesizeComponent

	componentParameter := &parametersv1alpha1.ComponentParameter{}
	configKey := client.ObjectKey{Namespace: synthesizedComp.Namespace,
		Name: cfgcore.GenerateComponentConfigurationName(synthesizedComp.ClusterName, synthesizedComp.Name)}
	if err := c.Get(ctx.GetContext(), configKey, componentParameter); err != nil {
		return client.IgnoreNotFound(err)
	}

	configRender, err := intctrlutil.ResolveComponentConfigRender(ctx.GetContext(), c, transCtx.CompDef)
	if err != nil {
		return client.IgnoreNotFound(err)
	}
	if configRender == nil {
		return nil
	}

	configNew := componentParameter.DeepCopy()
	if err = configuration.UpdateConfigPayload(&configNew.Spec, &transCtx.Component.Spec, &configRender.Spec); err != nil {
		return err
	}
	return c.Patch(ctx.GetContext(), configNew, client.MergeFrom(componentParameter.DeepCopy()))
}
