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
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
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

// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=core,resources=configmaps/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=core,resources=services/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=services/finalizers,verbs=update

// read + update access
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core,resources=pods/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=pods/exec,verbs=create

// dataprotection get list and delete
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=backuppolicytemplates,verbs=get;list
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backuppolicies,verbs=get;list;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backups,verbs=get;list;delete;deletecollection

// ClusterReconciler reconciles a Cluster object
type ClusterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("cluster", req.NamespacedName),
		Recorder: r.Recorder,
	}

	reqCtx.Log.V(1).Info("reconcile", "cluster", req.NamespacedName)

	requeueError := func(err error) (ctrl.Result, error) {
		if re, ok := err.(intctrlutil.RequeueError); ok {
			return intctrlutil.RequeueAfter(re.RequeueAfter(), reqCtx.Log, re.Reason())
		}
		if apierrors.IsConflict(err) {
			return intctrlutil.Requeue(reqCtx.Log, err.Error())
		}
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}

	// the cluster reconciliation loop is a 3-stage model: plan Init, plan Build and plan Execute
	// Init stage
	planBuilder := NewClusterPlanBuilder(reqCtx, r.Client)
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
	//    If you do need to create/update/delete object, make your intent operation a lifecycleVertex and put it into the DAG.
	//
	// TODO: transformers are vertices, theirs' dependencies are edges, make plan Build stage a DAG.
	plan, errBuild := planBuilder.
		AddTransformer(
			// handle cluster deletion first
			&clusterDeletionTransformer{},
			// check is recovering from halted cluster
			&clusterHaltRecoveryTransformer{},
			// update finalizer and cd&cv labels
			&clusterAssureMetaTransformer{},
			// validate cd & cv's existence and availability
			&clusterLoadRefResourcesTransformer{},
			// normalize the cluster and component API
			&ClusterAPINormalizationTransformer{},
			// handle cluster services
			&clusterServiceTransformer{},
			// handle restore before clusterComponentTransformer
			&clusterRestoreTransformer{Client: r.Client},
			// create all cluster components objects
			&clusterComponentTransformer{},
			// update cluster components' status
			&clusterComponentStatusTransformer{},
			// create default cluster connection credential secret object
			&clusterCredentialTransformer{},
			// build backuppolicy and backupschedule from backupPolicyTemplate
			&clusterBackupPolicyTransformer{},
			// add our finalizer to all objects
			&clusterOwnershipTransformer{},
			// make all workload objects depending on credential secret
			&secretTransformer{},
			// update cluster status
			&clusterStatusTransformer{},
			// always safe to put your transformer below
		).
		Build()

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
func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	retryDurationMS := viper.GetInt(constant.CfgKeyCtrlrReconcileRetryDurationMS)
	if retryDurationMS != 0 {
		requeueDuration = time.Millisecond * time.Duration(retryDurationMS)
	}
	// TODO: add filter predicate for core API objects
	b := ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.Cluster{}).
		Owns(&appsv1alpha1.Component{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&dpv1alpha1.BackupPolicy{}).
		Owns(&dpv1alpha1.BackupSchedule{}).
		Owns(&dpv1alpha1.Restore{}).
		Owns(&batchv1.Job{}).
		Owns(&appsv1alpha1.Configuration{}).
		Watches(&corev1.Pod{}, handler.EnqueueRequestsFromMapFunc(r.filterClusterResources))

	return b.Complete(r)
}

func (r *ClusterReconciler) filterClusterResources(ctx context.Context, obj client.Object) []reconcile.Request {
	labels := obj.GetLabels()
	if v, ok := labels[constant.AppManagedByLabelKey]; !ok || v != constant.AppName {
		return []reconcile.Request{}
	}
	if _, ok := labels[constant.AppInstanceLabelKey]; !ok {
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
