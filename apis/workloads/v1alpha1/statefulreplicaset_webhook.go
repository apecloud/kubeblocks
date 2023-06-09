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

package v1alpha1

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var statefulreplicasetlog = logf.Log.WithName("statefulreplicaset-resource")

func (r *StatefulReplicaSet) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-workloads-kubeblocks-io-v1alpha1-statefulreplicaset,mutating=true,failurePolicy=fail,sideEffects=None,groups=workloads.kubeblocks.io,resources=statefulreplicasets,verbs=create;update,versions=v1alpha1,name=mstatefulreplicaset.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &StatefulReplicaSet{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *StatefulReplicaSet) Default() {
	statefulreplicasetlog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-workloads-kubeblocks-io-v1alpha1-statefulreplicaset,mutating=false,failurePolicy=fail,sideEffects=None,groups=workloads.kubeblocks.io,resources=statefulreplicasets,verbs=create;update,versions=v1alpha1,name=vstatefulreplicaset.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &StatefulReplicaSet{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *StatefulReplicaSet) ValidateCreate() error {
	statefulreplicasetlog.Info("validate create", "name", r.Name)

	return r.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *StatefulReplicaSet) ValidateUpdate(old runtime.Object) error {
	statefulreplicasetlog.Info("validate update", "name", r.Name)

	return r.validate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *StatefulReplicaSet) ValidateDelete() error {
	statefulreplicasetlog.Info("validate delete", "name", r.Name)

	return r.validate()
}

func (r *StatefulReplicaSet) validate() error {
	var allErrs field.ErrorList

	// Leader is required
	hasHeader := false
	for _, role := range r.Spec.Roles {
		if role.IsLeader && len(role.Name) > 0 {
			hasHeader = true
		}
	}
	if !hasHeader {
		allErrs = append(allErrs,
			field.Required(field.NewPath("spec.roles"),
				"leader is required"))
	}

	// servicePort must provide
	if len(r.Spec.Service.Ports) == 0 {
		allErrs = append(allErrs,
			field.Required(field.NewPath("spec.service.ports"),
				"servicePort must provide"))
	}

	if len(allErrs) > 0 {
		return apierrors.NewInvalid(
			schema.GroupKind{
				Group: "workloads.kubeblocks.io/v1alpha1",
				Kind:  "StatefulReplicaSet",
			},
			r.Name, allErrs)
	}

	return nil
}
