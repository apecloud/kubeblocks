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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var componentdefinitionlog = logf.Log.WithName("componentdefinition-resource")

func (r *ComponentDefinition) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-apps-kubeblocks-io-v1alpha1-componentdefinition,mutating=true,failurePolicy=fail,sideEffects=None,groups=apps.kubeblocks.io,resources=componentdefinitions,verbs=create;update,versions=v1alpha1,name=mcomponentdefinition.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &ComponentDefinition{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *ComponentDefinition) Default() {
	componentdefinitionlog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-apps-kubeblocks-io-v1alpha1-componentdefinition,mutating=false,failurePolicy=fail,sideEffects=None,groups=apps.kubeblocks.io,resources=componentdefinitions,verbs=create;update,versions=v1alpha1,name=vcomponentdefinition.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &ComponentDefinition{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ComponentDefinition) ValidateCreate() (admission.Warnings, error) {
	componentdefinitionlog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ComponentDefinition) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	componentdefinitionlog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ComponentDefinition) ValidateDelete() (admission.Warnings, error) {
	componentdefinitionlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}
