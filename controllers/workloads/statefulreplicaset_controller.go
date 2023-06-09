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

package workloads

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/model"
	"github.com/apecloud/kubeblocks/internal/controller/statefulreplicaset"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// StatefulReplicaSetReconciler reconciles a StatefulReplicaSet object
type StatefulReplicaSetReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=workloads.kubeblocks.io,resources=statefulreplicasets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=workloads.kubeblocks.io,resources=statefulreplicasets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=workloads.kubeblocks.io,resources=statefulreplicasets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the StatefulReplicaSet object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *StatefulReplicaSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("StatefulReplicaSet", req.NamespacedName),
		Recorder: r.Recorder,
	}

	reqCtx.Log.V(1).Info("reconcile", "StatefulReplicaSet", req.NamespacedName)

	requeueError := func(err error) (ctrl.Result, error) {
		if re, ok := err.(model.RequeueError); ok {
			return intctrlutil.RequeueAfter(re.RequeueAfter(), reqCtx.Log, re.Reason())
		}
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	// the stateful_replica_set reconciliation loop is a two-phase model: plan Build and plan Execute
	// Init stage
	planBuilder := statefulreplicaset.NewSRSPlanBuilder(reqCtx, r.Client, req)
	if err := planBuilder.Init(); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	// Build stage
	// what you should do in most cases is writing your transformer.
	//
	// here are the how-to tips:
	// 1. one transformer for one scenario
	// 2. try not to modify the current transformers, make a new one
	// 3. transformers are independent with each-other, with some exceptions.
	//    Which means transformers' order is not important in most cases.
	//    If you don't know where to put your transformer, append it to the end and that would be ok.
	// 4. don't use client.Client for object write, use client.ReadonlyClient for object read.
	//    If you do need to create/update/delete object, make your intent operation a model.ObjectVertex and put it into the DAG.
	//
	// TODO: transformers are vertices, theirs' dependencies are edges, make plan Build stage a DAG.
	plan, err := planBuilder.
		AddTransformer(
			// fix meta
			&statefulreplicaset.FixMetaTransformer{},
			// handle deletion
			// handle cluster deletion first
			&statefulreplicaset.SRSDeletionTransformer{},
			// handle secondary objects generation
			&statefulreplicaset.ObjectGenerationTransformer{},
			// handle status
			&statefulreplicaset.SRSStatusTransformer{},
			// handle UpdateStrategy
			&statefulreplicaset.UpdateStrategyTransformer{},
			// handle member reconfiguration
			&statefulreplicaset.MemberReconfigurationTransformer{},
			// always safe to put your transformer below
		).
		Build()
	if err != nil {
		return requeueError(err)
	}
	// TODO: define error categories in Build stage and handle them here like this:
	// switch errBuild.(type) {
	// case NOTFOUND:
	// case ALREADYEXISY:
	// }

	// Execute stage
	if err = plan.Execute(); err != nil {
		return requeueError(err)
	}

	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *StatefulReplicaSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&workloads.StatefulReplicaSet{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&batchv1.Job{}).
		Watches(&source.Kind{Type: &corev1.Pod{}},
			&statefulreplicaset.EnqueueRequestForAncestor{
				Client:    r.Client,
				OwnerType: &workloads.StatefulReplicaSet{},
				UpToLevel: 2,
				InTypes:   []runtime.Object{&appsv1.StatefulSet{}},
			}).
		Complete(r)
}
