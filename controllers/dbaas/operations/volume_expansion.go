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

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type volumeExpansion struct{}

const (
	// VolumeExpansionTimeOut volume expansion timeout.
	VolumeExpansionTimeOut = 30 * time.Minute
)

func init() {
	ve := volumeExpansion{}
	// the volume expansion operation only support online expanding now, so this operation not affect the cluster availability.
	volumeExpansionBehaviour := OpsBehaviour{
		FromClusterPhases: []dbaasv1alpha1.Phase{
			dbaasv1alpha1.RunningPhase, dbaasv1alpha1.FailedPhase,
			dbaasv1alpha1.AbnormalPhase, dbaasv1alpha1.ConditionsErrorPhase,
		},
		ToClusterPhase: dbaasv1alpha1.VolumeExpandingPhase,
		OpsHandler:     ve,
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(dbaasv1alpha1.VolumeExpansionType, volumeExpansionBehaviour)
}

// ActionStartedCondition the started condition when handle the volume expansion request.
func (ve volumeExpansion) ActionStartedCondition(opsRequest *dbaasv1alpha1.OpsRequest) *metav1.Condition {
	return dbaasv1alpha1.NewVolumeExpandingCondition(opsRequest)
}

// Action Modify Cluster.spec.components[*].VolumeClaimTemplates[*].spec.resources
func (ve volumeExpansion) Action(opsRes *OpsResource) error {
	var (
		volumeExpansionMap = opsRes.OpsRequest.CovertVolumeExpansionListToMap()
		volumeExpansionOps dbaasv1alpha1.VolumeExpansion
		ok                 bool
	)
	for index, component := range opsRes.Cluster.Spec.Components {
		if volumeExpansionOps, ok = volumeExpansionMap[component.Name]; !ok {
			continue
		}
		for _, v := range volumeExpansionOps.VolumeClaimTemplates {
			for i, vct := range component.VolumeClaimTemplates {
				if vct.Name != v.Name {
					continue
				}
				if vct.Spec == nil {
					continue
				}
				opsRes.Cluster.Spec.Components[index].VolumeClaimTemplates[i].
					Spec.Resources.Requests[corev1.ResourceStorage] = v.Storage
			}
		}

	}
	return opsRes.Client.Update(opsRes.Ctx, opsRes.Cluster)
}

// ReconcileAction it will be performed when action is done and loop util OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for volume expansion opsRequest.
func (ve volumeExpansion) ReconcileAction(opsRes *OpsResource) (dbaasv1alpha1.Phase, time.Duration, error) {
	var (
		opsRequest = opsRes.OpsRequest
		// decide whether all pvcs of volumeClaimTemplate are Failed or Succeed
		allVCTCompleted      = true
		requeueAfter         time.Duration
		err                  error
		opsRequestPhase      = dbaasv1alpha1.RunningPhase
		oldOpsRequestStatus  = opsRequest.Status.DeepCopy()
		oldClusterStatus     = opsRes.Cluster.Status.DeepCopy()
		expectProgressCount  int
		succeedProgressCount int
	)

	patch := client.MergeFrom(opsRequest.DeepCopy())
	clusterPatch := client.MergeFrom(opsRes.Cluster.DeepCopy())
	if opsRequest.Status.Components == nil {
		ve.initStatusComponents(opsRequest)
	}
	storageMap := ve.getRequestStorageMap(opsRequest)
	// reconcile the status.components. when the volume expansion is successful,
	// sync the volumeClaimTemplate status and component phase On the OpsRequest and Cluster.
	for _, v := range opsRequest.Spec.VolumeExpansionList {
		statusComponent := opsRequest.Status.Components[v.ComponentName]
		completedOnComponent := true
		for _, vct := range v.VolumeClaimTemplates {
			succeedCount, expectCount, isCompleted, err := ve.handleVCTExpansionProgress(opsRes,
				&statusComponent, storageMap, v.ComponentName, vct.Name)
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
		ve.setComponentPhaseForClusterAndOpsRequest(&statusComponent, opsRes.Cluster, v.ComponentName, completedOnComponent)
		opsRequest.Status.Components[v.ComponentName] = statusComponent
	}
	opsRequest.Status.Progress = fmt.Sprintf("%d/%d", succeedProgressCount, expectProgressCount)

	// patch OpsRequest.status.components
	if !reflect.DeepEqual(oldOpsRequestStatus, opsRequest.Status) {
		if err = opsRes.Client.Status().Patch(opsRes.Ctx, opsRequest, patch); err != nil {
			return opsRequestPhase, requeueAfter, err
		}
	}

	// check all pvcs of volumeClaimTemplate are successful
	allVCTSucceed := expectProgressCount == succeedProgressCount
	if allVCTSucceed {
		opsRequestPhase = dbaasv1alpha1.SucceedPhase
	} else if allVCTCompleted {
		// all volume claim template volume expansion completed, but allVCTSucceed is false.
		// decide the OpsRequest is failed.
		opsRequestPhase = dbaasv1alpha1.FailedPhase
	}

	if ve.checkIsTimeOut(opsRequest, allVCTSucceed) {
		// if volume expansion timed out, do it
		opsRequestPhase = dbaasv1alpha1.FailedPhase
		err = errors.New(fmt.Sprintf("Timed out waiting for volume expansion completed, the timeout is %g minutes", VolumeExpansionTimeOut.Minutes()))
	}

	// when opsRequest completed or cluster status is changed, do it
	if patchErr := ve.patchClusterStatus(opsRes, opsRequestPhase, oldClusterStatus, clusterPatch); patchErr != nil {
		return "", requeueAfter, patchErr
	}

	return opsRequestPhase, requeueAfter, err
}

// GetRealAffectedComponentMap get the real affected component map for the operation
func (ve volumeExpansion) GetRealAffectedComponentMap(opsRequest *dbaasv1alpha1.OpsRequest) realAffectedComponentMap {
	return opsRequest.GetVolumeExpansionComponentNameMap()
}

// SaveLastConfiguration record last configuration to the OpsRequest.status.lastConfiguration
func (ve volumeExpansion) SaveLastConfiguration(opsRes *OpsResource) error {
	opsRequest := opsRes.OpsRequest
	componentNameMap := opsRequest.GetComponentNameMap()
	storageMap := ve.getRequestStorageMap(opsRequest)
	lastComponentInfo := map[string]dbaasv1alpha1.LastComponentConfiguration{}
	for _, v := range opsRes.Cluster.Spec.Components {
		if _, ok := componentNameMap[v.Name]; !ok {
			continue
		}
		lastVCTs := make([]dbaasv1alpha1.OpsRequestVolumeClaimTemplate, 0)
		for _, vct := range v.VolumeClaimTemplates {
			key := getComponentVCTKey(v.Name, vct.Name)
			if _, ok := storageMap[key]; !ok {
				continue
			}
			lastVCTs = append(lastVCTs, dbaasv1alpha1.OpsRequestVolumeClaimTemplate{
				Name:    vct.Name,
				Storage: vct.Spec.Resources.Requests[corev1.ResourceStorage],
			})
		}
		lastComponentInfo[v.Name] = dbaasv1alpha1.LastComponentConfiguration{
			VolumeClaimTemplates: lastVCTs,
		}
	}
	patch := client.MergeFrom(opsRequest.DeepCopy())
	opsRequest.Status.LastConfiguration = dbaasv1alpha1.LastConfiguration{
		Components: lastComponentInfo,
	}
	return opsRes.Client.Status().Patch(opsRes.Ctx, opsRequest, patch)
}

// checkIsTimeOut check whether the volume expansion operation has timed out
func (ve volumeExpansion) checkIsTimeOut(opsRequest *dbaasv1alpha1.OpsRequest, allVCTSucceed bool) bool {
	return !allVCTSucceed && time.Now().After(opsRequest.Status.StartTimestamp.Add(VolumeExpansionTimeOut))
}

// setClusterComponentPhaseToRunning when component expand volume completed, check whether change the component status.
func (ve volumeExpansion) setComponentPhaseForClusterAndOpsRequest(component *dbaasv1alpha1.OpsRequestStatusComponent,
	cluster *dbaasv1alpha1.Cluster,
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
	if p == dbaasv1alpha1.VolumeExpandingPhase {
		p = dbaasv1alpha1.RunningPhase
	}
	c.Phase = p
	cluster.Status.Components[componentName] = c
	component.Phase = p
}

// isExpansionCompleted check the expansion is completed
func (ve volumeExpansion) isExpansionCompleted(phase dbaasv1alpha1.ProgressStatus) bool {
	return slices.Contains([]dbaasv1alpha1.ProgressStatus{dbaasv1alpha1.FailedProgressStatus,
		dbaasv1alpha1.SucceedProgressStatus}, phase)
}

// patchClusterStatus patch cluster status
func (ve volumeExpansion) patchClusterStatus(opsRes *OpsResource,
	opsRequestPhase dbaasv1alpha1.Phase,
	oldClusterStatus *dbaasv1alpha1.ClusterStatus,
	clusterPatch client.Patch) error {
	// when the OpsRequest.status.phase is Succeed or Failed, do it
	if opsRequestIsCompleted(opsRequestPhase) && opsRes.Cluster.Status.Phase == dbaasv1alpha1.VolumeExpandingPhase {
		opsRes.Cluster.Status.Phase = dbaasv1alpha1.RunningPhase
	}
	// if cluster status changed, patch it
	if !reflect.DeepEqual(oldClusterStatus, opsRes.Cluster.Status) {
		return opsRes.Client.Status().Patch(opsRes.Ctx, opsRes.Cluster, clusterPatch)
	}
	return nil
}

// pvcIsResizing when pvc start resizing, it will set conditions type to Resizing/FileSystemResizePending
func (ve volumeExpansion) pvcIsResizing(pvc *corev1.PersistentVolumeClaim) bool {
	var isResizing bool
	for _, condition := range pvc.Status.Conditions {
		if condition.Type == corev1.PersistentVolumeClaimResizing || condition.Type == corev1.PersistentVolumeClaimFileSystemResizePending {
			isResizing = true
			break
		}
	}
	return isResizing
}

func (ve volumeExpansion) getRequestStorageMap(opsRequest *dbaasv1alpha1.OpsRequest) map[string]resource.Quantity {
	storageMap := map[string]resource.Quantity{}
	for _, v := range opsRequest.Spec.VolumeExpansionList {
		for _, vct := range v.VolumeClaimTemplates {
			key := getComponentVCTKey(v.ComponentName, vct.Name)
			storageMap[key] = vct.Storage
		}
	}
	return storageMap
}

// initVolumeExpansionStatus init status.components for the VolumeExpansion OpsRequest
func (ve volumeExpansion) initStatusComponents(opsRequest *dbaasv1alpha1.OpsRequest) {
	opsRequest.Status.Components = map[string]dbaasv1alpha1.OpsRequestStatusComponent{}
	for _, v := range opsRequest.Spec.VolumeExpansionList {
		opsRequest.Status.Components[v.ComponentName] = dbaasv1alpha1.OpsRequestStatusComponent{
			Phase: dbaasv1alpha1.VolumeExpandingPhase,
		}
	}
}

// handleVCTExpansionProgress check whether the pvc of the volume claim template is resizing/expansion succeeded/expansion completed.
func (ve volumeExpansion) handleVCTExpansionProgress(opsRes *OpsResource,
	statusComponent *dbaasv1alpha1.OpsRequestStatusComponent,
	storageMap map[string]resource.Quantity,
	componentName, vctName string) (succeedCount int, expectCount int, isCompleted bool, err error) {
	pvcList := &corev1.PersistentVolumeClaimList{}
	if err = opsRes.Client.List(opsRes.Ctx, pvcList, client.MatchingLabels{
		intctrlutil.AppInstanceLabelKey:             opsRes.Cluster.Name,
		intctrlutil.AppComponentLabelKey:            componentName,
		intctrlutil.VolumeClaimTemplateNameLabelKey: vctName,
	}, client.InNamespace(opsRes.Cluster.Namespace)); err != nil {
		return
	}
	vctKey := getComponentVCTKey(componentName, vctName)
	requestStorage := storageMap[vctKey]
	expectCount = len(pvcList.Items)
	var completedCount int
	for _, v := range pvcList.Items {
		objectKey := getPVCProgressObjectKey(v.Name)
		progressDetail := dbaasv1alpha1.ProgressDetail{ObjectKey: objectKey, Group: vctName}
		// if the volume expand succeed
		if v.Status.Capacity.Storage().Cmp(requestStorage) >= 0 {
			succeedCount += 1
			completedCount += 1
			message := fmt.Sprintf("Successfully expand volume: %s in Component: %s ", objectKey, componentName)
			progressDetail.SetStatusAndMessage(dbaasv1alpha1.SucceedProgressStatus, message)
			SetStatusComponentProgressDetail(opsRes.Recorder, opsRes.OpsRequest, &statusComponent.ProgressDetails, progressDetail)
			continue
		}
		if ve.pvcIsResizing(&v) {
			message := fmt.Sprintf("Start expanding volume: %s in Component: %s ", objectKey, componentName)
			progressDetail.SetStatusAndMessage(dbaasv1alpha1.ProcessingProgressStatus, message)
		} else {
			message := fmt.Sprintf("Waiting for an external controller to process the pvc: %s in Component: %s ", objectKey, componentName)
			progressDetail.SetStatusAndMessage(dbaasv1alpha1.PendingProgressStatus, message)
		}
		SetStatusComponentProgressDetail(opsRes.Recorder, opsRes.OpsRequest, &statusComponent.ProgressDetails, progressDetail)
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
