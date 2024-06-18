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
	checkValueAndValueFrom("host", r.Spec.Host)
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
