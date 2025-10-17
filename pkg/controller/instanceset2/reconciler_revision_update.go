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

package instanceset2

import (
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func NewRevisionUpdateReconciler() kubebuilderx.Reconciler {
	return &revisionUpdateReconciler{}
}

type revisionUpdateReconciler struct{}

var _ kubebuilderx.Reconciler = &revisionUpdateReconciler{}

func (r *revisionUpdateReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || !model.IsObjectUpdating(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (r *revisionUpdateReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	its, _ := tree.GetRoot().(*workloads.InstanceSet)

	its.Status.UpdatedReplicas = r.calculateUpdatedReplicas(its, tree.List(&workloads.Instance{}))
	its.Status.InitReplicas = r.buildInitReplicas(its)

	its.Status.ObservedGeneration = its.Generation

	return kubebuilderx.Continue, nil
}

func (r *revisionUpdateReconciler) calculateUpdatedReplicas(its *workloads.InstanceSet, instances []client.Object) int32 {
	updatedReplicas := int32(0)
	for i := range instances {
		inst, _ := instances[i].(*workloads.Instance)
		if isInstanceUpdated(its, inst) {
			updatedReplicas++
		}
	}
	return updatedReplicas
}

func (r *revisionUpdateReconciler) buildInitReplicas(its *workloads.InstanceSet) *int32 {
	initReplicas := its.Status.InitReplicas
	if initReplicas == nil && ptr.Deref(its.Spec.Replicas, 0) > 0 {
		initReplicas = its.Spec.Replicas
	}
	if initReplicas == nil {
		return nil // the replicas is not set or set to 0
	}

	if *initReplicas != ptr.Deref(its.Status.ReadyInitReplicas, 0) { // in init phase
		// in case the replicas is changed in the middle of init phase
		if ptr.Deref(its.Spec.Replicas, 0) == 0 {
			return nil
		} else {
			return its.Spec.Replicas
		}
	}
	return initReplicas
}
