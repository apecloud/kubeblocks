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
		Complete(r)
}
