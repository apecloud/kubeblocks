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
	"encoding/json"
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
	opsrequestlog           = logf.Log.WithName("opsrequest-resource")
	opsRequestAnnotationKey = "kubeblocks.io/ops-request"

	// OpsRequestBehaviourMapper records in which cluster phases OpsRequest can run
	OpsRequestBehaviourMapper = map[OpsType]OpsRequestBehaviour{}
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
//+kubebuilder:webhook:path=/validate-dbaas-kubeblocks-io-v1alpha1-opsrequest,mutating=false,failurePolicy=fail,sideEffects=None,groups=dbaas.kubeblocks.io,resources=opsrequests,verbs=create;update;delete,versions=v1alpha1,name=vopsrequest.kb.io,admissionReviewVersions=v1

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
	if r.isForbiddenUpdate() && !reflect.DeepEqual(lastOpsRequest.Spec, r.Spec) {
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
	if r.Status.Phase == RunningPhase {
		return newInvalidError(OpsRequestKind, r.Name, "status.phase", fmt.Sprintf("delete OpsRequest is forbidden when status.Phase is %s", r.Status.Phase))
	}
	return nil
}

// IsForbiddenUpdate OpsRequest cannot modify the spec when status is in [Succeed,Running,Failed].
func (r *OpsRequest) isForbiddenUpdate() bool {
	return slices.Contains([]Phase{SucceedPhase, RunningPhase, FailedPhase}, r.Status.Phase)
}

// validateClusterPhase validate whether the current cluster state supports the OpsRequest
func (r *OpsRequest) validateClusterPhase(cluster *Cluster) error {
	opsBehaviour := OpsRequestBehaviourMapper[r.Spec.Type]
	// if the OpsType is no cluster phases, ignores it
	if len(opsBehaviour.FromClusterPhases) == 0 {
		return nil
	}
	if !slices.Contains(opsBehaviour.FromClusterPhases, cluster.Status.Phase) {
		return newInvalidError(OpsRequestKind, r.Name, "spec.type", fmt.Sprintf("%s is forbidden when Cluster.status.Phase is %s", r.Spec.Type, cluster.Status.Phase))
	}
	// validate whether existing the same type OpsRequest
	var (
		opsRequestValue string
		opsRequestMap   map[Phase]string
		ok              bool
	)
	if cluster.Annotations == nil {
		return nil
	}
	if opsRequestValue, ok = cluster.Annotations[opsRequestAnnotationKey]; !ok {
		return nil
	}
	// opsRequest annotation value in cluster to map
	if err := json.Unmarshal([]byte(opsRequestValue), &opsRequestMap); err != nil {
		return nil
	}
	opsRequestName := opsRequestMap[opsBehaviour.ToClusterPhase]
	if opsRequestName != "" {
		return newInvalidError(OpsRequestKind, r.Name, "spec.type", fmt.Sprintf("Existing OpsRequest: %s is running in Cluster: %s, handle this OpsRequest first", opsRequestName, cluster.Name))
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
		r.validateRestart(allErrs, cluster)
	}
}

// validateUpgrade validate spec.restart is legal
func (r *OpsRequest) validateRestart(allErrs *field.ErrorList, cluster *Cluster) {
	restartList := r.Spec.RestartList
	if len(restartList) == 0 {
		addInvalidError(allErrs, "spec.restart", restartList, "can not be empty")
		return
	}
	// get component name slice
	componentNames := make([]string, len(restartList))
	for i, v := range restartList {
		componentNames[i] = v.ComponentName
	}
	// validate component name is legal
	supportedComponentMap := covertComponentNamesToMap(cluster.Status.Operations.Restartable)
	r.validateComponentName(allErrs, cluster, supportedComponentMap, componentNames)
}

// validateUpgrade validate spec.upgrade is legal
func (r *OpsRequest) validateUpgrade(ctx context.Context, allErrs *field.ErrorList, cluster *Cluster) {
	if !cluster.Status.Operations.Upgradable {
		addInvalidError(allErrs, "spec.type", r.Spec.Type, fmt.Sprintf("not supported in Cluster: %s, ClusterVersion must be greater than 1", r.Spec.ClusterRef))
		return
	}
	if r.Spec.Upgrade == nil {
		addNotFoundError(allErrs, "spec.upgrade", "")
		return
	}

	clusterVersion := &ClusterVersion{}
	clusterVersionRef := r.Spec.Upgrade.ClusterVersionRef
	if err := webhookMgr.client.Get(ctx, types.NamespacedName{Name: clusterVersionRef}, clusterVersion); err != nil {
		addInvalidError(allErrs, "spec.upgrade.clusterVersionRef", clusterVersionRef, err.Error())
	} else if cluster.Spec.ClusterVersionRef == r.Spec.Upgrade.ClusterVersionRef {
		addInvalidError(allErrs, "spec.upgrade.clusterVersionRef", clusterVersionRef, "can not equals Cluster.spec.clusterVersionRef")
	}
}

// validateVerticalScaling validate api is legal when spec.type is VerticalScaling
func (r *OpsRequest) validateVerticalScaling(allErrs *field.ErrorList, cluster *Cluster) {
	verticalScalingList := r.Spec.VerticalScalingList
	if len(verticalScalingList) == 0 {
		addInvalidError(allErrs, "spec.verticalScaling", verticalScalingList, "can not be empty")
		return
	}
	// validate whether the cluster support vertical scaling
	supportedComponentMap := covertComponentNamesToMap(cluster.Status.Operations.VerticalScalable)
	if err := r.validateClusterIsSupported(supportedComponentMap); err != nil {
		*allErrs = append(*allErrs, err)
		return
	}
	// validate resources is legal and get component name slice
	componentNames := make([]string, len(verticalScalingList))
	for i, v := range verticalScalingList {
		componentNames[i] = v.ComponentName
		if invalidValue, err := validateVerticalResourceList(v.Requests); err != nil {
			addInvalidError(allErrs, fmt.Sprintf("spec.verticalScaling[%d].requests", i), invalidValue, err.Error())
			continue
		}
		if invalidValue, err := validateVerticalResourceList(v.Limits); err != nil {
			addInvalidError(allErrs, fmt.Sprintf("spec.verticalScaling[%d].limits", i), invalidValue, err.Error())
			continue
		}
		if invalidValue, err := compareRequestsAndLimits(*v.ResourceRequirements); err != nil {
			addInvalidError(allErrs, fmt.Sprintf("spec.verticalScaling[%d].requests", i), invalidValue, err.Error())
		}
	}

	// validate component name is legal
	r.validateComponentName(allErrs, cluster, supportedComponentMap, componentNames)

}

// compareRequestsAndLimits compare the resource requests and limits
func compareRequestsAndLimits(resources corev1.ResourceRequirements) (string, error) {
	requests := resources.Requests
	limits := resources.Limits
	if requests == nil || limits == nil {
		return "", nil
	}
	for k, v := range requests {
		if limitQuantity, ok := limits[k]; !ok {
			continue
		} else if compareQuantity(&v, &limitQuantity) {
			return v.String(), errors.New(fmt.Sprintf(`must be less than or equal to %s limit`, k))
		}
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
	horizontalScalingList := r.Spec.HorizontalScalingList
	if len(horizontalScalingList) == 0 {
		addInvalidError(allErrs, "spec.horizontalScaling", horizontalScalingList, "can not be empty")
		return
	}
	// validate whether the cluster support horizontal scaling
	supportedComponentMap := covertOperationComponentsToMap(cluster.Status.Operations.HorizontalScalable)
	if err := r.validateClusterIsSupported(supportedComponentMap); err != nil {
		*allErrs = append(*allErrs, err)
		return
	}
	// validate replicas is legal and get component name slice
	componentNames := make([]string, len(horizontalScalingList))
	for i, v := range horizontalScalingList {
		componentNames[i] = v.ComponentName
		operationComponent := supportedComponentMap[v.ComponentName]
		if operationComponent == nil {
			continue
		}
		replicasMsg := invalidReplicas(v.Replicas, operationComponent)
		if replicasMsg != "" {
			addInvalidError(allErrs, fmt.Sprintf("spec.horizontalScaling[%d].replicas", i), v.Replicas, replicasMsg)
		}
	}
	r.validateComponentName(allErrs, cluster, supportedComponentMap, componentNames)
}

// validateVolumeExpansion validate volumeExpansion api is legal when spec.type is VolumeExpansion
func (r *OpsRequest) validateVolumeExpansion(allErrs *field.ErrorList, cluster *Cluster) {
	volumeExpansionList := r.Spec.VolumeExpansionList
	if len(volumeExpansionList) == 0 {
		addInvalidError(allErrs, "spec.volumeExpansion", volumeExpansionList, "can not be empty")
		return
	}
	// validate whether the cluster support volume expansion
	supportedComponentMap := covertOperationComponentsToMap(cluster.Status.Operations.VolumeExpandable)
	if err := r.validateClusterIsSupported(supportedComponentMap); err != nil {
		*allErrs = append(*allErrs, err)
		return
	}
	// validate volumeClaimTemplates is legal and get component name slice
	componentNames := make([]string, len(volumeExpansionList))
	for i, v := range volumeExpansionList {
		var (
			supportedVCTMap = map[string]struct{}{}
			invalidVCTNames []string
		)
		componentNames[i] = v.ComponentName
		operationComponent := supportedComponentMap[v.ComponentName]
		if operationComponent == nil {
			continue
		}
		// covert slice to map
		for _, vctName := range operationComponent.VolumeClaimTemplateNames {
			supportedVCTMap[vctName] = struct{}{}
		}
		// check the volumeClaimTemplate is support volumeExpansion
		for _, vct := range v.VolumeClaimTemplates {
			if _, ok := supportedVCTMap[vct.Name]; !ok {
				invalidVCTNames = append(invalidVCTNames, vct.Name)
			}
		}
		if len(invalidVCTNames) > 0 {
			message := "not support volume expansion, check the StorageClass whether allow volume expansion."
			addInvalidError(allErrs, fmt.Sprintf("spec.volumeExpansion[%d].volumeClaimTemplates[*].name", i), invalidVCTNames, message)
		}
	}

	r.validateComponentName(allErrs, cluster, supportedComponentMap, componentNames)
}

// validateClusterIsSupported validate whether cluster support the operation when it in component scope
func (r *OpsRequest) validateClusterIsSupported(supportedComponentMap map[string]*OperationComponent) *field.Error {
	if len(supportedComponentMap) > 0 {
		return nil
	}
	var (
		opsType = r.Spec.Type
		message string
	)
	switch opsType {
	case VolumeExpansionType:
		message = fmt.Sprintf("not supported in Cluster: %s, check the StorageClass whether allow volume expansion.", r.Spec.ClusterRef)
	default:
		message = fmt.Sprintf("not supported in Cluster: %s", r.Spec.ClusterRef)
	}
	return field.Invalid(field.NewPath("spec.type"), opsType, message)
}

// commonValidateWithComponentOps do common validation, when the operation in component scope
func (r *OpsRequest) validateComponentName(allErrs *field.ErrorList,
	cluster *Cluster,
	supportedComponentMap map[string]*OperationComponent,
	operationComponentNames []string) {
	var (
		clusterComponentNameMap    = map[string]struct{}{}
		notFoundComponentNames     []string
		notSupportedComponentNames []string
		ok                         bool
		opsType                    = r.Spec.Type
	)
	for _, v := range cluster.Spec.Components {
		clusterComponentNameMap[v.Name] = struct{}{}
	}
	for _, v := range operationComponentNames {
		// check component name whether exist in Cluster.spec.components[*].name
		if _, ok = clusterComponentNameMap[v]; !ok {
			notFoundComponentNames = append(notFoundComponentNames, v)
			continue
		}
		// check component name whether support the operation
		if _, ok = supportedComponentMap[v]; !ok {
			notSupportedComponentNames = append(notSupportedComponentNames, v)
		}
	}

	if len(notFoundComponentNames) > 0 {
		addInvalidError(allErrs, fmt.Sprintf("spec.%s[*].componentName", lowercaseInitial(opsType)),
			notFoundComponentNames, "not found in Cluster.spec.components[*].name")
	}

	if len(notSupportedComponentNames) > 0 {
		addInvalidError(allErrs, fmt.Sprintf("spec.%s[*].componentName", lowercaseInitial(opsType)),
			notSupportedComponentNames, fmt.Sprintf("not supported the %s operation", opsType))
	}
}

func lowercaseInitial(opsType OpsType) string {
	str := string(opsType)
	return strings.ToLower(str[:1]) + str[1:]
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
