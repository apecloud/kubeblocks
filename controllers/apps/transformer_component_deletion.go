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
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
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
	comp := transCtx.Component

	// clusterName, err := getClusterName(comp)
	// if err != nil {
	//	return newRequeueError(requeueDuration, err.Error())
	// }
	//
	// compName, err := component.ShortName(clusterName, comp.Name)
	// ml := constant.GetComponentWellKnownLabels(clusterName, compName)
	// snapshot, err := model.ReadCacheSnapshot(transCtx, comp, ml, kindsForCompDelete()...)
	// if err != nil {
	//	return newRequeueError(requeueDuration, err.Error())
	// }
	// for _, object := range snapshot {
	//	graphCli.Delete(dag, object)
	// }

	comp.Status.Phase = appsv1alpha1.DeletingClusterCompPhase
	graphCli.Delete(dag, comp)

	// fast return, that is stopping the plan.Build() stage and jump to plan.Execute() directly
	return graph.ErrPrematureStop
}

// func compOwnedKinds() []client.ObjectList {
//	return []client.ObjectList{
//		&workloads.ReplicatedStateMachineList{},
//		&policyv1.PodDisruptionBudgetList{},
//		&corev1.ServiceList{},
//		&corev1.ConfigMapList{},
//		&corev1.SecretList{},
//	}
// }
//
// func kindsForCompDelete() []client.ObjectList {
//	kinds := compOwnedKinds()
//	kinds = append(kinds, &batchv1.JobList{})
//	return kinds
// }
