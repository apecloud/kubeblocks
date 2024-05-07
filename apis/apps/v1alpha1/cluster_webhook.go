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
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
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
var clusterlog = logf.Log.WithName("cluster-resource")

func (r *Cluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:path=/validate-apps-kubeblocks-io-v1alpha1-cluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=apps.kubeblocks.io,resources=clusters,verbs=create;update,versions=v1alpha1,name=vcluster.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Cluster{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Cluster) ValidateCreate() (admission.Warnings, error) {
	clusterlog.Info("validate create", "name", r.Name)
	return nil, r.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Cluster) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	clusterlog.Info("validate update", "name", r.Name)
	lastCluster := old.(*Cluster)
	if lastCluster.Spec.ClusterDefRef != r.Spec.ClusterDefRef {
		return nil, newInvalidError(ClusterKind, r.Name, "spec.clusterDefinitionRef", "clusterDefinitionRef is immutable, you can not update it. ")
	}
	if err := r.validate(); err != nil {
		return nil, err
	}
	return nil, r.validateVolumeClaimTemplates(lastCluster)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Cluster) ValidateDelete() (admission.Warnings, error) {
	clusterlog.Info("validate delete", "name", r.Name)
	if r.Spec.TerminationPolicy == DoNotTerminate {
		return nil, fmt.Errorf("the deletion for a cluster with DoNotTerminate termination policy is denied")
	}
	return nil, nil
}

// validateVolumeClaimTemplates volumeClaimTemplates is forbidden modification except for storage size.
func (r *Cluster) validateVolumeClaimTemplates(lastCluster *Cluster) error {
	var allErrs field.ErrorList
	for i, component := range r.Spec.ComponentSpecs {
		lastComponent := getLastComponentByName(lastCluster, component.Name)
		if lastComponent == nil {
			continue
		}
		setVolumeClaimStorageSizeZero(component.VolumeClaimTemplates)
		setVolumeClaimStorageSizeZero(lastComponent.VolumeClaimTemplates)
		if !reflect.DeepEqual(component.VolumeClaimTemplates, lastComponent.VolumeClaimTemplates) {
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
func getLastComponentByName(lastCluster *Cluster, componentName string) *ClusterComponentSpec {
	for _, component := range lastCluster.Spec.ComponentSpecs {
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

// Validate Cluster.spec is legal
func (r *Cluster) validate() error {
	var (
		allErrs field.ErrorList
	)
	if webhookMgr == nil {
		return nil
	}

	r.validateComponents(&allErrs)

	if len(allErrs) > 0 {
		return apierrors.NewInvalid(
			schema.GroupKind{Group: APIVersion, Kind: ClusterKind},
			r.Name, allErrs)
	}
	return nil
}

// ValidateComponents validate spec.components is legal
func (r *Cluster) validateComponents(allErrs *field.ErrorList) {
	for i, v := range r.Spec.ComponentSpecs {
		r.validateComponentResources(allErrs, v.Resources, i)
	}
	r.validateComponentTLSSettings(allErrs)
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

func (r *Cluster) validateComponentTLSSettings(allErrs *field.ErrorList) {
	for index, component := range r.Spec.ComponentSpecs {
		if !component.TLS {
			continue
		}
		if component.Issuer == nil {
			*allErrs = append(*allErrs, field.Required(field.NewPath(fmt.Sprintf("spec.components[%d].issuer", index)), "Issuer must be set when Tls enabled"))
			continue
		}
		if component.Issuer.Name == IssuerUserProvided && component.Issuer.SecretRef == nil {
			*allErrs = append(*allErrs, field.Required(field.NewPath(fmt.Sprintf("spec.components[%d].issuer.secretRef", index)), "Secret must provide when issuer name is UserProvided"))
		}
	}
}
