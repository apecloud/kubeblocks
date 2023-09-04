/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package plan

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/component"
)

type configOperator struct {
	reconcileCtx
}

func NewConfigReconcileTask(cli client.Client,
	ctx context.Context,
	cluster *appsv1alpha1.Cluster,
	clusterVersion *appsv1alpha1.ClusterVersion,
	component *component.SynthesizedComponent,
	obj client.Object,
	podSpec *corev1.PodSpec,
	localObjs []client.Object,
) *configOperator {
	return &configOperator{
		reconcileCtx{
			Context:    ctx,
			cli:        cli,
			cluster:    cluster,
			clusterVer: clusterVersion,
			component:  component,
			obj:        obj,
			podSpec:    podSpec,
			cache:      localObjs,
		},
	}
}

func (c *configOperator) Reconcile() error {
	var component = c.component

	// Need to Merge configTemplateRef of ClusterVersion.Components[*].ConfigTemplateRefs and
	// ClusterDefinition.Components[*].ConfigTemplateRefs
	if len(component.ConfigTemplates) == 0 && len(component.ScriptTemplates) == 0 {
		return c.UpdateConfiguration()
	}

	return newPipeline(c.reconcileCtx).
		Prepare().
		RenderScriptTemplate().
		Configuration().
		CreateConfigTemplate().
		UpdateConfigurationStatus().
		UpdatePodVolumes().
		BuildConfigManagerSidecar().
		Complete()
}

func (c *configOperator) UpdateConfiguration() error {
	return newPipeline(c.reconcileCtx).
		Configuration().
		Complete()
}
