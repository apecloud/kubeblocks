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

package apps

import (
	"context"

	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=backuppolicytemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=backuppolicytemplates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=backuppolicytemplates/finalizers,verbs=update

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

	backupPolicyTemplate := &appsv1alpha1.BackupPolicyTemplate{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, backupPolicyTemplate); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	// infer clusterDefRef from spec.clusterDefRef
	if backupPolicyTemplate.Spec.ClusterDefRef != "" {
		backupPolicyTemplate.Labels[constant.ClusterDefLabelKey] = backupPolicyTemplate.Spec.ClusterDefRef
	}
	compDefList := &appsv1alpha1.ComponentDefinitionList{}
	if err := r.Client.List(ctx, compDefList); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	for _, backupPolicy := range backupPolicyTemplate.Spec.BackupPolicies {
		for _, compDef := range backupPolicy.ComponentDefs {
			matchedCompDefNames := r.getMatchedComponentDefs(compDefList, compDef)
			for _, compDefName := range matchedCompDefNames {
				backupPolicyTemplate.Labels[compDefName] = compDefName
			}
		}
	}

	if err := r.Client.Update(reqCtx.Ctx, backupPolicyTemplate); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	return intctrlutil.Reconciled()
}

func (r *BackupPolicyTemplateReconciler) getMatchedComponentDefs(compDefList *appsv1alpha1.ComponentDefinitionList, compDef string) []string {
	var compDefNames []string
	for i, item := range compDefList.Items {
		if component.CompDefMatched(item.Name, compDef) {
			compDefNames = append(compDefNames, compDefList.Items[i].Name)
		}
	}
	return compDefNames
}

func (r *BackupPolicyTemplateReconciler) isCompatibleWith(compDef appsv1alpha1.ComponentDefinition, bpt *appsv1alpha1.BackupPolicyTemplate) bool {
	for _, bp := range bpt.Spec.BackupPolicies {
		for _, compDefRegex := range bp.ComponentDefs {
			if component.CompDefMatched(compDef.Name, compDefRegex) {
				return true
			}
		}
	}
	return false
}

func (r *BackupPolicyTemplateReconciler) compatibleBackupPolicyTemplate(ctx context.Context, obj client.Object) []reconcile.Request {
	compDef, ok := obj.(*appsv1alpha1.ComponentDefinition)
	if !ok {
		return nil
	}
	bpts := &appsv1alpha1.BackupPolicyTemplateList{}
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
	return intctrlutil.NewNamespacedControllerManagedBy(mgr).
		For(&appsv1alpha1.BackupPolicyTemplate{}).
		Watches(&appsv1alpha1.ComponentDefinition{}, handler.EnqueueRequestsFromMapFunc(r.compatibleBackupPolicyTemplate)).
		Complete(r)
}
