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

	"golang.org/x/exp/slices"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
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
	volumeExpansionBehaviour := &OpsBehaviour{
		FromClusterPhases: []dbaasv1alpha1.Phase{
			dbaasv1alpha1.RunningPhase, dbaasv1alpha1.FailedPhase,
			dbaasv1alpha1.AbnormalPhase, dbaasv1alpha1.ConditionsErrorPhase,
		},
		ToClusterPhase:         dbaasv1alpha1.VolumeExpandingPhase,
		Action:                 ve.Action,
		ActionStartedCondition: dbaasv1alpha1.NewVolumeExpandingCondition,
		ReconcileAction:        ve.ReconcileAction,
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(dbaasv1alpha1.VolumeExpansionType, volumeExpansionBehaviour)
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
		// check all pvcs of volumeClaimTemplate are successful
		allVCTSucceed = true
		// decide whether all pvcs of volumeClaimTemplate are Failed or Succeed
		allVCTCompleted     = true
		requeueAfter        time.Duration
		err                 error
		opsRequestPhase     = dbaasv1alpha1.RunningPhase
		oldOpsRequestStatus = opsRequest.Status.DeepCopy()
		oldClusterStatus    = opsRes.Cluster.Status.DeepCopy()
	)

	patch := client.MergeFrom(opsRequest.DeepCopy())
	clusterPatch := client.MergeFrom(opsRes.Cluster.DeepCopy())
	if opsRequest.Status.Components == nil {
		ve.initStatusComponents(opsRequest)
	}
	// reconcile the status.components. when the volume expansion is successful,
	// sync the volumeClaimTemplate status and component phase On the OpsRequest and Cluster.
	for cName, component := range opsRequest.Status.Components {
		if component.VolumeClaimTemplates == nil {
			continue
		}
		completedOnComponent := true
		for vctName, vct := range component.VolumeClaimTemplates {
			if vct.Status == dbaasv1alpha1.SucceedPhase {
				continue
			}
			isResizing, isSucceed, isCompleted, err := ve.checkExpansionProgress(opsRes, vct, cName, vctName)
			if err != nil {
				return opsRequestPhase, requeueAfter, err
			}
			if isSucceed {
				ve.setVCTStatusMessage(vct, dbaasv1alpha1.SucceedPhase, "")
				continue
			}
			allVCTSucceed = false
			if isCompleted {
				// if not succeed and the expansion completed, it means the expansion failed.
				ve.setVCTStatusMessage(vct, dbaasv1alpha1.FailedPhase, "")
				continue
			}
			allVCTCompleted = false
			completedOnComponent = false
			requeueAfter = time.Minute
			// if pvcs is resizing
			if isResizing && vct.Status == dbaasv1alpha1.PendingPhase {
				ve.setVCTStatusMessage(vct, dbaasv1alpha1.RunningPhase, "")
			}
		}

		// when component expand volume completed, do it.
		ve.setComponentPhaseForClusterAndOpsRequest(&component, opsRes.Cluster, cName, completedOnComponent)
		opsRequest.Status.Components[cName] = component
	}

	// patch OpsRequest.status.components
	if !reflect.DeepEqual(oldOpsRequestStatus, opsRequest.Status) {
		if err = opsRes.Client.Status().Patch(opsRes.Ctx, opsRequest, patch); err != nil {
			return opsRequestPhase, requeueAfter, err
		}
	}

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

func (ve volumeExpansion) setVCTStatusMessage(vct *dbaasv1alpha1.VolumeClaimTemplateStatus, status dbaasv1alpha1.Phase, message string) {
	vct.Status = status
	vct.Message = message
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
func (ve volumeExpansion) isExpansionCompleted(phase dbaasv1alpha1.Phase) bool {
	return slices.Index([]dbaasv1alpha1.Phase{dbaasv1alpha1.FailedPhase, dbaasv1alpha1.SucceedPhase}, phase) != -1
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

// initVolumeExpansionStatus init status.components for the VolumeExpansion OpsRequest
func (ve volumeExpansion) initStatusComponents(opsRequest *dbaasv1alpha1.OpsRequest) {
	// get component map with VolumeClaimTemplateName slice
	opsRequest.Status.Components = map[string]dbaasv1alpha1.OpsRequestStatusComponent{}
	componentMap := map[string][]dbaasv1alpha1.OpsRequestVolumeClaimTemplate{}
	for _, v := range opsRequest.Spec.VolumeExpansionList {
		componentMap[v.ComponentName] = v.VolumeClaimTemplates
	}
	// update status.components
	for k, volumeExpansion := range componentMap {
		componentStatus := dbaasv1alpha1.OpsRequestStatusComponent{
			Phase:                dbaasv1alpha1.VolumeExpandingPhase,
			VolumeClaimTemplates: map[string]*dbaasv1alpha1.VolumeClaimTemplateStatus{},
		}
		for _, v := range volumeExpansion {
			vctStatus := &dbaasv1alpha1.VolumeClaimTemplateStatus{RequestStorage: v.Storage}
			vctStatus.StatusMessage.Status = dbaasv1alpha1.PendingPhase
			vctStatus.StatusMessage.Message = "waiting for an external controller to process the pvcs."
			vctStatus.PersistentVolumeClaimStatus = map[string]dbaasv1alpha1.StatusMessage{}
			componentStatus.VolumeClaimTemplates[v.Name] = vctStatus
		}
		opsRequest.Status.Components[k] = componentStatus
	}
}

// checkVolumeClaimTemplateProgress check whether the pvc of the volume claim template is resizing/expansion succeeded/expansion completed.
func (ve volumeExpansion) checkExpansionProgress(opsRes *OpsResource, vct *dbaasv1alpha1.VolumeClaimTemplateStatus, componentName, vctName string) (bool, bool, bool, error) {
	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := opsRes.Client.List(opsRes.Ctx, pvcList, client.MatchingLabels{
		intctrlutil.AppInstanceLabelKey:             opsRes.Cluster.Name,
		intctrlutil.AppComponentLabelKey:            componentName,
		intctrlutil.VolumeClaimTemplateNameLabelKey: vctName,
	}, client.InNamespace(opsRes.Cluster.Namespace)); err != nil {
		return false, false, false, err
	}
	var (
		// the pvcs of volumeClaimTemplate is resizing
		isResizing bool
		// all the pvcs of volumeClaimTemplate is successful
		isSucceed = true
		// all the pvcs of volumeClaimTemplate is Failed or Succeed
		isCompleted = true
	)
	for _, v := range pvcList.Items {
		persistentVolumeClaimStatus := dbaasv1alpha1.StatusMessage{}
		if _, ok := vct.PersistentVolumeClaimStatus[v.Name]; ok {
			persistentVolumeClaimStatus = vct.PersistentVolumeClaimStatus[v.Name]
		}
		if ve.pvcIsResizing(&v) && persistentVolumeClaimStatus.Status != dbaasv1alpha1.FailedPhase {
			persistentVolumeClaimStatus.Status = dbaasv1alpha1.RunningPhase
			isResizing = true
		}
		// if pvc volume expansion succeeded, do it
		if v.Status.Capacity.Storage().Cmp(vct.RequestStorage) == 0 {
			vct.PersistentVolumeClaimStatus[v.Name] = dbaasv1alpha1.StatusMessage{Status: dbaasv1alpha1.SucceedPhase}
			continue
		}
		if !ve.isExpansionCompleted(persistentVolumeClaimStatus.Status) {
			isCompleted = false
		}
		isSucceed = false
		vct.PersistentVolumeClaimStatus[v.Name] = persistentVolumeClaimStatus
	}
	return isResizing, isSucceed, isCompleted, nil
}
