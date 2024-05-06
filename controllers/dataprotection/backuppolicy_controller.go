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

package dataprotection

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

// BackupPolicyReconciler reconciles a BackupPolicy object
type BackupPolicyReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backuppolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backuppolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backuppolicies/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the backuppolicy closer to the desired state.
func (r *BackupPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("backupPolicy", req.NamespacedName),
		Recorder: r.Recorder,
	}

	backupPolicy := &dpv1alpha1.BackupPolicy{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, backupPolicy); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	// handle finalizer
	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, backupPolicy, dptypes.DataProtectionFinalizerName,
		func() (*ctrl.Result, error) {
			return nil, r.deleteExternalResources(reqCtx, backupPolicy)
		})
	if res != nil {
		return *res, err
	}

	if backupPolicy.Status.ObservedGeneration == backupPolicy.Generation &&
		backupPolicy.Status.Phase.IsAvailable() {
		return ctrl.Result{}, nil
	}

	patchStatus := func(phase dpv1alpha1.Phase, message string) error {
		patch := client.MergeFrom(backupPolicy.DeepCopy())
		backupPolicy.Status.Phase = phase
		backupPolicy.Status.Message = message
		backupPolicy.Status.ObservedGeneration = backupPolicy.Generation
		return r.Status().Patch(ctx, backupPolicy, patch)
	}

	if err = r.validateBackupPolicy(backupPolicy); err != nil {
		if err = patchStatus(dpv1alpha1.UnavailablePhase, err.Error()); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		return intctrlutil.Reconciled()
	}

	if err = patchStatus(dpv1alpha1.AvailablePhase, ""); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	intctrlutil.RecordCreatedEvent(r.Recorder, backupPolicy)
	return ctrl.Result{}, nil
}

func (r *BackupPolicyReconciler) validateBackupPolicy(backupPolicy *dpv1alpha1.BackupPolicy) error {
	checkTarget := func(targets []dpv1alpha1.BackupTarget) error {
		tMap := map[string]sets.Empty{}
		for _, v := range targets {
			if v.Name == "" {
				return fmt.Errorf(`target name can not be empty when using "targets" field`)
			}
			if _, ok := tMap[v.Name]; ok {
				return fmt.Errorf(`the target name can not be duplicated when using "targets" field`)
			}
			tMap[v.Name] = sets.Empty{}
		}
		return nil
	}
	if err := checkTarget(backupPolicy.Spec.Targets); err != nil {
		return err
	}
	for i := range backupPolicy.Spec.BackupMethods {
		if err := checkTarget(backupPolicy.Spec.BackupMethods[i].Targets); err != nil {
			return err
		}
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return intctrlutil.NewNamespacedControllerManagedBy(mgr).
		For(&dpv1alpha1.BackupPolicy{}).
		Complete(r)
}

func (r *BackupPolicyReconciler) deleteExternalResources(
	_ intctrlutil.RequestCtx,
	_ *dpv1alpha1.BackupPolicy) error {
	return nil
}