/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"math"
	"reflect"
	"strings"
	"time"

	"golang.org/x/exp/slices"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
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
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
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
	reqCtx.Log.Info("reconcile", "opsRequest", req.NamespacedName)
	opsCtrlHandler := &opsControllerHandler{}
	return opsCtrlHandler.Handle(reqCtx, &operations.OpsResource{Recorder: r.Recorder},
		r.fetchOpsRequest,
		r.fetchCluster,
		r.handleDeletion,
		r.addClusterLabelAndSetOwnerReference,
		r.handleCancelSignal,
		r.handleOpsRequestByPhase,
	)
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpsRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return intctrlutil.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.OpsRequest{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: int(math.Ceil(viper.GetFloat64(constant.CfgKBReconcileWorkers) / 2)),
		}).
		Watches(&appsv1alpha1.Cluster{}, handler.EnqueueRequestsFromMapFunc(r.parseRunningOpsRequests)).
		Watches(&workloadsv1alpha1.InstanceSet{}, handler.EnqueueRequestsFromMapFunc(r.parseRunningOpsRequestsForInstanceSet)).
		Watches(&dpv1alpha1.Backup{}, handler.EnqueueRequestsFromMapFunc(r.parseBackupOpsRequest)).
		Watches(&corev1.PersistentVolumeClaim{}, handler.EnqueueRequestsFromMapFunc(r.parseVolumeExpansionOpsRequest)).
		Watches(&corev1.Pod{}, handler.EnqueueRequestsFromMapFunc(r.parsePod)).
		Owns(&batchv1.Job{}).
		Owns(&dpv1alpha1.Restore{}).
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
	if opsRes.OpsRequest.Status.Phase == appsv1alpha1.OpsRunningPhase && !opsRes.Cluster.IsDeleting() {
		return nil, nil
	}
	return intctrlutil.HandleCRDeletion(reqCtx, r, opsRes.OpsRequest, constant.OpsRequestFinalizerName, func() (*ctrl.Result, error) {
		if err := r.deleteCreatedPodsInKBNamespace(reqCtx, opsRes.OpsRequest); err != nil {
			return nil, err
		}
		return nil, operations.DequeueOpsRequestInClusterAnnotation(reqCtx.Ctx, r.Client, opsRes)
	})
}

// fetchCluster fetches the Cluster from the OpsRequest.
func (r *OpsRequestReconciler) fetchCluster(reqCtx intctrlutil.RequestCtx, opsRes *operations.OpsResource) (*ctrl.Result, error) {
	cluster := &appsv1alpha1.Cluster{}
	opsBehaviour, ok := operations.GetOpsManager().OpsMap[opsRes.OpsRequest.Spec.Type]
	if !ok || opsBehaviour.OpsHandler == nil {
		return nil, operations.PatchOpsHandlerNotSupported(reqCtx.Ctx, r.Client, opsRes)
	}
	if opsBehaviour.IsClusterCreation {
		// check if the cluster already exists
		cluster.Name = opsRes.OpsRequest.Spec.GetClusterName()
		cluster.Namespace = opsRes.OpsRequest.GetNamespace()
		opsRes.Cluster = cluster
		return nil, nil
	}
	if err := r.Client.Get(reqCtx.Ctx, client.ObjectKey{
		Namespace: opsRes.OpsRequest.GetNamespace(),
		Name:      opsRes.OpsRequest.Spec.GetClusterName(),
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
		if err := operations.PatchOpsStatus(reqCtx.Ctx, r.Client, opsRes, appsv1alpha1.OpsPendingPhase,
			appsv1alpha1.NewWaitForProcessingCondition(opsRes.OpsRequest)); err != nil {
			return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
		}
		return intctrlutil.ResultToP(intctrlutil.Reconciled())
	case appsv1alpha1.OpsPendingPhase, appsv1alpha1.OpsCreatingPhase:
		return r.doOpsRequestAction(reqCtx, opsRes)
	case appsv1alpha1.OpsRunningPhase, appsv1alpha1.OpsCancellingPhase:
		return r.reconcileStatusDuringRunningOrCanceling(reqCtx, opsRes)
	case appsv1alpha1.OpsSucceedPhase:
		return r.handleSucceedOpsRequest(reqCtx, opsRes.OpsRequest)
	default:
		return r.handleUnsuccessfulCompletionOpsRequest(reqCtx, opsRes)
	}
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
	if opsRequest.Status.Phase == appsv1alpha1.OpsPendingPhase {
		return &ctrl.Result{}, operations.PatchOpsStatus(reqCtx.Ctx, r.Client, opsRes, appsv1alpha1.OpsCancelledPhase)
	}
	opsBehaviour := operations.GetOpsManager().OpsMap[opsRequest.Spec.Type]
	if opsBehaviour.CancelFunc == nil {
		r.Recorder.Eventf(opsRequest, corev1.EventTypeWarning, reasonOpsCancelActionNotSupported,
			"Type: %s does not support cancel action if the phase is not Pending.", opsRequest.Spec.Type)
		return nil, nil
	}
	deepCopyOps := opsRequest.DeepCopy()
	if err := opsBehaviour.CancelFunc(reqCtx, r.Client, opsRes); err != nil {
		if intctrlutil.IsTargetError(err, intctrlutil.ErrorIgnoreCancel) {
			r.Recorder.Eventf(opsRequest, corev1.EventTypeWarning, reasonOpsCancelActionNotSupported, err.Error())
			return nil, nil
		}
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
	if err := r.annotateRelatedOps(reqCtx, opsRequest); err != nil {
		return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
	}
	if err := r.deleteExternalJobs(reqCtx.Ctx, opsRequest); err != nil {
		return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
	}
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

func (r *OpsRequestReconciler) handleUnsuccessfulCompletionOpsRequest(reqCtx intctrlutil.RequestCtx, opsRes *operations.OpsResource) (*ctrl.Result, error) {
	opsRequest := opsRes.OpsRequest
	if err := r.annotateRelatedOps(reqCtx, opsRes.OpsRequest); err != nil {
		return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
	}
	if err := r.cleanupOpsAnnotationForCluster(reqCtx, opsRes.Cluster); err != nil {
		return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
	}
	if opsRequest.Status.CompletionTimestamp.IsZero() || opsRequest.Spec.TTLSecondsAfterUnsuccessfulCompletion == 0 {
		return intctrlutil.ResultToP(intctrlutil.Reconciled())
	}
	deadline := opsRequest.Status.CompletionTimestamp.Add(time.Duration(opsRequest.Spec.TTLSecondsAfterUnsuccessfulCompletion) * time.Second)
	if time.Now().Before(deadline) {
		return intctrlutil.ResultToP(intctrlutil.RequeueAfter(time.Until(deadline), reqCtx.Log, ""))
	}
	// the opsRequest will be deleted after spec.ttlSecondsAfterUnsuccessfulCompletion seconds when status.phase is Failed, Cancelled or Aborted
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
		if !apierrors.IsConflict(err) {
			r.Recorder.Eventf(opsRequest, corev1.EventTypeWarning, reasonOpsReconcileStatusFailed, "Failed to reconcile the status of OpsRequest: %s", err.Error())
		}
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
	if opsBehaviour.IsClusterCreation {
		return nil, nil
	}

	// add label of clusterRef
	opsRequest := opsRes.OpsRequest
	clusterName := opsRequest.Labels[constant.AppInstanceLabelKey]
	opsType := opsRequest.Labels[constant.OpsRequestTypeLabelKey]
	if clusterName == opsRequest.Spec.GetClusterName() && opsType == string(opsRequest.Spec.Type) {
		return nil, nil
	}
	patch := client.MergeFrom(opsRequest.DeepCopy())
	if opsRequest.Labels == nil {
		opsRequest.Labels = map[string]string{}
	}
	opsRequest.Labels[constant.AppInstanceLabelKey] = opsRequest.Spec.GetClusterName()
	opsRequest.Labels[constant.OpsRequestTypeLabelKey] = string(opsRequest.Spec.Type)
	scheme, _ := appsv1alpha1.SchemeBuilder.Build()
	if err := controllerutil.SetOwnerReference(opsRes.Cluster, opsRequest, scheme); err != nil {
		return intctrlutil.ResultToP(intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, ""))
	}
	// mutate the clusterRef to clusterName.
	// TODO: remove it after 0.9.0
	if opsRequest.Spec.ClusterName == "" {
		opsRequest.Spec.ClusterName = opsRequest.Spec.ClusterRef
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
		if !apierrors.IsConflict(err) {
			r.Recorder.Eventf(opsRequest, corev1.EventTypeWarning, reasonOpsDoActionFailed, "Failed to process the operation of OpsRequest: %s", err.Error())
		}
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
		if err := r.cleanupOpsAnnotationForCluster(reqCtx, &cluster); err != nil {
			return err
		}
	}
	return nil
}

func (r *OpsRequestReconciler) cleanupOpsAnnotationForCluster(reqCtx intctrlutil.RequestCtx, cluster *appsv1alpha1.Cluster) error {
	opsRequestSlice, _ := opsutil.GetOpsRequestSliceFromCluster(cluster)
	index, _ := operations.GetOpsRecorderFromSlice(opsRequestSlice, reqCtx.Req.Name)
	if index == -1 {
		return nil
	}
	// if the OpsRequest is abnormal, we should clear the OpsRequest annotation in referencing cluster.
	opsRequestSlice = slices.Delete(opsRequestSlice, index, index+1)
	return opsutil.UpdateClusterOpsAnnotations(reqCtx.Ctx, r.Client, cluster, opsRequestSlice)
}

func (r *OpsRequestReconciler) getRunningOpsRequestsFromCluster(cluster *appsv1alpha1.Cluster) []reconcile.Request {
	var (
		opsRequestSlice []appsv1alpha1.OpsRecorder
		err             error
		requests        []reconcile.Request
		clusterType     = "cluster"
		typeSet         = map[string]appsv1alpha1.OpsRecorder{}
	)
	if opsRequestSlice, err = opsutil.GetOpsRequestSliceFromCluster(cluster); err != nil {
		return nil
	}
	for i := range opsRequestSlice {
		ops := opsRequestSlice[i]
		if !ops.InQueue {
			// append running opsRequest
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: cluster.Namespace,
					Name:      ops.Name,
				},
			})
		}
		opsType := string(ops.Type)
		if !opsRequestSlice[i].QueueBySelf {
			// If opsRequest is not the type-scope queue, unified as "cluster" scope.
			opsType = clusterType
		}
		if _, ok := typeSet[opsType]; !ok {
			typeSet[opsType] = ops
		}
	}

	for _, v := range typeSet {
		if v.InQueue {
			// append the first opsRequest which is in the queue.
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: cluster.Namespace,
					Name:      v.Name,
				},
			})
		}
	}
	return requests
}

func (r *OpsRequestReconciler) parseRunningOpsRequests(ctx context.Context, object client.Object) []reconcile.Request {
	cluster := object.(*appsv1alpha1.Cluster)
	return r.getRunningOpsRequestsFromCluster(cluster)
}

func (r *OpsRequestReconciler) parseRunningOpsRequestsForInstanceSet(ctx context.Context, object client.Object) []reconcile.Request {
	its := object.(*workloadsv1alpha1.InstanceSet)
	clusterName := its.Labels[constant.AppInstanceLabelKey]
	if clusterName == "" {
		return nil
	}
	cluster := &appsv1alpha1.Cluster{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: clusterName, Namespace: its.Namespace}, cluster); err != nil {
		return nil
	}
	return r.getRunningOpsRequestsFromCluster(cluster)
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

func (r *OpsRequestReconciler) deleteExternalJobs(ctx context.Context, ops *appsv1alpha1.OpsRequest) error {
	jobList := &batchv1.JobList{}
	if err := r.Client.List(ctx, jobList, client.InNamespace(ops.Namespace), client.MatchingLabels{constant.OpsRequestNameLabelKey: ops.Name}); err != nil {
		return err
	}
	for i := range jobList.Items {
		if err := intctrlutil.BackgroundDeleteObject(r.Client, ctx, &jobList.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
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

func (r *OpsRequestReconciler) parsePod(ctx context.Context, object client.Object) []reconcile.Request {
	pod := object.(*corev1.Pod)
	var (
		requests []reconcile.Request
	)
	opsName := pod.Labels[constant.OpsRequestNameLabelKey]
	opsNamespace := pod.Labels[constant.OpsRequestNamespaceLabelKey]
	if opsName != "" && opsNamespace != "" {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: opsNamespace,
				Name:      opsName,
			},
		})
	}
	return requests
}

func (r *OpsRequestReconciler) deleteCreatedPodsInKBNamespace(reqCtx intctrlutil.RequestCtx, opsRequest *appsv1alpha1.OpsRequest) error {
	namespace := viper.GetString(constant.CfgKeyCtrlrMgrNS)
	if namespace == "" {
		return nil
	}
	podList := &corev1.PodList{}
	if err := r.Client.List(reqCtx.Ctx, podList, client.InNamespace(viper.GetString(constant.CfgKeyCtrlrMgrNS)), client.MatchingLabels{
		constant.OpsRequestNameLabelKey:      opsRequest.Name,
		constant.OpsRequestNamespaceLabelKey: opsRequest.Namespace,
	}); err != nil {
		return err
	}
	for i := range podList.Items {
		if err := intctrlutil.BackgroundDeleteObject(r.Client, reqCtx.Ctx, &podList.Items[i]); err != nil {
			return err
		}
	}
	return nil
}

// annotateRelatedOps annotates the related opsRequests to reconcile.
func (r *OpsRequestReconciler) annotateRelatedOps(reqCtx intctrlutil.RequestCtx, opsRequest *appsv1alpha1.OpsRequest) error {
	relatedOpsStr := opsRequest.Annotations[constant.RelatedOpsAnnotationKey]
	if relatedOpsStr == "" {
		return nil
	}
	relatedOpsNames := strings.Split(relatedOpsStr, ",")
	for _, opsName := range relatedOpsNames {
		relatedOps := &appsv1alpha1.OpsRequest{}
		if err := r.Client.Get(reqCtx.Ctx, client.ObjectKey{Name: opsName, Namespace: opsRequest.Namespace}, relatedOps); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		if relatedOps.Annotations[constant.ReconcileAnnotationKey] == opsRequest.ResourceVersion {
			continue
		}
		if relatedOps.Annotations == nil {
			relatedOps.Annotations = map[string]string{}
		}
		relatedOps.Annotations[constant.ReconcileAnnotationKey] = opsRequest.ResourceVersion
		if err := r.Client.Update(reqCtx.Ctx, relatedOps); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
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
