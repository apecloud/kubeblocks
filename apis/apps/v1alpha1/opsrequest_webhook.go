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
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-apps-kubeblocks-io-v1alpha1-opsrequest,mutating=false,failurePolicy=fail,sideEffects=None,groups=apps.kubeblocks.io,resources=opsrequests,verbs=create;update,versions=v1alpha1,name=vopsrequest.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &OpsRequest{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *OpsRequest) ValidateCreate() error {
	opsrequestlog.Info("validate create", "name", r.Name)
	return r.validateEntry(true)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *OpsRequest) ValidateUpdate(old runtime.Object) error {
	opsrequestlog.Info("validate update", "name", r.Name)
	lastOpsRequest := old.(*OpsRequest)
	if r.isForbiddenUpdate() && !reflect.DeepEqual(lastOpsRequest.Spec, r.Spec) {
		return newInvalidError(OpsRequestKind, r.Name, "spec", fmt.Sprintf("update OpsRequest is forbidden when status.Phase is %s", r.Status.Phase))
	}
	// if no spec updated, we should skip validation.
	// if not, we can not delete the OpsRequest when cluster has been deleted.
	// because when cluster not existed, r.validate will report an error.
	if reflect.DeepEqual(lastOpsRequest.Spec, r.Spec) {
		return nil
	}
	return r.validateEntry(false)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *OpsRequest) ValidateDelete() error {
	opsrequestlog.Info("validate delete", "name", r.Name)
	return nil
}

// IsForbiddenUpdate OpsRequest cannot modify the spec when status is in [Succeed,Running,Failed].
func (r *OpsRequest) isForbiddenUpdate() bool {
	return slices.Contains([]Phase{SucceedPhase, RunningPhase, FailedPhase}, r.Status.Phase)
}

// validateClusterPhase validates whether the current cluster state supports the OpsRequest
func (r *OpsRequest) validateClusterPhase(cluster *Cluster) error {
	opsBehaviour := OpsRequestBehaviourMapper[r.Spec.Type]
	// if the OpsType has no cluster phases, ignores it
	if len(opsBehaviour.FromClusterPhases) == 0 {
		return nil
	}
	if !slices.Contains(opsBehaviour.FromClusterPhases, cluster.Status.Phase) {
		return newInvalidError(OpsRequestKind, r.Name, "spec.type", fmt.Sprintf("%s is forbidden when Cluster.status.Phase is %s", r.Spec.Type, cluster.Status.Phase))
	}
	// validate whether existing the same type OpsRequest
	var (
		opsRequestValue string
		opsRecorder     []OpsRecorder
		ok              bool
	)
	if cluster.Annotations == nil {
		return nil
	}
	if opsRequestValue, ok = cluster.Annotations[opsRequestAnnotationKey]; !ok {
		return nil
	}
	// opsRequest annotation value in cluster to map
	if err := json.Unmarshal([]byte(opsRequestValue), &opsRecorder); err != nil {
		return nil
	}
	for _, v := range opsRecorder {
		if v.Name != r.Name {
			return newInvalidError(OpsRequestKind, r.Name, "spec.type", fmt.Sprintf("Existing OpsRequest: %s is running in Cluster: %s, handle this OpsRequest first", v.Name, cluster.Name))
		}
	}
	return nil
}

// getCluster gets cluster with webhook client
func (r *OpsRequest) getCluster(ctx context.Context, k8sClient client.Client) (*Cluster, error) {
	if k8sClient == nil {
		return nil, nil
	}
	cluster := &Cluster{}
	// get cluster resource
	if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: r.Namespace, Name: r.Spec.ClusterRef}, cluster); err != nil {
		return nil, newInvalidError(OpsRequestKind, r.Name, "spec.clusterRef", err.Error())
	}
	return cluster, nil
}

// Validate validates OpsRequest
func (r *OpsRequest) Validate(ctx context.Context,
	k8sClient client.Client,
	cluster *Cluster,
	isCreate bool) error {
	var allErrs field.ErrorList
	if isCreate {
		if err := r.validateClusterPhase(cluster); err != nil {
			return err
		}
	}
	r.validateOps(ctx, k8sClient, cluster, &allErrs)
	if len(allErrs) > 0 {
		return apierrors.NewInvalid(schema.GroupKind{Group: APIVersion, Kind: OpsRequestKind}, r.Name, allErrs)
	}
	return nil
}

// ValidateEntry OpsRequest webhook validate entry
func (r *OpsRequest) validateEntry(isCreate bool) error {
	if webhookMgr == nil || webhookMgr.client == nil {
		return nil
	}
	ctx := context.Background()
	k8sClient := webhookMgr.client
	cluster, err := r.getCluster(ctx, k8sClient)
	if err != nil {
		return err
	}
	return r.Validate(ctx, k8sClient, cluster, isCreate)
}

// validateOps validates ops attributes
func (r *OpsRequest) validateOps(ctx context.Context,
	k8sClient client.Client,
	cluster *Cluster,
	allErrs *field.ErrorList) {
	if cluster.Status.Operations == nil {
		cluster.Status.Operations = &Operations{}
	}

	// Check whether the corresponding attribute is legal according to the operation type
	switch r.Spec.Type {
	case UpgradeType:
		r.validateUpgrade(ctx, k8sClient, allErrs, cluster)
	case VerticalScalingType:
		r.validateVerticalScaling(allErrs, cluster)
	case HorizontalScalingType:
		r.validateHorizontalScaling(cluster, allErrs)
	case VolumeExpansionType:
		r.validateVolumeExpansion(allErrs, cluster)
	case RestartType:
		r.validateRestart(allErrs, cluster)
	case ReconfiguringType:
		r.validateReconfigure(allErrs, cluster)
	}
}

// validateUpgrade validates spec.restart
func (r *OpsRequest) validateRestart(allErrs *field.ErrorList, cluster *Cluster) {
	restartList := r.Spec.RestartList
	if len(restartList) == 0 {
		addInvalidError(allErrs, "spec.restart", restartList, "can not be empty")
		return
	}

	compNames := make([]string, len(restartList))
	for i, v := range restartList {
		compNames[i] = v.ComponentName
	}
	r.checkComponentExistence(allErrs, cluster, compNames)
}

// validateUpgrade validates spec.clusterOps.upgrade
func (r *OpsRequest) validateUpgrade(ctx context.Context,
	k8sClient client.Client,
	allErrs *field.ErrorList,
	cluster *Cluster) {
	if r.Spec.Upgrade == nil {
		addNotFoundError(allErrs, "spec.upgrade", "")
		return
	}

	cvList := &ClusterVersionList{}
	labelKey := "clusterdefinition.kubeblocks.io/name" // TODO(leon)
	if err := k8sClient.List(ctx, cvList, client.MatchingLabels{labelKey: cluster.Spec.ClusterDefRef}); err != nil {
		addInvalidError(allErrs, "spec.type", r.Spec.Type, err.Error())
		return
	}

	if len(cvList.Items) <= 1 {
		addInvalidError(allErrs, "spec.type", r.Spec.Type, fmt.Sprintf("not supported in Cluster: %s, ClusterVersion must be greater than 1", r.Spec.ClusterRef))
		return
	}

	targetClusterVersion := r.Spec.Upgrade.ClusterVersionRef
	for _, cv := range cvList.Items {
		if cv.Name == targetClusterVersion {
			return
		}
	}
	addInvalidError(allErrs, "spec.upgrade.clusterVersionRef", targetClusterVersion, fmt.Sprintf("target CluterVersion to upgrade not found"))
}

// validateVerticalScaling validates api when spec.type is VerticalScaling
func (r *OpsRequest) validateVerticalScaling(allErrs *field.ErrorList, cluster *Cluster) {
	verticalScalingList := r.Spec.VerticalScalingList
	if len(verticalScalingList) == 0 {
		addInvalidError(allErrs, "spec.verticalScaling", verticalScalingList, "can not be empty")
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
		if invalidValue, err := compareRequestsAndLimits(v.ResourceRequirements); err != nil {
			addInvalidError(allErrs, fmt.Sprintf("spec.verticalScaling[%d].requests", i), invalidValue, err.Error())
		}
	}

	r.checkComponentExistence(allErrs, cluster, componentNames)
}

// validateVerticalScaling validate api is legal when spec.type is VerticalScaling
func (r *OpsRequest) validateReconfigure(allErrs *field.ErrorList, cluster *Cluster) {
	reconfigure := r.Spec.Reconfigure
	if reconfigure == nil {
		addInvalidError(allErrs, "spec.reconfigure", reconfigure, "can not be empty")
		return
	}

	// TODO validate updated params
}

// compareRequestsAndLimits compares the resource requests and limits
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

// compareQuantity compares requests quantity and limits quantity
func compareQuantity(requestQuantity, limitQuantity *resource.Quantity) bool {
	return requestQuantity != nil && limitQuantity != nil && requestQuantity.Cmp(*limitQuantity) > 0
}

// validateHorizontalScaling validates api when spec.type is HorizontalScaling
func (r *OpsRequest) validateHorizontalScaling(cluster *Cluster, allErrs *field.ErrorList) {
	horizontalScalingList := r.Spec.HorizontalScalingList
	if len(horizontalScalingList) == 0 {
		addInvalidError(allErrs, "spec.horizontalScaling", horizontalScalingList, "can not be empty")
		return
	}

	componentNames := make([]string, len(horizontalScalingList))
	for i, v := range horizontalScalingList {
		componentNames[i] = v.ComponentName
	}

	// TODO(leon): whether to check against cluster definition?
	r.checkComponentExistence(allErrs, cluster, componentNames)
}

// validateVolumeExpansion validates volumeExpansion api when spec.type is VolumeExpansion
func (r *OpsRequest) validateVolumeExpansion(allErrs *field.ErrorList, cluster *Cluster) {
	volumeExpansionList := r.Spec.VolumeExpansionList
	if len(volumeExpansionList) == 0 {
		addInvalidError(allErrs, "spec.volumeExpansion", volumeExpansionList, "can not be empty")
		return
	}
	// validate whether the cluster support volume expansion
	supportedComponentMap := convertOperationComponentsToMap(cluster.Status.Operations.VolumeExpandable)
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
		// convert slice to map
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

// validateClusterIsSupported validates whether cluster supports the operation when it in component scope
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

// commonValidateWithComponentOps does common validation, when the operation in component scope
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
	for _, v := range cluster.Spec.ComponentSpecs {
		clusterComponentNameMap[v.Name] = struct{}{}
	}
	for _, v := range operationComponentNames {
		// check component name whether exist in Cluster.spec.components[*].name
		if _, ok = clusterComponentNameMap[v]; !ok {
			notFoundComponentNames = append(notFoundComponentNames, v)
			continue
		}
		// check if the component supports the operation
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

// checkComponentExistence checks whether components to be operated exist in cluster spec.
func (r *OpsRequest) checkComponentExistence(errs *field.ErrorList, cluster *Cluster, compNames []string) {
	compSpecNameMap := make(map[string]bool)
	for _, compSpec := range cluster.Spec.ComponentSpecs {
		compSpecNameMap[compSpec.Name] = true
	}

	var notFoundCompNames []string
	for _, compName := range compNames {
		if _, ok := compSpecNameMap[compName]; !ok {
			notFoundCompNames = append(notFoundCompNames, compName)
		}
	}

	if len(notFoundCompNames) > 0 {
		addInvalidError(errs, fmt.Sprintf("spec.%s[*].componentName", lowercaseInitial(r.Spec.Type)),
			notFoundCompNames, "not found in Cluster.spec.components[*].name")
	}
}

func lowercaseInitial(opsType OpsType) string {
	str := string(opsType)
	return strings.ToLower(str[:1]) + str[1:]
}

// convertOperationComponentsToMap converts supportedOperationComponent slice to map
func convertOperationComponentsToMap(componentNames []OperationComponent) map[string]*OperationComponent {
	supportedComponentMap := map[string]*OperationComponent{}
	for _, v := range componentNames {
		supportedComponentMap[v.Name] = &v
	}
	return supportedComponentMap
}

// checkResourceList checks if k8s resourceList is legal
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
