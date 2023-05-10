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

package apps

import (
	"context"
	"runtime"
	"time"

	"github.com/spf13/viper"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	appsconfig "github.com/apecloud/kubeblocks/controllers/apps/configuration"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusterdefinitions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusterdefinitions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusterdefinitions/finalizers,verbs=update

// ClusterDefinitionReconciler reconciles a ClusterDefinition object
type ClusterDefinitionReconciler struct {
	client.Client
	Scheme   *k8sruntime.Scheme
	Recorder record.EventRecorder
}

var clusterDefUpdateHandlers = map[string]func(client client.Client, ctx context.Context, clusterDef *appsv1alpha1.ClusterDefinition) error{}

func init() {
	viper.SetDefault(maxConcurReconClusterDefKey, runtime.NumCPU()*2)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ClusterDefinitionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("clusterDefinition", req.NamespacedName),
		Recorder: r.Recorder,
	}

	dbClusterDef := &appsv1alpha1.ClusterDefinition{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, dbClusterDef); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	res, err := intctrlutil.HandleCRDeletion(reqCtx, r, dbClusterDef, dbClusterDefFinalizerName, func() (*ctrl.Result, error) {
		recordEvent := func() {
			r.Recorder.Event(dbClusterDef, corev1.EventTypeWarning, "ExistsReferencedResources",
				"cannot be deleted because of existing referencing Cluster or ClusterVersion.")
		}
		if res, err := intctrlutil.ValidateReferenceCR(reqCtx, r.Client, dbClusterDef,
			constant.ClusterDefLabelKey, recordEvent, &appsv1alpha1.ClusterList{},
			&appsv1alpha1.ClusterVersionList{}); res != nil || err != nil {
			return res, err
		}
		return nil, r.deleteExternalResources(reqCtx, dbClusterDef)
	})
	if res != nil {
		return *res, err
	}

	if dbClusterDef.Status.ObservedGeneration == dbClusterDef.Generation &&
		slices.Contains(dbClusterDef.Status.GetTerminalPhases(), dbClusterDef.Status.Phase) {
		return intctrlutil.Reconciled()
	}

	if err := appsconfig.ReconcileConfigSpecsForReferencedCR(r.Client, reqCtx, dbClusterDef); err != nil {
		return intctrlutil.RequeueAfter(time.Second, reqCtx.Log, err.Error())
	}

	for _, handler := range clusterDefUpdateHandlers {
		if err := handler(r.Client, reqCtx.Ctx, dbClusterDef); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
	}

	statusPatch := client.MergeFrom(dbClusterDef.DeepCopy())
	dbClusterDef.Status.ObservedGeneration = dbClusterDef.Generation
	dbClusterDef.Status.Phase = appsv1alpha1.AvailablePhase
	if err = r.Client.Status().Patch(reqCtx.Ctx, dbClusterDef, statusPatch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	intctrlutil.RecordCreatedEvent(r.Recorder, dbClusterDef)
	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterDefinitionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.ClusterDefinition{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: viper.GetInt(maxConcurReconClusterDefKey),
		}).
		Complete(r)
}

func (r *ClusterDefinitionReconciler) deleteExternalResources(reqCtx intctrlutil.RequestCtx, clusterDef *appsv1alpha1.ClusterDefinition) error {
	//
	// delete any external resources associated with the cronJob
	//
	// Ensure that delete implementation is idempotent and safe to invoke
	// multiple times for same object.
	return appsconfig.DeleteConfigMapFinalizer(r.Client, reqCtx, clusterDef)
}
