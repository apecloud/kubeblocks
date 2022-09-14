/*
Copyright 2022 The Kubeblocks Authors

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

package dataprotection

import (
	"context"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"

	"k8s.io/client-go/tools/record"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
)

// BackupPolicyReconciler reconciles a BackupPolicy object
type BackupPolicyReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=dataprotection.infracreate.com,resources=backuppolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dataprotection.infracreate.com,resources=backuppolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dataprotection.infracreate.com,resources=backuppolicies/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the BackupPolicy object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *BackupPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// NOTES:
	// setup common request context
	reqCtx := intctrlutil.RequestCtx{
		Ctx: ctx,
		Req: req,
		Log: log.FromContext(ctx).WithValues("backupPolicy", req.NamespacedName),
	}

	backupPolicy := &dataprotectionv1alpha1.BackupPolicy{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, backupPolicy); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	// handle finalizer
	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, backupPolicy, dataProtectionFinalizerName, func() (*ctrl.Result, error) {
		return nil, r.deleteExternalResources(reqCtx, backupPolicy)
	})
	if err != nil {
		return *res, err
	} // TODO(user): your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dataprotectionv1alpha1.BackupPolicy{}).
		Complete(r)
}

func (r *BackupPolicyReconciler) deleteExternalResources(reqCtx intctrlutil.RequestCtx, backupPolicy *dataprotectionv1alpha1.BackupPolicy) error {
	//
	// delete any external resources associated with the cronJob
	//
	// Ensure that delete implementation is idempotent and safe to invoke
	// multiple times for same object.

	return nil
}
