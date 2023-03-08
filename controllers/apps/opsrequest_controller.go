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

package apps

import (
	"context"
	"time"

	"golang.org/x/exp/slices"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/operations"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
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
// TODO(user): Modify the Reconcile function to compare the state specified by
// the OpsRequest object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *OpsRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var (
		err error
		res *ctrl.Result
	)

	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("opsRequest", req.NamespacedName),
		Recorder: r.Recorder,
	}
	opsRequest := &appsv1alpha1.OpsRequest{}
	if err = r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, opsRequest); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	// when the opsRequest is Running, we can not delete it until user deletes the finalizer.
	if opsRequest.Status.Phase != appsv1alpha1.RunningPhase {
		res, err = intctrlutil.HandleCRDeletion(reqCtx, r, opsRequest, opsRequestFinalizerName, func() (*ctrl.Result, error) {
			return nil, r.deleteExternalResources(reqCtx, opsRequest)
		})
		if res != nil {
			return *res, err
		}
	}

	opsRes := &operations.OpsResource{
		Ctx:        ctx,
		OpsRequest: opsRequest,
		Recorder:   r.Recorder,
		Client:     r.Client,
	}

	switch opsRequest.Status.Phase {
	case "":
		// update status.phase to pending
		if err = operations.PatchOpsStatus(opsRes, appsv1alpha1.PendingPhase, appsv1alpha1.NewProgressingCondition(opsRequest)); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		return intctrlutil.Reconciled()
	case appsv1alpha1.SucceedPhase:
		return r.handleSucceedOpsRequest(reqCtx, opsRequest)
	case appsv1alpha1.FailedPhase:
		return intctrlutil.Reconciled()
	}

	// patch cluster label to OpsRequest
	if err = r.patchOpsRequestWithClusterLabel(reqCtx, opsRequest); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	// get cluster object and set it to OpsResource.Cluster
	if err = r.setClusterToOpsResource(opsRes); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	if opsRequest.Status.ObservedGeneration == opsRequest.Generation {
		// waiting until OpsRequest.status.phase is Succeed
		if requeueAfter, err := operations.GetOpsManager().Reconcile(opsRes); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		} else if requeueAfter != 0 {
			// if the reconcileAction need requeue, do it
			return intctrlutil.RequeueAfter(requeueAfter, reqCtx.Log, "")
		}
		return intctrlutil.Reconciled()
	}

	if err = r.setOwnerReferenceWithCluster(ctx, opsRes); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	// process opsRequest entry function
	if err = operations.GetOpsManager().Do(opsRes); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	if err = r.patchObservedGeneration(reqCtx, opsRequest); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpsRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.OpsRequest{}).
		Complete(r)
}

func (r *OpsRequestReconciler) deleteExternalResources(reqCtx intctrlutil.RequestCtx, opsRequest *appsv1alpha1.OpsRequest) error {
	// if the OpsRequest is deleted, we should clear the OpsRequest annotation in reference cluster.
	// this is mainly to prevent OpsRequest from being deleted by mistake, resulting in inconsistency.
	return r.deleteClusterOpsRequestAnnotation(reqCtx, opsRequest)
}

func (r *OpsRequestReconciler) deleteClusterOpsRequestAnnotation(reqCtx intctrlutil.RequestCtx,
	opsRequest *appsv1alpha1.OpsRequest) error {
	var (
		cluster         = &appsv1alpha1.Cluster{}
		opsRequestSlice []appsv1alpha1.OpsRecorder
		err             error
	)
	if err = r.Client.Get(reqCtx.Ctx, client.ObjectKey{
		Namespace: opsRequest.GetNamespace(),
		Name:      opsRequest.Spec.ClusterRef,
	}, cluster); err != nil {
		return client.IgnoreNotFound(err)
	}
	if opsRequestSlice, err = opsutil.GetOpsRequestSliceFromCluster(cluster); err != nil {
		return err
	}
	index, opsRecord := operations.GetOpsRecorderFromSlice(opsRequestSlice, opsRequest.Name)
	if opsRecord.Name == "" {
		return nil
	}
	opsRequestSlice = slices.Delete(opsRequestSlice, index, index+1)
	return opsutil.PatchClusterOpsAnnotations(reqCtx.Ctx, r.Client, cluster, opsRequestSlice)
}

// setOwnerReference st
func (r *OpsRequestReconciler) setOwnerReferenceWithCluster(ctx context.Context, opsRes *operations.OpsResource) error {
	patch := client.MergeFrom(opsRes.OpsRequest.DeepCopy())
	scheme, _ := appsv1alpha1.SchemeBuilder.Build()
	if err := controllerutil.SetOwnerReference(opsRes.Cluster, opsRes.OpsRequest, scheme); err != nil {
		return err
	}
	if err := r.Client.Patch(ctx, opsRes.OpsRequest, patch); err != nil {
		return err
	}
	return nil
}

// setClusterToOpsResource get cluster object and set it to OpsResource.Cluster
func (r *OpsRequestReconciler) setClusterToOpsResource(opsRes *operations.OpsResource) error {
	var (
		cluster = &appsv1alpha1.Cluster{}
		key     = client.ObjectKey{
			Namespace: opsRes.OpsRequest.GetNamespace(),
			Name:      opsRes.OpsRequest.Spec.ClusterRef,
		}
	)
	if err := opsRes.Client.Get(opsRes.Ctx, key, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			_ = operations.PatchClusterNotFound(opsRes)
		}
		return err
	}
	// set cluster variable
	opsRes.Cluster = cluster
	return nil
}

// handleSucceedOpsRequest the opsRequest will be deleted after one hour when status.phase is Succeed
func (r *OpsRequestReconciler) handleSucceedOpsRequest(reqCtx intctrlutil.RequestCtx, opsRequest *appsv1alpha1.OpsRequest) (ctrl.Result, error) {
	if opsRequest.Status.CompletionTimestamp.IsZero() || opsRequest.Spec.TTLSecondsAfterSucceed == 0 {
		return intctrlutil.Reconciled()
	}
	deadline := opsRequest.Status.CompletionTimestamp.Add(time.Duration(opsRequest.Spec.TTLSecondsAfterSucceed) * time.Second)
	if time.Now().Before(deadline) {
		return intctrlutil.RequeueAfter(time.Until(deadline), reqCtx.Log, "")
	}
	// the opsRequest will be deleted after spec.ttlSecondsAfterSucceed seconds when status.phase is Succeed
	if err := r.Client.Delete(reqCtx.Ctx, opsRequest); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

func (r *OpsRequestReconciler) patchOpsRequestWithClusterLabel(reqCtx intctrlutil.RequestCtx, opsRequest *appsv1alpha1.OpsRequest) error {
	// add label of clusterDefinitionRef
	if opsRequest.Labels == nil {
		opsRequest.Labels = map[string]string{}
	}
	clusterName := opsRequest.Labels[constant.AppInstanceLabelKey]
	if clusterName == opsRequest.Spec.ClusterRef {
		return nil
	}
	patch := client.MergeFrom(opsRequest.DeepCopy())
	opsRequest.Labels[constant.AppInstanceLabelKey] = opsRequest.Spec.ClusterRef
	return r.Client.Patch(reqCtx.Ctx, opsRequest, patch)
}

func (r *OpsRequestReconciler) patchObservedGeneration(reqCtx intctrlutil.RequestCtx, opsRequest *appsv1alpha1.OpsRequest) error {
	patch := client.MergeFrom(opsRequest.DeepCopy())
	opsRequest.Status.ObservedGeneration = opsRequest.Generation
	if err := r.Client.Status().Patch(reqCtx.Ctx, opsRequest, patch); err != nil {
		return err
	}
	return nil
}
