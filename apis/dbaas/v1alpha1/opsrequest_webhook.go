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
	"strings"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var opsrequestlog = logf.Log.WithName("opsrequest-resource")

func (r *OpsRequest) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-dbaas-infracreate-com-v1alpha1-opsrequest,mutating=true,failurePolicy=fail,sideEffects=None,groups=dbaas.infracreate.com,resources=opsrequests,verbs=create;update,versions=v1alpha1,name=mopsrequest.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &OpsRequest{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *OpsRequest) Default() {
	opsrequestlog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-dbaas-infracreate-com-v1alpha1-opsrequest,mutating=false,failurePolicy=fail,sideEffects=None,groups=dbaas.infracreate.com,resources=opsrequests,verbs=create;update,versions=v1alpha1,name=vopsrequest.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &OpsRequest{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *OpsRequest) ValidateCreate() error {
	opsrequestlog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return r.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *OpsRequest) ValidateUpdate(old runtime.Object) error {
	opsrequestlog.Info("validate update", "name", r.Name)
	if slices.Index([]Phase{RunningPhase, SucceedPhase}, r.Status.Phase) != -1 {
		return newInvalidError(OpsRequestKind, r.Name, "status.phase", fmt.Sprintf("can not update OpsRequest when status.Phase is %s", r.Status.Phase))
	}
	return r.validate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *OpsRequest) ValidateDelete() error {
	opsrequestlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}

func (r *OpsRequest) validate() error {
	var (
		allErrs field.ErrorList
		ctx     = context.Background()
		cluster = &Cluster{}
	)
	if webhookMgr == nil {
		return nil
	}
	// get cluster resource
	if err := webhookMgr.client.Get(ctx, types.NamespacedName{Namespace: r.Namespace, Name: r.Spec.ClusterRef}, cluster); err != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec.clusterRef"), r.Spec.ClusterRef, err.Error()))
	} else {
		r.validateOps(ctx, cluster, &allErrs)
	}
	if len(allErrs) > 0 {
		return apierrors.NewInvalid(schema.GroupKind{Group: APIVersion, Kind: OpsRequestKind}, r.Name, allErrs)
	}
	return nil
}

func (r *OpsRequest) getOpsDefinition(ctx context.Context, cluster *Cluster) (*OpsDefinition, error) {
	var (
		opsDefinitionList = &OpsDefinitionList{}
		opsDefinition     = &OpsDefinition{}
	)
	if err := webhookMgr.client.List(ctx, opsDefinitionList, client.MatchingLabels{ClusterDefLabelKey: cluster.Spec.ClusterDefRef}); err != nil {
		return nil, err
	}
	for _, v := range opsDefinitionList.Items {
		if v.Spec.Type == r.Spec.Type {
			opsDefinition = &v
			break
		}
	}
	return opsDefinition, nil
}

// validateOps validate ops attributes is legal
func (r *OpsRequest) validateOps(ctx context.Context, cluster *Cluster, allErrs *field.ErrorList) {
	var (
		err           error
		opsDefinition *OpsDefinition
	)
	if opsDefinition, err = r.getOpsDefinition(ctx, cluster); err != nil {
		*allErrs = append(*allErrs, field.Invalid(field.NewPath("spec.type"), r.Spec.Type, err.Error()))
		return
	}
	if opsDefinition == nil {
		*allErrs = append(*allErrs, field.Invalid(field.NewPath("spec.type"), r.Spec.Type,
			fmt.Sprintf("%s don't supported in spec.clusterDef %s", r.Spec.Type, r.Spec.ClusterRef)))
		return
	}
	// Check whether the corresponding attribute is legal according to the operation type
	switch r.Spec.Type {
	case UpgradeType:
		r.validateUpgrade(ctx, cluster, allErrs)
	case VerticalScalingType:
		r.validateVerticalScaling(opsDefinition, cluster, allErrs)
	default:
		r.validateComponentOps(opsDefinition, cluster, allErrs)
	}
}

// validateVerticalScaling validate spec.componentOps.verticalScaling is legal
func (r *OpsRequest) validateVerticalScaling(opsDef *OpsDefinition, cluster *Cluster, allErrs *field.ErrorList) {
	if ok := r.validateComponentOps(opsDef, cluster, allErrs); !ok {
		return
	}
	verticalScaling := r.Spec.ComponentOps.VerticalScaling
	if err := validateVerticalResourceList(verticalScaling.Requests); err != nil {
		*allErrs = append(*allErrs, field.Invalid(field.NewPath("spec.componentOps.verticalScaling.requests"),
			r.Spec.ComponentOps.VerticalScaling.Requests, err.Error()))
	}
	if err := validateVerticalResourceList(verticalScaling.Limits); err != nil {
		*allErrs = append(*allErrs, field.Invalid(field.NewPath("spec.componentOps.verticalScaling.limits"),
			r.Spec.ComponentOps.VerticalScaling.Limits, err.Error()))
	}
}

// validateUpgrade validate spec.clusterOps.upgrade is legal
func (r *OpsRequest) validateUpgrade(ctx context.Context, cluster *Cluster, allErrs *field.ErrorList) {
	if r.Spec.ClusterOps == nil || r.Spec.ClusterOps.Upgrade == nil {
		*allErrs = append(*allErrs, field.NotFound(field.NewPath("spec.clusterOps.upgrade"), ""))
		return
	}
	appVersion := &AppVersion{}
	if err := webhookMgr.client.Get(ctx, types.NamespacedName{
		Namespace: r.Namespace,
		Name:      r.Spec.ClusterOps.Upgrade.AppVersionRef,
	}, appVersion); err != nil {
		*allErrs = append(*allErrs, field.Invalid(field.NewPath("spec.clusterOps.upgrade.appVersionRef"),
			r.Spec.ClusterOps.Upgrade.AppVersionRef, err.Error()))

	} else if cluster.Spec.AppVersionRef == r.Spec.ClusterOps.Upgrade.AppVersionRef {
		*allErrs = append(*allErrs, field.Invalid(field.NewPath("spec.clusterOps.upgrade.appVersionRef"),
			r.Spec.ClusterOps.Upgrade.AppVersionRef, "the appVersionRef is equals Cluster.spec.appVersionRef"))
	}
}

// checkComponentOps check spec.componentOps.componentNames is legal
func (r *OpsRequest) validateComponentOps(opsDef *OpsDefinition, cluster *Cluster, allErrs *field.ErrorList) bool {
	var (
		clusterComponentNameMap = map[string]string{}
		supportComponentType    = map[string]struct{}{}
		NotFoundComponentName   []string
		invalidComponentName    []string
	)

	if r.Spec.ComponentOps == nil {
		*allErrs = append(*allErrs, field.Invalid(field.NewPath("spec.componentOps"), r.Spec.ComponentOps, "can not be null"))
		return false
	}
	if opsDef.Spec.Strategy == nil {
		return false
	}
	for _, v := range cluster.Spec.Components {
		clusterComponentNameMap[v.Name] = v.Type
	}
	for _, c := range opsDef.Spec.Strategy.Components {
		supportComponentType[c.Type] = struct{}{}
	}
	for _, v := range r.Spec.ComponentOps.ComponentNames {
		// check component name whether exist in Cluster.spec.components[*].name
		typeName, ok := clusterComponentNameMap[v]
		if !ok {
			NotFoundComponentName = append(NotFoundComponentName, v)
			continue
		}
		// check component type is supported by OpsDefinition.spec.Strategy.Components[*].type
		if _, ok = supportComponentType[typeName]; !ok {
			invalidComponentName = append(invalidComponentName, typeName)
		}
	}

	if len(NotFoundComponentName) > 0 {
		*allErrs = append(*allErrs, field.NotFound(field.NewPath("spec.componentOps.componentNames"),
			fmt.Sprintf("%v is not found in Cluster.spec.components[*].name", NotFoundComponentName)))
	}

	if len(invalidComponentName) > 0 {
		*allErrs = append(*allErrs, field.Invalid(field.NewPath("spec.componentOps.componentNames"), invalidComponentName,
			"not supported in OpsDefinition.spec.Strategy.Components[*].type"))
	}
	return true
}

// checkResourceList check k8s resourceList is legal
func validateVerticalResourceList(resourceList map[corev1.ResourceName]resource.Quantity) error {
	for k := range resourceList {
		if k != corev1.ResourceCPU && k != corev1.ResourceMemory && strings.HasPrefix(k.String(), corev1.ResourceHugePagesPrefix) {
			return fmt.Errorf("resource key is not cpu or memory or hugepages- ")
		}
	}
	return nil
}
