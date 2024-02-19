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
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	configctrl "github.com/apecloud/kubeblocks/pkg/controller/configuration"
	rsmcore "github.com/apecloud/kubeblocks/pkg/controller/rsm"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

type configReconcileContext struct {
	configctrl.ResourceFetcher[configReconcileContext]

	Name             string
	MatchingLabels   client.MatchingLabels
	ConfigMap        *corev1.ConfigMap
	BuiltinComponent *component.SynthesizedComponent

	Containers   []string
	StatefulSets []appv1.StatefulSet
	RSMList      []workloads.ReplicatedStateMachine
	Deployments  []appv1.Deployment

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
		ClusterComponent().
		RSM().
		SynthesizedComponent().
		Complete()
}

func (c *configReconcileContext) RSM() *configReconcileContext {
	stsFn := func() (err error) {
		c.RSMList, c.Containers, err = retrieveRelatedComponentsByConfigmap(
			c.Client,
			c.Context,
			c.Name,
			generics.RSMSignature,
			client.ObjectKeyFromObject(c.ConfigMap),
			client.InNamespace(c.Namespace),
			c.MatchingLabels)
		if err != nil {
			return
		}

		// fix uid mismatch bug: convert rsm to sts
		// NODE: all components use the StatefulSet
		for _, rsm := range c.RSMList {
			var stsObject appv1.StatefulSet
			if err = c.Client.Get(c.Context, client.ObjectKeyFromObject(rsmcore.ConvertRSMToSTS(&rsm)), &stsObject); err != nil {
				return
			}
			c.StatefulSets = append(c.StatefulSets, stsObject)
		}
		return
	}
	return c.Wrap(stsFn)
}

func (c *configReconcileContext) SynthesizedComponent() *configReconcileContext {
	return c.Wrap(func() (err error) {
		c.BuiltinComponent, err = component.BuildSynthesizedComponentWrapper(c.reqCtx, c.Client, c.ClusterObj, c.ClusterComObj)
		return err
	})
}
