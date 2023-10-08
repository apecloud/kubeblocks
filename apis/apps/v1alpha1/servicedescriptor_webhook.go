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
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var servicedescriptorlog = logf.Log.WithName("servicedescriptor-resource")

func (r *ServiceDescriptor) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-apps-kubeblocks-io-v1alpha1-servicedescriptor,mutating=true,failurePolicy=fail,sideEffects=None,groups=apps.kubeblocks.io,resources=servicedescriptors,verbs=create;update,versions=v1alpha1,name=mservicedescriptor.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &ServiceDescriptor{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *ServiceDescriptor) Default() {
	servicedescriptorlog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-apps-kubeblocks-io-v1alpha1-servicedescriptor,mutating=false,failurePolicy=fail,sideEffects=None,groups=apps.kubeblocks.io,resources=servicedescriptors,verbs=create;update,versions=v1alpha1,name=vservicedescriptor.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &ServiceDescriptor{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ServiceDescriptor) ValidateCreate() (admission.Warnings, error) {
	servicedescriptorlog.Info("validate create", "name", r.Name)

	return nil, r.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ServiceDescriptor) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	servicedescriptorlog.Info("validate update", "name", r.Name)

	return nil, r.validate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ServiceDescriptor) ValidateDelete() (admission.Warnings, error) {
	servicedescriptorlog.Info("validate delete", "name", r.Name)

	return nil, r.validate()
}

func (r *ServiceDescriptor) validate() error {
	var allErrs field.ErrorList

	checkValueAndValueFrom := func(filed string, cvs ...*CredentialVar) {
		if len(cvs) == 0 {
			return
		}
		for _, cv := range cvs {
			if cv == nil {
				continue
			}
			if cv.Value != "" && cv.ValueFrom != nil {
				allErrs = append(allErrs,
					field.Forbidden(field.NewPath("ServiceDescriptor filed").Child(filed),
						"value and valueFrom cannot be specified at the same time"))
			}
		}
	}

	checkValueAndValueFrom("auth", r.Spec.Auth.Username, r.Spec.Auth.Password)
	checkValueAndValueFrom("endpoint", r.Spec.Endpoint)
	checkValueAndValueFrom("port", r.Spec.Port)

	if len(allErrs) > 0 {
		return apierrors.NewInvalid(
			schema.GroupKind{
				Group: "apps.kubeblocks.io/v1alpha1",
				Kind:  "ServiceDescriptor",
			},
			r.Name, allErrs)
	}
	return nil
}
