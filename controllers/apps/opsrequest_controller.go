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

package apps

import (
	"context"
	"reflect"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloadsv1alpha1 "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/operations"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// OpsRequestReconciler reconciles a OpsRequest object
type OpsRequestReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=opsrequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=opsrequests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=opsrequests/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *OpsRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("opsRequest", req.NamespacedName),
		Recorder: r.Recorder,
	}
	opsCtrlHandler := &opsControllerHandler{}
	return opsCtrlHandler.Handle(reqCtx, &operations.OpsResource{Recorder: r.Recorder},
		r.fetchOpsRequest,
		r.handleDeletion,
		r.fetchCluster,
		r.addClusterLabelAndSetOwnerReference,
		r.handleCancelSignal,
		r.handleOpsRequestByPhase,
	)
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpsRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.OpsRequest{}).
		Watches(&appsv1alpha1.Cluster{}, handler.EnqueueRequestsFromMapFunc(r.parseAllOpsRequest)).
		Watches(&workloadsv1alpha1.ReplicatedStateMachine{}, handler.EnqueueRequestsFromMapFunc(r.parseAllOpsRequestForRSM)).
		Watches(&dpv1alpha1.Backup{}, handler.EnqueueRequestsFromMapFunc(r.parseBackupOpsRequest)).
		Watches(&corev1.PersistentVolumeClaim{}, handler.EnqueueRequestsFromMapFunc(r.parseVolumeExpansionOpsRequest)).
		Complete(r)
}

// fetchOpsRequestAndCluster fetches the OpsRequest from the request.
func (r *OpsRequestReconciler) fetchOpsRequest(reqCtx intctrlutil.RequestCtx, opsRes *operations.OpsResource) (*ctrl.Result, error) {
	opsRequest := &appsv1alpha1.OpsRequest{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, opsRequest); err != nil {
		if !apierrors.IsNotFound(err) {
			return intctrlutil.ResultToP(intctrlutil.RequeueWithError(err, reqCtx.Log, ""))
		}
		// if the opsRequest is not found, we need to check if this opsRequest is deleted abnormally
		if err = r.handleOpsReqDeletedDuringRunning(reqCtx); err != nil {
			return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
		}
		return intctrlutil.ResultToP(intctrlutil.Reconciled())
	}
	opsRes.OpsRequest = opsRequest
	return nil, nil
}

// handleDeletion handles the delete event of the OpsRequest.
func (r *OpsRequestReconciler) handleDeletion(reqCtx intctrlutil.RequestCtx, opsRes *operations.OpsResource) (*ctrl.Result, error) {
	if opsRes.OpsRequest.Status.Phase == appsv1alpha1.OpsRunningPhase {
		return nil, nil
	}
	return intctrlutil.HandleCRDeletion(reqCtx, r, opsRes.OpsRequest, opsRequestFinalizerName, func() (*ctrl.Result, error) {
		// if the OpsRequest is deleted, we should clear the OpsRequest annotation in reference cluster.
		// this is mainly to prevent OpsRequest from being deleted by mistake, resulting in inconsistency.
		return nil, operations.DeleteOpsRequestAnnotationInCluster(reqCtx.Ctx, r.Client, opsRes)
	})
}

// fetchCluster fetches the Cluster from the OpsRequest.
func (r *OpsRequestReconciler) fetchCluster(reqCtx intctrlutil.RequestCtx, opsRes *operations.OpsResource) (*ctrl.Result, error) {
	cluster := &appsv1alpha1.Cluster{}
	opsBehaviour, ok := operations.GetOpsManager().OpsMap[opsRes.OpsRequest.Spec.Type]
	if !ok || opsBehaviour.OpsHandler == nil {
		return nil, operations.PatchOpsHandlerNotSupported(reqCtx.Ctx, r.Client, opsRes)
	}
	if opsBehaviour.IsClusterCreationEnabled {
		// check if the cluster already exists
		cluster.Name = opsRes.OpsRequest.Spec.ClusterRef
		cluster.Namespace = opsRes.OpsRequest.GetNamespace()
		opsRes.Cluster = cluster
		return nil, nil
	}
	if err := r.Client.Get(reqCtx.Ctx, client.ObjectKey{
		Namespace: opsRes.OpsRequest.GetNamespace(),
		Name:      opsRes.OpsRequest.Spec.ClusterRef,
	}, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			_ = operations.PatchClusterNotFound(reqCtx.Ctx, r.Client, opsRes)
		}
		return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
	}
	// set cluster variable
	opsRes.Cluster = cluster
	return nil, nil
}

// handleOpsRequestByPhase handles the OpsRequest by its phase.
func (r *OpsRequestReconciler) handleOpsRequestByPhase(reqCtx intctrlutil.RequestCtx, opsRes *operations.OpsResource) (*ctrl.Result, error) {
	switch opsRes.OpsRequest.Status.Phase {
	case "":
		// update status.phase to pending
		if err := operations.PatchOpsStatus(reqCtx.Ctx, r.Client, opsRes, appsv1alpha1.OpsPendingPhase, appsv1alpha1.NewProgressingCondition(opsRes.OpsRequest)); err != nil {
			return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
		}
		return intctrlutil.ResultToP(intctrlutil.Reconciled())
	case appsv1alpha1.OpsPendingPhase, appsv1alpha1.OpsCreatingPhase:
		return r.doOpsRequestAction(reqCtx, opsRes)
	case appsv1alpha1.OpsRunningPhase, appsv1alpha1.OpsCancellingPhase:
		return r.reconcileStatusDuringRunningOrCanceling(reqCtx, opsRes)
	case appsv1alpha1.OpsSucceedPhase:
		return r.handleSucceedOpsRequest(reqCtx, opsRes.OpsRequest)
	case appsv1alpha1.OpsFailedPhase, appsv1alpha1.OpsCancelledPhase:
		return intctrlutil.ResultToP(intctrlutil.Reconciled())
	}
	return intctrlutil.ResultToP(intctrlutil.Reconciled())
}

// handleCancelSignal handles the cancel signal for opsRequest.
func (r *OpsRequestReconciler) handleCancelSignal(reqCtx intctrlutil.RequestCtx, opsRes *operations.OpsResource) (*ctrl.Result, error) {
	opsRequest := opsRes.OpsRequest
	if !opsRequest.Spec.Cancel {
		return nil, nil
	}
	if opsRequest.IsComplete() || opsRequest.Status.Phase == appsv1alpha1.OpsCancellingPhase {
		return nil, nil
	}
	opsBehaviour := operations.GetOpsManager().OpsMap[opsRequest.Spec.Type]
	if opsBehaviour.CancelFunc == nil {
		r.Recorder.Eventf(opsRequest, corev1.EventTypeWarning, reasonOpsCancelActionNotSupported,
			"Type: %s does not support cancel action.", opsRequest.Spec.Type)
		return nil, nil
	}
	deepCopyOps := opsRequest.DeepCopy()
	if err := opsBehaviour.CancelFunc(reqCtx, r.Client, opsRes); err != nil {
		r.Recorder.Eventf(opsRequest, corev1.EventTypeWarning, reasonOpsCancelActionFailed, err.Error())
		return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
	}
	opsRequest.Status.CancelTimestamp = metav1.Time{Time: time.Now()}
	if err := operations.PatchOpsStatusWithOpsDeepCopy(reqCtx.Ctx, r.Client, opsRes, deepCopyOps,
		appsv1alpha1.OpsCancellingPhase, appsv1alpha1.NewCancelingCondition(opsRes.OpsRequest)); err != nil {
		return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
	}
	return intctrlutil.ResultToP(intctrlutil.Reconciled())
}

// handleSucceedOpsRequest the opsRequest will be deleted after one hour when status.phase is Succeed
func (r *OpsRequestReconciler) handleSucceedOpsRequest(reqCtx intctrlutil.RequestCtx, opsRequest *appsv1alpha1.OpsRequest) (*ctrl.Result, error) {
	if opsRequest.Status.CompletionTimestamp.IsZero() || opsRequest.Spec.TTLSecondsAfterSucceed == 0 {
		return intctrlutil.ResultToP(intctrlutil.Reconciled())
	}
	deadline := opsRequest.Status.CompletionTimestamp.Add(time.Duration(opsRequest.Spec.TTLSecondsAfterSucceed) * time.Second)
	if time.Now().Before(deadline) {
		return intctrlutil.ResultToP(intctrlutil.RequeueAfter(time.Until(deadline), reqCtx.Log, ""))
	}
	// the opsRequest will be deleted after spec.ttlSecondsAfterSucceed seconds when status.phase is Succeed
	if err := r.Client.Delete(reqCtx.Ctx, opsRequest); err != nil {
		return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
	}
	return intctrlutil.ResultToP(intctrlutil.Reconciled())
}

// reconcileStatusDuringRunningOrCanceling reconciles the status of OpsRequest when it is running or canceling.
func (r *OpsRequestReconciler) reconcileStatusDuringRunningOrCanceling(reqCtx intctrlutil.RequestCtx, opsRes *operations.OpsResource) (*ctrl.Result, error) {
	opsRequest := opsRes.OpsRequest
	// wait for OpsRequest.status.phase to Succeed
	if requeueAfter, err := operations.GetOpsManager().Reconcile(reqCtx, r.Client, opsRes); err != nil {
		r.Recorder.Eventf(opsRequest, corev1.EventTypeWarning, reasonOpsReconcileStatusFailed, "Failed to reconcile the status of OpsRequest: %s", err.Error())
		return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
	} else if requeueAfter != 0 {
		// if the reconcileAction need requeue, do it
		return intctrlutil.ResultToP(intctrlutil.RequeueAfter(requeueAfter, reqCtx.Log, ""))
	}
	return intctrlutil.ResultToP(intctrlutil.Reconciled())
}

// addClusterLabelAndSetOwnerReference adds the cluster label and set the owner reference of the OpsRequest.
func (r *OpsRequestReconciler) addClusterLabelAndSetOwnerReference(reqCtx intctrlutil.RequestCtx, opsRes *operations.OpsResource) (*ctrl.Result, error) {
	// if the opsBehaviour will create cluster, the cluster don't exist now
	// so don't add label and set owner reference in here
	// it should be done in this opsRequest action
	opsBehaviour := operations.GetOpsManager().OpsMap[opsRes.OpsRequest.Spec.Type]
	if opsBehaviour.IsClusterCreationEnabled {
		return nil, nil
	}

	// add label of clusterRef
	opsRequest := opsRes.OpsRequest
	clusterName := opsRequest.Labels[constant.AppInstanceLabelKey]
	opsType := opsRequest.Labels[constant.OpsRequestTypeLabelKey]
	if clusterName == opsRequest.Spec.ClusterRef && opsType == string(opsRequest.Spec.Type) {
		return nil, nil
	}
	patch := client.MergeFrom(opsRequest.DeepCopy())
	if opsRequest.Labels == nil {
		opsRequest.Labels = map[string]string{}
	}
	opsRequest.Labels[constant.AppInstanceLabelKey] = opsRequest.Spec.ClusterRef
	opsRequest.Labels[constant.OpsRequestTypeLabelKey] = string(opsRequest.Spec.Type)
	scheme, _ := appsv1alpha1.SchemeBuilder.Build()
	if err := controllerutil.SetOwnerReference(opsRes.Cluster, opsRequest, scheme); err != nil {
		return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
	}
	if err := r.Client.Patch(reqCtx.Ctx, opsRequest, patch); err != nil {
		return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
	}
	return intctrlutil.ResultToP(intctrlutil.Reconciled())
}

// doOpsRequestAction will do the action of the OpsRequest.
func (r *OpsRequestReconciler) doOpsRequestAction(reqCtx intctrlutil.RequestCtx, opsRes *operations.OpsResource) (*ctrl.Result, error) {
	// process opsRequest entry function
	opsRequest := opsRes.OpsRequest
	opsDeepCopy := opsRequest.DeepCopy()
	res, err := operations.GetOpsManager().Do(reqCtx, r.Client, opsRes)
	if err != nil {
		r.Recorder.Eventf(opsRequest, corev1.EventTypeWarning, reasonOpsDoActionFailed, "Failed to process the operation of OpsRequest: %s", err.Error())
		if !reflect.DeepEqual(opsRequest.Status, opsDeepCopy.Status) {
			if patchErr := r.Client.Status().Patch(reqCtx.Ctx, opsRequest, client.MergeFrom(opsDeepCopy)); patchErr != nil {
				return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
			}
		}
		return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
	}

	if res != nil {
		return res, nil
	}
	opsRequest.Status.Phase = appsv1alpha1.OpsRunningPhase
	opsRequest.Status.ClusterGeneration = opsRes.Cluster.Generation
	if err = r.Client.Status().Patch(reqCtx.Ctx, opsRequest, client.MergeFrom(opsDeepCopy)); err != nil {
		return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
	}
	return intctrlutil.ResultToP(intctrlutil.Reconciled())
}

// handleOpsReqDeletedDuringRunning handles the cluster annotation if the OpsRequest is deleted during running.
func (r *OpsRequestReconciler) handleOpsReqDeletedDuringRunning(reqCtx intctrlutil.RequestCtx) error {
	clusterList := &appsv1alpha1.ClusterList{}
	if err := r.Client.List(reqCtx.Ctx, clusterList, client.InNamespace(reqCtx.Req.Namespace)); err != nil {
		return err
	}
	for _, cluster := range clusterList.Items {
		opsRequestSlice, _ := opsutil.GetOpsRequestSliceFromCluster(&cluster)
		index, _ := operations.GetOpsRecorderFromSlice(opsRequestSlice, reqCtx.Req.Name)
		if index == -1 {
			continue
		}
		// if the OpsRequest is abnormal, we should clear the OpsRequest annotation in referencing cluster.
		opsRequestSlice = slices.Delete(opsRequestSlice, index, index+1)
		return opsutil.PatchClusterOpsAnnotations(reqCtx.Ctx, r.Client, &cluster, opsRequestSlice)
	}
	return nil
}

func (r *OpsRequestReconciler) getRequestsFromCluster(cluster *appsv1alpha1.Cluster) []reconcile.Request {
	var (
		opsRequestSlice []appsv1alpha1.OpsRecorder
		err             error
		requests        []reconcile.Request
	)
	if opsRequestSlice, err = opsutil.GetOpsRequestSliceFromCluster(cluster); err != nil {
		return nil
	}
	for _, v := range opsRequestSlice {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: cluster.Namespace,
				Name:      v.Name,
			},
		})
	}
	return requests
}

func (r *OpsRequestReconciler) parseAllOpsRequest(ctx context.Context, object client.Object) []reconcile.Request {
	cluster := object.(*appsv1alpha1.Cluster)
	return r.getRequestsFromCluster(cluster)
}

func (r *OpsRequestReconciler) parseAllOpsRequestForRSM(ctx context.Context, object client.Object) []reconcile.Request {
	rsm := object.(*workloadsv1alpha1.ReplicatedStateMachine)
	clusterName := rsm.Labels[constant.AppInstanceLabelKey]
	if clusterName == "" {
		return nil
	}
	cluster := &appsv1alpha1.Cluster{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: clusterName, Namespace: rsm.Namespace}, cluster); err != nil {
		return nil
	}
	return r.getRequestsFromCluster(cluster)
}

func (r *OpsRequestReconciler) parseVolumeExpansionOpsRequest(ctx context.Context, object client.Object) []reconcile.Request {
	pvc := object.(*corev1.PersistentVolumeClaim)
	if pvc.Labels[constant.AppManagedByLabelKey] != constant.AppName {
		return nil
	}
	clusterName := pvc.Labels[constant.AppInstanceLabelKey]
	if clusterName == "" {
		return nil
	}
	opsRequestList, err := appsv1alpha1.GetRunningOpsByOpsType(ctx, r.Client,
		pvc.Labels[constant.AppInstanceLabelKey], pvc.Namespace, string(appsv1alpha1.VolumeExpansionType))
	if err != nil {
		return nil
	}
	var requests []reconcile.Request
	for _, v := range opsRequestList {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: v.Namespace,
				Name:      v.Name,
			},
		})
	}
	return requests
}

func (r *OpsRequestReconciler) parseBackupOpsRequest(ctx context.Context, object client.Object) []reconcile.Request {
	backup := object.(*dpv1alpha1.Backup)
	var (
		requests []reconcile.Request
	)
	opsRequestRecorder := opsutil.GetOpsRequestFromBackup(backup)
	if opsRequestRecorder != nil {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: backup.Namespace,
				Name:      opsRequestRecorder.Name,
			},
		})
	}
	return requests
}

type opsRequestStep func(reqCtx intctrlutil.RequestCtx, opsRes *operations.OpsResource) (*ctrl.Result, error)

type opsControllerHandler struct {
}

func (h *opsControllerHandler) Handle(reqCtx intctrlutil.RequestCtx,
	opsRes *operations.OpsResource,
	steps ...opsRequestStep) (ctrl.Result, error) {
	for _, step := range steps {
		res, err := step(reqCtx, opsRes)
		if res != nil {
			return *res, err
		}
		if err != nil {
			return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
		}
	}
	return intctrlutil.Reconciled()
}
