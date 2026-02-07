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
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/render"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/parameters"
)

type reconcileContext struct {
	intctrlutil.RequestCtx
	parameters.ResourceFetcher[reconcileContext]

	configMap      *corev1.ConfigMap
	its            *workloads.InstanceSet
	configRender   *parametersv1alpha1.ParamConfigRenderer
	parametersDefs map[string]*parametersv1alpha1.ParametersDefinition
}

func newReconcileContext(reqCtx intctrlutil.RequestCtx, resource *render.ResourceCtx, cm *corev1.ConfigMap, cluster *appsv1.Cluster) *reconcileContext {
	rctx := reconcileContext{
		ResourceFetcher: parameters.ResourceFetcher[reconcileContext]{
			ClusterObj: cluster,
		},
		RequestCtx: reqCtx,
		configMap:  cm,
	}
	return rctx.Init(resource, &rctx)
}

func (c *reconcileContext) objects() error {
	return c.Cluster().
		ComponentAndComponentDef().
		ComponentSpec().
		workload().
		parametersDefinitions().
		Complete()
}

func (c *reconcileContext) workload() *reconcileContext {
	return c.Wrap(func() error {
		itsKey := client.ObjectKey{
			Namespace: c.Namespace,
			Name:      constant.GenerateWorkloadNamePattern(c.ClusterName, c.ComponentName),
		}
		its := &workloads.InstanceSet{}
		if err := c.Client.Get(c.Context, itsKey, its); err == nil {
			c.its = its
		}
		return nil
	})
}

func (c *reconcileContext) parametersDefinitions() *reconcileContext {
	return c.Wrap(func() (err error) {
		configRender, paramsDefs, err := parameters.ResolveCmpdParametersDefs(c.Context, c.Client, c.ComponentDefObj)
		if err != nil {
			return err
		}

		paramsDefMap := make(map[string]*parametersv1alpha1.ParametersDefinition)
		for _, paramsDef := range paramsDefs {
			paramsDefMap[paramsDef.Spec.FileName] = paramsDef
		}
		c.configRender = configRender
		c.parametersDefs = paramsDefMap
		return nil
	})
}
