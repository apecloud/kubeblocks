/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package components

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
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
		fmt.Printf("reconcile get object error: %s\n", err.Error())
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	handler := func(cluster *appsv1alpha1.Cluster, compSpec *appsv1alpha1.ClusterComponentSpec,
		compDef *appsv1alpha1.ClusterComponentDefinition) (ctrl.Result, error) {
		// patch the current componentSpec workload's custom labels
		if err := patchWorkloadCustomLabel(reqCtx.Ctx, r.Client, cluster, compSpec); err != nil {
			reqCtx.Recorder.Event(cluster, corev1.EventTypeWarning, "Component Workload Controller PatchWorkloadCustomLabelFailed", err.Error())
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		}

		if requeueAfter, err := handleWorkloadUpdate(reqCtx.Ctx, r.Client, pObj, cluster, compSpec, compDef); err != nil {
			return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
		} else if requeueAfter != 0 {
			// if the reconcileAction need requeue, do it
			return intctrlutil.RequeueAfter(requeueAfter, reqCtx.Log, "")
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

// handleWorkloadUpdate updates cluster.Status.Components if the component status changed
func handleWorkloadUpdate(ctx context.Context, cli client.Client, obj client.Object, cluster *appsv1alpha1.Cluster,
	compSpec *appsv1alpha1.ClusterComponentSpec, compDef *appsv1alpha1.ClusterComponentDefinition) (time.Duration, error) {
	// make a copy of cluster before any operations
	clusterDeepCopy := cluster.DeepCopy()

	dag := graph.NewDAG()
	component, err := NewComponentByType(cli, cluster, compSpec, *compDef, dag)
	if err != nil {
		return 0, err
	}

	// patch role labels and update roles in cluster status
	if err := component.HandleRoleChange(ctx, obj); err != nil {
		return 0, nil
	}

	if err := component.HandleRestart(ctx, obj); err != nil {
		return 0, nil
	}

	// update component status
	// TODO: wait & requeue
	newStatus, err := component.GetLatestStatus(ctx, obj)
	if err != nil {
		return 0, err
	} else if newStatus != nil {
		status := cluster.Status.Components[component.GetName()]
		status.Phase = newStatus.Phase
		status.Message = newStatus.Message
		status.PodsReady = newStatus.PodsReady
		status.PodsReadyTime = newStatus.PodsReadyTime
		cluster.Status.Components[component.GetName()] = status
	}

	// TODO(refactor)
	//if err = opsutil.MarkRunningOpsRequestAnnotation2(ctx, cli, cluster, dag); err != nil {
	//	return 0, err
	//}

	var rootVertex *types.ComponentVertex

	newCompStatus := cluster.Status.Components[component.GetName()]
	oldCompStatus := clusterDeepCopy.Status.Components[component.GetName()]
	if !reflect.DeepEqual(cluster.Annotations, clusterDeepCopy.Annotations) {
		rootVertex = types.AddVertex4Update(dag, cluster)
	} else if !reflect.DeepEqual(oldCompStatus, newCompStatus) {
		rootVertex = types.AddVertex4Status(dag, cluster, clusterDeepCopy)
	} else {
		rootVertex = types.AddVertex4Noop(dag, cluster) // as a placeholder to pass the dag validation
	}

	for _, v := range dag.Vertices() {
		vv, _ := v.(*types.ComponentVertex)
		if _, ok := vv.Obj.(*appsv1alpha1.Cluster); !ok {
			dag.Connect(rootVertex, v)
		}
	}

	if err := dag.WalkReverseTopoOrder(types.ExecuteComponentVertex); err != nil {
		return 0, err
	}

	// TODO: wait
	return 0, nil
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
	componentSpec := cluster.GetComponentByName(componentName)
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
