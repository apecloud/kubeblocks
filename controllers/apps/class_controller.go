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
	"fmt"

	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/class"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=componentclassdefinitions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=componentclassdefinitions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=componentclassdefinitions/finalizers,verbs=update

type ComponentClassReconciler struct {
	client.Client
	Scheme   *k8sruntime.Scheme
	Recorder record.EventRecorder
}

func (r *ComponentClassReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("classDefinition", req.NamespacedName),
		Recorder: r.Recorder,
	}

	classDefinition := &appsv1alpha1.ComponentClassDefinition{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, classDefinition); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	var ml []client.ListOption
	constraintsList := &appsv1alpha1.ComponentResourceConstraintList{}
	if err := r.Client.List(reqCtx.Ctx, constraintsList, ml...); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	constraintsMap := make(map[string]appsv1alpha1.ComponentResourceConstraint)
	for idx := range constraintsList.Items {
		cf := constraintsList.Items[idx]
		if _, ok := cf.GetLabels()[types.ResourceConstraintProviderLabelKey]; !ok {
			continue
		}
		constraintsMap[cf.GetName()] = cf
	}

	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, classDefinition, dbClusterFinalizerName, func() (*ctrl.Result, error) {
		// TODO validate if existing cluster reference classes being deleted
		return nil, nil
	})
	if res != nil {
		return *res, err
	}

	if classDefinition.Status.ObservedGeneration == classDefinition.Generation {
		return intctrlutil.Reconciled()
	}

	classInstances, err := class.ParseComponentClasses(*classDefinition)
	if err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "parse component classes failed")
	}

	patch := client.MergeFrom(classDefinition.DeepCopy())
	var classList []appsv1alpha1.ComponentClassInstance
	for _, v := range classInstances {
		constraint, ok := constraintsMap[v.ResourceConstraintRef]
		if !ok {
			return intctrlutil.CheckedRequeueWithError(nil, reqCtx.Log, fmt.Sprintf("resource constraint %s not found", v.ResourceConstraintRef))
		}
		if !constraint.MatchClass(v) {
			return intctrlutil.CheckedRequeueWithError(nil, reqCtx.Log, fmt.Sprintf("class %s does not conform to constraint %s", v.Name, v.ResourceConstraintRef))
		}
		classList = append(classList, *v)
	}
	classDefinition.Status.Classes = classList
	classDefinition.Status.ObservedGeneration = classDefinition.Generation
	if err = r.Client.Status().Patch(ctx, classDefinition, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "patch component class status failed")
	}

	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *ComponentClassReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&appsv1alpha1.ComponentClassDefinition{}).Complete(r)
}
