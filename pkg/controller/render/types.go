/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package render

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
)

type ResourceCtx struct {
	context.Context

	Err    error
	Client client.Client

	Namespace     string
	ClusterName   string
	ComponentName string
}

type ReconcileCtx struct {
	*ResourceCtx

	Cluster              *appsv1.Cluster
	Component            *appsv1.Component
	SynthesizedComponent *component.SynthesizedComponent
	PodSpec              *corev1.PodSpec

	Cache []client.Object
}

type TemplateRender interface {
	// RenderConfigMapTemplate renders a ConfigMap template based on the provided specification.
	//
	// Parameters:
	// - templateSpec: The specification for the component template.
	//
	// Returns:
	// - A map containing the rendered template data.
	// - An error if the rendering fails.
	RenderConfigMapTemplate(templateSpec appsv1.ComponentFileTemplate) (map[string]string, error)

	// RenderComponentTemplate renders a component template and validates the rendered data.
	//
	// Parameters:
	// - templateSpec: The specification for the component template.
	// - cmName: The name of the ConfigMap.
	// - dataValidator: A function to validate the rendered data.
	//
	// Returns:
	// - A pointer to the rendered ConfigMap.
	// - An error if the rendering or validation fails.
	RenderComponentTemplate(templateSpec appsv1.ComponentFileTemplate,
		cmName string,
		dataValidator RenderedValidator) (*corev1.ConfigMap, error)
}

type RenderedValidator = func(map[string]string) error
