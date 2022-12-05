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
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
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
var (
	opsrequestlog = logf.Log.WithName("opsrequest-resource")

	// ClusterPhasesMapperForOps records in which cluster phases OpsRequest can run
	ClusterPhasesMapperForOps = map[OpsType][]Phase{}
)

func (r *OpsRequest) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-dbaas-kubeblocks-io-v1alpha1-opsrequest,mutating=true,failurePolicy=fail,sideEffects=None,groups=dbaas.kubeblocks.io,resources=opsrequests,verbs=create;update,versions=v1alpha1,name=mopsrequest.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &OpsRequest{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *OpsRequest) Default() {
	opsrequestlog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-dbaas-kubeblocks-io-v1alpha1-opsrequest,mutating=false,failurePolicy=fail,sideEffects=None,groups=dbaas.kubeblocks.io,resources=opsrequests,verbs=create;update,versions=v1alpha1,name=vopsrequest.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &OpsRequest{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *OpsRequest) ValidateCreate() error {
	opsrequestlog.Info("validate create", "name", r.Name)
	return r.validate(true)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *OpsRequest) ValidateUpdate(old runtime.Object) error {
	opsrequestlog.Info("validate update", "name", r.Name)
	lastOpsRequest := old.(*OpsRequest)
	if r.Status.Phase == SucceedPhase && !reflect.DeepEqual(lastOpsRequest.Spec, r.Spec) {
		return newInvalidError(OpsRequestKind, r.Name, "spec", fmt.Sprintf("update OpsRequest is forbidden when status.Phase is %s", r.Status.Phase))
	}
	// we can not delete the OpsRequest when cluster has been deleted. because can not edit the finalizer when cluster not existed.
	// so if no spec updated, skip validation.
	if reflect.DeepEqual(lastOpsRequest.Spec, r.Spec) {
		return nil
	}
	return r.validate(false)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *OpsRequest) ValidateDelete() error {
	opsrequestlog.Info("validate delete", "name", r.Name)

	return nil
}

// validateClusterPhase validate whether the current cluster state supports the OpsRequest
func (r *OpsRequest) validateClusterPhase(cluster *Cluster) error {
	clusterPhases := ClusterPhasesMapperForOps[r.Spec.Type]
	// if the OpsType is no cluster phases, ignores it
	if len(clusterPhases) == 0 {
		return nil
	}
	if !slices.Contains(clusterPhases, cluster.Status.Phase) {
		return newInvalidError(OpsRequestKind, r.Name, "spec.type", fmt.Sprintf("%s is forbidden when Cluster.status.Phase is %s", r.Spec.Type, cluster.Status.Phase))
	}
	return nil
}

func (r *OpsRequest) validate(isCreate bool) error {
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
		return newInvalidError(OpsRequestKind, r.Name, "spec.clusterRef", err.Error())
	}
	if isCreate {
		if err := r.validateClusterPhase(cluster); err != nil {
			return err
		}
	}
	r.validateOps(ctx, cluster, &allErrs)
	if len(allErrs) > 0 {
		return apierrors.NewInvalid(schema.GroupKind{Group: APIVersion, Kind: OpsRequestKind}, r.Name, allErrs)
	}
	return nil
}

// validateOps validate ops attributes is legal
func (r *OpsRequest) validateOps(ctx context.Context, cluster *Cluster, allErrs *field.ErrorList) {
	if cluster.Status.Operations == nil {
		cluster.Status.Operations = &Operations{}
	}

	// Check whether the corresponding attribute is legal according to the operation type
	switch r.Spec.Type {
	case UpgradeType:
		r.validateUpgrade(ctx, allErrs, cluster)
	case VerticalScalingType:
		r.validateVerticalScaling(allErrs, cluster)
	case HorizontalScalingType:
		r.validateHorizontalScaling(cluster, allErrs)
	case VolumeExpansionType:
		r.validateVolumeExpansion(allErrs, cluster)
	case RestartType:
		supportedComponentMap := covertComponentNamesToMap(cluster.Status.Operations.Restartable)
		r.commonValidationWithComponentOps(allErrs, cluster, supportedComponentMap, nil)
	}
}

// validateUpgrade validate spec.clusterOps.upgrade is legal
func (r *OpsRequest) validateUpgrade(ctx context.Context, allErrs *field.ErrorList, cluster *Cluster) {
	if !cluster.Status.Operations.Upgradable {
		addInvalidError(allErrs, "spec.type", r.Spec.Type, fmt.Sprintf("not supported in Cluster: %s, appversion must be greater than 1", r.Spec.ClusterRef))
		return
	}
	if r.Spec.ClusterOps == nil || r.Spec.ClusterOps.Upgrade == nil {
		addNotFoundError(allErrs, "spec.clusterOps.upgrade", "")
		return
	}

	appVersion := &AppVersion{}
	appVersionRef := r.Spec.ClusterOps.Upgrade.AppVersionRef
	if err := webhookMgr.client.Get(ctx, types.NamespacedName{Name: appVersionRef}, appVersion); err != nil {
		addInvalidError(allErrs, "spec.clusterOps.upgrade.appVersionRef", appVersionRef, err.Error())
	} else if cluster.Spec.AppVersionRef == r.Spec.ClusterOps.Upgrade.AppVersionRef {
		addInvalidError(allErrs, "spec.clusterOps.upgrade.appVersionRef", appVersionRef, "can not equals Cluster.spec.appVersionRef")
	}
}

// validateVerticalScaling validate api is legal when spec.type is VerticalScaling
func (r *OpsRequest) validateVerticalScaling(allErrs *field.ErrorList, cluster *Cluster) {
	supportedComponentMap := covertComponentNamesToMap(cluster.Status.Operations.VerticalScalable)
	customValidate := func(componentOps *ComponentOps, index int, operationComponent *OperationComponent) *field.Error {
		if componentOps.VerticalScaling == nil {
			return field.NotFound(field.NewPath(fmt.Sprintf("spec.componentOps[%d].verticalScaling", index)), "can not be empty")
		}
		if invalidValue, err := validateVerticalResourceList(componentOps.VerticalScaling.Requests); err != nil {
			return field.Invalid(field.NewPath(fmt.Sprintf("spec.componentOps[%d].verticalScaling.requests", index)), invalidValue, err.Error())
		}
		if invalidValue, err := validateVerticalResourceList(componentOps.VerticalScaling.Limits); err != nil {
			return field.Invalid(field.NewPath(fmt.Sprintf("spec.componentOps[%d].verticalScaling.limits", index)), invalidValue, err.Error())
		}
		if invalidValue, err := compareRequestsAndLimits(*componentOps.VerticalScaling); err != nil {
			return field.Invalid(field.NewPath(fmt.Sprintf("spec.componentOps[%d].verticalScaling.requests", index)), invalidValue, err.Error())
		}
		return nil
	}
	r.commonValidationWithComponentOps(allErrs, cluster, supportedComponentMap, customValidate)
}

// compareRequestsAndLimits compare the resource requests and limits
func compareRequestsAndLimits(resources corev1.ResourceRequirements) (string, error) {
	requests := resources.Requests
	limits := resources.Limits
	if requests == nil || limits == nil {
		return "", nil
	}
	if compareQuantity(requests.Cpu(), limits.Cpu()) {
		return requests.Cpu().String(), errors.New(`must be less than or equal to cpu limit`)
	}
	if compareQuantity(requests.Memory(), limits.Memory()) {
		return requests.Memory().String(), errors.New(`must be less than or equal to memory limit`)
	}
	return "", nil
}

// compareQuantity compare requests quantity and limits quantity
func compareQuantity(requestQuantity, limitQuantity *resource.Quantity) bool {
	return requestQuantity != nil && limitQuantity != nil && requestQuantity.Cmp(*limitQuantity) > 0
}

// invalidReplicas verify whether the replicas is invalid
func invalidReplicas(replicas int32, operationComponent *OperationComponent) string {
	if operationComponent.Min != 0 && replicas < operationComponent.Min {
		return fmt.Sprintf("replicas must greater than %d", operationComponent.Min)
	}
	if operationComponent.Max != 0 && replicas > operationComponent.Max {
		return fmt.Sprintf("replicas must less than %d", operationComponent.Max)
	}
	return ""
}

// validateHorizontalScaling validate api is legal when spec.type is HorizontalScaling
func (r *OpsRequest) validateHorizontalScaling(cluster *Cluster, allErrs *field.ErrorList) {
	supportedComponentMap := covertOperationComponentsToMap(cluster.Status.Operations.HorizontalScalable)
	customValidate := func(componentOps *ComponentOps, index int, operationComponent *OperationComponent) *field.Error {
		if componentOps.HorizontalScaling == nil {
			return field.NotFound(field.NewPath(fmt.Sprintf("spec.componentOps[%d].horizontalScaling", index)), "can not be empty")
		}
		replicas := componentOps.HorizontalScaling.Replicas
		replicasMsg := invalidReplicas(replicas, operationComponent)
		if replicasMsg != "" {
			return field.Invalid(field.NewPath(fmt.Sprintf("spec.componentOps[%d].horizontalScaling.replicas", index)), replicas, replicasMsg)
		}
		return nil
	}
	r.commonValidationWithComponentOps(allErrs, cluster, supportedComponentMap, customValidate)
}

// validateVolumeExpansion validate volumeExpansion api is legal when spec.type is VolumeExpansion
func (r *OpsRequest) validateVolumeExpansion(allErrs *field.ErrorList, cluster *Cluster) {
	supportedComponentMap := covertOperationComponentsToMap(cluster.Status.Operations.VolumeExpandable)
	customValidate := func(componentOps *ComponentOps, index int, operationComponent *OperationComponent) *field.Error {
		var (
			supportedVctMap = map[string]struct{}{}
			invalidVctNames []string
		)
		if componentOps.VolumeExpansion == nil {
			return field.NotFound(field.NewPath(fmt.Sprintf("spec.componentOps[%d].volumeExpansion", index)), "can not be empty")
		}
		// covert slice to map
		for _, v := range operationComponent.VolumeClaimTemplateNames {
			supportedVctMap[v] = struct{}{}
		}
		// check the volumeClaimTemplate is support volumeExpansion
		for _, v := range componentOps.VolumeExpansion {
			if _, ok := supportedVctMap[v.Name]; !ok {
				invalidVctNames = append(invalidVctNames, v.Name)
			}
		}
		if len(invalidVctNames) > 0 {
			return field.Invalid(field.NewPath(fmt.Sprintf("spec.componentOps[%d].volumeExpansion[*].name", index)), invalidVctNames, "not support volume expansion, check the StorageClass whether allow volume expansion.")
		}
		return nil
	}
	r.commonValidationWithComponentOps(allErrs, cluster, supportedComponentMap, customValidate)
}

// commonValidateWithComponentOps do common validation, when the operation in componentOps scope
func (r *OpsRequest) commonValidationWithComponentOps(allErrs *field.ErrorList, cluster *Cluster,
	supportedComponentMap map[string]*OperationComponent,
	customValidate func(*ComponentOps, int, *OperationComponent) *field.Error) bool {
	var (
		clusterComponentNameMap    = map[string]struct{}{}
		tmpComponentNameMap        = map[string]int{}
		notFoundComponentNames     []string
		notSupportedComponentNames []string
		duplicateComponents        []string
		operationComponent         *OperationComponent
		ok                         bool
	)
	// check whether cluster support the operation when it in component scope
	if len(supportedComponentMap) == 0 {
		switch r.Spec.Type {
		case VolumeExpansionType:
			addInvalidError(allErrs, "spec.type", r.Spec.Type, fmt.Sprintf("not supported in Cluster: %s, check the StorageClass whether allow volume expansion.", r.Spec.ClusterRef))
		default:
			addInvalidError(allErrs, "spec.type", r.Spec.Type, fmt.Sprintf("not supported in Cluster: %s", r.Spec.ClusterRef))
		}
		return false
	}
	if len(r.Spec.ComponentOpsList) == 0 {
		addInvalidError(allErrs, "spec.componentOps", r.Spec.ComponentOpsList, "can not be empty")
		return false
	}
	for _, v := range cluster.Spec.Components {
		clusterComponentNameMap[v.Name] = struct{}{}
	}
	for index, componentOps := range r.Spec.ComponentOpsList {
		if len(componentOps.ComponentNames) == 0 {
			addNotFoundError(allErrs, fmt.Sprintf("spec.componentOps[%d].componentNames", index), "can not be empty")
			continue
		}
		for _, v := range componentOps.ComponentNames {
			// check the duplicate component name in r.Spec.ComponentOpsList[*].componentNames
			if _, ok = tmpComponentNameMap[v]; ok {
				duplicateComponents = append(duplicateComponents, v)
				continue
			}
			tmpComponentNameMap[v] = index
			// check component name whether exist in Cluster.spec.components[*].name
			if _, ok = clusterComponentNameMap[v]; !ok {
				notFoundComponentNames = append(notFoundComponentNames, v)
				continue
			}
			// check component name whether support the operation
			if operationComponent, ok = supportedComponentMap[v]; !ok {
				notSupportedComponentNames = append(notSupportedComponentNames, v)
				continue
			}
			// do custom validation
			if customValidate == nil {
				continue
			}
			if err := customValidate(&componentOps, index, operationComponent); err != nil {
				*allErrs = append(*allErrs, err)
			}
		}
	}

	if len(duplicateComponents) > 0 {
		addInvalidError(allErrs, "spec.componentOps[*].componentNames", duplicateComponents, "duplicate component name exists")
	}

	if len(notFoundComponentNames) > 0 {
		addInvalidError(allErrs, "spec.componentOps[*].componentNames", notFoundComponentNames, "not found in Cluster.spec.components[*].name")
	}

	if len(notSupportedComponentNames) > 0 {
		addInvalidError(allErrs, "spec.componentOps[*].componentNames", notSupportedComponentNames, fmt.Sprintf("not supported %s", r.Spec.Type))
	}
	return true
}

// covertComponentNamesToMap covert supportedComponent slice to map
func covertComponentNamesToMap(componentNames []string) map[string]*OperationComponent {
	supportedComponentMap := map[string]*OperationComponent{}
	for _, v := range componentNames {
		supportedComponentMap[v] = nil
	}
	return supportedComponentMap
}

// covertOperationComponentsToMap covert supportedOperationComponent slice to map
func covertOperationComponentsToMap(componentNames []OperationComponent) map[string]*OperationComponent {
	supportedComponentMap := map[string]*OperationComponent{}
	for _, v := range componentNames {
		supportedComponentMap[v.Name] = &v
	}
	return supportedComponentMap
}

// checkResourceList check k8s resourceList is legal
func validateVerticalResourceList(resourceList map[corev1.ResourceName]resource.Quantity) (string, error) {
	for k := range resourceList {
		if k != corev1.ResourceCPU && k != corev1.ResourceMemory && !strings.HasPrefix(k.String(), corev1.ResourceHugePagesPrefix) {
			return string(k), fmt.Errorf("resource key is not cpu or memory or hugepages- ")
		}
	}
	return "", nil
}

func addInvalidError(allErrs *field.ErrorList, fieldPath string, value interface{}, msg string) {
	*allErrs = append(*allErrs, field.Invalid(field.NewPath(fieldPath), value, msg))
}

func addNotFoundError(allErrs *field.ErrorList, fieldPath string, value interface{}) {
	*allErrs = append(*allErrs, field.NotFound(field.NewPath(fieldPath), value))
}
