/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package components

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// StatefulSetReconciler reconciles a statefulset object
type StatefulSetReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets/status,verbs=get
// +kubebuilder:rbac:groups=apps,resources=statefulsets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *StatefulSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var (
		sts = &appsv1.StatefulSet{}
		err error
	)

	reqCtx := intctrlutil.RequestCtx{
		Ctx: ctx,
		Req: req,
		Log: log.FromContext(ctx).WithValues("statefulSet", req.NamespacedName),
	}

	if err = r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, sts); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	// skip if sts is being deleted
	if !sts.DeletionTimestamp.IsZero() {
		return intctrlutil.Reconciled()
	}

	return workloadCompClusterReconcile(reqCtx, r.Client, sts,
		func(cluster *appsv1alpha1.Cluster, componentSpec *appsv1alpha1.ClusterComponentSpec, component types.Component) (ctrl.Result, error) {
			compCtx := newComponentContext(reqCtx, r.Client, r.Recorder, component, sts, componentSpec)
			// patch the current componentSpec workload's custom labels
			if err := patchWorkloadCustomLabel(reqCtx.Ctx, r.Client, cluster, componentSpec); err != nil {
				reqCtx.Recorder.Event(cluster, corev1.EventTypeWarning, "StatefulSet Controller PatchWorkloadCustomLabelFailed", err.Error())
				return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
			}
			reqCtx.Log.V(1).Info("before updateComponentStatusInClusterStatus",
				"generation", sts.Generation, "observed generation", sts.Status.ObservedGeneration,
				"replicas", sts.Status.Replicas)
			if requeueAfter, err := updateComponentStatusInClusterStatus(compCtx, cluster); err != nil {
				return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
			} else if requeueAfter != 0 {
				// if the reconcileAction need requeue, do it
				return intctrlutil.RequeueAfter(requeueAfter, reqCtx.Log, "")
			}
			return intctrlutil.Reconciled()
		})
}

// SetupWithManager sets up the controller with the Manager.
func (r *StatefulSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.StatefulSet{}).
		Owns(&corev1.Pod{}).
		WithEventFilter(predicate.NewPredicateFuncs(intctrlutil.WorkloadFilterPredicate)).
		Named("statefulset-watcher").
		Complete(r)
}
