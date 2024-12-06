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

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/render"
)

type configOperator struct {
	render.ReconcileCtx
}

func NewConfigReconcileTask(resourceCtx *render.ResourceCtx,
	cluster *appsv1.Cluster,
	component *appsv1.Component,
	synthesizedComponent *component.SynthesizedComponent,
	podSpec *corev1.PodSpec,
	localObjs []client.Object,
) *configOperator {
	return &configOperator{
		render.ReconcileCtx{
			ResourceCtx:          resourceCtx,
			Cluster:              cluster,
			Component:            component,
			SynthesizedComponent: synthesizedComponent,
			PodSpec:              podSpec,
			Cache:                localObjs,
		},
	}
}

func (c *configOperator) Reconcile() error {
	var synthesizedComponent = c.SynthesizedComponent

	if len(synthesizedComponent.ConfigTemplates) == 0 && len(synthesizedComponent.ScriptTemplates) == 0 {
		return c.UpdateConfiguration()
	}

	return NewCreatePipeline(c.ReconcileCtx).
		ComponentAndComponentDef().
		Prepare().
		RenderScriptTemplate().
		SyncComponentParameter().
		ComponentParameter().
		CreateConfigTemplate().
		UpdatePodVolumes().
		BuildConfigManagerSidecar().
		UpdateConfigRelatedObject().
		Complete()
}

func (c *configOperator) UpdateConfiguration() error {
	return NewCreatePipeline(c.ReconcileCtx).
		ComponentAndComponentDef().
		SyncComponentParameter().
		UpdateConfigRelatedObject().
		Complete()
}
