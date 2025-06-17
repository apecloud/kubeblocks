/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package rollout

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// RolloutReconciler reconciles a Rollout object
type RolloutReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=rollouts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=rollouts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.kubeblocks.io,resources=rollouts/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *RolloutReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("rollout", req.NamespacedName),
		Recorder: r.Recorder,
	}

	reqCtx.Log.V(1).Info("reconcile", "rollout", req.NamespacedName)

	planBuilder := newRolloutPlanBuilder(reqCtx, r.Client)
	if err := planBuilder.Init(); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	requeueError := func(err error) (ctrl.Result, error) {
		if re, ok := err.(intctrlutil.RequeueError); ok {
			return intctrlutil.RequeueAfter(re.RequeueAfter(), reqCtx.Log, re.Reason())
		}
		if apierrors.IsConflict(err) {
			return intctrlutil.Requeue(reqCtx.Log, err.Error())
		}
		c := planBuilder.(*rolloutPlanBuilder)
		appsutil.SendWarningEventWithError(r.Recorder, c.transCtx.Rollout, corev1.EventTypeWarning, err)
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}

	plan, errBuild := planBuilder.
		AddTransformer(
			&rolloutDeletionTransformer{},
			&rolloutMetaTransformer{},
			&rolloutLoadTransformer{},
			&rolloutSetupTransformer{},
			&rolloutTearDownTransformer{},
			&rolloutInplaceTransformer{},
			&rolloutReplaceTransformer{},
			&rolloutCreateTransformer{},
			&rolloutUpdateTransformer{},
			&rolloutStatusTransformer{},
		).Build()

	if errExec := plan.Execute(); errExec != nil {
		return requeueError(errExec)
	}
	if errBuild != nil {
		return requeueError(errBuild)
	}
	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *RolloutReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.Rollout{}).
		Watches(&appsv1.Cluster{}, handler.EnqueueRequestsFromMapFunc(r.cluster)).
		Complete(r)
}

func (r *RolloutReconciler) cluster(ctx context.Context, obj client.Object) []reconcile.Request {
	// TODO: it's too heavy to obtain the associated rollout for the cluster, refactor it later.
	rolloutList := &appsv1alpha1.RolloutList{}
	listOpts := []client.ListOption{
		client.InNamespace(obj.GetNamespace()),
		client.MatchingLabels{
			rolloutClusterNameLabel: obj.GetName(),
		},
	}
	err := r.Client.List(ctx, rolloutList, listOpts...)
	if err != nil || len(rolloutList.Items) == 0 {
		return []reconcile.Request{}
	}
	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Namespace: obj.GetNamespace(),
				Name:      rolloutList.Items[0].Name,
			},
		},
	}
}
