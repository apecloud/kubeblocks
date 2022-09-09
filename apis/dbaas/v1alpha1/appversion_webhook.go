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
	"reflect"

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
var appversionlog = logf.Log.WithName("appversion-resource")

func (r *AppVersion) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-dbaas-infracreate-com-v1alpha1-appversion,mutating=true,failurePolicy=fail,sideEffects=None,groups=dbaas.infracreate.com,resources=appversions,verbs=create;update,versions=v1alpha1,name=mappversion.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &AppVersion{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *AppVersion) Default() {
	appversionlog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-dbaas-infracreate-com-v1alpha1-appversion,mutating=false,failurePolicy=fail,sideEffects=None,groups=dbaas.infracreate.com,resources=appversions,verbs=create;update,versions=v1alpha1,name=vappversion.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &AppVersion{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *AppVersion) ValidateCreate() error {
	appversionlog.Info("validate create", "name", r.Name)
	return r.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *AppVersion) ValidateUpdate(old runtime.Object) error {
	appversionlog.Info("validate update", "name", r.Name)
	// determine whether r.spec content is modified
	lastAppVersion := old.(*AppVersion)
	if !reflect.DeepEqual(lastAppVersion.Spec, r.Spec) {
		return newInvalidError(AppVersionKind, r.Name, "", "AppVersion.spec is immutable, you can not update it.")
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *AppVersion) ValidateDelete() error {
	appversionlog.Info("validate delete", "name", r.Name)
	return nil
}

// Validate AppVersion.spec is legal
func (r *AppVersion) validate() error {
	var (
		allErrs    field.ErrorList
		ctx        = context.Background()
		clusterDef = &ClusterDefinition{}
	)
	if webhookMgr == nil {
		return nil
	}
	err := webhookMgr.client.Get(ctx, types.NamespacedName{
		Namespace: r.Namespace,
		Name:      r.Spec.ClusterDefinitionRef,
	}, clusterDef)

	if err != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec.clusterDefinitionRef"),
			r.Spec.ClusterDefinitionRef, err.Error()))
	} else {
		notFoundComponentTypes, noContainersComponents := r.GetInconsistentComponentsInfo(clusterDef)

		if len(notFoundComponentTypes) > 0 {
			allErrs = append(allErrs, field.NotFound(field.NewPath("spec.components[*].type"),
				getComponentTypeNotFoundMsg(notFoundComponentTypes, r.Spec.ClusterDefinitionRef)))
		}

		if len(noContainersComponents) > 0 {
			allErrs = append(allErrs, field.NotFound(field.NewPath("spec.components[*].type"),
				fmt.Sprintf("spec.components[*].type %v missing spec.components[*].containers in ClusterDefinition.spec.components[*] and AppVersion.spec.components[*]", noContainersComponents)))
		}
	}

	if len(allErrs) > 0 {
		return apierrors.NewInvalid(
			schema.GroupKind{Group: APIVersion, Kind: AppVersionKind},
			r.Name, allErrs)
	}
	return nil
}

// GetInconsistentComponentsInfo get appVersion invalid component type and no containers component compared with clusterDefinitionDef
func (r *AppVersion) GetInconsistentComponentsInfo(clusterDef *ClusterDefinition) ([]string, []string) {

	var (
		// clusterDefinition components to map. the value of map represents whether there is a default containers
		componentMap          = map[string]bool{}
		notFoundComponentType = make([]string, 0)
		noContainersComponent = make([]string, 0)
	)

	for _, v := range clusterDef.Spec.Components {
		if v.PodSpec == nil || v.PodSpec.Containers == nil || len(v.PodSpec.Containers) == 0 {
			componentMap[v.TypeName] = false
		} else {
			componentMap[v.TypeName] = true
		}
	}
	// get not found component type in clusterDefinition
	for _, v := range r.Spec.Components {
		if _, ok := componentMap[v.Type]; !ok {
			notFoundComponentType = append(notFoundComponentType, v.Type)
		} else {
			if v.PodSpec.Containers != nil && len(v.PodSpec.Containers) > 0 {
				componentMap[v.Type] = true
			}
		}
	}
	// get no containers components in clusterDefinition and appVersion
	for k, v := range componentMap {
		if !v {
			noContainersComponent = append(noContainersComponent, k)
		}
	}
	return notFoundComponentType, noContainersComponent
}

func getComponentTypeNotFoundMsg(invalidComponentTypes []string, clusterDefName string) string {
	return fmt.Sprintf(" %v is not found in ClusterDefinition.spec.components[*].typeName %s",
		invalidComponentTypes, clusterDefName)
}

// NewInvalidError create an invalid api error
func newInvalidError(kind, resourceName, path, reason string) error {
	return apierrors.NewInvalid(schema.GroupKind{Group: APIVersion, Kind: kind},
		resourceName, field.ErrorList{field.InternalError(field.NewPath(path),
			fmt.Errorf(reason))})
}
