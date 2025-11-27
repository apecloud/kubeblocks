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

package parameters

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/render"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/parameters"
)

type ReconcileContext struct {
	intctrlutil.RequestCtx
	parameters.ResourceFetcher[ReconcileContext]

	MatchingLabels   client.MatchingLabels
	ConfigMap        *corev1.ConfigMap
	BuiltinComponent *component.SynthesizedComponent

	ConfigRender   *parametersv1alpha1.ParamConfigRenderer
	ParametersDefs map[string]*parametersv1alpha1.ParametersDefinition
}

func newParameterReconcileContext(reqCtx intctrlutil.RequestCtx,
	resourceCtx *render.ResourceCtx,
	cm *corev1.ConfigMap,
	cluster *appsv1.Cluster,
	matchingLabels client.MatchingLabels) *ReconcileContext {
	configContext := ReconcileContext{
		ResourceFetcher: parameters.ResourceFetcher[ReconcileContext]{
			ClusterObj: cluster,
		},
		RequestCtx:     reqCtx,
		ConfigMap:      cm,
		MatchingLabels: matchingLabels,
	}
	return configContext.Init(resourceCtx, &configContext)
}

func (c *ReconcileContext) GetRelatedObjects() error {
	return c.Cluster().
		ComponentAndComponentDef().
		ComponentSpec().
		SynthesizedComponent().
		ParametersDefinitions().
		Complete()
}

func (c *ReconcileContext) SynthesizedComponent() *ReconcileContext {
	return c.Wrap(func() (err error) {
		// build synthesized component for the component
		c.BuiltinComponent, err = component.BuildSynthesizedComponent(c.Ctx, c.Client, c.ComponentDefObj, c.ComponentObj)
		return err
	})
}

func (c *ReconcileContext) ParametersDefinitions() *ReconcileContext {
	return c.Wrap(func() (err error) {
		configRender, paramsDefs, err := parameters.ResolveCmpdParametersDefs(c.Context, c.Client, c.ComponentDefObj)
		if err != nil {
			return err
		}

		paramsDefMap := make(map[string]*parametersv1alpha1.ParametersDefinition)
		for _, paramsDef := range paramsDefs {
			paramsDefMap[paramsDef.Spec.FileName] = paramsDef
		}
		c.ConfigRender = configRender
		c.ParametersDefs = paramsDefMap
		return nil
	})
}
