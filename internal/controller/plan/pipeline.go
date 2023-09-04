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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/configuration/core"
	cfgutil "github.com/apecloud/kubeblocks/internal/configuration/util"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type ReconcileCtx struct {
	context.Context

	Client     client.Client
	Cluster    *appsv1alpha1.Cluster
	ClusterVer *appsv1alpha1.ClusterVersion
	Component  *component.SynthesizedComponent
	PodSpec    *corev1.PodSpec

	obj   client.Object
	cache []client.Object
}

type pipeline struct {
	err error

	configuration *appsv1alpha1.Configuration
	renderWrapper renderWrapper
	ReconcileCtx
}

func NewPipeline(ctx ReconcileCtx) *pipeline {
	return &pipeline{ReconcileCtx: ctx}
}

func (p *pipeline) wrap(fn func() error) (ret *pipeline) {
	ret = p
	if ret.err != nil {
		return
	}
	ret.err = fn()
	return
}

func (p *pipeline) Prepare() *pipeline {
	buildTemplate := func() (err error) {
		templateBuilder := newTemplateBuilder(p.Cluster.Name, p.Cluster.Namespace, p.Cluster, p.ClusterVer, p, p.Client)
		// Prepare built-in objects and built-in functions
		if err = templateBuilder.injectBuiltInObjectsAndFunctions(p.PodSpec, p.Component.ConfigTemplates, p.Component, p.cache); err != nil {
			return
		}
		p.renderWrapper = newTemplateRenderWrapper(templateBuilder, p.Cluster, p, p.Client)
		return
	}

	return p.wrap(buildTemplate)
}

func (p *pipeline) RenderScriptTemplate() *pipeline {
	return p.wrap(func() error {
		return p.renderWrapper.renderScriptTemplate(p.Cluster, p.Component, p.cache)
	})
}

func (p *pipeline) Configuration() *pipeline {
	buildConfiguration := func() (err error) {
		expectConfiguration := p.createConfiguration()
		configuration := appsv1alpha1.Configuration{}
		err = p.Client.Get(p, client.ObjectKeyFromObject(expectConfiguration), &configuration)
		switch {
		case err == nil:
			return p.updateConfiguration(&configuration, expectConfiguration)
		case !apierrors.IsNotFound(err):
			return p.Client.Create(p, expectConfiguration)
		default:
			return err
		}
	}
	return p.wrap(buildConfiguration)
}

func (p *pipeline) CreateConfigTemplate() *pipeline {
	return p.wrap(func() error {
		return p.renderWrapper.renderConfigTemplate(p.Cluster, p.Component, p.cache)
	})
}

func (p *pipeline) RerenderTemplate(string) *pipeline {
	return p.wrap(func() error {
		return p.renderWrapper.renderConfigTemplate(p.Cluster, p.Component, p.cache)
	})
}

func (p *pipeline) UpdateConfigTemplate(configSpec string, status *appsv1alpha1.ConfigurationItemDetailStatus) *pipeline {
	return p.wrap(func() error {
		var i int
		templates := p.Component.ConfigTemplates
		for i = 0; i < len(templates); i++ {
			if templates[i].Name == configSpec {
				break
			}
		}
		if i >= len(templates) {
			return core.MakeError("not found config spec: %s", configSpec)
		}
		return p.renderWrapper.updateConfigTemplate(p.Cluster, p.Component, templates[i], status)
	})
}

func (p *pipeline) UpdateConfigurationStatus() *pipeline {
	return p.wrap(func() error {
		if p.configuration != nil {
			return nil
		}

		patch := client.MergeFrom(p.configuration)
		updated := p.configuration.DeepCopy()
		for _, item := range p.configuration.Spec.ConfigItemDetails {
			checkAndUpdateItemStatus(updated, item)
		}
		return p.Client.Status().Patch(p, updated, patch)
	})
}

func checkAndUpdateItemStatus(updated *appsv1alpha1.Configuration, item appsv1alpha1.ConfigurationItemDetail) {
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
	if status == nil {
		updated.Status.ConfigurationItemStatus = append(updated.Status.ConfigurationItemStatus,
			appsv1alpha1.ConfigurationItemDetailStatus{
				Name:  item.Name,
				Phase: appsv1alpha1.CInitPhase,
			})
	}
}

func (p *pipeline) UpdatePodVolumes() *pipeline {
	return p.wrap(func() error {
		return intctrlutil.CreateOrUpdatePodVolumes(p.PodSpec, p.renderWrapper.volumes)
	})
}

func (p *pipeline) BuildConfigManagerSidecar() *pipeline {
	return p.wrap(func() error {
		return buildConfigManagerWithComponent(p.PodSpec, p.Component.ConfigTemplates, p, p.Client, p.Cluster, p.Component)
	})
}

func (p *pipeline) UpdateConfigMeta() *pipeline {
	updateMeta := func() error {
		updateResourceAnnotationsWithTemplate(p.obj, p.renderWrapper.templateAnnotations)
		if err := injectTemplateEnvFrom(p.Cluster, p.Component, p.PodSpec, p.Client, p, p.renderWrapper.renderedObjs); err != nil {
			return err
		}
		return createConfigObjects(p.Client, p, p.renderWrapper.renderedObjs)
	}

	return p.wrap(updateMeta)
}

func (p *pipeline) Complete() error {
	return p.err
}

func (p *pipeline) createConfiguration() *appsv1alpha1.Configuration {
	builder := builder.NewConfigurationBuilder(p.Cluster.Namespace,
		core.GenerateComponentConfigurationName(p.Cluster.Name, p.Component.Name),
	)

	for _, template := range p.Component.ConfigTemplates {
		builder.AddConfigurationItem(template.Name)
	}
	return builder.Component(p.Component.Name).
		ClusterRef(p.Cluster.Name).
		ClusterVerRef(p.ClusterVer.Name).
		ClusterDefRef(p.Component.ClusterDefName).
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
	return p.Client.Patch(p, updated, patch)
}
