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

package v1

import (
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
// var clusterlog = logf.Log.WithName("cluster-resource")

func (r *Cluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

func (r *Cluster) ValidateCreate() (admission.Warnings, error) {
	return nil, r.validateInstanceTemplateLabels()
}

func (r *Cluster) ValidateUpdate(_ runtime.Object) (admission.Warnings, error) {
	return nil, r.validateInstanceTemplateLabels()
}

func (r *Cluster) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

func (r *Cluster) validateInstanceTemplateLabels() error {
	var allErrs field.ErrorList
	specPath := field.NewPath("spec")
	for i := range r.Spec.ComponentSpecs {
		allErrs = append(allErrs, validateInstanceTemplatesLabels(r.Spec.ComponentSpecs[i].Instances,
			specPath.Child("componentSpecs").Index(i).Child("instances"))...)
	}
	for i := range r.Spec.Shardings {
		shardingPath := specPath.Child("shardings").Index(i)
		allErrs = append(allErrs, validateInstanceTemplatesLabels(r.Spec.Shardings[i].Template.Instances,
			shardingPath.Child("template").Child("instances"))...)
		for j := range r.Spec.Shardings[i].ShardTemplates {
			allErrs = append(allErrs, validateInstanceTemplatesLabels(r.Spec.Shardings[i].ShardTemplates[j].Instances,
				shardingPath.Child("shardTemplates").Index(j).Child("instances"))...)
		}
	}
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(GroupVersion.WithKind("Cluster").GroupKind(), r.Name, allErrs)
}

func validateInstanceTemplatesLabels(instances []InstanceTemplate, path *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	for i := range instances {
		allErrs = append(allErrs, validateLabels(instances[i].Labels, path.Index(i).Child("labels"))...)
	}
	return allErrs
}

func validateLabels(labels map[string]string, path *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	for k, v := range labels {
		for _, msg := range validation.IsQualifiedName(k) {
			allErrs = append(allErrs, field.Invalid(path.Key(k), k, fmt.Sprintf("invalid label key: %s", msg)))
		}
		for _, msg := range validation.IsValidLabelValue(v) {
			allErrs = append(allErrs, field.Invalid(path.Key(k), v, fmt.Sprintf("invalid label value: %s", msg)))
		}
	}
	return allErrs
}
