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
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/controllers/k8score"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type PersistentVolumeClaimEventHandler struct {
}

const (
	// PVCEventTimeOut timeout of the pvc event
	PVCEventTimeOut = 30 * time.Second

	// VolumeResizeFailed the event reason of volume resize failed on external-resizer(the csi driver sidecar)
	VolumeResizeFailed = "VolumeResizeFailed"
	// FileSystemResizeFailed the event reason of fileSystem resize failed on kubelet volume manager
	FileSystemResizeFailed = "FileSystemResizeFailed"
)

func init() {
	k8score.PersistentVolumeClaimHandlerMap["volume-expansion"] = handleVolumeExpansionWithPVC
	k8score.EventHandlerMap["volume-expansion"] = PersistentVolumeClaimEventHandler{}
}

// handleVolumeExpansionOperation handles the pvc for the volume expansion OpsRequest.
// it will be triggered when the PersistentVolumeClaim has changed.
func handleVolumeExpansionWithPVC(reqCtx intctrlutil.RequestCtx, cli client.Client, pvc *corev1.PersistentVolumeClaim) error {
	opsRequestList, err := appsv1alpha1.GetRunningOpsByOpsType(reqCtx.Ctx, cli,
		pvc.Labels[constant.AppInstanceLabelKey], pvc.Namespace, string(appsv1alpha1.VolumeExpansionType))
	if err != nil {
		return err
	}
	if len(opsRequestList) == 0 {
		return nil
	}
	// notice the OpsRequest to reconcile
	for _, ops := range opsRequestList {
		if err = opsutil.PatchOpsRequestReconcileAnnotation(reqCtx.Ctx, cli, pvc.Namespace, ops.Name); err != nil {
			return err
		}
	}
	return nil
}

// Handle the warning events of PVCs. if the events are resize-failed events, update the OpsRequest.status.
func (pvcEventHandler PersistentVolumeClaimEventHandler) Handle(cli client.Client,
	reqCtx intctrlutil.RequestCtx,
	recorder record.EventRecorder,
	event *corev1.Event) error {
	if !pvcEventHandler.isTargetResizeFailedEvents(event) {
		return nil
	}
	if !k8score.IsOvertimeEvent(event, PVCEventTimeOut) {
		return nil
	}
	var (
		pvc = &corev1.PersistentVolumeClaim{}
		err error
	)
	// get pvc object
	if err = cli.Get(reqCtx.Ctx, client.ObjectKey{
		Name:      event.InvolvedObject.Name,
		Namespace: event.InvolvedObject.Namespace,
	}, pvc); err != nil {
		return err
	}

	// check if the pvc is managed by kubeblocks
	if !intctrlutil.WorkloadFilterPredicate(pvc) {
		return nil
	}

	// here, if the volume expansion ops is running, change the pvc status to Failed on the OpsRequest.
	return pvcEventHandler.handlePVCFailedStatusOnRunningOpsRequests(cli, reqCtx, recorder, event, pvc)
}

// isTargetResizeFailedEvents checks the event is the resize-failed events.
func (pvcEventHandler PersistentVolumeClaimEventHandler) isTargetResizeFailedEvents(event *corev1.Event) bool {
	// ignores ExternalExpanding event, this event always exists when using csi driver.
	return event.Type == corev1.EventTypeWarning && event.InvolvedObject.Kind == constant.PersistentVolumeClaimKind &&
		slices.Index([]string{VolumeResizeFailed, FileSystemResizeFailed}, event.Reason) != -1
}

// handlePVCFailedStatusOnOpsRequest if the volume expansion ops is running, changes the pvc status to Failed on the OpsRequest,
func (pvcEventHandler PersistentVolumeClaimEventHandler) handlePVCFailedStatusOnRunningOpsRequests(cli client.Client,
	reqCtx intctrlutil.RequestCtx,
	recorder record.EventRecorder,
	event *corev1.Event,
	pvc *corev1.PersistentVolumeClaim) error {
	var (
		cluster = &appsv1alpha1.Cluster{}
		err     error
	)
	// get cluster object from the pvc
	if err = cli.Get(reqCtx.Ctx, client.ObjectKey{
		Name:      pvc.Labels[constant.AppInstanceLabelKey],
		Namespace: pvc.Namespace,
	}, cluster); err != nil {
		return err
	}
	opsRequestList, err := appsv1alpha1.GetRunningOpsByOpsType(reqCtx.Ctx, cli,
		pvc.Labels[constant.AppInstanceLabelKey], pvc.Namespace, string(appsv1alpha1.VolumeExpansionType))
	if err != nil {
		return err
	}
	if len(opsRequestList) == 0 {
		return nil
	}
	for _, ops := range opsRequestList {
		if err = pvcEventHandler.handlePVCFailedStatus(cli, reqCtx, recorder, event, pvc, &ops); err != nil {
			return err
		}
	}
	return nil
}

func (pvcEventHandler PersistentVolumeClaimEventHandler) handlePVCFailedStatus(cli client.Client,
	reqCtx intctrlutil.RequestCtx,
	recorder record.EventRecorder,
	event *corev1.Event,
	pvc *corev1.PersistentVolumeClaim,
	opsRequest *appsv1alpha1.OpsRequest) error {
	compsStatus := opsRequest.Status.Components
	if compsStatus == nil {
		return nil
	}
	componentName := pvc.Labels[constant.KBAppComponentLabelKey]
	vctName := pvc.Labels[constant.VolumeClaimTemplateNameLabelKey]
	patch := client.MergeFrom(opsRequest.DeepCopy())
	var isChanged bool
	// change the pvc status to Failed in OpsRequest.status.components.
	for cName, component := range compsStatus {
		if cName != componentName {
			continue
		}
		// save the failed message to the progressDetail.
		objectKey := getPVCProgressObjectKey(pvc.Name)
		progressDetail := findStatusProgressDetail(component.ProgressDetails, objectKey)
		if progressDetail == nil || progressDetail.Message != event.Message {
			isChanged = true
		}
		progressDetail = &appsv1alpha1.ProgressStatusDetail{
			Group:     vctName,
			ObjectKey: objectKey,
			Status:    appsv1alpha1.FailedProgressStatus,
			Message:   event.Message,
		}

		setComponentStatusProgressDetail(recorder, opsRequest, &component.ProgressDetails, *progressDetail)
		compsStatus[cName] = component
		break
	}
	if !isChanged {
		return nil
	}
	if err := cli.Status().Patch(reqCtx.Ctx, opsRequest, patch); err != nil {
		return err
	}
	recorder.Event(opsRequest, corev1.EventTypeWarning, event.Reason, event.Message)
	return nil
}
