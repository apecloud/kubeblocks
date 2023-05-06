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

	// skip if pod is being deleted
	if !pod.DeletionTimestamp.IsZero() {
		return intctrlutil.Reconciled()
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
	if componentStatus.ConsensusSetStatus.Leader.Pod == constant.ComponentStatusDefaultPodName {
		return intctrlutil.Reconciled()
	}

	// sync leader status from cluster.status
	patch := client.MergeFrom(pod.DeepCopy())
	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}
	pod.Annotations[constant.LeaderAnnotationKey] = componentStatus.ConsensusSetStatus.Leader.Pod
	if err = r.Client.Patch(reqCtx.Ctx, pod, patch); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	r.Recorder.Eventf(pod, corev1.EventTypeNormal, "AddAnnotation", "add annotation %s=%s", constant.LeaderAnnotationKey, componentStatus.ConsensusSetStatus.Leader.Pod)
	return intctrlutil.Reconciled()
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		WithEventFilter(predicate.NewPredicateFuncs(intctrlutil.WorkloadFilterPredicate)).
		Named("pod-watcher").
		Complete(r)
}
