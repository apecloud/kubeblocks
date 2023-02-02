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

package k8score

import (
	"context"

	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type HandleStorageClass func(reqCtx intctrlutil.RequestCtx, cli client.Client, storageClass *storagev1.StorageClass) error

// StorageClassReconciler reconciles a StorageClass object
type StorageClassReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

var StorageClassHandlerMap = map[string]HandleStorageClass{}

//+kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the StorageClass object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *StorageClassReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx: ctx,
		Req: req,
		Log: log.FromContext(ctx).WithValues("StorageClass", req.NamespacedName),
	}

	reqCtx.Log.V(1).Info("StorageClass watcher")

	storageClass := &storagev1.StorageClass{}
	if err := r.Client.Get(ctx, req.NamespacedName, storageClass); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "getStorageClassError")
	}

	for _, handleStorageClass := range StorageClassHandlerMap {
		// ignores the not found error.
		if err := handleStorageClass(reqCtx, r.Client, storageClass); err != nil && !apierrors.IsNotFound(err) {
			return intctrlutil.RequeueWithError(err, reqCtx.Log, "handleStorageClassError")
		}
	}
	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *StorageClassReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&storagev1.StorageClass{}).
		Complete(r)
}
