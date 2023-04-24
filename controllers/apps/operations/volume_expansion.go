/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	"time"

	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type volumeExpansionOpsHandler struct{}

var _ OpsHandler = volumeExpansionOpsHandler{}

const (
	// VolumeExpansionTimeOut volume expansion timeout.
	VolumeExpansionTimeOut = 30 * time.Minute
)

func init() {
	// the volume expansion operation only support online expanding now, so this operation not affect the cluster availability.
	volumeExpansionBehaviour := OpsBehaviour{
		FromClusterPhases:                  appsv1alpha1.GetClusterUpRunningPhases(),
		ToClusterPhase:                     appsv1alpha1.SpecReconcilingClusterPhase,
		MaintainClusterPhaseBySelf:         true,
		OpsHandler:                         volumeExpansionOpsHandler{},
		ProcessingReasonInClusterCondition: ProcessingReasonVolumeExpanding,
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.VolumeExpansionType, volumeExpansionBehaviour)
}

// ActionStartedCondition the started condition when handle the volume expansion request.
func (ve volumeExpansionOpsHandler) ActionStartedCondition(opsRequest *appsv1alpha1.OpsRequest) *metav1.Condition {
	return appsv1alpha1.NewVolumeExpandingCondition(opsRequest)
}

// Action modifies Cluster.spec.components[*].VolumeClaimTemplates[*].spec.resources
func (ve volumeExpansionOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	var (
		volumeExpansionMap = opsRes.OpsRequest.Spec.ToVolumeExpansionListToMap()
		volumeExpansionOps appsv1alpha1.VolumeExpansion
		ok                 bool
	)
	for index, component := range opsRes.Cluster.Spec.ComponentSpecs {
		if volumeExpansionOps, ok = volumeExpansionMap[component.Name]; !ok {
			continue
		}
		compSpec := &opsRes.Cluster.Spec.ComponentSpecs[index]
		for _, v := range volumeExpansionOps.VolumeClaimTemplates {
			for i, vct := range component.VolumeClaimTemplates {
				if vct.Name != v.Name {
					continue
				}
				compSpec.VolumeClaimTemplates[i].
					Spec.Resources.Requests[corev1.ResourceStorage] = v.Storage
			}
		}
	}
	return cli.Update(reqCtx.Ctx, opsRes.Cluster)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for volume expansion opsRequest.
func (ve volumeExpansionOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	var (
		opsRequest = opsRes.OpsRequest
		// decide whether all pvcs of volumeClaimTemplate are Failed or Succeed
		allVCTCompleted      = true
		requeueAfter         time.Duration
		err                  error
		opsRequestPhase      = appsv1alpha1.OpsRunningPhase
		oldOpsRequestStatus  = opsRequest.Status.DeepCopy()
		oldClusterStatus     = opsRes.Cluster.Status.DeepCopy()
		expectProgressCount  int
		succeedProgressCount int
	)

	patch := client.MergeFrom(opsRequest.DeepCopy())
	clusterPatch := client.MergeFrom(opsRes.Cluster.DeepCopy())
	if opsRequest.Status.Components == nil {
		ve.initComponentStatus(opsRequest)
	}
	storageMap := ve.getRequestStorageMap(opsRequest)
	// reconcile the status.components. when the volume expansion is successful,
	// sync the volumeClaimTemplate status and component phase On the OpsRequest and Cluster.
	for _, v := range opsRequest.Spec.VolumeExpansionList {
		compStatus := opsRequest.Status.Components[v.ComponentName]
		completedOnComponent := true
		for _, vct := range v.VolumeClaimTemplates {
			succeedCount, expectCount, isCompleted, err := ve.handleVCTExpansionProgress(reqCtx, cli, opsRes,
				&compStatus, storageMap, v.ComponentName, vct.Name)
			if err != nil {
				return "", requeueAfter, err
			}
			expectProgressCount += expectCount
			succeedProgressCount += succeedCount
			if !isCompleted {
				requeueAfter = time.Minute
				allVCTCompleted = false
				completedOnComponent = false
			}
		}
		// when component expand volume completed, do it.
		ve.setComponentPhaseForClusterAndOpsRequest(&compStatus, opsRes.Cluster, v.ComponentName, completedOnComponent)
		opsRequest.Status.Components[v.ComponentName] = compStatus
	}
	opsRequest.Status.Progress = fmt.Sprintf("%d/%d", succeedProgressCount, expectProgressCount)

	// patch OpsRequest.status.components
	if !reflect.DeepEqual(oldOpsRequestStatus, opsRequest.Status) {
		if err = cli.Status().Patch(reqCtx.Ctx, opsRequest, patch); err != nil {
			return opsRequestPhase, requeueAfter, err
		}
	}

	// check all pvcs of volumeClaimTemplate are successful
	allVCTSucceed := expectProgressCount == succeedProgressCount
	if allVCTSucceed {
		opsRequestPhase = appsv1alpha1.OpsSucceedPhase
	} else if allVCTCompleted {
		// all volume claim template volume expansion completed, but allVCTSucceed is false.
		// decide the OpsRequest is failed.
		opsRequestPhase = appsv1alpha1.OpsFailedPhase
	}

	if ve.checkIsTimeOut(opsRequest, allVCTSucceed) {
		// if volume expansion timed out, do it
		opsRequestPhase = appsv1alpha1.OpsFailedPhase
		err = errors.New(fmt.Sprintf("Timed out waiting for volume expansion completed, the timeout is %g minutes", VolumeExpansionTimeOut.Minutes()))
	}

	// when opsRequest completed or cluster status is changed, do it
	if patchErr := ve.patchClusterStatus(reqCtx, cli, opsRes, opsRequestPhase, oldClusterStatus, clusterPatch); patchErr != nil {
		return "", requeueAfter, patchErr
	}

	return opsRequestPhase, requeueAfter, err
}

// GetRealAffectedComponentMap gets the real affected component map for the operation
func (ve volumeExpansionOpsHandler) GetRealAffectedComponentMap(opsRequest *appsv1alpha1.OpsRequest) realAffectedComponentMap {
	return realAffectedComponentMap(opsRequest.Spec.GetVolumeExpansionComponentNameSet())
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (ve volumeExpansionOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	opsRequest := opsRes.OpsRequest
	componentNameSet := opsRequest.GetComponentNameSet()
	storageMap := ve.getRequestStorageMap(opsRequest)
	lastComponentInfo := map[string]appsv1alpha1.LastComponentConfiguration{}
	for _, v := range opsRes.Cluster.Spec.ComponentSpecs {
		if _, ok := componentNameSet[v.Name]; !ok {
			continue
		}
		lastVCTs := make([]appsv1alpha1.OpsRequestVolumeClaimTemplate, 0)
		for _, vct := range v.VolumeClaimTemplates {
			key := getComponentVCTKey(v.Name, vct.Name)
			if _, ok := storageMap[key]; !ok {
				continue
			}
			lastVCTs = append(lastVCTs, appsv1alpha1.OpsRequestVolumeClaimTemplate{
				Name:    vct.Name,
				Storage: vct.Spec.Resources.Requests[corev1.ResourceStorage],
			})
		}
		lastComponentInfo[v.Name] = appsv1alpha1.LastComponentConfiguration{
			VolumeClaimTemplates: lastVCTs,
		}
	}
	opsRequest.Status.LastConfiguration.Components = lastComponentInfo
	return nil
}

// checkIsTimeOut check whether the volume expansion operation has timed out
func (ve volumeExpansionOpsHandler) checkIsTimeOut(opsRequest *appsv1alpha1.OpsRequest, allVCTSucceed bool) bool {
	return !allVCTSucceed && time.Now().After(opsRequest.Status.StartTimestamp.Add(VolumeExpansionTimeOut))
}

// setClusterComponentPhaseToRunning when component expand volume completed, check whether change the component status.
func (ve volumeExpansionOpsHandler) setComponentPhaseForClusterAndOpsRequest(component *appsv1alpha1.OpsRequestComponentStatus,
	cluster *appsv1alpha1.Cluster,
	componentName string,
	completedOnComponent bool) {
	if !completedOnComponent {
		return
	}
	c, ok := cluster.Status.Components[componentName]
	if !ok {
		return
	}
	p := c.Phase
	if p == appsv1alpha1.SpecReconcilingClusterCompPhase {
		p = appsv1alpha1.RunningClusterCompPhase
	}
	c.Phase = p
	cluster.Status.SetComponentStatus(componentName, c)
	component.Phase = p
}

// isExpansionCompleted check the expansion is completed
func (ve volumeExpansionOpsHandler) isExpansionCompleted(phase appsv1alpha1.ProgressStatus) bool {
	return slices.Contains([]appsv1alpha1.ProgressStatus{appsv1alpha1.FailedProgressStatus,
		appsv1alpha1.SucceedProgressStatus}, phase)
}

// patchClusterStatus patch cluster status
func (ve volumeExpansionOpsHandler) patchClusterStatus(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	opsRequestPhase appsv1alpha1.OpsPhase,
	oldClusterStatus *appsv1alpha1.ClusterStatus,
	clusterPatch client.Patch) error {
	// when the OpsRequest.status.phase is Succeed or Failed, do it
	if opsRequestIsCompleted(opsRequestPhase) && opsRes.Cluster.Status.Phase == appsv1alpha1.SpecReconcilingClusterPhase {
		opsRes.Cluster.Status.Phase = appsv1alpha1.RunningClusterPhase
	}
	// if cluster status changed, patch it
	if !reflect.DeepEqual(oldClusterStatus, opsRes.Cluster.Status) {
		return cli.Status().Patch(reqCtx.Ctx, opsRes.Cluster, clusterPatch)
	}
	return nil
}

// pvcIsResizing when pvc start resizing, it will set conditions type to Resizing/FileSystemResizePending
func (ve volumeExpansionOpsHandler) pvcIsResizing(pvc *corev1.PersistentVolumeClaim) bool {
	var isResizing bool
	for _, condition := range pvc.Status.Conditions {
		if condition.Type == corev1.PersistentVolumeClaimResizing || condition.Type == corev1.PersistentVolumeClaimFileSystemResizePending {
			isResizing = true
			break
		}
	}
	return isResizing
}

func (ve volumeExpansionOpsHandler) getRequestStorageMap(opsRequest *appsv1alpha1.OpsRequest) map[string]resource.Quantity {
	storageMap := map[string]resource.Quantity{}
	for _, v := range opsRequest.Spec.VolumeExpansionList {
		for _, vct := range v.VolumeClaimTemplates {
			key := getComponentVCTKey(v.ComponentName, vct.Name)
			storageMap[key] = vct.Storage
		}
	}
	return storageMap
}

// initComponentStatus init status.components for the VolumeExpansion OpsRequest
func (ve volumeExpansionOpsHandler) initComponentStatus(opsRequest *appsv1alpha1.OpsRequest) {
	opsRequest.Status.Components = map[string]appsv1alpha1.OpsRequestComponentStatus{}
	for _, v := range opsRequest.Spec.VolumeExpansionList {
		opsRequest.Status.Components[v.ComponentName] = appsv1alpha1.OpsRequestComponentStatus{
			Phase: appsv1alpha1.SpecReconcilingClusterCompPhase,
		}
	}
}

// handleVCTExpansionProgress check whether the pvc of the volume claim template is resizing/expansion succeeded/expansion completed.
func (ve volumeExpansionOpsHandler) handleVCTExpansionProgress(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	compStatus *appsv1alpha1.OpsRequestComponentStatus,
	storageMap map[string]resource.Quantity,
	componentName, vctName string) (succeedCount int, expectCount int, isCompleted bool, err error) {
	pvcList := &corev1.PersistentVolumeClaimList{}
	if err = cli.List(reqCtx.Ctx, pvcList, client.MatchingLabels{
		constant.AppInstanceLabelKey:             opsRes.Cluster.Name,
		constant.KBAppComponentLabelKey:          componentName,
		constant.VolumeClaimTemplateNameLabelKey: vctName,
	}, client.InNamespace(opsRes.Cluster.Namespace)); err != nil {
		return
	}
	vctKey := getComponentVCTKey(componentName, vctName)
	requestStorage := storageMap[vctKey]
	expectCount = len(pvcList.Items)
	var completedCount int
	for _, v := range pvcList.Items {
		objectKey := getPVCProgressObjectKey(v.Name)
		progressDetail := appsv1alpha1.ProgressStatusDetail{ObjectKey: objectKey, Group: vctName}
		// if the volume expand succeed
		if v.Status.Capacity.Storage().Cmp(requestStorage) >= 0 {
			succeedCount += 1
			completedCount += 1
			message := fmt.Sprintf("Successfully expand volume: %s in Component: %s ", objectKey, componentName)
			progressDetail.SetStatusAndMessage(appsv1alpha1.SucceedProgressStatus, message)
			setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest, &compStatus.ProgressDetails, progressDetail)
			continue
		}
		if ve.pvcIsResizing(&v) {
			message := fmt.Sprintf("Start expanding volume: %s in Component: %s ", objectKey, componentName)
			progressDetail.SetStatusAndMessage(appsv1alpha1.ProcessingProgressStatus, message)
		} else {
			message := fmt.Sprintf("Waiting for an external controller to process the pvc: %s in Component: %s ", objectKey, componentName)
			progressDetail.SetStatusAndMessage(appsv1alpha1.PendingProgressStatus, message)
		}
		setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest, &compStatus.ProgressDetails, progressDetail)
		if ve.isExpansionCompleted(progressDetail.Status) {
			completedCount += 1
		}
	}
	isCompleted = completedCount == len(pvcList.Items)
	return succeedCount, expectCount, isCompleted, nil
}

func getComponentVCTKey(componentName, vctName string) string {
	return fmt.Sprintf("%s/%s", componentName, vctName)
}

func getPVCProgressObjectKey(pvcName string) string {
	return fmt.Sprintf("PVC/%s", pvcName)
}
