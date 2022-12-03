/*
Copyright ApeCloud Inc.

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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var clusterlog = logf.Log.WithName("cluster-resource")

func (r *Cluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-dbaas-kubeblocks-io-v1alpha1-cluster,mutating=true,failurePolicy=fail,sideEffects=None,groups=dbaas.kubeblocks.io,resources=clusters,verbs=create;update,versions=v1alpha1,name=mcluster.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Cluster{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Cluster) Default() {
	clusterlog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-dbaas-kubeblocks-io-v1alpha1-cluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=dbaas.kubeblocks.io,resources=clusters,verbs=create;update,versions=v1alpha1,name=vcluster.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Cluster{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Cluster) ValidateCreate() error {
	clusterlog.Info("validate create", "name", r.Name)
	if err := r.validate(); err != nil {
		return err
	}
	return r.validateVolumeClaimTemplates(nil)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Cluster) ValidateUpdate(old runtime.Object) error {
	clusterlog.Info("validate update", "name", r.Name)
	oldCluster := old.(*Cluster)
	if oldCluster.Spec.ClusterDefRef != r.Spec.ClusterDefRef {
		return newInvalidError(ClusterKind, r.Name, "spec.clusterDefinitionRef", "clusterDefinitionRef is immutable, you can not update it. ")
	}
	if err := r.validate(); err != nil {
		return err
	}
	return r.validateVolumeClaimTemplates(oldCluster)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Cluster) ValidateDelete() error {
	clusterlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}

// validateVolumeClaimTemplates volumeClaimTemplates is forbidden modification except for storage size.
func (r *Cluster) validateVolumeClaimTemplates(oldCluster *Cluster) error {
	var allErrs field.ErrorList
	for i, component := range r.Spec.Components {
		oldComponent := getLastComponentByName(oldCluster, component.Name)
		currVCTs := component.VolumeClaimTemplates
		if oldComponent == nil {
			// if no old component, do it
			validateVolumeClaimStorage(&allErrs, nil, currVCTs, i)
			continue
		}
		oldVCTs := oldComponent.VolumeClaimTemplates
		validateVolumeClaimStorage(&allErrs, oldVCTs, currVCTs, i)
		setVolumeClaimStorageSizeZero(oldVCTs)
		setVolumeClaimStorageSizeZero(currVCTs)
		if !reflect.DeepEqual(oldVCTs, currVCTs) {
			path := fmt.Sprintf("spec.components[%d].volumeClaimTemplates", i)
			allErrs = append(allErrs, field.Invalid(field.NewPath(path),
				nil, "volumeClaimTemplates is forbidden modification except for storage size."))
		}
	}
	if len(allErrs) > 0 {
		return apierrors.NewInvalid(
			schema.GroupKind{Group: APIVersion, Kind: ClusterKind},
			r.Name, allErrs)
	}
	return nil
}

// getLastComponentByName the cluster maybe delete or add a component, so we get the component by name.
func getLastComponentByName(oldCluster *Cluster, componentName string) *ClusterComponent {
	if oldCluster == nil {
		return nil
	}
	for _, component := range oldCluster.Spec.Components {
		if component.Name == componentName {
			return &component
		}
	}
	return nil
}

// setVolumeClaimStorageSizeZero set the volumeClaimTemplates storage size to zero. then we can diff last/current volumeClaimTemplates.
func setVolumeClaimStorageSizeZero(volumeClaimTemplates []ClusterComponentVolumeClaimTemplate) {
	for i := range volumeClaimTemplates {
		volumeClaimTemplates[i].Spec.Resources = corev1.ResourceRequirements{}
	}
}

// getVCTMaps covert volume claim templates slice to map
func getVCTMaps(oldVCTs []ClusterComponentVolumeClaimTemplate) map[string]resource.Quantity {
	var oldVctMaps = map[string]resource.Quantity{}
	for _, v := range oldVCTs {
		if v.Spec == nil {
			continue
		}
		// if the request is nil, set the default value to 0
		storageSize := resource.MustParse("0")
		if v.Spec.Resources.Requests != nil {
			storageSize = v.Spec.Resources.Requests[corev1.ResourceStorage]
		}
		oldVctMaps[v.Name] = storageSize
	}
	return oldVctMaps
}

// validateVolumeClaimStorage check whether the requests.storage has a value and greater than the last value.
func validateVolumeClaimStorage(allErrs *field.ErrorList, oldVCTs, currVCTs []ClusterComponentVolumeClaimTemplate, index int) {
	oldVctMaps := getVCTMaps(oldVCTs)
	for i, v := range currVCTs {
		if v.Spec == nil {
			continue
		}
		requests := v.Spec.Resources.Requests
		path := fmt.Sprintf("spec.components[%d].volumeClaimTemplates[%d]", index, i)
		if requests == nil || requests.Storage() == nil {
			addInvalidError(allErrs, path, "", "requests.resources.storage is required")
			continue
		}
		if lastStorageSize, ok := oldVctMaps[v.Name]; ok && requests.Storage().Cmp(lastStorageSize) < 0 {
			addInvalidError(allErrs, path, "", "requests.resources.storage cannot less than last value")
		}
	}
}

// Validate Cluster.spec is legal
func (r *Cluster) validate() error {
	var (
		allErrs    field.ErrorList
		ctx        = context.Background()
		clusterDef = &ClusterDefinition{}
	)
	if webhookMgr == nil {
		return nil
	}

	r.validateAppVersionRef(&allErrs)

	err := webhookMgr.client.Get(ctx, types.NamespacedName{Name: r.Spec.ClusterDefRef}, clusterDef)

	if err != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec.clusterDefinitionRef"),
			r.Spec.ClusterDefRef, err.Error()))
	} else {
		r.validateComponents(&allErrs, clusterDef)
	}

	if len(allErrs) > 0 {
		return apierrors.NewInvalid(
			schema.GroupKind{Group: APIVersion, Kind: ClusterKind},
			r.Name, allErrs)
	}
	return nil
}

// ValidateAppVersionRef validate spec.appVersionRef is legal
func (r *Cluster) validateAppVersionRef(allErrs *field.ErrorList) {
	appVersion := &AppVersion{}
	err := webhookMgr.client.Get(context.Background(), types.NamespacedName{
		Namespace: r.Namespace,
		Name:      r.Spec.AppVersionRef,
	}, appVersion)
	if err != nil {
		*allErrs = append(*allErrs, field.Invalid(field.NewPath("spec.appVersionRef"),
			r.Spec.ClusterDefRef, err.Error()))
	}
}

// ValidateComponents validate spec.components is legal
func (r *Cluster) validateComponents(allErrs *field.ErrorList, clusterDef *ClusterDefinition) {
	var (
		// invalid component type slice
		invalidComponentTypes = make([]string, 0)
		componentNameMap      = make(map[string]struct{})
		componentTypeMap      = make(map[string]struct{})
		componentMap          = make(map[string]ClusterDefinitionComponent)
	)

	for _, v := range clusterDef.Spec.Components {
		componentTypeMap[v.TypeName] = struct{}{}
		componentMap[v.TypeName] = v
	}

	for i, v := range r.Spec.Components {
		if _, ok := componentTypeMap[v.Type]; !ok {
			invalidComponentTypes = append(invalidComponentTypes, v.Type)
		}

		componentNameMap[v.Name] = struct{}{}
		r.validateComponentResources(allErrs, v.Resources, i)
	}
	if len(invalidComponentTypes) > 0 {
		*allErrs = append(*allErrs, field.NotFound(field.NewPath("spec.components[*].type"),
			getComponentTypeNotFoundMsg(invalidComponentTypes, r.Spec.ClusterDefRef)))
	}
}

// validateComponentResources validate component resources
func (r *Cluster) validateComponentResources(allErrs *field.ErrorList, resources corev1.ResourceRequirements, index int) {
	if invalidValue, err := validateVerticalResourceList(resources.Requests); err != nil {
		*allErrs = append(*allErrs, field.Invalid(field.NewPath(fmt.Sprintf("spec.components[%d].resources.requests", index)), invalidValue, err.Error()))
	}
	if invalidValue, err := validateVerticalResourceList(resources.Limits); err != nil {
		*allErrs = append(*allErrs, field.Invalid(field.NewPath(fmt.Sprintf("spec.components[%d].resources.limits", index)), invalidValue, err.Error()))
	}
	if invalidValue, err := compareRequestsAndLimits(resources); err != nil {
		*allErrs = append(*allErrs, field.Invalid(field.NewPath(fmt.Sprintf("spec.components[%d].resources.requests", index)), invalidValue, err.Error()))
	}
}
