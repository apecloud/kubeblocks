/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package view

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	viewv1 "github.com/apecloud/kubeblocks/apis/view/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func init() {
	model.AddScheme(viewv1.AddToScheme)
}

// ReconciliationViewReconciler reconciles a ReconciliationView object
type ReconciliationViewReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	Recorder             record.EventRecorder
	ObjectStore          ObjectRevisionStore
	ObjectTreeRootFinder ObjectTreeRootFinder
	InformerManager      InformerManager
}

//+kubebuilder:rbac:groups=view.kubeblocks.io,resources=reconciliationviews,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=view.kubeblocks.io,resources=reconciliationviews/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=view.kubeblocks.io,resources=reconciliationviews/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ReconciliationViewReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("ReconciliationView", req.NamespacedName)

	res, err := kubebuilderx.NewController(ctx, r.Client, req, r.Recorder, logger).
		Prepare(viewResources()).
		Do(viewResourcesValidation(ctx, r.Client)).
		Do(updateInformerManager(r.InformerManager)).
		Do(viewCalculation(ctx, r.Client, r.Scheme, r.ObjectStore)).
		Do(viewStateEvaluation(ctx, r.Client, r.Scheme, r.ObjectStore)).
		Do(planGeneration(ctx, r.Client)).
		Commit()

	// TODO(free6om): err handling

	return res, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReconciliationViewReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.ObjectStore = NewObjectStore(r.Scheme)
	r.ObjectTreeRootFinder = NewObjectTreeRootFinder(r.Client)
	r.InformerManager = NewInformerManager(r.Client, mgr.GetCache(), r.Scheme, r.ObjectTreeRootFinder.GetEventChannel())
	if err := r.InformerManager.Start(); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&viewv1.ReconciliationView{}).
		WatchesRawSource(&source.Channel{Source: r.ObjectTreeRootFinder.GetEventChannel()}, r.ObjectTreeRootFinder.GetEventHandler()).
		Complete(r)
}
