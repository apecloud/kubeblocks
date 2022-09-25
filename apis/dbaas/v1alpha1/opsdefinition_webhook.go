/*
Copyright 2022.

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
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var opsdefinitionlog = logf.Log.WithName("opsdefinition-resource")

func (r *OpsDefinition) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-dbaas-infracreate-com-v1alpha1-opsdefinition,mutating=true,failurePolicy=fail,sideEffects=None,groups=dbaas.infracreate.com,resources=opsdefinitions,verbs=create;update,versions=v1alpha1,name=mopsdefinition.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &OpsDefinition{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *OpsDefinition) Default() {
	opsdefinitionlog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-dbaas-infracreate-com-v1alpha1-opsdefinition,mutating=false,failurePolicy=fail,sideEffects=None,groups=dbaas.infracreate.com,resources=opsdefinitions,verbs=create;update,versions=v1alpha1,name=vopsdefinition.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &OpsDefinition{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *OpsDefinition) ValidateCreate() error {
	opsdefinitionlog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return r.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *OpsDefinition) ValidateUpdate(old runtime.Object) error {
	opsdefinitionlog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return r.validate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *OpsDefinition) ValidateDelete() error {
	opsdefinitionlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}

func (r *OpsDefinition) validate() error {
	var (
		clusterDef = &ClusterDefinition{}
		allErrs    field.ErrorList
	)
	if webhookMgr == nil {
		return nil
	}
	if err := webhookMgr.client.Get(context.Background(), types.NamespacedName{
		Namespace: r.Namespace, Name: r.Spec.ClusterDefinitionRef,
	}, clusterDef); err != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec.clusterDefinitionRef"),
			r.Spec.ClusterDefinitionRef, err.Error()))
	} else {
		r.validateStrategyComponents(clusterDef, &allErrs)
	}

	if len(allErrs) > 0 {
		return apierrors.NewInvalid(
			schema.GroupKind{Group: APIVersion, Kind: OpsRequestKind},
			r.Name, allErrs)
	}
	return nil
}

func (r *OpsDefinition) validateStrategyComponents(clusterDef *ClusterDefinition, allErrs *field.ErrorList) {
	var (
		strategy              = r.Spec.Strategy
		componentMap          = map[string]*ClusterDefinitionComponent{}
		notFoundComponentType []string
	)
	// get component type map of clusterDefinition
	for _, v := range clusterDef.Spec.Components {
		componentMap[v.TypeName] = &v
	}
	// get component types where is not exists in Cluster.spec.components[*].typeName
	if strategy == nil || strategy.Components == nil {
		return
	}
	for _, v := range strategy.Components {
		if _, ok := componentMap[v.Type]; !ok {
			notFoundComponentType = append(notFoundComponentType, v.Type)
		}
	}
	if len(notFoundComponentType) > 0 {
		*allErrs = append(*allErrs, field.NotFound(field.NewPath("spec.Strategy.Components[*].type"),
			fmt.Sprintf("%v is not found in Cluster.spec.components[*].typeName", notFoundComponentType)))
	}
}
