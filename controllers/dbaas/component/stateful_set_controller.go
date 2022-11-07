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

package component

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

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// StatefulSetReconciler reconciles a statefulset object
type StatefulSetReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// NOTES: controller-gen RBAC marker is maintained at rbac.go

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the AppVersion object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *StatefulSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx: ctx,
		Req: req,
		Log: log.FromContext(ctx).WithValues("statefulSet", req.NamespacedName),
	}

	sts := &appsv1.StatefulSet{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, sts); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	if err := checkComponentStatusAndSyncCluster(reqCtx, r.Client, sts, handleStatefulSetAndCheckStatus); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	return intctrlutil.Reconciled()
}

func handleStatefulSetAndCheckStatus(reqCtx intctrlutil.RequestCtx, cli client.Client, cluster *dbaasv1alpha1.Cluster, object client.Object) (bool, error) {
	var (
		statefulStatusRevisionIsEquals bool
		sts                            = object.(*appsv1.StatefulSet)
		err                            error
	)
	// handle update operations by component type. when statefulSet is ok, statefulStatusRevisionIsEquals will be true
	if statefulStatusRevisionIsEquals, err = handleUpdateByComponentType(reqCtx, cli, sts, cluster); err != nil {
		return false, err
	}
	return StatefulSetIsReady(sts, statefulStatusRevisionIsEquals), nil
}

// handleUpdateByComponentType handle cluster update operations according to component type and check statefulSet revision
func handleUpdateByComponentType(reqCtx intctrlutil.RequestCtx, cli client.Client, sts *appsv1.StatefulSet, cluster *dbaasv1alpha1.Cluster) (bool, error) {
	var (
		componentDef                   *dbaasv1alpha1.ClusterDefinitionComponent
		err                            error
		statefulStatusRevisionIsEquals bool
		labels                         = sts.GetLabels()
	)
	for _, v := range cluster.Spec.Components {
		if v.Name != labels[intctrlutil.AppComponentLabelKey] {
			continue
		}
		if componentDef, err = GetComponentFromClusterDefinition(reqCtx.Ctx, cli, cluster, v.Type); err != nil || componentDef == nil {
			return false, err
		}
		switch componentDef.ComponentType {
		case dbaasv1alpha1.Consensus:
			// Consensus do not judge whether the revisions are consistent
			if statefulStatusRevisionIsEquals, err = handleConsensusSetUpdate(reqCtx.Ctx, cli, cluster, sts); err != nil {
				return false, err
			}
		case dbaasv1alpha1.Stateful:
			// when stateful updateStrategy is rollingUpdate, need to check revision
			if sts.Status.UpdateRevision == sts.Status.CurrentRevision {
				statefulStatusRevisionIsEquals = true
			}
		}
		break
	}
	return statefulStatusRevisionIsEquals, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *StatefulSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.StatefulSet{}).
		Owns(&corev1.Pod{}).
		WithEventFilter(predicate.NewPredicateFuncs(filterLabels)).
		Complete(r)
}
