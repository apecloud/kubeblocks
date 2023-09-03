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
	"github.com/apecloud/kubeblocks/internal/configuration/core"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type reconcileCtx struct {
	cli        client.Client
	ctx        context.Context
	cluster    *appsv1alpha1.Cluster
	clusterVer *appsv1alpha1.ClusterVersion
	component  *component.SynthesizedComponent

	obj     client.Object
	cache   []client.Object
	podSpec *corev1.PodSpec
}

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
			cli:        cli,
			ctx:        ctx,
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
	var (
		cli       = c.cli
		ctx       = c.ctx
		component = c.component
	)

	// Need to Merge configTemplateRef of ClusterVersion.Components[*].ConfigTemplateRefs and
	// ClusterDefinition.Components[*].ConfigTemplateRefs
	if len(component.ConfigTemplates) == 0 && len(component.ScriptTemplates) == 0 {
		return c.UpdateConfiguration()
	}

	clusterName := c.cluster.Name
	namespaceName := c.cluster.Namespace
	templateBuilder := newTemplateBuilder(clusterName, namespaceName, c.cluster, c.clusterVer, ctx, cli)
	// Prepare built-in objects and built-in functions
	if err := templateBuilder.injectBuiltInObjectsAndFunctions(c.podSpec, component.ConfigTemplates, component, c.cache); err != nil {
		return err
	}

	renderWrapper := newTemplateRenderWrapper(templateBuilder, c.cluster, ctx, cli)
	if err := renderWrapper.renderConfigTemplate(c.cluster, component, c.cache); err != nil {
		return err
	}
	if err := renderWrapper.renderScriptTemplate(c.cluster, component, c.cache); err != nil {
		return err
	}

	if len(renderWrapper.templateAnnotations) > 0 {
		updateResourceAnnotationsWithTemplate(c.obj, renderWrapper.templateAnnotations)
	}

	// Generate Pod Volumes for ConfigMap objects
	if err := intctrlutil.CreateOrUpdatePodVolumes(c.podSpec, renderWrapper.volumes); err != nil {
		return core.WrapError(err, "failed to generate pod volume")
	}

	if err := buildConfigManagerWithComponent(c.podSpec, component.ConfigTemplates, ctx, cli, c.cluster, component); err != nil {
		return core.WrapError(err, "failed to generate sidecar for configmap's reloader")
	}

	if err := injectTemplateEnvFrom(c.cluster, component, c.podSpec, cli, ctx, renderWrapper.renderedObjs); err != nil {
		return err
	}
	return createConfigObjects(cli, ctx, renderWrapper.renderedObjs)
}

func (c *configOperator) UpdateConfiguration() error {
	// TODO update configuration
	return nil
}
