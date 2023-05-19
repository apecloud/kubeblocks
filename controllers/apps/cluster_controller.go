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

	snapshotv1beta1 "github.com/kubernetes-csi/external-snapshotter/client/v3/apis/volumesnapshot/v1beta1"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	"github.com/spf13/viper"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	probeutil "github.com/apecloud/kubeblocks/cmd/probe/util"
	"github.com/apecloud/kubeblocks/controllers/k8score"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/lifecycle"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters/finalizers,verbs=update

// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=backuppolicytemplates,verbs=get;list

// owned K8s core API resources controller-gen RBAC marker
// full access on core API resources
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=core,resources=secrets/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=core,resources=configmaps/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=core,resources=services/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=services/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=resourcequotas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=resourcequotas/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=resourcequotas/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/finalizers,verbs=update

// +kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=replicasets/status,verbs=get
// +kubebuilder:rbac:groups=apps,resources=replicasets/finalizers,verbs=update

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get
// +kubebuilder:rbac:groups=apps,resources=deployments/finalizers,verbs=update

// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=apps,resources=statefulsets/status,verbs=get
// +kubebuilder:rbac:groups=apps,resources=statefulsets/finalizers,verbs=update

// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets/finalizers,verbs=update

// read + update access
// +kubebuilder:rbac:groups=core,resources=endpoints,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core,resources=pods/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=pods/exec,verbs=create

// read only + watch access
// +kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch

// dataprotection get list and delete
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backuppolicies,verbs=get;list;create;update;patch;delete;deletecollection
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backups,verbs=get;list;delete;deletecollection

// componentresourceconstraint get list
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=componentresourceconstraints,verbs=get;list;watch

// ClusterReconciler reconciles a Cluster object
type ClusterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// ClusterStatusEventHandler is the event handler for the cluster status event
type ClusterStatusEventHandler struct{}

var _ k8score.EventHandler = &ClusterStatusEventHandler{}

func init() {
	k8score.EventHandlerMap["cluster-status-handler"] = &ClusterStatusEventHandler{}
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
		if re, ok := err.(lifecycle.RequeueError); ok {
			return intctrlutil.RequeueAfter(re.RequeueAfter(), reqCtx.Log, re.Reason())
		}
		return intctrlutil.RequeueWithError(err, reqCtx.Log, "")
	}

	// the cluster reconciliation loop is a 3-stage model: plan Init, plan Build and plan Execute
	// Init stage
	planBuilder := lifecycle.NewClusterPlanBuilder(reqCtx, r.Client, req)
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
			// handle deletion
			// handle cluster deletion first
			&lifecycle.ClusterDeletionTransformer{},
			// fix meta
			// fix finalizer and cd&cv labels
			&lifecycle.FixMetaTransformer{},
			// validate
			// validate cd & cv's existence and availability
			&lifecycle.ValidateAndLoadRefResourcesTransformer{},
			// validate config
			&lifecycle.ValidateEnableLogsTransformer{},
			// fix spec
			// fill class related info
			&lifecycle.FillClassTransformer{},
			// generate objects
			// cluster to K8s objects and put them into dag
			&lifecycle.ClusterTransformer{Client: r.Client},
			// tls certs secret
			&lifecycle.TLSCertsTransformer{},
			// transform backupPolicy tpl to backuppolicy.dataprotection.kubeblocks.io
			&lifecycle.BackupPolicyTPLTransformer{},
			// add our finalizer to all objects
			&lifecycle.OwnershipTransformer{},
			// make all workload objects depending on credential secret
			&lifecycle.CredentialTransformer{},
			// make all workload objects depending on all none workload objects
			&lifecycle.WorkloadsLastTransformer{},
			// make config configmap immutable
			&lifecycle.ConfigTransformer{},
			// read old snapshot from cache, and generate diff plan
			&lifecycle.ObjectActionTransformer{},
			// day-2 ops
			// horizontal scaling
			&lifecycle.StsHorizontalScalingTransformer{},
			// stateful set pvc Update
			&lifecycle.StsPVCTransformer{},
			// update cluster status
			&lifecycle.ClusterStatusTransformer{},
			// handle PITR
			&lifecycle.PITRTransformer{Client: r.Client},
			// update the real-time component replicas info to pods
			&lifecycle.StsPodsTransformer{},
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
	requeueDuration = time.Duration(viper.GetInt(constant.CfgKeyCtrlrReconcileRetryDurationMS))
	// TODO: add filter predicate for core API objects
	b := ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.Cluster{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Owns(&dataprotectionv1alpha1.BackupPolicy{}).
		Owns(&dataprotectionv1alpha1.Backup{})
	if viper.GetBool("VOLUMESNAPSHOT") {
		if intctrlutil.InVolumeSnapshotV1Beta1() {
			b.Owns(&snapshotv1beta1.VolumeSnapshot{}, builder.OnlyMetadata, builder.Predicates{})
		} else {
			b.Owns(&snapshotv1.VolumeSnapshot{}, builder.OnlyMetadata, builder.Predicates{})
		}
	}
	return b.Complete(r)
}

// Handle is the event handler for the cluster status event.
func (r *ClusterStatusEventHandler) Handle(cli client.Client, reqCtx intctrlutil.RequestCtx, recorder record.EventRecorder, event *corev1.Event) error {
	if event.Reason != string(probeutil.CheckRoleOperation) {
		return handleEventForClusterStatus(reqCtx.Ctx, cli, recorder, event)
	}

	// parse probe event message when field path is probe-role-changed-check
	message := k8score.ParseProbeEventMessage(reqCtx, event)
	if message == nil {
		reqCtx.Log.Info("parse probe event message failed", "message", event.Message)
		return nil
	}

	// if probe message event is checkRoleFailed, it means the cluster is abnormal, need to handle the cluster status
	if message.Event == probeutil.OperationFailed {
		return handleEventForClusterStatus(reqCtx.Ctx, cli, recorder, event)
	}
	return nil
}
