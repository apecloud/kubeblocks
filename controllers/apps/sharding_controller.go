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

package apps

import (
	"context"
	"math"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters/finalizers,verbs=update

// owned K8s core API resources controller-gen RBAC marker
// full access on core API resources
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=components,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=components/status,verbs=get
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=components/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=core,resources=secrets/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=core,resources=services/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=services/finalizers,verbs=update

// dataprotection get list and delete
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=backuppolicytemplates,verbs=get;list
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backuppolicies,verbs=get;list;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backups,verbs=get;list;delete;deletecollection

// ShardingReconciler reconciles a Cluster object with shardingSpecs definition
type ShardingReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ShardingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("sharding", req.NamespacedName),
		Recorder: r.Recorder,
	}

	reqCtx.Log.V(1).Info("reconcile", "sharding", req.NamespacedName)

	planBuilder := newShardingPlanBuilder(reqCtx, r.Client)
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
		c := planBuilder.(*shardingPlanBuilder)
		sendWarningEventWithError(r.Recorder, c.transCtx.Cluster, corev1.EventTypeWarning, err)
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
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
	//    If you do need to create/update/delete object, make your intent operation a lifecycleVertex and put it into the DAG.
	plan, errBuild := planBuilder.
		AddTransformer(
			// handle cluster sharding shared account
			&shardingSharedAccountTransformer{},
			// normalize the cluster shardingSpecs API
			&shardingAPINormalizationTransformer{},
			// handle cluster services with sharding selector
			&shardingServiceTransformer{},
			// create/update/delete cluster sharding components
			&shardingComponentTransformer{},
			// handle the restore for cluster sharding components
			&shardingRestoreTransformer{},
		).Build()

	// Execute stage
	// errBuild not nil means build stage partial success or validation error
	// execute the plan first, delay error handling
	if errExec := plan.Execute(); errExec != nil {
		return requeueError(errExec)
	}
	if errBuild != nil {
		return requeueError(errBuild)
	}
	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *ShardingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return intctrlutil.NewNamespacedControllerManagedBy(mgr).
		For(&appsv1.Cluster{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: int(math.Ceil(viper.GetFloat64(constant.CfgKBReconcileWorkers) / 4)),
		}).
		Watches(&appsv1.Component{}, handler.EnqueueRequestsFromMapFunc(r.filterShardingResources)).
		Watches(&corev1.Service{}, handler.EnqueueRequestsFromMapFunc(r.filterShardingResources)). // sharding services
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.filterShardingResources)).  // sharding shared account secret
		Complete(r)
}

func (r *ShardingReconciler) filterShardingResources(_ context.Context, obj client.Object) []reconcile.Request {
	labels := obj.GetLabels()
	if v, ok := labels[constant.AppManagedByLabelKey]; !ok || v != constant.AppName {
		return []reconcile.Request{}
	}
	if _, ok := labels[constant.AppInstanceLabelKey]; !ok {
		return []reconcile.Request{}
	}
	if _, ok := labels[constant.KBAppShardingNameLabelKey]; !ok {
		return []reconcile.Request{}
	}
	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Namespace: obj.GetNamespace(),
				Name:      labels[constant.AppInstanceLabelKey],
			},
		},
	}
}
