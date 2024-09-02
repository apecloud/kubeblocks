/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils/boolptr"
)

// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backuppolicytemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backuppolicytemplates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backuppolicytemplates/finalizers,verbs=update

type BackupPolicyTemplateReconciler struct {
	client.Client
	Scheme   *k8sruntime.Scheme
	Recorder record.EventRecorder
}

func (r *BackupPolicyTemplateReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("backupPolicyTemplate", req.NamespacedName),
		Recorder: r.Recorder,
	}

	backupPolicyTemplate := &dpv1alpha1.BackupPolicyTemplate{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, backupPolicyTemplate); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	oldBPT := backupPolicyTemplate.DeepCopy()
	message := ""
	backupMethodMap := map[string]sets.Empty{}
	actionSetNotFound := false
	// validate the referred actionSetName of the backupMethod
	for _, v := range backupPolicyTemplate.Spec.BackupMethods {
		backupMethodMap[v.Name] = sets.Empty{}
		if boolptr.IsSetToFalse(v.SnapshotVolumes) && v.ActionSetName == "" {
			message += fmt.Sprintf(`backupMethod "%s" is missing an ActionSet name;`, v.Name)
			continue
		}
		if v.ActionSetName == "" {
			continue
		}
		actionSet := &dpv1alpha1.ActionSet{}
		if err := r.Client.Get(ctx, client.ObjectKey{Name: v.ActionSetName}, actionSet); err != nil {
			if apierrors.IsNotFound(err) {
				message += fmt.Sprintf(`ActionSet "%s" not found;`, v.ActionSetName)
				actionSetNotFound = true
				continue
			}
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
	}
	// validate the schedules
	for _, v := range backupPolicyTemplate.Spec.Schedules {
		if _, ok := backupMethodMap[v.BackupMethod]; !ok {
			message += fmt.Sprintf(`backupMethod "%s" not found in the spec.backupMethods;`, v.BackupMethod)
		}
	}
	backupPolicyTemplate.Status.ObservedGeneration = backupPolicyTemplate.Generation
	backupPolicyTemplate.Status.Message = message
	if len(message) > 0 {
		backupPolicyTemplate.Status.Phase = dpv1alpha1.UnavailablePhase
	} else {
		backupPolicyTemplate.Status.Phase = dpv1alpha1.AvailablePhase
	}
	if !reflect.DeepEqual(oldBPT.Status, backupPolicyTemplate.Status) {
		if err := r.Client.Status().Patch(reqCtx.Ctx, backupPolicyTemplate, client.MergeFrom(oldBPT)); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
	}
	if actionSetNotFound {
		return intctrlutil.CheckedRequeueWithError(fmt.Errorf("some ActionSets not found"), reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupPolicyTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return intctrlutil.NewNamespacedControllerManagedBy(mgr).
		For(&dpv1alpha1.BackupPolicyTemplate{}).
		Complete(r)
}
