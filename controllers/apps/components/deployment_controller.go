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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/stateless"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// DeploymentReconciler reconciles a deployment object
type DeploymentReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get
// +kubebuilder:rbac:groups=apps,resources=deployments/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ClusterVersion object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *DeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var (
		deploy  = &appsv1.Deployment{}
		cluster *appsv1alpha1.Cluster
		err     error
	)

	reqCtx := intctrlutil.RequestCtx{
		Ctx: ctx,
		Req: req,
		Log: log.FromContext(ctx).WithValues("deployment", req.NamespacedName),
	}

	if err = r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, deploy); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	if cluster, err = util.GetClusterByObject(reqCtx.Ctx, r.Client, deploy); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	} else if cluster == nil {
		return intctrlutil.Reconciled()
	}

	clusterDef := &appsv1alpha1.ClusterDefinition{}
	if err = r.Client.Get(ctx, client.ObjectKey{Name: cluster.Spec.ClusterDefRef}, clusterDef); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	componentName := deploy.GetLabels()[intctrlutil.AppComponentLabelKey]
	componentSpec := cluster.GetComponentByName(componentName)
	if componentSpec == nil {
		return intctrlutil.Reconciled()
	}
	componentDef := clusterDef.GetComponentDefByName(componentSpec.ComponentDefRef)
	statelessComp := stateless.NewStateless(reqCtx.Ctx, r.Client, cluster, componentSpec, componentDef)
	compCtx := newComponentContext(reqCtx, r.Client, r.Recorder, statelessComp, deploy, componentSpec)
	if requeueAfter, err := updateComponentStatusInClusterStatus(compCtx, cluster); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	} else if requeueAfter != 0 {
		// if the reconcileAction need requeue, do it
		return intctrlutil.RequeueAfter(requeueAfter, reqCtx.Log, "")
	}

	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}).
		Owns(&appsv1.ReplicaSet{}).
		WithEventFilter(predicate.NewPredicateFuncs(intctrlutil.WorkloadFilterPredicate)).
		Complete(r)
}
