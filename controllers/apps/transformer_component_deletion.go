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
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// componentDeletionTransformer handles component deletion
type componentDeletionTransformer struct{}

var _ graph.Transformer = &componentDeletionTransformer{}

func (t *componentDeletionTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	if transCtx.Component.GetDeletionTimestamp().IsZero() {
		return nil
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	obj := transCtx.Component

	ml := labelsForCompDelete(obj)
	snapshot, err := model.ReadCacheSnapshot(transCtx, obj, ml, kindsForCompDelete()...)
	if err != nil {
		return err
	}
	for _, object := range snapshot {
		graphCli.Delete(dag, object)
	}

	transCtx.Component.Status.Phase = appsv1alpha1.DeletingClusterCompPhase
	graphCli.Delete(dag, obj)

	// fast return, that is stopping the plan.Build() stage and jump to plan.Execute() directly
	return graph.ErrPrematureStop
}

func compOwnedKinds() []client.ObjectList {
	return []client.ObjectList{
		&workloads.ReplicatedStateMachineList{},
		&corev1.ServiceList{},
		&corev1.ConfigMapList{},
		&corev1.SecretList{},
	}
}

func kindsForCompDelete() []client.ObjectList {
	kinds := compOwnedKinds()
	kinds = append(kinds, &batchv1.JobList{})
	return kinds
}

func labelsForCompDelete(comp *appsv1alpha1.Component) map[string]string {
	return map[string]string{
		constant.AppManagedByLabelKey:   constant.AppName,
		constant.AppInstanceLabelKey:    comp.Spec.Cluster,
		constant.KBAppComponentLabelKey: comp.Name,
	}
}
