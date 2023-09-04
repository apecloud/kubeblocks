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

type reconcileCtx struct {
	context.Context

	cli        client.Client
	cluster    *appsv1alpha1.Cluster
	clusterVer *appsv1alpha1.ClusterVersion
	component  *component.SynthesizedComponent

	obj     client.Object
	cache   []client.Object
	podSpec *corev1.PodSpec
}

type pipeline struct {
	err error

	configuration *appsv1alpha1.Configuration
	renderWrapper renderWrapper
	reconcileCtx
}

func newPipeline(ctx reconcileCtx) *pipeline {
	return &pipeline{reconcileCtx: ctx}
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
		templateBuilder := newTemplateBuilder(p.cluster.Name, p.cluster.Namespace, p.cluster, p.clusterVer, p, p.cli)
		// Prepare built-in objects and built-in functions
		if err = templateBuilder.injectBuiltInObjectsAndFunctions(p.podSpec, p.component.ConfigTemplates, p.component, p.cache); err != nil {
			return
		}
		p.renderWrapper = newTemplateRenderWrapper(templateBuilder, p.cluster, p, p.cli)
		return
	}

	return p.wrap(buildTemplate)
}

func (p *pipeline) RenderScriptTemplate() *pipeline {
	return p.wrap(func() error {
		return p.renderWrapper.renderScriptTemplate(p.cluster, p.component, p.cache)
	})
}

func (p *pipeline) Configuration() *pipeline {
	buildConfiguration := func() (err error) {
		expectConfiguration := p.createConfiguration()
		configuration := appsv1alpha1.Configuration{}
		err = p.cli.Get(p, client.ObjectKeyFromObject(expectConfiguration), &configuration)
		switch {
		case err == nil:
			return p.updateConfiguration(&configuration, expectConfiguration)
		case !apierrors.IsNotFound(err):
			return p.cli.Create(p, expectConfiguration)
		default:
			return err
		}
	}
	return p.wrap(buildConfiguration)
}

func (p *pipeline) CreateConfigTemplate() *pipeline {
	return p.wrap(func() error {
		return p.renderWrapper.renderConfigTemplate(p.cluster, p.component, p.cache)
	})
}

func (p *pipeline) UpdateConfigurationStatus() *pipeline {
	return p.wrap(func() error {
		if p.configuration != nil {
			return nil
		}

		// TODO update configuration status
		return nil
	})
}

func (p *pipeline) UpdatePodVolumes() *pipeline {
	return p.wrap(func() error {
		return intctrlutil.CreateOrUpdatePodVolumes(p.podSpec, p.renderWrapper.volumes)
	})
}

func (p *pipeline) BuildConfigManagerSidecar() *pipeline {
	return p.wrap(func() error {
		return buildConfigManagerWithComponent(p.podSpec, p.component.ConfigTemplates, p, p.cli, p.cluster, p.component)
	})
}

func (p *pipeline) Complete() error {
	if p.err != nil {
		return p.err
	}

	updateResourceAnnotationsWithTemplate(p.obj, p.renderWrapper.templateAnnotations)
	if err := injectTemplateEnvFrom(p.cluster, p.component, p.podSpec, p.cli, p, p.renderWrapper.renderedObjs); err != nil {
		return err
	}
	return createConfigObjects(p.cli, p, p.renderWrapper.renderedObjs)
}

func (p *pipeline) createConfiguration() *appsv1alpha1.Configuration {
	builder := builder.NewConfigurationBuilder(p.cluster.Namespace,
		core.GenerateComponentConfigurationName(p.cluster.Name, p.component.Name),
	)

	for _, template := range p.component.ConfigTemplates {
		builder.AddConfigurationItem(template.Name)
	}
	return builder.Component(p.component.Name).
		ClusterRef(p.cluster.Name).
		ClusterVerRef(p.clusterVer.Name).
		ClusterDefRef(p.component.ClusterDefName).
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
	return p.cli.Patch(p, updated, patch)
}
