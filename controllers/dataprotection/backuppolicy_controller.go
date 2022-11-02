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

package dataprotection

import (
	"context"
	"fmt"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
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

//+kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backuppolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backuppolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backuppolicies/finalizers,verbs=update

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
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("backupPolicy", req.NamespacedName),
		Recorder: r.Recorder,
	}

	backupPolicy := &dataprotectionv1alpha1.BackupPolicy{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, backupPolicy); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	// handle finalizer
	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, backupPolicy, dataProtectionFinalizerName, func() (*ctrl.Result, error) {
		return nil, r.deleteExternalResources(reqCtx, backupPolicy)
	})
	if res != nil {
		return *res, err
	}

	switch backupPolicy.Status.Phase {
	case "", dataprotectionv1alpha1.PolicyNew:
		return r.doNewPhaseAction(reqCtx, backupPolicy)
	case dataprotectionv1alpha1.PolicyInProgress:
		return r.doInProgressPhaseAction(reqCtx, backupPolicy)
	default:
		return intctrlutil.Reconciled()
	}
}

func assignBackupPolicy(policy *dataprotectionv1alpha1.BackupPolicy, template *dataprotectionv1alpha1.BackupPolicyTemplate) {
	if policy != nil && template != nil {
		if policy.Spec.BackupToolName == "" {
			policy.Spec.BackupToolName = template.Spec.BackupToolName
		}
		if policy.Spec.TTL == nil {
			policy.Spec.TTL = &template.Spec.TTL
		}
		if policy.Spec.Schedule == "" {
			policy.Spec.Schedule = template.Spec.Schedule
		}

		if policy.Spec.Target.DatabaseEngine == "" {
			policy.Spec.Target.DatabaseEngine = template.Spec.DatabaseEngine
		}

		if policy.Spec.Hooks == nil {
			policy.Spec.Hooks = &template.Spec.Hooks
		}
		if policy.Spec.RemoteVolume == nil {
			policy.Spec.RemoteVolume = &template.Spec.RemoteVolume
		}
		if policy.Spec.OnFailAttempted == 0 {
			policy.Spec.OnFailAttempted = template.Spec.OnFailAttempted
		}
	}
}

func (r *BackupPolicyReconciler) doNewPhaseAction(
	reqCtx intctrlutil.RequestCtx, backupPolicy *dataprotectionv1alpha1.BackupPolicy) (ctrl.Result, error) {

	if backupPolicy.Spec.BackupPolicyTemplateName != "" {
		backupPolicyTemplate := &dataprotectionv1alpha1.BackupPolicyTemplate{}
		key := types.NamespacedName{Namespace: backupPolicy.Namespace, Name: backupPolicy.Spec.BackupPolicyTemplateName}
		if err := r.Client.Get(reqCtx.Ctx, key, backupPolicyTemplate); err != nil {
			msg := fmt.Sprintf("Failed to get backupPolicyTemplateName: %s", err.Error())
			r.Recorder.Event(backupPolicy, corev1.EventTypeWarning, "BackupPolicyTemplateFailed", msg)
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, msg)
		}
		assignBackupPolicy(backupPolicy, backupPolicyTemplate)

		// update spec
		if err := r.Client.Update(reqCtx.Ctx, backupPolicy); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}

		// update Phase to InProgress
		backupPolicy.Status.Phase = dataprotectionv1alpha1.PolicyInProgress
		if err := r.Client.Status().Update(reqCtx.Ctx, backupPolicy); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
	} else {
		// check required columns and record event
		if backupPolicy.Spec.Schedule == "" {
			r.Recorder.Event(backupPolicy, corev1.EventTypeWarning, "BackupPolicyCheck", "Missing schedule.")
		}
		if backupPolicy.Spec.Target.DatabaseEngine == "" {
			r.Recorder.Event(backupPolicy, corev1.EventTypeWarning, "BackupPolicyCheck", "Missing target.databaseEngine.")
		}
	}
	return intctrlutil.Reconciled()
}

func (r *BackupPolicyReconciler) doInProgressPhaseAction(
	reqCtx intctrlutil.RequestCtx, backupPolicy *dataprotectionv1alpha1.BackupPolicy) (ctrl.Result, error) {

	// update Phase to InProgress
	backupPolicy.Status.Phase = dataprotectionv1alpha1.PolicyAvailable
	if err := r.Client.Status().Update(reqCtx.Ctx, backupPolicy); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
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
