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

package configuration

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type ReconcileCtx struct {
	*intctrlutil.ResourceCtx

	Cluster   *appsv1alpha1.Cluster
	Component *component.SynthesizedComponent
	PodSpec   *corev1.PodSpec

	Cache []client.Object
}

type pipeline struct {
	// configuration *appsv1alpha1.Configuration
	renderWrapper renderWrapper

	ctx ReconcileCtx
	intctrlutil.ResourceFetcher[pipeline]
}

type updatePipeline struct {
	reconcile     bool
	renderWrapper renderWrapper

	item       appsv1alpha1.ConfigurationItemDetail
	itemStatus *appsv1alpha1.ConfigurationItemDetailStatus
	configSpec *appsv1alpha1.ComponentConfigSpec
	// replace of ConfigMapObj
	// originalCM  *corev1.ConfigMap
	newCM       *corev1.ConfigMap
	configPatch *core.ConfigPatchInfo

	ctx ReconcileCtx
	intctrlutil.ResourceFetcher[updatePipeline]
}

func NewCreatePipeline(ctx ReconcileCtx) *pipeline {
	p := &pipeline{ctx: ctx}
	return p.Init(ctx.ResourceCtx, p)
}

func NewReconcilePipeline(ctx ReconcileCtx, item appsv1alpha1.ConfigurationItemDetail, itemStatus *appsv1alpha1.ConfigurationItemDetailStatus, configSpec *appsv1alpha1.ComponentConfigSpec) *updatePipeline {
	p := &updatePipeline{
		reconcile:  true,
		item:       item,
		itemStatus: itemStatus,
		ctx:        ctx,
		configSpec: configSpec,
	}
	return p.Init(ctx.ResourceCtx, p)
}

func (p *pipeline) Prepare() *pipeline {
	buildTemplate := func() (err error) {
		ctx := p.ctx
		templateBuilder := newTemplateBuilder(p.ClusterName, p.Namespace, ctx.Cluster, p.Context, p.Client, ctx.Cache)
		// Prepare built-in objects and built-in functions
		if err = templateBuilder.injectBuiltInObjectsAndFunctions(ctx.PodSpec, ctx.Component.ConfigTemplates, ctx.Component, ctx.Cache); err != nil {
			return
		}
		p.renderWrapper = newTemplateRenderWrapper(templateBuilder, ctx.Cluster, p.Context, ctx.Client)
		return
	}

	return p.Wrap(buildTemplate)
}

func (p *pipeline) RenderScriptTemplate() *pipeline {
	return p.Wrap(func() error {
		ctx := p.ctx
		return p.renderWrapper.renderScriptTemplate(ctx.Cluster, ctx.Component, ctx.Cache)
	})
}

func (p *pipeline) UpdateConfiguration() *pipeline {
	buildConfiguration := func() (err error) {
		expectedConfiguration := p.createConfiguration()
		if intctrlutil.SetControllerReference(p.ctx.Cluster, expectedConfiguration) != nil {
			return
		}

		existingConfiguration := appsv1alpha1.Configuration{}
		err = p.ResourceFetcher.Client.Get(p.Context, client.ObjectKeyFromObject(expectedConfiguration), &existingConfiguration)
		switch {
		case err == nil:
			return p.updateConfiguration(expectedConfiguration, &existingConfiguration)
		case apierrors.IsNotFound(err):
			return p.ResourceFetcher.Client.Create(p.Context, expectedConfiguration)
		default:
			return err
		}
	}
	return p.Wrap(buildConfiguration)
}

func (p *pipeline) CreateConfigTemplate() *pipeline {
	return p.Wrap(func() error {
		ctx := p.ctx
		return p.renderWrapper.renderConfigTemplate(ctx.Cluster, ctx.Component, ctx.Cache, p.ConfigurationObj)
	})
}

func (p *pipeline) UpdateConfigurationStatus() *pipeline {
	return p.Wrap(func() error {
		if p.ConfigurationObj == nil {
			return nil
		}

		existing := p.ConfigurationObj
		reversion := fromConfiguration(existing)
		patch := client.MergeFrom(existing)
		updated := existing.DeepCopy()
		for _, item := range existing.Spec.ConfigItemDetails {
			CheckAndUpdateItemStatus(updated, item, reversion)
		}
		return p.ResourceFetcher.Client.Status().Patch(p.Context, updated, patch)
	})
}

func CheckAndUpdateItemStatus(updated *appsv1alpha1.Configuration, item appsv1alpha1.ConfigurationItemDetail, reversion string) {
	foundStatus := func(name string) *appsv1alpha1.ConfigurationItemDetailStatus {
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
			appsv1alpha1.ConfigurationItemDetailStatus{
				Name:           item.Name,
				Phase:          appsv1alpha1.CInitPhase,
				UpdateRevision: reversion,
			})
	}
}

func (p *pipeline) UpdatePodVolumes() *pipeline {
	return p.Wrap(func() error {
		return intctrlutil.CreateOrUpdatePodVolumes(p.ctx.PodSpec, p.renderWrapper.volumes)
	})
}

func (p *pipeline) BuildConfigManagerSidecar() *pipeline {
	return p.Wrap(func() error {
		return buildConfigManagerWithComponent(p.ctx.PodSpec, p.ctx.Component.ConfigTemplates, p.Context, p.Client, p.ctx.Cluster, p.ctx.Component)
	})
}

func (p *pipeline) UpdateConfigRelatedObject() *pipeline {
	updateMeta := func() error {
		if err := injectTemplateEnvFrom(p.ctx.Cluster, p.ctx.Component, p.ctx.PodSpec, p.Client, p.Context, p.renderWrapper.renderedObjs); err != nil {
			return err
		}
		return createConfigObjects(p.Client, p.Context, p.renderWrapper.renderedObjs)
	}

	return p.Wrap(updateMeta)
}

func (p *pipeline) createConfiguration() *appsv1alpha1.Configuration {
	builder := builder.NewConfigurationBuilder(p.Namespace,
		core.GenerateComponentConfigurationName(p.ClusterName, p.ComponentName),
	)
	for _, template := range p.ctx.Component.ConfigTemplates {
		builder.AddConfigurationItem(template)
	}
	return builder.Component(p.ComponentName).
		ClusterRef(p.ClusterName).
		AddLabelsInMap(constant.GetClusterWellKnownLabels(p.ClusterName)).
		GetObject()
}

func (p *pipeline) updateConfiguration(expected *appsv1alpha1.Configuration, existing *appsv1alpha1.Configuration) error {
	fromMap := func(items []appsv1alpha1.ConfigurationItemDetail) *cfgutil.Sets {
		sets := cfgutil.NewSet()
		for _, item := range items {
			sets.Add(item.Name)
		}
		return sets
	}

	oldSets := fromMap(existing.Spec.ConfigItemDetails)
	newSets := fromMap(expected.Spec.ConfigItemDetails)

	addSets := cfgutil.Difference(newSets, oldSets)
	delSets := cfgutil.Difference(oldSets, newSets)

	newConfigItems := make([]appsv1alpha1.ConfigurationItemDetail, 0)
	for _, item := range existing.Spec.ConfigItemDetails {
		if !delSets.InArray(item.Name) {
			newConfigItems = append(newConfigItems, item)
		}
	}
	for _, item := range expected.Spec.ConfigItemDetails {
		if addSets.InArray(item.Name) {
			newConfigItems = append(newConfigItems, item)
		}
	}

	patch := client.MergeFrom(existing)
	updated := existing.DeepCopy()
	updated.Spec.ConfigItemDetails = newConfigItems
	return p.Client.Patch(p.Context, updated, patch)
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
		templateBuilder := newTemplateBuilder(p.ClusterName, p.Namespace, p.ctx.Cluster, p.Context, p.Client, p.ctx.Cache)
		// Prepare built-in objects and built-in functions
		if err = templateBuilder.injectBuiltInObjectsAndFunctions(p.ctx.PodSpec, []appsv1alpha1.ComponentConfigSpec{*p.configSpec}, p.ctx.Component, p.ctx.Cache); err != nil {
			return
		}
		p.renderWrapper = newTemplateRenderWrapper(templateBuilder, p.ctx.Cluster, p.Context, p.Client)
		return
	}
	return p.Wrap(buildTemplate)
}

func (p *updatePipeline) ConfigSpec() *appsv1alpha1.ComponentConfigSpec {
	return p.configSpec
}

func (p *updatePipeline) InitConfigSpec() *updatePipeline {
	return p.Wrap(func() (err error) {
		if p.configSpec == nil {
			p.configSpec = component.GetConfigSpecByName(p.ctx.Component, p.item.Name)
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
			p.newCM, err = p.renderWrapper.rerenderConfigTemplate(p.ctx.Cluster, p.ctx.Component, *p.configSpec, &p.item)
		} else {
			p.newCM = p.ConfigMapObj.DeepCopy()
		}
		return
	})
}

func (p *updatePipeline) ApplyParameters() *updatePipeline {
	patchMerge := func(p *updatePipeline, spec appsv1alpha1.ComponentConfigSpec, cm *corev1.ConfigMap, item appsv1alpha1.ConfigurationItemDetail) error {
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
			p.ConfigConstraintObj.Spec.FormatterConfig.Format,
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

func (p *updatePipeline) Sync() *updatePipeline {
	return p.Wrap(func() error {
		if p.ConfigConstraintObj != nil && !p.isDone() {
			if err := SyncEnvConfigmap(*p.configSpec, p.newCM, &p.ConfigConstraintObj.Spec, p.Client, p.Context); err != nil {
				return err
			}
		}
		switch {
		case p.isDone():
			return nil
		case p.ConfigMapObj == nil && p.newCM != nil:
			return p.Client.Create(p.Context, p.newCM)
		case p.ConfigMapObj != nil:
			patch := client.MergeFrom(p.ConfigMapObj)
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
