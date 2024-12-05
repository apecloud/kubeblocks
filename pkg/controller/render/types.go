/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
	RenderConfigMapTemplate(templateSpec appsv1.ComponentTemplateSpec) (map[string]string, error)

	RenderComponentTemplate(templateSpec appsv1.ComponentTemplateSpec,
		cmName string,
		dataValidator RenderedValidator) (*corev1.ConfigMap, error)
}

type RenderedValidator = func(map[string]string) error
