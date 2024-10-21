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

package trace

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	tracev1 "github.com/apecloud/kubeblocks/apis/trace/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func init() {
	model.AddScheme(tracev1.AddToScheme)
}

// ReconciliationTraceReconciler reconciles a ReconciliationTrace object
type ReconciliationTraceReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	Recorder             record.EventRecorder
	ObjectRevisionStore  ObjectRevisionStore
	ObjectTreeRootFinder ObjectTreeRootFinder
	InformerManager      InformerManager
}

//+kubebuilder:rbac:groups=trace.kubeblocks.io,resources=reconciliationtraces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=trace.kubeblocks.io,resources=reconciliationtraces/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=trace.kubeblocks.io,resources=reconciliationtraces/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ReconciliationTraceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("ReconciliationTrace", req.NamespacedName)

	res, err := kubebuilderx.NewController(ctx, r.Client, req, r.Recorder, logger).
		Prepare(traceResources()).
		Do(resourcesValidation(ctx, r.Client)).
		Do(assureFinalizer()).
		Do(handleDeletion(r.ObjectRevisionStore)).
		Do(dryRun(ctx, r.Client, r.Scheme)).
		Do(updateCurrentState(ctx, r.Client, r.Scheme, r.ObjectRevisionStore)).
		Do(updateDesiredState(ctx, r.Client, r.Scheme, r.ObjectRevisionStore)).
		Commit()

	return res, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReconciliationTraceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.ObjectRevisionStore = NewObjectStore(r.Scheme)
	r.ObjectTreeRootFinder = NewObjectTreeRootFinder(r.Client)
	r.InformerManager = NewInformerManager(r.Client, mgr.GetCache(), r.Scheme, r.ObjectTreeRootFinder.GetEventChannel())

	return ctrl.NewControllerManagedBy(mgr).
		For(&tracev1.ReconciliationTrace{}).
		WatchesRawSource(&source.Channel{Source: r.ObjectTreeRootFinder.GetEventChannel()}, r.ObjectTreeRootFinder.GetEventHandler()).
		Complete(r)
}
