/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset2"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

type InstanceSetReconciler2 struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

func (r *InstanceSetReconciler2) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("InstanceSet2", req.NamespacedName)
	return kubebuilderx.NewController(ctx, r.Client, req, r.Recorder, logger).
		Prepare(instanceset2.NewTreeLoader()).
		Do(instanceset2.NewAPIVersionReconciler()).
		Do(instanceset2.NewFixMetaReconciler()).
		Do(instanceset2.NewDeletionReconciler()).
		Do(instanceset2.NewValidationReconciler()).
		Do(instanceset2.NewStatusReconciler()).
		Do(instanceset2.NewRevisionUpdateReconciler()).
		Do(instanceset2.NewAssistantObjectReconciler()).
		Do(instanceset2.NewAlignmentReconciler()).
		Do(instanceset2.NewUpdateReconciler()).
		Commit()
}

func (r *InstanceSetReconciler2) SetupWithManager(mgr ctrl.Manager, multiClusterMgr multicluster.Manager) error {
	if multiClusterMgr == nil {
		return r.setupWithManager(mgr)
	}
	return r.setupWithMultiClusterManager(mgr, multiClusterMgr)
}

func (r *InstanceSetReconciler2) setupWithManager(mgr ctrl.Manager) error {
	return intctrlutil.NewControllerManagedBy(mgr).
		For(&workloads.InstanceSet{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: viper.GetInt(constant.CfgKBReconcileWorkers),
		}).
		Owns(&workloads.Instance{}).
		Owns(&corev1.Service{}). // headless service
		Complete(r)
}

func (r *InstanceSetReconciler2) setupWithMultiClusterManager(mgr ctrl.Manager, multiClusterMgr multicluster.Manager) error {
	b := intctrlutil.NewControllerManagedBy(mgr).
		For(&workloads.InstanceSet{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: viper.GetInt(constant.CfgKBReconcileWorkers),
		})

	eventHandler := handler.EnqueueRequestsFromMapFunc(r.instanceFilter)
	multiClusterMgr.Watch(b, &workloads.Instance{}, eventHandler).
		Watch(b, &corev1.Service{}, eventHandler) // headless service

	return b.Complete(r)
}

func (r *InstanceSetReconciler2) instanceFilter(ctx context.Context, obj client.Object) []reconcile.Request {
	labels := obj.GetLabels()
	if v, ok := labels[constant.AppManagedByLabelKey]; !ok || v != constant.AppName {
		return []reconcile.Request{}
	}
	if _, ok := labels[constant.AppInstanceLabelKey]; !ok {
		return []reconcile.Request{}
	}
	if _, ok := labels[constant.KBAppComponentLabelKey]; !ok {
		return []reconcile.Request{}
	}
	name := constant.GenerateWorkloadNamePattern(labels[constant.AppInstanceLabelKey], labels[constant.KBAppComponentLabelKey])
	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Namespace: obj.GetNamespace(),
				Name:      name,
			},
		},
	}
}
