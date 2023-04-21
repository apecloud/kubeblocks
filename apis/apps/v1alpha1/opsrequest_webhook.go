/*
Copyright (C) 2022 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
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
	// OpsRequestBehaviourMapper records the opsRequest behaviour according to the OpsType.
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
		return fmt.Errorf("update OpsRequest: %s is forbidden when status.Phase is %s", r.Name, r.Status.Phase)
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
	if opsRequestValue, ok = cluster.Annotations[opsRequestAnnotationKey]; ok {
		// opsRequest annotation value in cluster to map
		if err := json.Unmarshal([]byte(opsRequestValue), &opsRecorder); err != nil {
			return nil
		}
	}
	opsNamesInQueue := make([]string, len(opsRecorder))
	for i, v := range opsRecorder {
		// judge whether the opsRequest meets the following conditions:
		// 1. the opsRequest is Reentrant.
		// 2. the opsRequest supports concurrent execution of the same kind.
		if v.Name != r.Name {
			return fmt.Errorf("existing OpsRequest: %s is running in Cluster: %s, handle this OpsRequest first", v.Name, cluster.Name)
		}
		opsNamesInQueue[i] = v.Name
	}
	// check if the opsRequest can be executed in the current cluster phase unless this opsRequest is reentrant.
	if !slices.Contains(opsBehaviour.FromClusterPhases, cluster.Status.Phase) &&
		!slices.Contains(opsNamesInQueue, r.Name) {
		return fmt.Errorf("opsRequest kind: %s is forbidden when Cluster.status.Phase is %s", r.Spec.Type, cluster.Status.Phase)
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
		return nil, fmt.Errorf("get cluster: %s failed, err: %s", r.Spec.ClusterRef, err.Error())
	}
	return cluster, nil
}

// Validate validates OpsRequest
func (r *OpsRequest) Validate(ctx context.Context,
	k8sClient client.Client,
	cluster *Cluster,
	isCreate bool) error {
	if isCreate {
		if err := r.validateClusterPhase(cluster); err != nil {
			return err
		}
	}
	return r.validateOps(ctx, k8sClient, cluster)
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
	cluster *Cluster) error {
	// Check whether the corresponding attribute is legal according to the operation type
	switch r.Spec.Type {
	case UpgradeType:
		return r.validateUpgrade(ctx, k8sClient)
	case VerticalScalingType:
		return r.validateVerticalScaling(ctx, k8sClient, cluster)
	case HorizontalScalingType:
		return r.validateHorizontalScaling(ctx, k8sClient, cluster)
	case VolumeExpansionType:
		return r.validateVolumeExpansion(ctx, k8sClient, cluster)
	case RestartType:
		return r.validateRestart(cluster)
	case ReconfiguringType:
		return r.validateReconfigure(cluster)
	}
	return nil
}

// validateUpgrade validates spec.restart
func (r *OpsRequest) validateRestart(cluster *Cluster) error {
	restartList := r.Spec.RestartList
	if len(restartList) == 0 {
		return notEmptyError("spec.restart")
	}

	compNames := make([]string, len(restartList))
	for i, v := range restartList {
		compNames[i] = v.ComponentName
	}
	return r.checkComponentExistence(cluster, compNames)
}

// validateUpgrade validates spec.clusterOps.upgrade
func (r *OpsRequest) validateUpgrade(ctx context.Context,
	k8sClient client.Client) error {
	if r.Spec.Upgrade == nil {
		return notEmptyError("spec.upgrade")
	}

	clusterVersion := &ClusterVersion{}
	clusterVersionRef := r.Spec.Upgrade.ClusterVersionRef
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: clusterVersionRef}, clusterVersion); err != nil {
		return fmt.Errorf("get clusterVersion: %s failed, err: %s", clusterVersionRef, err.Error())
	}
	return nil
}

// validateVerticalScaling validates api when spec.type is VerticalScaling
func (r *OpsRequest) validateVerticalScaling(ctx context.Context, k8sClient client.Client, cluster *Cluster) error {
	verticalScalingList := r.Spec.VerticalScalingList
	if len(verticalScalingList) == 0 {
		return notEmptyError("spec.verticalScaling")
	}

	compClasses, err := getClasses(ctx, k8sClient, cluster.Spec.ClusterDefRef)
	if err != nil {
		return nil
	}

	getComponent := func(name string) *ClusterComponentSpec {
		for _, comp := range cluster.Spec.ComponentSpecs {
			if comp.Name == name {
				return &comp
			}
		}
		return nil
	}

	// validate resources is legal and get component name slice
	componentNames := make([]string, len(verticalScalingList))
	for i, v := range verticalScalingList {
		componentNames[i] = v.ComponentName

		if invalidValue, err := validateVerticalResourceList(v.Requests); err != nil {
			return invalidValueError(invalidValue, err.Error())
		}
		if invalidValue, err := validateVerticalResourceList(v.Limits); err != nil {
			return invalidValueError(invalidValue, err.Error())
		}
		if invalidValue, err := compareRequestsAndLimits(v.ResourceRequirements); err != nil {
			return invalidValueError(invalidValue, err.Error())
		}
		comp := getComponent(v.ComponentName)
		if comp == nil {
			continue
		}
		if classes, ok := compClasses[comp.ComponentDefRef]; ok {
			if comp.ClassDefRef.Class != "" {
				if _, ok = classes[comp.ClassDefRef.Class]; !ok {
					return field.Invalid(field.NewPath(fmt.Sprintf("spec.components[%d].classDefRef", i)), comp.ClassDefRef.Class, err.Error())
				}
			}
			if err = validateMatchingClass(classes, v.ResourceRequirements); err != nil {
				return errors.Wrapf(err, "can not find matching class for component %s", v.ComponentName)
			}
		}
	}
	return r.checkComponentExistence(cluster, componentNames)
}

// validateVerticalScaling validate api is legal when spec.type is VerticalScaling
func (r *OpsRequest) validateReconfigure(cluster *Cluster) error {
	reconfigure := r.Spec.Reconfigure
	if reconfigure == nil {
		return notEmptyError("spec.reconfigure")
	}
	return nil
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
func (r *OpsRequest) validateHorizontalScaling(ctx context.Context, cli client.Client, cluster *Cluster) error {
	horizontalScalingList := r.Spec.HorizontalScalingList
	if len(horizontalScalingList) == 0 {
		return notEmptyError("spec.horizontalScaling")
	}

	componentNames := make([]string, len(horizontalScalingList))
	for i, v := range horizontalScalingList {
		componentNames[i] = v.ComponentName
	}
	return r.checkComponentExistence(cluster, componentNames)
}

// validateVolumeExpansion validates volumeExpansion api when spec.type is VolumeExpansion
func (r *OpsRequest) validateVolumeExpansion(ctx context.Context, cli client.Client, cluster *Cluster) error {
	volumeExpansionList := r.Spec.VolumeExpansionList
	if len(volumeExpansionList) == 0 {
		return notEmptyError("spec.volumeExpansion")
	}

	componentNames := make([]string, len(volumeExpansionList))
	for i, v := range volumeExpansionList {
		componentNames[i] = v.ComponentName
	}
	if err := r.checkComponentExistence(cluster, componentNames); err != nil {
		return err
	}

	return r.checkVolumesAllowExpansion(ctx, cli, cluster)
}

// checkComponentExistence checks whether components to be operated exist in cluster spec.
func (r *OpsRequest) checkComponentExistence(cluster *Cluster, compNames []string) error {
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
		return fmt.Errorf("components: %v not found, you can view the components by command: "+
			"kbcli cluster describe %s -n %s", notFoundCompNames, cluster.Name, r.Namespace)
	}
	return nil
}

func (r *OpsRequest) checkVolumesAllowExpansion(ctx context.Context, cli client.Client, cluster *Cluster) error {
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
			if e.storageClassName == nil {
				e.storageClassName = r.getSCNameByPvc(ctx, cli, cname, vname)
			}
			allowExpansion, err := r.checkStorageClassAllowExpansion(ctx, cli, e.storageClassName)
			if err != nil {
				continue // ignore the error and take it as not-supported
			}
			vols[cname][vname] = Entity{e.existInSpec, e.storageClassName, allowExpansion}
		}
	}

	for cname, compVols := range vols {
		var (
			notFound     []string
			notSupport   []string
			notSupportSc []string
		)
		for vct, e := range compVols {
			if !e.existInSpec {
				notFound = append(notFound, vct)
			}
			if !e.allowExpansion {
				notSupport = append(notSupport, vct)
				if e.storageClassName != nil {
					notSupportSc = append(notSupportSc, *e.storageClassName)
				}
			}
		}
		if len(notFound) > 0 {
			return fmt.Errorf("volumeClaimTemplates: %v not found in component: %s, you can view infos by command: "+
				"kbcli cluster describe %s -n %s", notFound, cname, cluster.Name, r.Namespace)
		}
		if len(notSupport) > 0 {
			var notSupportScString string
			if len(notSupportSc) > 0 {
				notSupportScString = fmt.Sprintf("storageClass: %v of ", notSupportSc)
			}
			return fmt.Errorf(notSupportScString+"volumeClaimTemplate: %s not support volume expansion in component: %s, you can view infos by command: "+
				"kubectl get sc", notSupport, cname)
		}
	}
	return nil
}

// checkStorageClassAllowExpansion checks whether the specified storage class supports volume expansion.
func (r *OpsRequest) checkStorageClassAllowExpansion(ctx context.Context,
	cli client.Client,
	storageClassName *string) (bool, error) {
	if storageClassName == nil {
		return false, nil
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

// getSCNameByPvc gets the storageClassName by pvc.
func (r *OpsRequest) getSCNameByPvc(ctx context.Context,
	cli client.Client,
	compName,
	vctName string) *string {
	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := cli.List(ctx, pvcList, client.InNamespace(r.Namespace), client.MatchingLabels{
		"app.kubernetes.io/instance":        r.Spec.ClusterRef,
		"apps.kubeblocks.io/component-name": compName,
		"vct.kubeblocks.io/name":            vctName,
	}, client.Limit(1)); err != nil {
		return nil
	}
	if len(pvcList.Items) == 0 {
		return nil
	}
	return pvcList.Items[0].Spec.StorageClassName
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

func getClasses(ctx context.Context, k8sClient client.Client, clusterDef string) (map[string]map[string]*ComponentClassInstance, error) {
	ml := []client.ListOption{
		client.MatchingLabels{"clusterdefinition.kubeblocks.io/name": clusterDef},
	}
	var classDefinitionList ComponentClassDefinitionList
	if err := k8sClient.List(ctx, &classDefinitionList, ml...); err != nil {
		return nil, err
	}
	var (
		componentClasses = make(map[string]map[string]*ComponentClassInstance)
	)
	for _, classDefinition := range classDefinitionList.Items {
		componentType := classDefinition.GetLabels()["apps.kubeblocks.io/component-def-ref"]
		if componentType == "" {
			return nil, fmt.Errorf("failed to find component type")
		}
		classes := make(map[string]*ComponentClassInstance)
		for idx := range classDefinition.Status.Classes {
			cls := classDefinition.Status.Classes[idx]
			classes[cls.Name] = &cls
		}
		if _, ok := componentClasses[componentType]; !ok {
			componentClasses[componentType] = classes
		} else {
			for k, v := range classes {
				if _, exists := componentClasses[componentType][k]; exists {
					return nil, fmt.Errorf("duplicate component class %s", k)
				}
				componentClasses[componentType][k] = v
			}
		}
	}
	return componentClasses, nil
}

func validateMatchingClass(classes map[string]*ComponentClassInstance, resource corev1.ResourceRequirements) error {
	if cls := chooseComponentClasses(classes, resource.Requests); cls == nil {
		return fmt.Errorf("can not find matching class with specified requests")
	}
	if cls := chooseComponentClasses(classes, resource.Limits); cls == nil {
		return fmt.Errorf("can not find matching class with specified limits")
	}
	return nil
}

func notEmptyError(target string) error {
	return fmt.Errorf(`"%s" can not be empty`, target)
}

func invalidValueError(target string, value string) error {
	return fmt.Errorf(`invalid value for "%s": %s`, target, value)
}
