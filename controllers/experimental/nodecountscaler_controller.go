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

package experimental

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	experimental "github.com/apecloud/kubeblocks/apis/experimental/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

// NodeCountScalerReconciler reconciles a NodeCountScaler object
type NodeCountScalerReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=experimental.kubeblocks.io,resources=nodecountscalers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=experimental.kubeblocks.io,resources=nodecountscalers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=experimental.kubeblocks.io,resources=nodecountscalers/finalizers,verbs=update

// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters,verbs=get;list;watch;update;patch

// +kubebuilder:rbac:groups=workloads.kubeblocks.io,resources=instancesets,verbs=get;list;watch
// +kubebuilder:rbac:groups=workloads.kubeblocks.io,resources=instancesets/status,verbs=get

//+kubebuilder:rbac:groups="",resources=nodes,verbs=list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *NodeCountScalerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("NodeCountScaler", req.NamespacedName)

	return kubebuilderx.NewController(ctx, r.Client, req, r.Recorder, logger).
		Prepare(objectTree()).
		Do(scaleTargetCluster()).
		Do(updateStatus()).
		Commit()
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeCountScalerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&experimental.NodeCountScaler{}).
		Watches(&corev1.Node{}, &nodeScalingHandler{r.Client}).
		Watches(&appsv1.Cluster{}, &clusterHandler{r.Client}).
		Complete(r)
}
