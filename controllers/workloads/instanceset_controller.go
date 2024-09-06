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

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/handler"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// InstanceSetReconciler reconciles an InstanceSet object
type InstanceSetReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=workloads.kubeblocks.io,resources=instancesets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=workloads.kubeblocks.io,resources=instancesets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=workloads.kubeblocks.io,resources=instancesets/finalizers,verbs=update

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

	res, err := kubebuilderx.NewController(ctx, r.Client, req, r.Recorder, logger).
		Prepare(instanceset.NewTreeLoader()).
		Do(instanceset.NewFixMetaReconciler()).
		Do(instanceset.NewDeletionReconciler()).
		Do(instanceset.NewStatusReconciler()).
		Do(instanceset.NewRevisionUpdateReconciler()).
		Do(instanceset.NewAssistantObjectReconciler()).
		Do(instanceset.NewReplicasAlignmentReconciler()).
		Do(instanceset.NewUpdateReconciler()).
		Commit()

	// TODO(free6om): handle error based on ErrorCode (after defined)

	return res, err
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
	itsFinder := handler.NewLabelFinder(&workloads.InstanceSet{}, instanceset.WorkloadsManagedByLabelKey, workloads.Kind, instanceset.WorkloadsInstanceLabelKey)
	podHandler := handler.NewBuilder(ctx).AddFinder(itsFinder).Build()
	return intctrlutil.NewNamespacedControllerManagedBy(mgr).
		For(&workloads.InstanceSet{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: viper.GetInt(constant.CfgKBReconcileWorkers),
		}).
		Watches(&corev1.Pod{}, podHandler).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}

func (r *InstanceSetReconciler) setupWithMultiClusterManager(mgr ctrl.Manager,
	multiClusterMgr multicluster.Manager, ctx *handler.FinderContext) error {
	nameLabels := []string{constant.AppInstanceLabelKey, constant.KBAppComponentLabelKey}
	delegatorFinder := handler.NewDelegatorFinder(&workloads.InstanceSet{}, nameLabels)
	// TODO: modify handler.getObjectFromKey to support running Job in data clusters
	jobHandler := handler.NewBuilder(ctx).AddFinder(delegatorFinder).Build()

	b := intctrlutil.NewNamespacedControllerManagedBy(mgr).
		For(&workloads.InstanceSet{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: viper.GetInt(constant.CfgKBReconcileWorkers),
		})

	multiClusterMgr.Watch(b, &batchv1.Job{}, jobHandler).
		Own(b, &corev1.Pod{}, &workloads.InstanceSet{}).
		Own(b, &corev1.PersistentVolumeClaim{}, &workloads.InstanceSet{})

	return b.Complete(r)
}
