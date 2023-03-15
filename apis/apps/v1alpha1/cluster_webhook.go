/*
Copyright ApeCloud, Inc.

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

//+kubebuilder:webhook:path=/mutate-apps-kubeblocks-io-v1alpha1-cluster,mutating=true,failurePolicy=fail,sideEffects=None,groups=apps.kubeblocks.io,resources=clusters,verbs=create;update,versions=v1alpha1,name=mcluster.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Cluster{}

// Default set default value by implements webhook.Defaulter so a webhook will be registered for the type
func (r *Cluster) Default() {
	clusterlog.Info("default", "name", r.Name)
	var (
		ctx        = context.Background()
		clusterDef = &ClusterDefinition{}
	)

	_ = webhookMgr.client.Get(ctx, types.NamespacedName{Name: r.Spec.ClusterDefRef}, clusterDef)
	for i := range r.Spec.ComponentSpecs {
		comSpec := &r.Spec.ComponentSpecs[i]
		if comSpec.PrimaryIndex != nil {
			continue
		}
		for _, compDef := range clusterDef.Spec.ComponentDefs {
			if compDef.WorkloadType != Replication || comSpec.ComponentDefRef != compDef.Name {
				continue
			}
			comSpec.PrimaryIndex = new(int32)
		}
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:path=/validate-apps-kubeblocks-io-v1alpha1-cluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=apps.kubeblocks.io,resources=clusters,verbs=create;update,versions=v1alpha1,name=vcluster.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Cluster{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Cluster) ValidateCreate() error {
	clusterlog.Info("validate create", "name", r.Name)
	return r.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Cluster) ValidateUpdate(old runtime.Object) error {
	clusterlog.Info("validate update", "name", r.Name)
	lastCluster := old.(*Cluster)
	if lastCluster.Spec.ClusterDefRef != r.Spec.ClusterDefRef {
		return newInvalidError(ClusterKind, r.Name, "spec.clusterDefinitionRef", "clusterDefinitionRef is immutable, you can not update it. ")
	}
	if err := r.validate(); err != nil {
		return err
	}
	return r.validateVolumeClaimTemplates(lastCluster)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Cluster) ValidateDelete() error {
	clusterlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}

// validatePrimaryIndex checks primaryIndex value cannot be larger than replicas.
func (r *Cluster) validatePrimaryIndex(allErrs *field.ErrorList) {
	for index, component := range r.Spec.ComponentSpecs {
		if component.PrimaryIndex == nil || component.Replicas == 0 {
			continue
		}
		if *component.PrimaryIndex > component.Replicas-1 {
			path := fmt.Sprintf("spec.components[%d].PrimaryIndex", index)
			*allErrs = append(*allErrs, field.Invalid(field.NewPath(path),
				*component.PrimaryIndex, "PrimaryIndex cannot be larger than Replicas."))
		}
	}
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
		if volumeClaimTemplates[i].Spec == nil {
			continue
		}
		volumeClaimTemplates[i].Spec.Resources = corev1.ResourceRequirements{}
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

	r.validateClusterVersionRef(&allErrs)

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

// ValidateClusterVersionRef validate spec.clusterVersionRef is legal
func (r *Cluster) validateClusterVersionRef(allErrs *field.ErrorList) {
	clusterVersion := &ClusterVersion{}
	err := webhookMgr.client.Get(context.Background(), types.NamespacedName{
		Namespace: r.Namespace,
		Name:      r.Spec.ClusterVersionRef,
	}, clusterVersion)
	if err != nil {
		*allErrs = append(*allErrs, field.Invalid(field.NewPath("spec.clusterVersionRef"),
			r.Spec.ClusterDefRef, err.Error()))
	}
}

// ValidateComponents validate spec.components is legal
func (r *Cluster) validateComponents(allErrs *field.ErrorList, clusterDef *ClusterDefinition) {
	var (
		// invalid component slice
		invalidComponentDefs = make([]string, 0)
		componentNameMap     = make(map[string]struct{})
		componentDefMap      = make(map[string]struct{})
		componentMap         = make(map[string]ClusterComponentDefinition)
	)

	for _, v := range clusterDef.Spec.ComponentDefs {
		componentDefMap[v.Name] = struct{}{}
		componentMap[v.Name] = v
	}

	for i, v := range r.Spec.ComponentSpecs {
		if _, ok := componentDefMap[v.ComponentDefRef]; !ok {
			invalidComponentDefs = append(invalidComponentDefs, v.ComponentDefRef)
		}

		componentNameMap[v.Name] = struct{}{}
		r.validateComponentResources(allErrs, v.Resources, i)
	}

	r.validatePrimaryIndex(allErrs)

	r.validateComponentTLSSettings(allErrs)

	if len(invalidComponentDefs) > 0 {
		*allErrs = append(*allErrs, field.NotFound(field.NewPath("spec.components[*].type"),
			getComponentDefNotFoundMsg(invalidComponentDefs, r.Spec.ClusterDefRef)))
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
