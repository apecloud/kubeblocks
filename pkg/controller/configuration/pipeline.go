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
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type ReconcileCtx struct {
	*ResourceCtx

	Cluster              *appsv1.Cluster
	Component            *appsv1.Component
	SynthesizedComponent *component.SynthesizedComponent
	PodSpec              *corev1.PodSpec
	Configuration        *appsv1alpha1.ComponentConfiguration

	Cache []client.Object
}

type reloadActionBuilderHelper struct {
	// configuration *appsv1alpha1.Configuration
	renderWrapper renderWrapper

	ctx ReconcileCtx
	ResourceFetcher[reloadActionBuilderHelper]
}

type updatePipeline struct {
	reconcile     bool
	renderWrapper renderWrapper

	item       appsv1alpha1.ConfigTemplateItemDetail
	itemStatus *appsv1alpha1.ConfigTemplateItemDetailStatus
	configSpec *appsv1.ComponentConfigSpec
	
	newCM       *corev1.ConfigMap
	configPatch *core.ConfigPatchInfo

	ctx ReconcileCtx
	ResourceFetcher[updatePipeline]
}

func NewReloadActionBuilderHelper(ctx ReconcileCtx) *reloadActionBuilderHelper {
	p := &reloadActionBuilderHelper{ctx: ctx}
	p.ConfigurationObj = ctx.Configuration
	return p.Init(ctx.ResourceCtx, p)
}

func NewReconcilePipeline(ctx ReconcileCtx, item appsv1alpha1.ConfigTemplateItemDetail, itemStatus *appsv1alpha1.ConfigTemplateItemDetailStatus, configSpec *appsv1.ComponentConfigSpec) *updatePipeline {
	p := &updatePipeline{
		reconcile:  true,
		item:       item,
		itemStatus: itemStatus,
		ctx:        ctx,
		configSpec: configSpec,
	}
	return p.Init(ctx.ResourceCtx, p)
}

func (p *reloadActionBuilderHelper) Prepare() *reloadActionBuilderHelper {
	buildTemplate := func() (err error) {
		ctx := p.ctx
		templateBuilder := newTemplateBuilder(p.ClusterName, p.Namespace, p.Context, p.Client)
		// Prepare built-in objects and built-in functions
		templateBuilder.injectBuiltInObjectsAndFunctions(ctx.PodSpec, ctx.SynthesizedComponent, ctx.Cache, ctx.Cluster)
		p.renderWrapper = newTemplateRenderWrapper(p.Context, ctx.Client, templateBuilder, ctx.Cluster, ctx.Component)
		return
	}

	return p.Wrap(buildTemplate)
}

func (p *reloadActionBuilderHelper) RenderScriptTemplate() *reloadActionBuilderHelper {
	return p.Wrap(func() error {
		ctx := p.ctx
		return p.renderWrapper.renderScriptTemplate(ctx.Cluster, ctx.SynthesizedComponent, ctx.Cache)
	})
}

func CheckAndUpdateItemStatus(updated *appsv1alpha1.ComponentConfiguration, item appsv1alpha1.ConfigTemplateItemDetail, reversion string) {
	foundStatus := func(name string) *appsv1alpha1.ConfigTemplateItemDetailStatus {
		for i := range updated.Status.ConfigurationItemStatus {
			status := &updated.Status.ConfigurationItemStatus[i]
			if status.Name == name {
				return status
			}
		}
		return nil
	}

	status := foundStatus(item.Name)
	if status != nil && status.Phase == "" {
		status.Phase = appsv1alpha1.CInitPhase
	}
	if status == nil {
		updated.Status.ConfigurationItemStatus = append(updated.Status.ConfigurationItemStatus,
			appsv1alpha1.ConfigTemplateItemDetailStatus{
				Name:           item.Name,
				Phase:          appsv1alpha1.CInitPhase,
				UpdateRevision: reversion,
			})
	}
}

func (p *reloadActionBuilderHelper) UpdatePodVolumes() *reloadActionBuilderHelper {
	return p.Wrap(func() error {
		component := p.ctx.SynthesizedComponent
		volumes := make(map[string]appsv1alpha1.ComponentTemplateSpec, len(component.ConfigTemplates))
		for _, configSpec := range component.ConfigTemplates {
			cmName := core.GetComponentCfgName(component.ClusterName, component.Name, configSpec.Name)
			volumes[cmName] = configSpec.ComponentTemplateSpec
		}
		for _, configSpec := range component.ScriptTemplates {
			cmName := core.GetComponentCfgName(component.ClusterName, component.Name, configSpec.Name)
			volumes[cmName] = configSpec
		}
		return intctrlutil.CreateOrUpdatePodVolumes(p.ctx.PodSpec, volumes,
			configSetFromComponent(p.ctx.SynthesizedComponent.ConfigTemplates))
	})
}

func (p *reloadActionBuilderHelper) BuildConfigManagerSidecar() *reloadActionBuilderHelper {
	return p.Wrap(func() error {
		return buildConfigManagerWithComponent(p.ctx.PodSpec, p.ctx.SynthesizedComponent.ConfigTemplates, p.Context, p.Client, p.ctx.Cluster, p.ctx.SynthesizedComponent)
	})
}

func (p *reloadActionBuilderHelper) InitConfigRelatedObject() *reloadActionBuilderHelper {
	updateMeta := func() error {
		cluster := p.ctx.Cluster
		component := p.ctx.SynthesizedComponent
		if err := p.renderWrapper.renderConfigTemplate(cluster, component, p.ctx.Cache, p.ConfigurationObj); err != nil {
			return err
		}
		if err := injectTemplateEnvFrom(cluster, component, p.ctx.PodSpec, p.Client, p.Context, p.renderWrapper.renderedObjs); err != nil {
			return err
		}
		return createConfigObjects(p.Client, p.Context, p.renderWrapper.renderedObjs, p.renderWrapper.renderedSecretObjs)
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
		templateBuilder := newTemplateBuilder(p.ClusterName, p.Namespace, p.Context, p.Client)
		// Prepare built-in objects and built-in functions
		templateBuilder.injectBuiltInObjectsAndFunctions(p.ctx.PodSpec, p.ctx.SynthesizedComponent, p.ctx.Cache, p.ctx.Cluster)
		p.renderWrapper = newTemplateRenderWrapper(p.Context, p.Client, templateBuilder, p.ctx.Cluster, p.ctx.Component)
		return
	}
	return p.Wrap(buildTemplate)
}

func (p *updatePipeline) ConfigSpec() *appsv1.ComponentConfigSpec {
	return p.configSpec
}

func (p *updatePipeline) InitConfigSpec() *updatePipeline {
	return p.Wrap(func() (err error) {
		if p.configSpec == nil {
			p.configSpec = component.GetConfigSpecByName(p.ctx.SynthesizedComponent, p.item.Name)
			if p.configSpec == nil {
				return core.MakeError("not found config spec: %s", p.item.Name)
			}
		}
		return
	})
}

func (p *updatePipeline) RerenderTemplate() *updatePipeline {
	return p.Wrap(func() (err error) {
		if p.isDone() {
			return
		}
		if intctrlutil.IsRerender(p.ConfigMapObj, p.item) {
			p.newCM, err = p.renderWrapper.rerenderConfigTemplate(p.ctx.Cluster, p.ctx.SynthesizedComponent, *p.configSpec, &p.item)
		} else {
			p.newCM = p.ConfigMapObj.DeepCopy()
		}
		return
	})
}

func (p *updatePipeline) ApplyParameters() *updatePipeline {
	patchMerge := func(p *updatePipeline, spec appsv1.ComponentConfigSpec, cm *corev1.ConfigMap, item appsv1alpha1.ConfigTemplateItemDetail) error {
		if p.isDone() || len(item.ConfigFileParams) == 0 {
			return nil
		}
		newData, err := DoMerge(cm.Data, item.ConfigFileParams, p.ConfigConstraintObj, spec)
		if err != nil {
			return err
		}
		if p.ConfigConstraintObj == nil {
			cm.Data = newData
			return nil
		}

		p.configPatch, _, err = core.CreateConfigPatch(cm.Data,
			newData,
			p.ConfigConstraintObj.Spec.FileFormatConfig.Format,
			p.configSpec.Keys,
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
		return patchMerge(p, *p.configSpec, p.newCM, p.item)
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
		if p.ConfigConstraintObj != nil && !p.isDone() {
			if err := SyncEnvSourceObject(*p.configSpec, p.newCM, &p.ConfigConstraintObj.Spec, p.Client, p.Context, p.ctx.Cluster, p.ctx.SynthesizedComponent); err != nil {
				return err
			}
		}
		if InjectEnvEnabled(*p.configSpec) && toSecret(*p.configSpec) {
			return nil
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

func (p *updatePipeline) SyncStatus() *updatePipeline {
	return p.Wrap(func() (err error) {
		if p.isDone() {
			return
		}
		if p.configSpec == nil || p.itemStatus == nil {
			return
		}
		p.itemStatus.Phase = appsv1alpha1.CMergedPhase
		return
	})
}
