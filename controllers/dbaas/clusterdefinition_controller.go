/*
Copyright 2022 The KubeBlocks Authors

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

package dbaas

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

var clusterDefUpdateHandlers = map[string]func(client client.Client, ctx context.Context, clusterDef *dbaasv1alpha1.ClusterDefinition) error{}

// ClusterDefinitionReconciler reconciles a ClusterDefinition object
type ClusterDefinitionReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=dbaas.infracreate.com,resources=clusterdefinitions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dbaas.infracreate.com,resources=clusterdefinitions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dbaas.infracreate.com,resources=clusterdefinitions/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ClusterDefinition object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *ClusterDefinitionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx: ctx,
		Req: req,
		Log: log.FromContext(ctx).WithValues("clusterDefinition", req.NamespacedName),
	}

	dbClusterDef := &dbaasv1alpha1.ClusterDefinition{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, dbClusterDef); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, dbClusterDef, dbClusterDefFinalizerName, func() (*ctrl.Result, error) {
		statusHandler := func() error {
			patch := client.MergeFrom(dbClusterDef.DeepCopy())
			dbClusterDef.Status.Phase = dbaasv1alpha1.DeletingPhase
			dbClusterDef.Status.Message = "cannot be deleted because of existing referencing Cluster or AppVersion."
			return r.Client.Status().Patch(ctx, dbClusterDef, patch)
		}
		if res, err := intctrlutil.ValidateReferenceCR(reqCtx, r.Client, dbClusterDef,
			clusterDefLabelKey, statusHandler, &dbaasv1alpha1.ClusterList{},
			&dbaasv1alpha1.AppVersionList{}); res != nil || err != nil {
			return res, err
		}
		return nil, r.deleteExternalResources(reqCtx, dbClusterDef)
	})
	if res != nil {
		return *res, err
	}

	if dbClusterDef.Status.ObservedGeneration == dbClusterDef.GetObjectMeta().GetGeneration() {
		return intctrlutil.Reconciled()
	}

	for _, handler := range clusterDefUpdateHandlers {
		if err := handler(r.Client, reqCtx.Ctx, dbClusterDef); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
	}

	statusPatch := client.MergeFrom(dbClusterDef.DeepCopy())
	dbClusterDef.Status.ObservedGeneration = dbClusterDef.GetObjectMeta().GetGeneration()
	dbClusterDef.Status.Phase = dbaasv1alpha1.AvailablePhase
	if err = r.Client.Status().Patch(reqCtx.Ctx, dbClusterDef, statusPatch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterDefinitionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dbaasv1alpha1.ClusterDefinition{}).
		Complete(r)
}

func (r *ClusterDefinitionReconciler) deleteExternalResources(reqCtx intctrlutil.RequestCtx, clusterDef *dbaasv1alpha1.ClusterDefinition) error {
	//
	// delete any external resources associated with the cronJob
	//
	// Ensure that delete implementation is idempotent and safe to invoke
	// multiple times for same object.
	return nil
}
