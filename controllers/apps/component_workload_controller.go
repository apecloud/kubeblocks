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

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
)

func NewDeploymentReconciler(mgr ctrl.Manager) error {
	return newComponentWorkloadReconciler(mgr, "deployment-controller", generics.DeploymentSignature, generics.ReplicaSetSignature)
}

func NewStatefulSetReconciler(mgr ctrl.Manager) error {
	return newComponentWorkloadReconciler(mgr, "stateful-set-controller", generics.StatefulSetSignature, generics.PodSignature)
}

func newComponentWorkloadReconciler[T generics.Object, PT generics.PObject[T], LT generics.ObjList[T], S generics.Object, PS generics.PObject[S], LS generics.ObjList[S]](
	mgr ctrl.Manager, name string, _ func(T, LT), _ func(S, LS)) error {
	return (&componentWorkloadReconciler[T, PT, S, PS]{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor(name),
	}).SetupWithManager(mgr)
}

// componentWorkloadReconciler reconciles a component workload object
type componentWorkloadReconciler[T generics.Object, PT generics.PObject[T], S generics.Object, PS generics.PObject[S]] struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get
// +kubebuilder:rbac:groups=apps,resources=deployments/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *componentWorkloadReconciler[T, PT, S, PS]) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx: ctx,
		Req: req,
		Log: log.FromContext(ctx).WithValues("deployment", req.NamespacedName),
	}

	var obj T
	pObj := PT(&obj)
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, pObj); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	// skip if workload is being deleted
	if !pObj.GetDeletionTimestamp().IsZero() {
		return intctrlutil.Reconciled()
	}

	handler := func(cluster *appsv1alpha1.Cluster, compSpec *appsv1alpha1.ClusterComponentSpec,
		compDef *appsv1alpha1.ClusterComponentDefinition) (ctrl.Result, error) {
		// patch the current componentSpec workload's custom labels
		if err := patchWorkloadCustomLabel(reqCtx.Ctx, r.Client, cluster, compSpec); err != nil {
			reqCtx.Recorder.Event(cluster, corev1.EventTypeWarning, "Component Workload Controller PatchWorkloadCustomLabelFailed", err.Error())
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}

		if err := notifyClusterStatusChange(reqCtx.Ctx, r.Client, r.Recorder, cluster, nil); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}
		return intctrlutil.Reconciled()
	}
	return workloadCompClusterReconcile(reqCtx, r.Client, pObj, handler)
}

// SetupWithManager sets up the controller with the Manager.
func (r *componentWorkloadReconciler[T, PT, S, PS]) SetupWithManager(mgr ctrl.Manager) error {
	var (
		obj1 T
		obj2 S
	)
	return ctrl.NewControllerManagedBy(mgr).
		For(PT(&obj1)).
		Owns(PS(&obj2)).
		WithEventFilter(predicate.NewPredicateFuncs(intctrlutil.WorkloadFilterPredicate)).
		Complete(r)
}

func workloadCompClusterReconcile(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	operand client.Object,
	processor func(*appsv1alpha1.Cluster, *appsv1alpha1.ClusterComponentSpec, *appsv1alpha1.ClusterComponentDefinition) (ctrl.Result, error)) (ctrl.Result, error) {
	var err error
	var cluster *appsv1alpha1.Cluster

	if cluster, err = util.GetClusterByObject(reqCtx.Ctx, cli, operand); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	} else if cluster == nil {
		return intctrlutil.Reconciled()
	}

	clusterDef := &appsv1alpha1.ClusterDefinition{}
	if err = cli.Get(reqCtx.Ctx, client.ObjectKey{Name: cluster.Spec.ClusterDefRef}, clusterDef); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	componentName := operand.GetLabels()[constant.KBAppComponentLabelKey]
	componentSpec := cluster.Spec.GetComponentByName(componentName)
	if componentSpec == nil {
		return intctrlutil.Reconciled()
	}
	componentDef := clusterDef.GetComponentDefByName(componentSpec.ComponentDefRef)

	return processor(cluster, componentSpec, componentDef)
}

// patchWorkloadCustomLabel patches workload custom labels.
func patchWorkloadCustomLabel(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	componentSpec *appsv1alpha1.ClusterComponentSpec) error {
	if cluster == nil || componentSpec == nil {
		return nil
	}
	compDef, err := util.GetComponentDefByCluster(ctx, cli, *cluster, componentSpec.ComponentDefRef)
	if err != nil {
		return err
	}
	for _, customLabelSpec := range compDef.CustomLabelSpecs {
		// TODO if the customLabelSpec.Resources is empty, we should add the label to the workload resources under the component.
		for _, resource := range customLabelSpec.Resources {
			gvk, err := util.ParseCustomLabelPattern(resource.GVK)
			if err != nil {
				return err
			}
			// only handle workload kind
			if !slices.Contains(util.GetCustomLabelWorkloadKind(), gvk.Kind) {
				continue
			}
			if err := util.PatchGVRCustomLabels(ctx, cli, cluster, resource, componentSpec.Name, customLabelSpec.Key, customLabelSpec.Value); err != nil {
				return err
			}
		}
	}
	return nil
}
