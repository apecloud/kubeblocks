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
)

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=apps,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=pods/status,verbs=get
// +kubebuilder:rbac:groups=apps,resources=pods/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ClusterVersion object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var (
		pod             = &corev1.Pod{}
		err             error
		cluster         *appsv1alpha1.Cluster
		ok              bool
		componentName   string
		componentStatus appsv1alpha1.ClusterComponentStatus
	)

	reqCtx := intctrlutil.RequestCtx{
		Ctx: ctx,
		Req: req,
		Log: log.FromContext(ctx).WithValues("pod", req.NamespacedName),
	}

	if err = r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, pod); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	if cluster, err = util.GetClusterByObject(reqCtx.Ctx, r.Client, pod); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	if cluster == nil {
		return intctrlutil.Reconciled()
	}

	if componentName, ok = pod.Labels[constant.KBAppComponentLabelKey]; !ok {
		return intctrlutil.Reconciled()
	}

	if cluster.Status.Components == nil {
		return intctrlutil.Reconciled()
	}
	if componentStatus, ok = cluster.Status.Components[componentName]; !ok {
		return intctrlutil.Reconciled()
	}
	if componentStatus.ConsensusSetStatus == nil {
		return intctrlutil.Reconciled()
	}
	if componentStatus.ConsensusSetStatus.Leader.Pod == util.ComponentStatusDefaultPodName {
		return intctrlutil.Reconciled()
	}

	// sync leader status from cluster.status
	patch := client.MergeFrom(pod.DeepCopy())
	pod.Annotations[constant.LeaderAnnotationKey] = componentStatus.ConsensusSetStatus.Leader.Pod
	if err = r.Client.Patch(reqCtx.Ctx, pod, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}

	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		WithEventFilter(predicate.NewPredicateFuncs(intctrlutil.WorkloadFilterPredicate)).
		Complete(r)
}
