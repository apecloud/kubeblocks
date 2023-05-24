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
	"regexp"
	"strconv"
	"time"

	"github.com/pkg/errors"
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

var pvcNameRegex = regexp.MustCompile("(.*)-([0-9]+)$")

const (
	// VolumeExpansionTimeOut volume expansion timeout.
	VolumeExpansionTimeOut = 30 * time.Minute
)

func init() {
	// the volume expansion operation only support online expanding now, so this operation not affect the cluster availability.
	volumeExpansionBehaviour := OpsBehaviour{
		OpsHandler: volumeExpansionOpsHandler{},
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
		opsRequest             = opsRes.OpsRequest
		requeueAfter           time.Duration
		err                    error
		opsRequestPhase        = appsv1alpha1.OpsRunningPhase
		oldOpsRequestStatus    = opsRequest.Status.DeepCopy()
		oldClusterStatus       = opsRes.Cluster.Status.DeepCopy()
		expectProgressCount    int
		succeedProgressCount   int
		completedProgressCount int
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
		for _, vct := range v.VolumeClaimTemplates {
			succeedCount, expectCount, completedCount, err := ve.handleVCTExpansionProgress(reqCtx, cli, opsRes,
				&compStatus, storageMap, v.ComponentName, vct.Name)
			if err != nil {
				return "", requeueAfter, err
			}
			expectProgressCount += expectCount
			succeedProgressCount += succeedCount
			completedProgressCount += completedCount
		}
		opsRequest.Status.Components[v.ComponentName] = compStatus
	}
	if completedProgressCount != expectProgressCount {
		requeueAfter = time.Minute
	}
	opsRequest.Status.Progress = fmt.Sprintf("%d/%d", completedProgressCount, expectProgressCount)
	// patch OpsRequest.status.components
	if !reflect.DeepEqual(oldOpsRequestStatus, opsRequest.Status) {
		if err = cli.Status().Patch(reqCtx.Ctx, opsRequest, patch); err != nil {
			return opsRequestPhase, requeueAfter, err
		}
	}

	// check all pvcs of volumeClaimTemplate are successful
	if expectProgressCount == completedProgressCount {
		if expectProgressCount == succeedProgressCount {
			opsRequestPhase = appsv1alpha1.OpsSucceedPhase
		} else {
			opsRequestPhase = appsv1alpha1.OpsFailedPhase
		}
	} else {
		// check whether the volume expansion operation has timed out
		if time.Now().After(opsRequest.Status.StartTimestamp.Add(VolumeExpansionTimeOut)) {
			// if volume expansion timed out, do it
			opsRequestPhase = appsv1alpha1.OpsFailedPhase
			err = errors.New(fmt.Sprintf("Timed out waiting for volume expansion completed, the timeout is %g minutes", VolumeExpansionTimeOut.Minutes()))
		}
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
		opsRequest.Status.Components[v.ComponentName] = appsv1alpha1.OpsRequestComponentStatus{}
	}
}

// handleVCTExpansionProgress check whether the pvc of the volume claim template is resizing/expansion succeeded/expansion completed.
func (ve volumeExpansionOpsHandler) handleVCTExpansionProgress(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	compStatus *appsv1alpha1.OpsRequestComponentStatus,
	storageMap map[string]resource.Quantity,
	componentName, vctName string) (int, int, int, error) {
	var (
		succeedCount   int
		expectCount    int
		completedCount int
		err            error
	)
	pvcList := &corev1.PersistentVolumeClaimList{}
	if err = cli.List(reqCtx.Ctx, pvcList, client.MatchingLabels{
		constant.AppInstanceLabelKey:             opsRes.Cluster.Name,
		constant.KBAppComponentLabelKey:          componentName,
		constant.VolumeClaimTemplateNameLabelKey: vctName,
	}, client.InNamespace(opsRes.Cluster.Namespace)); err != nil {
		return 0, 0, 0, err
	}
	comp := opsRes.Cluster.Spec.GetComponentByName(componentName)
	if comp == nil {
		err = fmt.Errorf("comp %s of cluster %s not found", componentName, opsRes.Cluster.Name)
		return 0, 0, 0, err
	}
	expectCount = int(comp.Replicas)
	vctKey := getComponentVCTKey(componentName, vctName)
	requestStorage := storageMap[vctKey]
	var ordinal int
	for _, v := range pvcList.Items {
		// filter PVC(s) with ordinal larger than comp.Replicas - 1, which left by scale-in
		ordinal, err = getPVCOrdinal(v.Name)
		if err != nil {
			return 0, 0, 0, err
		}
		if ordinal > expectCount-1 {
			continue
		}
		objectKey := getPVCProgressObjectKey(v.Name)
		progressDetail := findStatusProgressDetail(compStatus.ProgressDetails, objectKey)
		if progressDetail == nil {
			progressDetail = &appsv1alpha1.ProgressStatusDetail{ObjectKey: objectKey, Group: vctName}
		}
		if progressDetail.Status == appsv1alpha1.FailedProgressStatus {
			completedCount += 1
			continue
		}
		currStorageSize := v.Status.Capacity.Storage()
		// should check if the spec.resources.requests.storage equals to the requested storage
		// and pvc is bound if the pvc is re-created for recovery.
		if currStorageSize.Cmp(requestStorage) == 0 &&
			v.Spec.Resources.Requests.Storage().Cmp(requestStorage) == 0 &&
			v.Status.Phase == corev1.ClaimBound {
			succeedCount += 1
			completedCount += 1
			message := fmt.Sprintf("Successfully expand volume: %s in Component: %s", objectKey, componentName)
			progressDetail.SetStatusAndMessage(appsv1alpha1.SucceedProgressStatus, message)
			setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest, &compStatus.ProgressDetails, *progressDetail)
			continue
		}
		if ve.pvcIsResizing(&v) {
			message := fmt.Sprintf("Start expanding volume: %s in Component: %s ", objectKey, componentName)
			progressDetail.SetStatusAndMessage(appsv1alpha1.ProcessingProgressStatus, message)
		} else {
			message := fmt.Sprintf("Waiting for an external controller to process the pvc: %s in Component: %s ", objectKey, componentName)
			progressDetail.SetStatusAndMessage(appsv1alpha1.PendingProgressStatus, message)
		}
		setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest, &compStatus.ProgressDetails, *progressDetail)
	}
	return succeedCount, expectCount, completedCount, nil
}

func getComponentVCTKey(componentName, vctName string) string {
	return fmt.Sprintf("%s/%s", componentName, vctName)
}

func getPVCProgressObjectKey(pvcName string) string {
	return fmt.Sprintf("PVC/%s", pvcName)
}

func getPVCOrdinal(pvcName string) (int, error) {
	subMatches := pvcNameRegex.FindStringSubmatch(pvcName)
	if len(subMatches) < 3 {
		return 0, fmt.Errorf("wrong pvc name: %s", pvcName)
	}
	return strconv.Atoi(subMatches[2])
}
