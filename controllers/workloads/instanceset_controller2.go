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

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	workloadsv1alpha1 "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/handler"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset2"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
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
		Do(instanceset2.NewReplicasAlignmentReconciler()).
		Do(instanceset2.NewUpdateReconciler()).
		Commit()
}

func (r *InstanceSetReconciler2) SetupWithManager(mgr ctrl.Manager) error {
	ctx := &handler.FinderContext{
		Context: context.Background(),
		Reader:  r.Client,
		Scheme:  *r.Scheme,
	}
	return r.setupWithManager(mgr, ctx)
}

func (r *InstanceSetReconciler2) setupWithManager(mgr ctrl.Manager, ctx *handler.FinderContext) error {
	itsFinder := handler.NewLabelFinder(&workloads.InstanceSet{}, instanceset2.WorkloadsManagedByLabelKey, workloads.InstanceSetKind, instanceset2.WorkloadsInstanceLabelKey)
	podHandler := handler.NewBuilder(ctx).AddFinder(itsFinder).Build()
	return intctrlutil.NewControllerManagedBy(mgr).
		For(&workloads.InstanceSet{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: viper.GetInt(constant.CfgKBReconcileWorkers),
		}).
		Owns(&workloadsv1alpha1.Instance{}).
		Watches(&corev1.Pod{}, podHandler).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
