/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package operations

import (
	"fmt"
	"reflect"
	"slices"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/sharding"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type volumeExpansionOpsHandler struct {
}

type volumeExpansionHelper struct {
	compOps           ComponentOpsInterface
	fullComponentName string
	vctName           string
	expectCount       int
	templateName      string
	stopped           bool
	explicitOffline   sets.Set[string]
}

var _ OpsHandler = volumeExpansionOpsHandler{}

const (
	// VolumeExpansionTimeOut volume expansion timeout.
	VolumeExpansionTimeOut = 30 * time.Minute
)

func init() {
	// Expansion progress is checked on PVCs, including those retained after Stop.
	volumeExpansionBehaviour := OpsBehaviour{
		OpsHandler:  volumeExpansionOpsHandler{},
		QueueBySelf: true,
	}
	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(opsv1alpha1.VolumeExpansionType, volumeExpansionBehaviour)
}

// ActionStartedCondition the started condition when handle the volume expansion request.
func (ve volumeExpansionOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return opsv1alpha1.NewVolumeExpandingCondition(opsRes.OpsRequest), nil
}

// Action modifies Cluster.spec.components[*].VolumeClaimTemplates[*].spec.resources
func (ve volumeExpansionOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	applyVolumeExpansion := func(compSpec *appsv1.ClusterComponentSpec, obj ComponentOpsInterface) error {
		setVolumeStorage := func(volumeExpansionVCTs []opsv1alpha1.OpsRequestVolumeClaimTemplate,
			targetVCTs []appsv1.PersistentVolumeClaimTemplate) {
			for _, v := range volumeExpansionVCTs {
				for i, vct := range targetVCTs {
					if vct.Name != v.Name {
						continue
					}
					targetVCTs[i].Spec.Resources.Requests[corev1.ResourceStorage] = v.Storage
				}
			}
		}
		volumeExpansion := obj.(opsv1alpha1.VolumeExpansion)
		setVolumeStorage(volumeExpansion.VolumeClaimTemplates, compSpec.VolumeClaimTemplates)
		for _, instanceExpansion := range volumeExpansion.Instances {
			for i := range compSpec.Instances {
				if compSpec.Instances[i].Name != instanceExpansion.Name {
					continue
				}
				// An instance template may inherit the component VCT. Materialize
				// that inherited VCT before applying the per-instance override.
				for _, requested := range instanceExpansion.VolumeClaimTemplates {
					found := false
					for _, existing := range compSpec.Instances[i].VolumeClaimTemplates {
						if existing.Name == requested.Name {
							found = true
							break
						}
					}
					if !found {
						for _, inherited := range compSpec.VolumeClaimTemplates {
							if inherited.Name == requested.Name {
								compSpec.Instances[i].VolumeClaimTemplates = append(compSpec.Instances[i].VolumeClaimTemplates, *inherited.DeepCopy())
								break
							}
						}
					}
				}
				setVolumeStorage(instanceExpansion.VolumeClaimTemplates, compSpec.Instances[i].VolumeClaimTemplates)
				break
			}
		}
		return nil
	}
	compOpsSet := newComponentOpsHelper(opsRes.OpsRequest.Spec.VolumeExpansionList)
	if err := compOpsSet.updateClusterComponentsAndShardings(opsRes.Cluster, applyVolumeExpansion); err != nil {
		return err
	}
	return cli.Update(reqCtx.Ctx, opsRes.Cluster)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for volume expansion opsRequest.
func (ve volumeExpansionOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (opsv1alpha1.OpsPhase, time.Duration, error) {
	var (
		opsRequest             = opsRes.OpsRequest
		requeueAfter           time.Duration
		err                    error
		opsRequestPhase        = opsv1alpha1.OpsRunningPhase
		oldOpsRequestStatus    = opsRequest.Status.DeepCopy()
		expectProgressCount    int
		succeedProgressCount   int
		completedProgressCount int
	)
	patch := client.MergeFrom(opsRequest.DeepCopy())
	if opsRequest.Status.Components == nil {
		ve.initComponentStatus(opsRequest)
	}
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.VolumeExpansionList)
	storageMap := ve.getRequestStorageMap(opsRequest)
	var veHelpers []volumeExpansionHelper
	currentDetails := map[string][]opsv1alpha1.ProgressStatusDetail{}
	for name := range compOpsHelper.componentOpsSet {
		currentDetails[name] = nil
	}
	for _, compSpec := range opsRes.Cluster.Spec.ComponentSpecs {
		compOps, ok := compOpsHelper.componentOpsSet[compSpec.Name]
		if !ok {
			continue
		}
		veHelpers = append(veHelpers, buildVolumeExpansionHelpers(compSpec, compOps, compSpec.Name)...)
	}
	for _, spec := range opsRes.Cluster.Spec.Shardings {
		compOps, ok := compOpsHelper.componentOpsSet[spec.Name]
		if !ok {
			continue
		}
		shardingComps, err := sharding.ListShardingComponents(reqCtx.Ctx, cli, opsRes.Cluster, spec.Name)
		if err != nil {
			return opsRequestPhase, 0, err
		}
		for _, v := range shardingComps {
			if slices.ContainsFunc(spec.ShardTemplates, func(t appsv1.ShardTemplate) bool {
				return t.Name == v.Labels[constant.KBAppShardTemplateLabelKey] && t.VolumeClaimTemplates != nil
			}) {
				continue
			}
			// ShardTemplates are intentionally homogeneous for this operation;
			// use the sharding component VCTs and instance templates as the source
			// of truth while retaining each rendered shard's replica count.
			physical := spec.Template
			physical.Replicas = v.Spec.Replicas
			veHelpers = append(veHelpers, buildVolumeExpansionHelpers(physical, compOps, v.Labels[constant.KBAppComponentLabelKey])...)
		}
	}
	// reconcile the status.components. when the volume expansion is successful,
	// sync the volumeClaimTemplate status and component phase On the OpsRequest and Cluster.
	for _, veHelper := range veHelpers {
		opsCompStatus := oldOpsRequestStatus.Components[veHelper.compOps.GetComponentName()]
		key := getComponentVCTKey(veHelper.compOps.GetComponentName(), veHelper.templateName, veHelper.vctName)
		requestStorage, ok := storageMap[key]
		if !ok && veHelper.templateName != "" {
			requestStorage, ok = storageMap[getComponentVCTKey(veHelper.compOps.GetComponentName(), "", veHelper.vctName)]
		}
		if !ok {
			continue
		}
		succeedCount, completedCount, err := ve.handleVCTExpansionProgress(reqCtx, cli, opsRes,
			&opsCompStatus, requestStorage, veHelper)
		if err != nil {
			return "", requeueAfter, err
		}
		expectProgressCount += veHelper.expectCount
		succeedProgressCount += succeedCount
		completedProgressCount += completedCount
		name := veHelper.compOps.GetComponentName()
		currentDetails[name] = append(currentDetails[name], opsCompStatus.ProgressDetails...)
	}
	for name, details := range currentDetails {
		status := opsRequest.Status.Components[name]
		status.ProgressDetails = details
		opsRequest.Status.Components[name] = status
	}
	if completedProgressCount != expectProgressCount {
		requeueAfter = time.Minute
	}
	opsRequest.Status.Progress = fmt.Sprintf("%d/%d", completedProgressCount, expectProgressCount)
	// patch OpsRequest.status.components
	if !reflect.DeepEqual(*oldOpsRequestStatus, opsRequest.Status) {
		if err = cli.Status().Patch(reqCtx.Ctx, opsRequest, patch); err != nil {
			return opsRequestPhase, requeueAfter, err
		}
	}

	// check all PVCs of volumeClaimTemplate are successful
	if expectProgressCount == completedProgressCount {
		if expectProgressCount == succeedProgressCount {
			opsRequestPhase = opsv1alpha1.OpsSucceedPhase
		} else {
			opsRequestPhase = opsv1alpha1.OpsFailedPhase
		}
		return opsRequestPhase, requeueAfter, err
	}
	// check whether the volume expansion operation has timed out
	if time.Now().After(opsRequest.Status.StartTimestamp.Add(VolumeExpansionTimeOut)) {
		// if volume expansion timed out
		opsRequestPhase = opsv1alpha1.OpsFailedPhase
		err = errors.New(fmt.Sprintf("Timed out waiting for volume expansion to complete, the timeout value is %g minutes", VolumeExpansionTimeOut.Minutes()))
	}
	return opsRequestPhase, requeueAfter, err
}

func buildVolumeExpansionHelpers(compSpec appsv1.ClusterComponentSpec, compOps ComponentOpsInterface, fullComponentName string) []volumeExpansionHelper {
	volumeExpansion := compOps.(opsv1alpha1.VolumeExpansion)
	instanceVCTs := map[string]sets.Set[string]{}
	for _, instance := range volumeExpansion.Instances {
		instanceVCTs[instance.Name] = sets.New[string]()
		for _, vct := range instance.VolumeClaimTemplates {
			instanceVCTs[instance.Name].Insert(vct.Name)
		}
	}
	stopped := compSpec.Stop != nil && *compSpec.Stop
	explicitOffline := sets.New(compSpec.OfflineInstances...)
	var veHelpers []volumeExpansionHelper
	expectReplicas := compSpec.Replicas
	for _, template := range compSpec.Instances {
		expectReplicas -= template.GetReplicas()
	}
	for _, vct := range volumeExpansion.VolumeClaimTemplates {
		veHelpers = append(veHelpers, volumeExpansionHelper{
			compOps:           compOps,
			fullComponentName: fullComponentName,
			expectCount:       int(expectReplicas),
			vctName:           vct.Name,
			stopped:           stopped,
			explicitOffline:   explicitOffline,
		})
		for _, template := range compSpec.Instances {
			// An explicit VCT replaces the component declaration for this volume.
			if slices.ContainsFunc(template.VolumeClaimTemplates, func(v appsv1.PersistentVolumeClaimTemplate) bool {
				return v.Name == vct.Name
			}) && !instanceVCTs[template.Name].Has(vct.Name) {
				continue
			}
			if instanceVCTs[template.Name].Has(vct.Name) {
				continue
			}
			veHelpers = append(veHelpers, volumeExpansionHelper{
				compOps:           compOps,
				fullComponentName: fullComponentName,
				expectCount:       int(template.GetReplicas()),
				vctName:           vct.Name,
				templateName:      template.Name,
				stopped:           stopped,
				explicitOffline:   explicitOffline,
			})
		}
	}
	for _, instanceExpansion := range volumeExpansion.Instances {
		for _, template := range compSpec.Instances {
			if template.Name != instanceExpansion.Name {
				continue
			}
			for _, vct := range instanceExpansion.VolumeClaimTemplates {
				veHelpers = append(veHelpers, volumeExpansionHelper{
					compOps: compOps, fullComponentName: fullComponentName, expectCount: int(template.GetReplicas()),
					vctName: vct.Name, templateName: template.Name, stopped: stopped, explicitOffline: explicitOffline,
				})
			}
			break
		}
	}
	return veHelpers
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (ve volumeExpansionOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	opsRequest := opsRes.OpsRequest
	compOpsHelper := newComponentOpsHelper(opsRequest.Spec.VolumeExpansionList)
	storageMap := ve.getRequestStorageMap(opsRequest)
	compOpsHelper.saveLastConfigurations(opsRes, func(compSpec appsv1.ClusterComponentSpec, comOps ComponentOpsInterface) opsv1alpha1.LastComponentConfiguration {
		getLastVCTs := func(vcts []appsv1.PersistentVolumeClaimTemplate) []appsv1.PersistentVolumeClaimTemplate {
			lastVCTs := make([]appsv1.PersistentVolumeClaimTemplate, 0)
			for _, vct := range vcts {
				key := getComponentVCTKey(comOps.GetComponentName(), vct.Name)
				if _, ok := storageMap[key]; !ok {
					continue
				}
				lastVCTs = append(lastVCTs, vct)
			}
			return lastVCTs
		}
		// save the last vcts of the componnet
		lastVCTS := getLastVCTs(compSpec.VolumeClaimTemplates)
		var convertedLastVCTs []opsv1alpha1.OpsRequestVolumeClaimTemplate
		for _, v := range lastVCTS {
			convertedLastVCTs = append(convertedLastVCTs, opsv1alpha1.OpsRequestVolumeClaimTemplate{
				Name:    v.Name,
				Storage: v.Spec.Resources.Requests[corev1.ResourceStorage],
			})
		}
		lastInstances := make([]opsv1alpha1.LastInstanceConfiguration, 0)
		volumeExpansion := comOps.(opsv1alpha1.VolumeExpansion)
		for _, requested := range volumeExpansion.Instances {
			for _, instance := range compSpec.Instances {
				if instance.Name != requested.Name {
					continue
				}
				last := opsv1alpha1.LastInstanceConfiguration{Name: instance.Name, VolumeClaimTemplates: nil}
				for _, vct := range instance.VolumeClaimTemplates {
					for _, target := range requested.VolumeClaimTemplates {
						if vct.Name == target.Name {
							last.VolumeClaimTemplates = append(last.VolumeClaimTemplates, opsv1alpha1.OpsRequestVolumeClaimTemplate{Name: vct.Name, Storage: vct.Spec.Resources.Requests[corev1.ResourceStorage]})
						}
					}
				}
				lastInstances = append(lastInstances, last)
				break
			}
		}
		return opsv1alpha1.LastComponentConfiguration{VolumeClaimTemplates: convertedLastVCTs, Instances: lastInstances}
	})
	return nil
}

func (ve volumeExpansionOpsHandler) getRequestStorageMap(opsRequest *opsv1alpha1.OpsRequest) map[string]resource.Quantity {
	storageMap := map[string]resource.Quantity{}
	setStorageMap := func(vct opsv1alpha1.OpsRequestVolumeClaimTemplate, compOps opsv1alpha1.ComponentOps) {
		key := getComponentVCTKey(compOps.GetComponentName(), vct.Name)
		storageMap[key] = vct.Storage
	}
	for _, v := range opsRequest.Spec.VolumeExpansionList {
		for _, vct := range v.VolumeClaimTemplates {
			setStorageMap(vct, v.ComponentOps)
		}
		for _, instance := range v.Instances {
			for _, vct := range instance.VolumeClaimTemplates {
				storageMap[getComponentVCTKey(v.ComponentName, instance.Name, vct.Name)] = vct.Storage
			}
		}
	}
	return storageMap
}

// initComponentStatus inits status.components for the VolumeExpansion OpsRequest
func (ve volumeExpansionOpsHandler) initComponentStatus(opsRequest *opsv1alpha1.OpsRequest) {
	opsRequest.Status.Components = map[string]opsv1alpha1.OpsRequestComponentStatus{}
	for _, v := range opsRequest.Spec.VolumeExpansionList {
		opsRequest.Status.Components[v.ComponentName] = opsv1alpha1.OpsRequestComponentStatus{}
	}
}

// handleVCTExpansionProgress checks whether the pvc of the volume claim template is in (resizing, expansion succeeded, expansion completed).
func (ve volumeExpansionOpsHandler) handleVCTExpansionProgress(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	compStatus *opsv1alpha1.OpsRequestComponentStatus,
	requestStorage resource.Quantity,
	veHelper volumeExpansionHelper) (int, int, error) {
	var (
		succeedCount   int
		completedCount int
	)
	previous := *compStatus
	compStatus.ProgressDetails = nil
	if veHelper.expectCount <= 0 {
		return 0, 0, nil
	}
	runtime, err := opsRes.GetRuntime(veHelper.compOps.GetComponentName())
	if err != nil {
		return 0, 0, err
	}
	workload, err := runtime.GetWorkload(opsRes.Cluster.Namespace, opsRes.Cluster.Name, veHelper.fullComponentName)
	if err != nil {
		return 0, 0, err
	}
	desiredState := workloads.InstanceDesiredStateActive
	if veHelper.stopped {
		// Stop retains the normal allocation as Offline, while explicitly
		// offlined instances remain outside this operation's replica count.
		desiredState = workloads.InstanceDesiredStateOffline
	}
	instances, err := instanceTemplatesByState(workload.GetInstanceStatuses(), desiredState,
		func(status workloads.InstanceStatus) bool {
			return !veHelper.explicitOffline.Has(status.PodName) &&
				(status.TemplateName == nil || *status.TemplateName == veHelper.templateName)
		})
	if err != nil {
		return 0, 0, err
	}
	if len(instances) != veHelper.expectCount {
		return 0, 0, nil
	}
	for instanceName := range instances {
		instance, getErr := runtime.GetInstance(opsRes.Cluster.Namespace, opsRes.Cluster.Name, veHelper.fullComponentName, instanceName)
		if getErr != nil {
			if apierrors.IsNotFound(getErr) {
				continue
			}
			return 0, 0, getErr
		}
		volume, ok := instance.GetVolume(veHelper.vctName)
		if !ok {
			continue
		}
		objectKey := getPVCProgressObjectKey(volume.GetClaimName())
		progressDetail := opsv1alpha1.ProgressStatusDetail{ObjectKey: objectKey, Group: veHelper.vctName}
		if existing := findStatusProgressDetail(previous.ProgressDetails, objectKey); existing != nil {
			progressDetail = *existing
			// Keep the old value so the setter can detect status and message changes.
			compStatus.ProgressDetails = append(compStatus.ProgressDetails, *existing)
		}
		if progressDetail.Status == opsv1alpha1.FailedProgressStatus {
			completedCount += 1
			continue
		}
		// should check if the spec.resources.requests.storage equals to the requested storage
		// and current storage size is greater than or equal to request storage size.
		// and pvc is bound if the pvc is re-created for recovery.
		capacity := volume.GetCapacity()
		requestedStorage := volume.GetRequestedStorage()
		if capacity.Cmp(requestStorage) >= 0 &&
			requestedStorage.Cmp(requestStorage) == 0 &&
			volume.IsBound() {
			succeedCount += 1
			completedCount += 1
			message := fmt.Sprintf("Successfully expand volume: %s in component: %s", objectKey, veHelper.compOps.GetComponentName())
			progressDetail.SetStatusAndMessage(opsv1alpha1.SucceedProgressStatus, message)
			setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest, &compStatus.ProgressDetails, progressDetail)
			continue
		}
		if volume.IsExpanding() {
			message := fmt.Sprintf("Start expanding volume: %s in component: %s", objectKey, veHelper.compOps.GetComponentName())
			progressDetail.SetStatusAndMessage(opsv1alpha1.ProcessingProgressStatus, message)
		} else {
			message := fmt.Sprintf("Waiting for an external controller to process the pvc: %s in component: %s", objectKey, veHelper.compOps.GetComponentName())
			progressDetail.SetStatusAndMessage(opsv1alpha1.PendingProgressStatus, message)
		}
		setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest, &compStatus.ProgressDetails, progressDetail)
	}
	return succeedCount, completedCount, nil
}

func getComponentVCTKey(compoName string, names ...string) string {
	if len(names) == 2 && names[0] != "" {
		return fmt.Sprintf("%s.%s.%s", compoName, names[0], names[1])
	}
	if len(names) == 2 {
		return fmt.Sprintf("%s.%s", compoName, names[1])
	}
	return fmt.Sprintf("%s.%s", compoName, names[0])
}

func getPVCProgressObjectKey(pvcName string) string {
	return fmt.Sprintf("PVC/%s", pvcName)
}
