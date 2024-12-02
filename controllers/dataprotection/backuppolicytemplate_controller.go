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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dputils "github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
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
	if err := r.setComponentDefLabels(reqCtx, oldBPT, backupPolicyTemplate); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	if err := r.validateAvailable(reqCtx, oldBPT, backupPolicyTemplate); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

func (r *BackupPolicyTemplateReconciler) setComponentDefLabels(reqCtx intctrlutil.RequestCtx, oldBPT, bpt *dpv1alpha1.BackupPolicyTemplate) error {
	compDefList := &appsv1.ComponentDefinitionList{}
	if err := r.Client.List(reqCtx.Ctx, compDefList); err != nil {
		return err
	}
	if bpt.Labels == nil {
		bpt.Labels = map[string]string{}
	}
	for _, item := range compDefList.Items {
		for _, compDef := range bpt.Spec.CompDefs {
			// set componentDef labels
			if component.PrefixOrRegexMatched(item.Name, compDef) {
				bpt.Labels[item.Name] = item.Name
			}
		}
	}
	if !reflect.DeepEqual(oldBPT.Labels, bpt.Labels) {
		return r.Client.Update(reqCtx.Ctx, bpt)
	}
	return nil
}

func (r *BackupPolicyTemplateReconciler) validateAvailable(reqCtx intctrlutil.RequestCtx, oldBPT, bpt *dpv1alpha1.BackupPolicyTemplate) error {
	message := ""
	backupMethodMap := map[string]*dpv1alpha1.ActionSet{}
	actionSetNotFound := false
	// validate the referred actionSetName of the backupMethod
	for _, v := range bpt.Spec.BackupMethods {
		// confirm the method exists
		backupMethodMap[v.Name] = nil
		if boolptr.IsSetToFalse(v.SnapshotVolumes) && v.ActionSetName == "" {
			message += fmt.Sprintf(`backupMethod "%s" is missing an ActionSet name;`, v.Name)
			continue
		}
		if v.ActionSetName == "" {
			continue
		}
		actionSet := &dpv1alpha1.ActionSet{}
		if err := r.Client.Get(reqCtx.Ctx, client.ObjectKey{Name: v.ActionSetName}, actionSet); err != nil {
			if apierrors.IsNotFound(err) {
				message += fmt.Sprintf(`ActionSet "%s" not found;`, v.ActionSetName)
				actionSetNotFound = true
				continue
			}
			return err
		}
		// record found actionSets
		backupMethodMap[v.Name] = actionSet
	}
	// validate the schedule names
	if err := dputils.ValidateScheduleNames(bpt.Spec.Schedules); err != nil {
		message += fmt.Sprintf(`fails to validate schedule name: %v;`, err)
	}
	// validate the schedules
	for _, v := range bpt.Spec.Schedules {
		actionSet, ok := backupMethodMap[v.BackupMethod]
		if !ok {
			message += fmt.Sprintf(`backupMethod "%s" not found in the spec.backupMethods;`, v.BackupMethod)
			continue
		}
		// validate schedule parameters
		if actionSet != nil {
			if err := dputils.ValidateParameters(actionSet, v.Parameters, true); err != nil {
				message += fmt.Sprintf(`fails to validate parameters of backupMethod "%s": %v;`, v.BackupMethod, err)
			}
		}
	}

	bpt.Status.ObservedGeneration = bpt.Generation
	bpt.Status.Message = message
	if len(message) > 0 {
		bpt.Status.Phase = dpv1alpha1.UnavailablePhase
	} else {
		bpt.Status.Phase = dpv1alpha1.AvailablePhase
	}
	if !reflect.DeepEqual(oldBPT.Status, bpt.Status) {
		if err := r.Client.Status().Patch(reqCtx.Ctx, bpt, client.MergeFrom(oldBPT)); err != nil {
			return err
		}
	}
	if actionSetNotFound {
		return fmt.Errorf("some ActionSets not found")
	}
	return nil
}

func (r *BackupPolicyTemplateReconciler) isCompatibleWith(compDef appsv1.ComponentDefinition, bpt *dpv1alpha1.BackupPolicyTemplate) bool {
	for _, compDefRegex := range bpt.Spec.CompDefs {
		if component.PrefixOrRegexMatched(compDef.Name, compDefRegex) {
			return true
		}
	}
	return false
}

func (r *BackupPolicyTemplateReconciler) compatibleBackupPolicyTemplate(ctx context.Context, obj client.Object) []reconcile.Request {
	compDef, ok := obj.(*appsv1.ComponentDefinition)
	if !ok {
		return nil
	}
	bpts := &dpv1alpha1.BackupPolicyTemplateList{}
	if err := r.Client.List(ctx, bpts); err != nil {
		return nil
	}
	requests := make([]reconcile.Request, 0)
	for i := range bpts.Items {
		if r.isCompatibleWith(*compDef, &bpts.Items[i]) {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: bpts.Items[i].Name,
				},
			})
		}
	}
	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupPolicyTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return intctrlutil.NewControllerManagedBy(mgr).
		For(&dpv1alpha1.BackupPolicyTemplate{}).
		Watches(&appsv1.ComponentDefinition{}, handler.EnqueueRequestsFromMapFunc(r.compatibleBackupPolicyTemplate)).
		Complete(r)
}
