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

package workloads

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/handler"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	"github.com/apecloud/kubeblocks/pkg/controller/rsm"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// InstanceSetReconciler reconciles a InstanceSet object
type InstanceSetReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=workloads.kubeblocks.io,resources=instancesets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=workloads.kubeblocks.io,resources=instancesets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=workloads.kubeblocks.io,resources=instancesets/finalizers,verbs=update

// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=apps,resources=statefulsets/status,verbs=get
// +kubebuilder:rbac:groups=apps,resources=statefulsets/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=core,resources=pods/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=pods/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=core,resources=configmaps/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=core,resources=services/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=services/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the InstanceSet object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *InstanceSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("InstanceSet", req.NamespacedName)

	provider, err := instanceset.CurrentReplicaProvider(ctx, r.Client, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, err
	}
	if provider == instanceset.PodProvider {
		err = kubebuilderx.NewController(ctx, r.Client, req, r.Recorder, logger).
			Prepare(instanceset.NewTreeLoader()).
			Do(instanceset.NewFixMetaReconciler()).
			Do(instanceset.NewDeletionReconciler()).
			Do(instanceset.NewStatusReconciler()).
			Do(instanceset.NewRevisionUpdateReconciler()).
			Do(instanceset.NewAssistantObjectReconciler()).
			Do(instanceset.NewReplicasAlignmentReconciler()).
			Do(instanceset.NewUpdateReconciler()).
			Commit()
		return ctrl.Result{}, err
	}

	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      logger,
		Recorder: r.Recorder,
	}

	reqCtx.Log.V(1).Info("reconcile", "InstanceSet", req.NamespacedName)

	requeueError := func(err error) (ctrl.Result, error) {
		if re, ok := err.(model.RequeueError); ok {
			return intctrlutil.RequeueAfter(re.RequeueAfter(), reqCtx.Log, re.Reason())
		}
		if apierrors.IsConflict(err) {
			return intctrlutil.Requeue(reqCtx.Log, err.Error())
		}
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	// the InstanceSet reconciliation loop is a two-phase model: plan Build and plan Execute
	// Init stage
	planBuilder := rsm.NewRSMPlanBuilder(reqCtx, r.Client, req)
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
			&rsm.FixMetaTransformer{},
			// handle deletion
			// handle cluster deletion first
			&rsm.ObjectDeletionTransformer{},
			// handle secondary objects generation
			&rsm.ObjectGenerationTransformer{},
			// handle status
			&rsm.ObjectStatusTransformer{},
			// handle MemberUpdateStrategy
			&rsm.UpdateStrategyTransformer{},
			// handle member reconfiguration
			&rsm.MemberReconfigurationTransformer{},
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
func (r *InstanceSetReconciler) SetupWithManager(mgr ctrl.Manager, multiClusterMgr multicluster.Manager) error {
	ctx := &handler.FinderContext{
		Context: context.Background(),
		Reader:  r.Client,
		Scheme:  *r.Scheme,
	}

	if multiClusterMgr == nil {
		return r.setupWithManager(mgr, ctx)
	}
	return r.setupWithMultiClusterManager(mgr, multiClusterMgr, ctx)
}

func (r *InstanceSetReconciler) setupWithManager(mgr ctrl.Manager, ctx *handler.FinderContext) error {
	if viper.GetBool(rsm.FeatureGateRSMCompatibilityMode) {
		nameLabels := []string{constant.AppInstanceLabelKey, constant.KBAppComponentLabelKey}
		delegatorFinder := handler.NewDelegatorFinder(&workloads.InstanceSet{}, nameLabels)
		ownerFinder := handler.NewOwnerFinder(&appsv1.StatefulSet{})
		stsHandler := handler.NewBuilder(ctx).AddFinder(delegatorFinder).Build()
		jobHandler := handler.NewBuilder(ctx).AddFinder(delegatorFinder).Build()
		// pod owned by legacy StatefulSet
		stsPodHandler := handler.NewBuilder(ctx).AddFinder(ownerFinder).AddFinder(delegatorFinder).Build()

		return intctrlutil.NewNamespacedControllerManagedBy(mgr).
			For(&workloads.InstanceSet{}).
			WithOptions(controller.Options{
				MaxConcurrentReconciles: viper.GetInt(constant.CfgKBReconcileWorkers),
			}).
			Watches(&appsv1.StatefulSet{}, stsHandler).
			Watches(&batchv1.Job{}, jobHandler).
			Watches(&corev1.Pod{}, stsPodHandler).
			Owns(&corev1.Pod{}).
			Owns(&corev1.PersistentVolumeClaim{}).
			Complete(r)
	}

	stsOwnerFinder := handler.NewOwnerFinder(&appsv1.StatefulSet{})
	itsOwnerFinder := handler.NewOwnerFinder(&workloads.InstanceSet{})
	podHandler := handler.NewBuilder(ctx).AddFinder(stsOwnerFinder).AddFinder(itsOwnerFinder).Build()
	return intctrlutil.NewNamespacedControllerManagedBy(mgr).
		For(&workloads.InstanceSet{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: viper.GetInt(constant.CfgKBReconcileWorkers),
		}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&batchv1.Job{}).
		Watches(&corev1.Pod{}, podHandler).
		Owns(&corev1.Pod{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}

func (r *InstanceSetReconciler) setupWithMultiClusterManager(mgr ctrl.Manager,
	multiClusterMgr multicluster.Manager, ctx *handler.FinderContext) error {
	nameLabels := []string{constant.AppInstanceLabelKey, constant.KBAppComponentLabelKey}
	delegatorFinder := handler.NewDelegatorFinder(&workloads.InstanceSet{}, nameLabels)
	ownerFinder := handler.NewOwnerFinder(&appsv1.StatefulSet{})
	stsHandler := handler.NewBuilder(ctx).AddFinder(delegatorFinder).Build()
	// pod owned by legacy StatefulSet
	stsPodHandler := handler.NewBuilder(ctx).AddFinder(ownerFinder).AddFinder(delegatorFinder).Build()
	// TODO: modify handler.getObjectFromKey to support running Job in data clusters
	jobHandler := handler.NewBuilder(ctx).AddFinder(delegatorFinder).Build()

	b := intctrlutil.NewNamespacedControllerManagedBy(mgr).
		For(&workloads.InstanceSet{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: viper.GetInt(constant.CfgKBReconcileWorkers),
		})

	multiClusterMgr.Watch(b, &appsv1.StatefulSet{}, stsHandler).
		Watch(b, &corev1.Pod{}, stsPodHandler).
		Watch(b, &batchv1.Job{}, jobHandler).
		Own(b, &corev1.Pod{}, &workloads.InstanceSet{}).
		Own(b, &corev1.PersistentVolumeClaim{}, &workloads.InstanceSet{})

	return b.Complete(r)
}
