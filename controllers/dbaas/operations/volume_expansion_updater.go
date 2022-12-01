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
	"time"

	"golang.org/x/exp/slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/k8score"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type PersistentVolumeClaimEventHandler struct {
}

const (
	// PvcEventTimeOut timeout of the pvc event
	PvcEventTimeOut = 30 * time.Second

	// PvcEventOccursTimes occurs times of the pvc event
	PvcEventOccursTimes int32 = 5

	// VolumeResizeFailed the event reason of volume resize failed on external-resizer(the csi driver sidecar)
	VolumeResizeFailed = "VolumeResizeFailed"
	// FileSystemResizeFailed the event reason of fileSystem resize failed on kubelet volume manager
	FileSystemResizeFailed = "FileSystemResizeFailed"
)

func init() {
	k8score.PersistentVolumeClaimHandlerMap["volume-expansion"] = handleVolumeExpansionWithPvc
	k8score.EventHandlerMap["volume-expansion"] = PersistentVolumeClaimEventHandler{}
}

// handleVolumeExpansionOperation handle the pvc for the volume expansion OpsRequest.
// it will be triggered when the PersistentVolumeClaim has changed.
func handleVolumeExpansionWithPvc(reqCtx intctrlutil.RequestCtx, cli client.Client, pvc *corev1.PersistentVolumeClaim) error {
	clusterName := pvc.Labels[intctrlutil.AppInstanceLabelKey]
	cluster := &dbaasv1alpha1.Cluster{}
	if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Name: clusterName, Namespace: pvc.Namespace}, cluster); err != nil {
		return err
	}
	// check whether the cluster is expanding volume
	opsRequestName := getOpsRequestNameFromAnnotation(cluster, dbaasv1alpha1.VolumeExpandingPhase)
	if opsRequestName == nil {
		return nil
	}
	// notice the OpsRequest to reconcile
	return PatchOpsRequestAnnotation(reqCtx.Ctx, cli, cluster, *opsRequestName)
}

// Handle the warning events on pvcs. if the events is resize failed events, update the OpsRequest.status.
func (pvcEventHandler PersistentVolumeClaimEventHandler) Handle(cli client.Client, reqCtx intctrlutil.RequestCtx, recorder record.EventRecorder, event *corev1.Event) error {
	if !pvcEventHandler.isTargetResizeFailedEvents(event) {
		return nil
	}
	if !k8score.IsOvertimeAndOccursTimesForEvent(event, PvcEventTimeOut, PvcEventOccursTimes) {
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

	// check the pvc is managed by kubeblocks
	if !intctrlutil.WorkloadFilterPredicate(pvc) {
		return nil
	}

	// here, if the volume expansion ops is running. we will change the pvc status to Failed on the OpsRequest.
	return pvcEventHandler.handlePVCFailedStatusOnOpsRequest(cli, reqCtx, recorder, event, pvc)
}

// isTargetResizeFailedEvents check the event is the resize failed events.
func (pvcEventHandler PersistentVolumeClaimEventHandler) isTargetResizeFailedEvents(event *corev1.Event) bool {
	// ignores ExternalExpanding event, this event is always exists when using csi driver.
	return event.Type == corev1.EventTypeWarning && event.InvolvedObject.Kind == intctrlutil.PersistentVolumeClaimKind &&
		slices.Index([]string{VolumeResizeFailed, FileSystemResizeFailed}, event.Reason) != -1
}

// handlePVCFailedStatusOnOpsRequest if the volume expansion ops is running. we will change the pvc status to Failed on the OpsRequest,
func (pvcEventHandler PersistentVolumeClaimEventHandler) handlePVCFailedStatusOnOpsRequest(cli client.Client, reqCtx intctrlutil.RequestCtx, recorder record.EventRecorder, event *corev1.Event, pvc *corev1.PersistentVolumeClaim) error {
	var (
		cluster = &dbaasv1alpha1.Cluster{}
		err     error
	)
	// get cluster object from the pvc
	if err = cli.Get(reqCtx.Ctx, client.ObjectKey{
		Name:      pvc.Labels[intctrlutil.AppInstanceLabelKey],
		Namespace: pvc.Namespace,
	}, cluster); err != nil {
		return err
	}
	// get the volume expansion ops which is running on cluster.
	opsRequestName := getOpsRequestNameFromAnnotation(cluster, dbaasv1alpha1.VolumeExpandingPhase)
	if opsRequestName == nil {
		return nil
	}
	opsRequest := &dbaasv1alpha1.OpsRequest{}
	if err = cli.Get(reqCtx.Ctx, client.ObjectKey{Name: *opsRequestName, Namespace: pvc.Namespace}, opsRequest); err != nil {
		return err
	}
	statusComponents := opsRequest.Status.Components
	if statusComponents == nil {
		return nil
	}
	componentName := pvc.Labels[intctrlutil.AppComponentLabelKey]
	vctName := pvc.Labels[intctrlutil.VolumeClaimTemplateNameLabelKey]
	isChanged := false
	patch := client.MergeFrom(opsRequest.DeepCopy())
	// change the pvc status to Failed in OpsRequest.status.components.
	for cName, component := range statusComponents {
		if cName != componentName {
			continue
		}
		if vctStatus, ok := component.VolumeClaimTemplates[vctName]; ok {
			if vctStatus.Message != event.Message {
				isChanged = true
			}
			vctStatus.PersistentVolumeClaimStatus[pvc.Name] = dbaasv1alpha1.StatusMessage{
				Status:  dbaasv1alpha1.FailedPhase,
				Message: event.Message,
			}
		}
		break
	}
	if isChanged {
		if err = cli.Status().Patch(reqCtx.Ctx, opsRequest, patch); err != nil {
			return err
		}
		recorder.Event(opsRequest, corev1.EventTypeWarning, event.Reason, event.Message)
	}
	return nil
}
