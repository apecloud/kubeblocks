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

	"golang.org/x/exp/maps"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/kubectl/pkg/util/storage"

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
// +kubebuilder:webhook:path=/validate-apps-kubeblocks-io-v1alpha1-opsrequest,mutating=false,failurePolicy=fail,sideEffects=None,groups=apps.kubeblocks.io,resources=opsrequests,verbs=create;update,versions=v1alpha1,name=vopsrequest.kb.io,admissionReviewVersions=v1

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
	return slices.Contains([]OpsPhase{OpsCreatingPhase, OpsRunningPhase, OpsSucceedPhase, OpsFailedPhase}, r.Status.Phase)
}

// validateClusterPhase validates whether the current cluster state supports the OpsRequest
func (r *OpsRequest) validateClusterPhase(cluster *Cluster) error {
	opsBehaviour := OpsRequestBehaviourMapper[r.Spec.Type]
	// if the OpsType has no cluster phases, ignores it
	if len(opsBehaviour.FromClusterPhases) == 0 {
		return nil
	}
	// validate whether existing the same type OpsRequest
	var (
		opsRequestValue string
		opsRecorder     []OpsRecorder
		ok              bool
	)
	if opsRequestValue, ok = cluster.Annotations[opsRequestAnnotationKey]; !ok {
		return nil
	}
	// opsRequest annotation value in cluster to map
	if err := json.Unmarshal([]byte(opsRequestValue), &opsRecorder); err != nil {
		return nil
	}
	opsNamesInQueue := make([]string, len(opsRecorder))
	for i, v := range opsRecorder {
		// judge whether the opsRequest meets the following conditions:
		// 1. the opsRequest is Reentrant.
		// 2. the opsRequest supports concurrent execution of the same kind.
		if v.Name != r.Name && !slices.Contains(opsBehaviour.FromClusterPhases, v.ToClusterPhase) {
			return newInvalidError(OpsRequestKind, r.Name, "spec.type", fmt.Sprintf("Existing OpsRequest: %s is running in Cluster: %s, handle this OpsRequest first", v.Name, cluster.Name))
		}
		opsNamesInQueue[i] = v.Name
	}
	// check if the opsRequest can be executed in the current cluster phase unless this opsRequest is reentrant.
	if !slices.Contains(opsBehaviour.FromClusterPhases, cluster.Status.Phase) &&
		!slices.Contains(opsNamesInQueue, r.Name) {
		return newInvalidError(OpsRequestKind, r.Name, "spec.type", fmt.Sprintf("%s is forbidden when Cluster.status.Phase is %s", r.Spec.Type, cluster.Status.Phase))
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

func (r *OpsRequest) getClusterDefinition(ctx context.Context, cli client.Client, cluster *Cluster) (*ClusterDefinition, error) {
	cd := &ClusterDefinition{}
	if err := cli.Get(ctx, types.NamespacedName{Name: cluster.Spec.ClusterDefRef}, cd); err != nil {
		return nil, err
	}
	return cd, nil
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
	// Check whether the corresponding attribute is legal according to the operation type
	switch r.Spec.Type {
	case UpgradeType:
		r.validateUpgrade(ctx, k8sClient, cluster, allErrs)
	case VerticalScalingType:
		r.validateVerticalScaling(cluster, allErrs)
	case HorizontalScalingType:
		r.validateHorizontalScaling(ctx, k8sClient, cluster, allErrs)
	case VolumeExpansionType:
		r.validateVolumeExpansion(ctx, k8sClient, cluster, allErrs)
	case RestartType:
		r.validateRestart(cluster, allErrs)
	case ReconfiguringType:
		r.validateReconfigure(cluster, allErrs)
	}
}

// validateUpgrade validates spec.restart
func (r *OpsRequest) validateRestart(cluster *Cluster, allErrs *field.ErrorList) {
	restartList := r.Spec.RestartList
	if len(restartList) == 0 {
		addInvalidError(allErrs, "spec.restart", restartList, "can not be empty")
		return
	}

	compNames := make([]string, len(restartList))
	for i, v := range restartList {
		compNames[i] = v.ComponentName
	}
	r.checkComponentExistence(nil, cluster, compNames, allErrs)
}

// validateUpgrade validates spec.clusterOps.upgrade
func (r *OpsRequest) validateUpgrade(ctx context.Context,
	k8sClient client.Client,
	cluster *Cluster,
	allErrs *field.ErrorList) {
	if r.Spec.Upgrade == nil {
		addNotFoundError(allErrs, "spec.upgrade", "")
		return
	}

	clusterVersion := &ClusterVersion{}
	clusterVersionRef := r.Spec.Upgrade.ClusterVersionRef
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: clusterVersionRef}, clusterVersion); err != nil {
		addInvalidError(allErrs, "spec.upgrade.clusterVersionRef", clusterVersionRef, err.Error())
	}
}

// validateVerticalScaling validates api when spec.type is VerticalScaling
func (r *OpsRequest) validateVerticalScaling(cluster *Cluster, allErrs *field.ErrorList) {
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

	r.checkComponentExistence(nil, cluster, componentNames, allErrs)
}

// validateVerticalScaling validate api is legal when spec.type is VerticalScaling
func (r *OpsRequest) validateReconfigure(cluster *Cluster, allErrs *field.ErrorList) {
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
func (r *OpsRequest) validateHorizontalScaling(ctx context.Context, cli client.Client, cluster *Cluster, allErrs *field.ErrorList) {
	horizontalScalingList := r.Spec.HorizontalScalingList
	if len(horizontalScalingList) == 0 {
		addInvalidError(allErrs, "spec.horizontalScaling", horizontalScalingList, "can not be empty")
		return
	}

	componentNames := make([]string, len(horizontalScalingList))
	for i, v := range horizontalScalingList {
		componentNames[i] = v.ComponentName
	}

	clusterDef, err := r.getClusterDefinition(ctx, cli, cluster)
	if err != nil {
		addInvalidError(allErrs, "spec.horizontalScaling", horizontalScalingList, "get cluster definition error: "+err.Error())
		return
	}
	r.checkComponentExistence(clusterDef, cluster, componentNames, allErrs)
}

// validateVolumeExpansion validates volumeExpansion api when spec.type is VolumeExpansion
func (r *OpsRequest) validateVolumeExpansion(ctx context.Context, cli client.Client, cluster *Cluster, allErrs *field.ErrorList) {
	volumeExpansionList := r.Spec.VolumeExpansionList
	if len(volumeExpansionList) == 0 {
		addInvalidError(allErrs, "spec.volumeExpansion", volumeExpansionList, "can not be empty")
		return
	}

	componentNames := make([]string, len(volumeExpansionList))
	for i, v := range volumeExpansionList {
		componentNames[i] = v.ComponentName
	}
	r.checkComponentExistence(nil, cluster, componentNames, allErrs)

	r.checkVolumesAllowExpansion(ctx, cli, cluster, allErrs)
}

// checkComponentExistence checks whether components to be operated exist in cluster spec.
func (r *OpsRequest) checkComponentExistence(clusterDef *ClusterDefinition, cluster *Cluster, compNames []string, errs *field.ErrorList) {
	compSpecNameMap := make(map[string]bool)
	for _, compSpec := range cluster.Spec.ComponentSpecs {
		compSpecNameMap[compSpec.Name] = true
	}

	// To keep the compatibility, do a cross validation with ClusterDefinition's components to meet the topology constraint,
	// but we should still carefully consider the necessity for the validation here.
	validCompNameMap := make(map[string]bool)
	if clusterDef == nil {
		validCompNameMap = compSpecNameMap
	} else {
		for _, compSpec := range cluster.Spec.ComponentSpecs {
			for _, compDef := range clusterDef.Spec.ComponentDefs {
				if compSpec.ComponentDefRef == compDef.Name {
					validCompNameMap[compSpec.Name] = true
					break
				}
			}
		}
	}

	var notFoundCompNames []string
	var notSupportCompNames []string
	for _, compName := range compNames {
		if _, ok := compSpecNameMap[compName]; !ok {
			notFoundCompNames = append(notFoundCompNames, compName)
			continue
		}
		if _, ok := validCompNameMap[compName]; !ok {
			notSupportCompNames = append(notSupportCompNames, compName)
		}
	}

	if len(notFoundCompNames) > 0 {
		addInvalidError(errs, fmt.Sprintf("spec.%s[*].componentName", lowercaseInitial(r.Spec.Type)),
			notFoundCompNames, "not found in Cluster.spec.components[*].name")
	}
	if len(notSupportCompNames) > 0 {
		addInvalidError(errs, fmt.Sprintf("spec.%s[*].componentName", lowercaseInitial(r.Spec.Type)),
			notSupportCompNames, fmt.Sprintf("not supported the %s operation", r.Spec.Type))
	}
}

func (r *OpsRequest) checkVolumesAllowExpansion(ctx context.Context, cli client.Client, cluster *Cluster, errs *field.ErrorList) {
	type Entity struct {
		existInSpec      bool
		storageClassName *string
		allowExpansion   bool
	}

	// component name -> vct name -> entity
	vols := make(map[string]map[string]Entity)
	for _, comp := range r.Spec.VolumeExpansionList {
		for _, vct := range comp.VolumeClaimTemplates {
			if _, ok := vols[comp.ComponentName]; !ok {
				vols[comp.ComponentName] = make(map[string]Entity)
			}
			vols[comp.ComponentName][vct.Name] = Entity{false, nil, false}
		}
	}

	// traverse the spec to update volumes
	for _, comp := range cluster.Spec.ComponentSpecs {
		if _, ok := vols[comp.Name]; !ok {
			continue // ignore not-exist component
		}
		for _, vct := range comp.VolumeClaimTemplates {
			if _, ok := vols[comp.Name][vct.Name]; !ok {
				continue
			}
			vols[comp.Name][vct.Name] = Entity{true, vct.Spec.StorageClassName, false}
		}
	}

	// check all used storage classes
	for cname, compVols := range vols {
		for vname := range compVols {
			e := vols[cname][vname]
			if !e.existInSpec {
				continue
			}
			if allowExpansion, err := checkStorageClassAllowExpansion(ctx, cli, e.storageClassName); err != nil {
				continue // ignore the error and take it as not-supported
			} else {
				vols[cname][vname] = Entity{e.existInSpec, e.storageClassName, allowExpansion}
			}
		}
	}

	for i, v := range maps.Values(vols) {
		invalid := make([]string, 0)
		for vct, e := range v {
			if !e.existInSpec || !e.allowExpansion {
				invalid = append(invalid, vct)
			}
		}
		if len(invalid) > 0 {
			message := "not support volume expansion, check the StorageClass whether allow volume expansion."
			addInvalidError(errs, fmt.Sprintf("spec.volumeExpansion[%d].volumeClaimTemplates[*].name", i), invalid, message)
		}
	}
}

// checkStorageClassAllowExpansion checks whether the specified storage class supports volume expansion.
func checkStorageClassAllowExpansion(ctx context.Context, cli client.Client, storageClassName *string) (bool, error) {
	if storageClassName == nil {
		// TODO: check the real storage class by pvc.
		return checkDefaultStorageClassAllowExpansion(ctx, cli)
	}

	storageClass := &storagev1.StorageClass{}
	// take not found error as unsupported
	if err := cli.Get(ctx, types.NamespacedName{Name: *storageClassName}, storageClass); err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	if storageClass == nil || storageClass.AllowVolumeExpansion == nil {
		return false, nil
	}
	return *storageClass.AllowVolumeExpansion, nil
}

// checkDefaultStorageClassAllowExpansion checks whether the default storage class supports volume expansion.
func checkDefaultStorageClassAllowExpansion(ctx context.Context, cli client.Client) (bool, error) {
	storageClassList := &storagev1.StorageClassList{}
	if err := cli.List(ctx, storageClassList); err != nil {
		return false, err
	}
	for _, sc := range storageClassList.Items {
		if sc.Annotations == nil || sc.Annotations[storage.IsDefaultStorageClassAnnotation] != "true" {
			continue
		}
		return sc.AllowVolumeExpansion != nil && *sc.AllowVolumeExpansion, nil
	}
	return false, nil
}

func lowercaseInitial(opsType OpsType) string {
	str := string(opsType)
	return strings.ToLower(str[:1]) + str[1:]
}

// validateVerticalResourceList checks if k8s resourceList is legal
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
