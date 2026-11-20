/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package component

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

// ResolveFileTemplateVars resolves file variables using the instance template's
// resource and service version overrides for references to the current Component.
// A nil instance resolves the Component's variables without overrides.
func ResolveFileTemplateVars(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent,
	definedVars []appsv1.EnvVar, instance *appsv1.InstanceTemplate) (map[string]string, error) {
	vars, _, err := resolveTemplateNEnvVars(ctx, cli, synthesizedComp, definedVars, instance)
	return vars, err
}

func componentWithInstanceTemplateOverrides(component *appsv1.Component, instance *appsv1.InstanceTemplate) *appsv1.Component {
	comp := component.DeepCopy()
	if instance.Resources != nil {
		merge := func(src corev1.ResourceList, dst *corev1.ResourceList) {
			for _, name := range []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory} {
				if value, ok := src[name]; ok {
					if *dst == nil {
						*dst = make(corev1.ResourceList)
					}
					(*dst)[name] = value.DeepCopy()
				}
			}
		}
		merge(instance.Resources.Requests, &comp.Spec.Resources.Requests)
		merge(instance.Resources.Limits, &comp.Spec.Resources.Limits)
	}
	for _, claim := range instance.VolumeClaimTemplates {
		found := false
		for i := range comp.Spec.VolumeClaimTemplates {
			if comp.Spec.VolumeClaimTemplates[i].Name == claim.Name {
				comp.Spec.VolumeClaimTemplates[i] = *claim.DeepCopy()
				found = true
				break
			}
		}
		if !found {
			comp.Spec.VolumeClaimTemplates = append(comp.Spec.VolumeClaimTemplates, *claim.DeepCopy())
		}
	}
	if instance.ServiceVersion != "" {
		comp.Spec.ServiceVersion = instance.ServiceVersion
	}
	return comp
}
