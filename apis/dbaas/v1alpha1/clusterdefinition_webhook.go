/*
Copyright 2022 The Kubeblocks Authors

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
	"fmt"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var (
	clusterdefinitionlog = logf.Log.WithName("clusterdefinition-resource")
	componentTag         = "component"
	roleGroupTag         = "roleGroupTag"
)

func (r *ClusterDefinition) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-dbaas-infracreate-com-v1alpha1-clusterdefinition,mutating=true,failurePolicy=fail,sideEffects=None,groups=dbaas.infracreate.com,resources=clusterdefinitions,verbs=create;update,versions=v1alpha1,name=mclusterdefinition.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &ClusterDefinition{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *ClusterDefinition) Default() {
	clusterdefinitionlog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-dbaas-infracreate-com-v1alpha1-clusterdefinition,mutating=false,failurePolicy=fail,sideEffects=None,groups=dbaas.infracreate.com,resources=clusterdefinitions,verbs=create;update,versions=v1alpha1,name=vclusterdefinition.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &ClusterDefinition{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ClusterDefinition) ValidateCreate() error {
	clusterdefinitionlog.Info("validate create", "name", r.Name)
	return r.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ClusterDefinition) ValidateUpdate(old runtime.Object) error {
	clusterdefinitionlog.Info("validate update", "name", r.Name)
	return r.validate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ClusterDefinition) ValidateDelete() error {
	clusterdefinitionlog.Info("validate delete", "name", r.Name)
	return nil
}

// Validate ClusterDefinition.spec is legal
func (r *ClusterDefinition) validate() error {
	var (
		allErrs field.ErrorList
	)
	// clusterDefinition components to map
	componentMap := make(map[string]struct{})
	for _, v := range r.Spec.Components {
		componentMap[v.TypeName] = struct{}{}
	}

	fieldPath := field.NewPath("spec.cluster.strategies")
	r.validateClusterDefinitionStrategies(&allErrs, componentMap, r.Spec.Cluster.Strategies, fieldPath, componentTag, "")

	roleGroupMap := r.validateComponents(&allErrs)

	r.validateRoleGroupTemplates(&allErrs, roleGroupMap)
	if len(allErrs) > 0 {
		return apierrors.NewInvalid(
			schema.GroupKind{Group: APIVersion, Kind: ClusterDefinitionKind},
			r.Name, allErrs)
	}
	return nil
}

// ValidateClusterDefinitionStrategies validate spec.cluster.strategies is legal, including strategy.orders and cluster.components consistent
func (r *ClusterDefinition) validateClusterDefinitionStrategies(allErrs *field.ErrorList,
	m map[string]struct{},
	strategies ClusterDefinitionStrategies,
	rootField *field.Path,
	tag string,
	resourceType string) {
	var (
		strategy ClusterDefinitionStrategy
		v        = reflect.ValueOf(strategies)
		t        = reflect.TypeOf(strategies)
		filedNum = t.NumField()
	)
	for i := 0; i < filedNum; i++ {
		filedName := t.Field(i).Name
		strategy = v.FieldByName(filedName).Interface().(ClusterDefinitionStrategy)
		if strategy.Order != nil {
			orderField := rootField.Child(filedName).Child("order")
			// determine whether there is a missing type
			if len(strategy.Order) != len(m) {
				*allErrs = append(*allErrs, field.NotFound(orderField, r.getMissingMsg(tag, resourceType)))
				continue
			}
			// determine whether there is a nonexistent type
			invalidElements := r.getInvalidElementsInArray(m, strategy.Order)
			if len(invalidElements) > 0 {
				*allErrs = append(*allErrs, field.NotFound(orderField, r.getNotFoundMsg(invalidElements, tag, resourceType)))
			}
		}
	}
}

// ValidateComponents validate spec.components is legal
func (r *ClusterDefinition) validateComponents(allErrs *field.ErrorList) map[string]struct{} {
	roleGroupMap := make(map[string]struct{})
	for _, v := range r.Spec.Components {
		tmpRoleGroupMap := make(map[string]struct{})
		for _, role := range v.RoleGroups {
			roleGroupMap[role] = struct{}{}
			tmpRoleGroupMap[role] = struct{}{}
		}
		path := fmt.Sprintf("spec.components[%s].strategies", v.TypeName)
		r.validateClusterDefinitionStrategies(allErrs, tmpRoleGroupMap, v.Strategies, field.NewPath(path), roleGroupTag, v.TypeName)
	}
	return roleGroupMap
}

// ValidateRoleGroupTemplates validate spec.roleGroupTemplates is legal
func (r *ClusterDefinition) validateRoleGroupTemplates(allErrs *field.ErrorList, roleGroupMap map[string]struct{}) {
	invalidElements := make([]string, 0)
	for _, v := range r.Spec.RoleGroupTemplates {
		if _, ok := roleGroupMap[v.TypeName]; !ok {
			invalidElements = append(invalidElements, v.TypeName)
		}
	}
	if len(invalidElements) > 0 {
		*allErrs = append(*allErrs, field.NotFound(field.NewPath("spec.roleGroupTemplates"), r.getNotFoundMsg(invalidElements, roleGroupTag, "?")))
	}
}

func (r *ClusterDefinition) getInvalidElementsInArray(m map[string]struct{}, arr []string) []string {
	invalidElements := make([]string, 0)
	for _, v := range arr {
		if _, ok := m[v]; !ok {
			invalidElements = append(invalidElements, v)
		}
	}
	return invalidElements
}

func (r *ClusterDefinition) getNotFoundMsg(invalidElements []string, tag string, componentType string) string {
	if tag == componentTag {
		return fmt.Sprintf("component type %s Not Found in spec.components[*].typeName", invalidElements)
	}
	return fmt.Sprintf("roleGroup %s Not Found in spec.components[%s].roleGroups", invalidElements, componentType)

}

func (r *ClusterDefinition) getMissingMsg(tag, componentType string) string {
	if tag == componentTag {
		return "missing component types compared with spec.components[*].typeName"
	}
	return fmt.Sprintf("missing roleGroup compared with spec.components[%s].roleGroups", componentType)
}
