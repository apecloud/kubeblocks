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

package operations

import (
	"context"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	opsutil "github.com/apecloud/kubeblocks/controllers/dbaas/operations/util"
	"github.com/apecloud/kubeblocks/controllers/k8score"
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
	clusterName := pvc.Labels[intctrlutil.AppInstanceLabelKey]
	cluster := &dbaasv1alpha1.Cluster{}
	if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Name: clusterName, Namespace: pvc.Namespace}, cluster); err != nil {
		return err
	}
	// check whether the cluster is expanding volume
	opsRequestName := getOpsRequestNameFromAnnotation(cluster, dbaasv1alpha1.VolumeExpandingPhase)
	if opsRequestName == "" {
		return nil
	}
	// notice the OpsRequest to reconcile
	err := opsutil.PatchOpsRequestReconcileAnnotation(reqCtx.Ctx, cli, cluster, opsRequestName)
	// if the OpsRequest is not found, means it is deleted by user.
	// we should delete the invalid OpsRequest annotation in the cluster and reconcile the cluster phase.
	if apierrors.IsNotFound(err) {
		opsRequestSlice, _ := opsutil.GetOpsRequestSliceFromCluster(cluster)
		notExistOps := map[string]struct{}{
			opsRequestName: {},
		}
		if err = opsutil.RemoveClusterInvalidOpsRequestAnnotation(reqCtx.Ctx, cli, cluster,
			opsRequestSlice, notExistOps); err != nil {
			return err
		}
		return handleClusterVolumeExpandingPhase(reqCtx.Ctx, cli, cluster)
	}
	return err
}

// handleClusterVolumeExpandingPhase this function will reconcile the cluster status phase when the OpsRequest is deleted.
func handleClusterVolumeExpandingPhase(ctx context.Context,
	cli client.Client,
	cluster *dbaasv1alpha1.Cluster) error {
	if cluster.Status.Phase != dbaasv1alpha1.VolumeExpandingPhase {
		return nil
	}
	patch := client.MergeFrom(cluster.DeepCopy())
	for k, v := range cluster.Status.Components {
		if v.Phase == dbaasv1alpha1.VolumeExpandingPhase {
			v.Phase = dbaasv1alpha1.RunningPhase
			cluster.Status.Components[k] = v
		}
	}
	cluster.Status.Phase = dbaasv1alpha1.RunningPhase
	return cli.Status().Patch(ctx, cluster, patch)
}

// Handle the warning events on pvcs. if the events are resize failed events, update the OpsRequest.status.
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

	// here, if the volume expansion ops is running. we will change the pvc status to Failed on the OpsRequest.
	return pvcEventHandler.handlePVCFailedStatusOnOpsRequest(cli, reqCtx, recorder, event, pvc)
}

// isTargetResizeFailedEvents checks the event is the resize failed events.
func (pvcEventHandler PersistentVolumeClaimEventHandler) isTargetResizeFailedEvents(event *corev1.Event) bool {
	// ignores ExternalExpanding event, this event is always exists when using csi driver.
	return event.Type == corev1.EventTypeWarning && event.InvolvedObject.Kind == intctrlutil.PersistentVolumeClaimKind &&
		slices.Index([]string{VolumeResizeFailed, FileSystemResizeFailed}, event.Reason) != -1
}

// handlePVCFailedStatusOnOpsRequest if the volume expansion ops is running. we will change the pvc status to Failed on the OpsRequest,
func (pvcEventHandler PersistentVolumeClaimEventHandler) handlePVCFailedStatusOnOpsRequest(cli client.Client,
	reqCtx intctrlutil.RequestCtx,
	recorder record.EventRecorder,
	event *corev1.Event,
	pvc *corev1.PersistentVolumeClaim) error {
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
	if opsRequestName == "" {
		return nil
	}
	opsRequest := &dbaasv1alpha1.OpsRequest{}
	if err = cli.Get(reqCtx.Ctx, client.ObjectKey{Name: opsRequestName, Namespace: pvc.Namespace}, opsRequest); err != nil {
		return err
	}
	statusComponents := opsRequest.Status.Components
	if statusComponents == nil {
		return nil
	}
	componentName := pvc.Labels[intctrlutil.AppComponentLabelKey]
	vctName := pvc.Labels[intctrlutil.VolumeClaimTemplateNameLabelKey]
	patch := client.MergeFrom(opsRequest.DeepCopy())
	var isChanged bool
	// change the pvc status to Failed in OpsRequest.status.components.
	for cName, component := range statusComponents {
		if cName != componentName {
			continue
		}
		// save the failed message to the progressDetail.
		objectKey := getPVCProgressObjectKey(pvc.Name)
		progressDetail := FindStatusProgressDetail(component.ProgressDetails, objectKey)
		if progressDetail == nil || progressDetail.Message != event.Message {
			isChanged = true
		}
		progressDetail = &dbaasv1alpha1.ProgressDetail{
			Group:     vctName,
			ObjectKey: objectKey,
			Status:    dbaasv1alpha1.FailedProgressStatus,
			Message:   event.Message,
		}

		SetStatusComponentProgressDetail(recorder, opsRequest, &component.ProgressDetails, *progressDetail)
		statusComponents[cName] = component
		break
	}
	if !isChanged {
		return nil
	}
	if err = cli.Status().Patch(reqCtx.Ctx, opsRequest, patch); err != nil {
		return err
	}
	recorder.Event(opsRequest, corev1.EventTypeWarning, event.Reason, event.Message)
	return nil
}
