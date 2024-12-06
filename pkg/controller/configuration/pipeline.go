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
	"context"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/render"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

type pipeline struct {
	// configuration *appsv1alpha1.Configuration
	renderWrapper renderWrapper

	ctx render.ReconcileCtx
	ResourceFetcher[pipeline]

	configRender   *parametersv1alpha1.ParameterDrivenConfigRender
	parametersDefs []*parametersv1alpha1.ParametersDefinition
}

type updatePipeline struct {
	renderWrapper renderWrapper

	reconcile  bool
	item       parametersv1alpha1.ConfigTemplateItemDetail
	itemStatus *parametersv1alpha1.ConfigTemplateItemDetailStatus
	configSpec *appsv1.ComponentTemplateSpec
	// replace of ConfigMapObj
	// originalCM  *corev1.ConfigMap
	newCM       *corev1.ConfigMap
	configPatch *core.ConfigPatchInfo

	ctx render.ReconcileCtx
	ResourceFetcher[updatePipeline]

	configRender   *parametersv1alpha1.ParameterDrivenConfigRender
	parametersDefs []*parametersv1alpha1.ParametersDefinition
}

func NewCreatePipeline(ctx render.ReconcileCtx) *pipeline {
	p := &pipeline{ctx: ctx}
	return p.Init(ctx.ResourceCtx, p)
}

func NewReconcilePipeline(ctx render.ReconcileCtx, item parametersv1alpha1.ConfigTemplateItemDetail, itemStatus *parametersv1alpha1.ConfigTemplateItemDetailStatus, cm *corev1.ConfigMap, componentParameter *parametersv1alpha1.ComponentParameter) *updatePipeline {
	p := &updatePipeline{
		reconcile:  true,
		item:       item,
		itemStatus: itemStatus,
		ctx:        ctx,
		configSpec: item.ConfigSpec,
	}
	p.Init(ctx.ResourceCtx, p)
	p.ConfigMapObj = cm
	p.ComponentParameterObj = componentParameter
	return p
}

func (p *pipeline) Prepare() *pipeline {
	buildTemplate := func() (err error) {
		ctx := p.ctx
		p.renderWrapper = newTemplateRenderWrapper(p.Context, ctx.Client, render.NewTemplateBuilder(&ctx), ctx.Cluster, ctx.Component)
		p.configRender, p.parametersDefs, err = intctrlutil.ResolveCmpdParametersDefs(ctx.Context, ctx.Client, p.ComponentDefObj)
		return err
	}

	return p.Wrap(buildTemplate)
}

func (p *pipeline) RenderScriptTemplate() *pipeline {
	return p.Wrap(func() error {
		ctx := p.ctx
		return p.renderWrapper.renderScriptTemplate(ctx.Cluster, ctx.SynthesizedComponent, ctx.Cache, ctx.Component)
	})
}

func (p *pipeline) SyncComponentParameter() *pipeline {
	buildConfiguration := func() (err error) {
		var existingObject *parametersv1alpha1.ComponentParameter
		var expectedObject *parametersv1alpha1.ComponentParameter

		if existingObject, err = runningComponentParameter(p.Context, p.Client, p.ctx.SynthesizedComponent); err != nil {
			return err
		}
		if expectedObject, err = buildComponentParameter(p.Context, p.Client, p.ctx.SynthesizedComponent, p.ctx.Component, p.ComponentDefObj, p.configRender, p.parametersDefs); err != nil {
			return err
		}

		switch {
		case expectedObject == nil:
			return p.Client.Delete(p.Context, existingObject)
		case existingObject == nil:
			return p.Client.Create(p.Context, expectedObject)
		default:
			return updateComponentParameters(p.Context, p.Client, expectedObject, existingObject)
		}
	}
	return p.Wrap(buildConfiguration)
}

func (p *pipeline) CreateConfigTemplate() *pipeline {
	return p.Wrap(func() error {
		ctx := p.ctx
		revision := strconv.FormatInt(p.ComponentParameterObj.GetGeneration(), 10)
		return p.renderWrapper.renderConfigTemplate(ctx.Cluster, ctx.SynthesizedComponent, ctx.Cache, p.ComponentParameterObj, p.configRender, p.parametersDefs, revision)
	})
}

func (p *pipeline) UpdatePodVolumes() *pipeline {
	mapName := func(tpl appsv1.ComponentTemplateSpec) string {
		return tpl.Name
	}
	return p.Wrap(func() error {
		return intctrlutil.CreateOrUpdatePodVolumes(p.ctx.PodSpec,
			p.renderWrapper.volumes,
			generics.Map(p.ctx.SynthesizedComponent.ConfigTemplates, mapName))
	})
}

func (p *pipeline) BuildConfigManagerSidecar() *pipeline {
	return p.Wrap(func() error {
		return buildConfigManagerWithComponent(p.ctx.ResourceCtx, p.ctx.Cluster, p.ctx.SynthesizedComponent, p.ComponentDefObj)
	})
}

func (p *pipeline) UpdateConfigRelatedObject() *pipeline {
	updateMeta := func() error {
		if err := syncInjectEnvFromCM(p.Context, p.Client, p.ctx.SynthesizedComponent, p.configRender, p.renderWrapper.renderedObjs, true); err != nil {
			return err
		}
		return createConfigObjects(p.Client, p.Context, p.renderWrapper.renderedObjs)
	}

	return p.Wrap(updateMeta)
}

func (p *updatePipeline) isDone() bool {
	return !p.reconcile
}

func (p *updatePipeline) PrepareForTemplate() *updatePipeline {
	buildTemplate := func() (err error) {
		p.reconcile = !intctrlutil.IsApplyConfigChanged(p.ConfigMapObj, p.item)
		if p.isDone() {
			return
		}
		p.renderWrapper = newTemplateRenderWrapper(p.Context, p.Client, render.NewTemplateBuilder(&p.ctx), p.ctx.Cluster, p.ctx.Component)
		p.configRender, p.parametersDefs, err = intctrlutil.ResolveCmpdParametersDefs(p.Context, p.Client, p.ComponentDefObj)
		return
	}
	return p.Wrap(buildTemplate)
}

func (p *updatePipeline) RerenderTemplate() *updatePipeline {
	return p.Wrap(func() (err error) {
		if p.isDone() {
			return
		}
		if intctrlutil.IsRerender(p.ConfigMapObj, p.item) {
			p.newCM, err = p.renderWrapper.rerenderConfigTemplate(p.ctx.Cluster, p.ctx.SynthesizedComponent, *p.configSpec, &p.item, p.configRender, p.parametersDefs)
		} else {
			p.newCM = p.ConfigMapObj.DeepCopy()
		}
		return
	})
}

func (p *updatePipeline) ApplyParameters() *updatePipeline {
	patchMerge := func(p *updatePipeline, cm *corev1.ConfigMap, item parametersv1alpha1.ConfigTemplateItemDetail) error {
		if p.isDone() || len(item.ConfigFileParams) == 0 || p.configRender == nil {
			return nil
		}
		newData, err := DoMerge(cm.Data, item.ConfigFileParams, p.parametersDefs, p.configRender.Spec.Configs)
		if err != nil {
			return err
		}
		if p.configRender == nil {
			cm.Data = newData
			return nil
		}

		p.configPatch, _, err = core.CreateConfigPatch(cm.Data,
			newData,
			p.configRender.Spec,
			false)
		if err != nil {
			return err
		}
		cm.Data = newData
		return nil
	}

	return p.Wrap(func() error {
		if p.isDone() {
			return nil
		}
		return patchMerge(p, p.newCM, p.item)
	})
}

func (p *updatePipeline) UpdateConfigVersion(revision string) *updatePipeline {
	return p.Wrap(func() error {
		if p.isDone() {
			return nil
		}

		if err := updateConfigMetaForCM(p.newCM, &p.item, revision); err != nil {
			return err
		}
		annotations := p.newCM.Annotations
		if annotations == nil {
			annotations = make(map[string]string)
		}

		// delete disable reconcile annotation
		if _, ok := annotations[constant.DisableUpgradeInsConfigurationAnnotationKey]; ok {
			annotations[constant.DisableUpgradeInsConfigurationAnnotationKey] = strconv.FormatBool(false)
		}
		p.newCM.Annotations = annotations
		// p.itemStatus.UpdateRevision = revision
		return nil
	})
}

// TODO(leon)
func (p *updatePipeline) Sync() *updatePipeline {
	return p.Wrap(func() error {
		if !p.isDone() {
			if err := syncInjectEnvFromCM(p.Context, p.Client, p.ctx.SynthesizedComponent, p.configRender, []*corev1.ConfigMap{p.newCM}, false); err != nil {
				return err
			}
		}
		if err := intctrlutil.SetControllerReference(p.ComponentParameterObj, p.newCM); err != nil {
			return err
		}

		switch {
		case p.isDone():
			return nil
		case p.ConfigMapObj == nil && p.newCM != nil:
			return p.Client.Create(p.Context, p.newCM)
		case p.ConfigMapObj != nil:
			patch := client.MergeFrom(p.ConfigMapObj)
			if p.ConfigMapObj != nil {
				p.newCM.Labels = intctrlutil.MergeMetadataMaps(p.newCM.Labels, p.ConfigMapObj.Labels)
				p.newCM.Annotations = intctrlutil.MergeMetadataMaps(p.newCM.Annotations, p.ConfigMapObj.Annotations)
			}
			return p.Client.Patch(p.Context, p.newCM, patch)
		}
		return core.MakeError("unexpected condition")
	})
}

func syncInjectEnvFromCM(ctx context.Context, cli client.Client, synthesizedComp *component.SynthesizedComponent, configRender *parametersv1alpha1.ParameterDrivenConfigRender, configMaps []*corev1.ConfigMap, onlyCreate bool) error {
	var podSpec *corev1.PodSpec

	if onlyCreate {
		podSpec = synthesizedComp.PodSpec
	}
	envObjs, err := InjectTemplateEnvFrom(synthesizedComp, podSpec, configRender, configMaps)
	if err != nil {
		return err
	}
	for _, obj := range envObjs {
		if err = cli.Create(ctx, obj, inDataContext()); err == nil {
			continue
		}
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		if onlyCreate {
			continue
		}
		if err = cli.Update(ctx, obj, inDataContext()); err != nil {
			return err
		}
	}
	return nil
}
