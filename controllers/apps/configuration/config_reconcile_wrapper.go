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

package configuration

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	configctrl "github.com/apecloud/kubeblocks/pkg/controller/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

type configReconcileContext struct {
	configctrl.ResourceFetcher[configReconcileContext]

	Name             string
	MatchingLabels   client.MatchingLabels
	ConfigMap        *corev1.ConfigMap
	BuiltinComponent *component.SynthesizedComponent

	Containers      []string
	InstanceSetList []workloads.InstanceSet

	reqCtx intctrlutil.RequestCtx
}

func newConfigReconcileContext(resourceCtx *configctrl.ResourceCtx,
	cm *corev1.ConfigMap,
	configSpecName string,
	reqCtx intctrlutil.RequestCtx,
	matchingLabels client.MatchingLabels) *configReconcileContext {
	configContext := configReconcileContext{
		reqCtx:         reqCtx,
		ConfigMap:      cm,
		Name:           configSpecName,
		MatchingLabels: matchingLabels,
	}
	return configContext.Init(resourceCtx, &configContext)
}

func (c *configReconcileContext) GetRelatedObjects() error {
	return c.Cluster().
		ComponentAndComponentDef().
		ComponentSpec().
		Workload().
		SynthesizedComponent().
		Complete()
}

func (c *configReconcileContext) Workload() *configReconcileContext {
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

func (c *configReconcileContext) SynthesizedComponent() *configReconcileContext {
	return c.Wrap(func() (err error) {
		if c.ComponentDefObj != nil && c.ComponentObj != nil && len(c.ComponentObj.Spec.CompDef) > 0 {
			// build synthesized component for native component
			c.BuiltinComponent, err = component.BuildSynthesizedComponent(c.reqCtx, c.Client, c.ClusterObj, c.ComponentDefObj, c.ComponentObj)
		} else {
			// build synthesized component for generated component
			c.BuiltinComponent, err = component.BuildSynthesizedComponentWrapper(c.reqCtx, c.Client, c.ClusterObj, c.ClusterComObj)
		}
		return err
	})
}
