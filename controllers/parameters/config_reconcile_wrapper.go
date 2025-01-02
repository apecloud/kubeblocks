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

package parameters

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	configctrl "github.com/apecloud/kubeblocks/pkg/controller/configuration"
	"github.com/apecloud/kubeblocks/pkg/controller/render"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

type ReconcileContext struct {
	intctrlutil.RequestCtx
	configctrl.ResourceFetcher[ReconcileContext]

	ctx              context.Context
	Name             string
	MatchingLabels   client.MatchingLabels
	ConfigMap        *corev1.ConfigMap
	BuiltinComponent *component.SynthesizedComponent

	Containers      []string
	InstanceSetList []workloads.InstanceSet

	ConfigRender   *parametersv1alpha1.ParamConfigRenderer
	ParametersDefs map[string]*parametersv1alpha1.ParametersDefinition
}

func newConfigReconcileContext(ctx context.Context,
	resourceCtx *render.ResourceCtx,
	cm *corev1.ConfigMap,
	configSpecName string,
	matchingLabels client.MatchingLabels) *ReconcileContext {
	configContext := ReconcileContext{
		ctx:            ctx,
		ConfigMap:      cm,
		Name:           configSpecName,
		MatchingLabels: matchingLabels,
	}
	return configContext.Init(resourceCtx, &configContext)
}

func (c *ReconcileContext) GetRelatedObjects() error {
	return c.Cluster().
		ComponentAndComponentDef().
		ComponentSpec().
		Workload().
		SynthesizedComponent().
		Complete()
}

func (c *ReconcileContext) Workload() *ReconcileContext {
	stsFn := func() (err error) {
		c.InstanceSetList, c.Containers, err = retrieveRelatedComponentsByConfigmap(
			c.Client,
			c.Context,
			c.Name,
			generics.InstanceSetSignature,
			client.ObjectKeyFromObject(c.ConfigMap),
			client.InNamespace(c.Namespace),
			c.MatchingLabels)
		return
	}
	return c.Wrap(stsFn)
}

func (c *ReconcileContext) SynthesizedComponent() *ReconcileContext {
	return c.Wrap(func() (err error) {
		// build synthesized component for the component
		c.BuiltinComponent, err = component.BuildSynthesizedComponent(c.ctx, c.Client, c.ComponentDefObj, c.ComponentObj, c.ClusterObj)
		return err
	})
}

func (c *ReconcileContext) ParametersDefinitions() *ReconcileContext {
	return c.Wrap(func() (err error) {
		configRender, paramsDefs, err := intctrlutil.ResolveCmpdParametersDefs(c.Context, c.Client, c.ComponentDefObj)
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
