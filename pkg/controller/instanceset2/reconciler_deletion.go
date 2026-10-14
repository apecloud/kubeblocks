/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"maps"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func NewDeletionReconciler() kubebuilderx.Reconciler {
	return &deletionReconciler{}
}

type deletionReconciler struct{}

var _ kubebuilderx.Reconciler = &deletionReconciler{}

func (r *deletionReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || !model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	if model.IsReconciliationPaused(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (r *deletionReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	its := tree.GetRoot().(*workloads.InstanceSet)
	updated := false
	var desiredNames sets.Set[string]
	for _, obj := range tree.List(&workloads.Instance{}) {
		inst := obj.(*workloads.Instance)
		if model.IsObjectDeleting(inst) {
			continue
		}
		if ptr.Deref(inst.Spec.ScaledDown, false) && desiredNames == nil {
			itsExt, err := instancetemplate.BuildInstanceSetExt(its, tree)
			if err != nil {
				return kubebuilderx.Continue, err
			}
			nameBuilder, err := instancetemplate.NewPodNameBuilder(itsExt, &instancetemplate.PodNameBuilderOpts{EventLogger: tree.EventRecorder})
			if err != nil {
				return kubebuilderx.Continue, err
			}
			names, err := nameBuilder.GenerateAllInstanceNames()
			if err != nil {
				return kubebuilderx.Continue, err
			}
			desiredNames = sets.New(names...)
		}
		cancelledScaleDown := ptr.Deref(inst.Spec.ScaledDown, false) && desiredNames.Has(inst.Name)
		if !cancelledScaleDown && equality.Semantic.DeepEqual(
			inst.Spec.PersistentVolumeClaimRetentionPolicy, its.Spec.PersistentVolumeClaimRetentionPolicy) {
			continue
		}
		instCopy := inst.DeepCopy()
		if cancelledScaleDown {
			instCopy.Spec.ScaledDown = nil
		}
		instCopy.Spec.PersistentVolumeClaimRetentionPolicy = its.Spec.PersistentVolumeClaimRetentionPolicy.DeepCopy()
		if err := tree.Update(instCopy); err != nil {
			return kubebuilderx.Continue, err
		}
		updated = true
	}
	if updated {
		return kubebuilderx.RetryAfter(time.Second), nil
	}

	if has, err := r.deleteSecondaryObjects(tree); has {
		return kubebuilderx.Continue, err
	}

	tree.DeleteRoot()
	return kubebuilderx.Continue, nil
}

func (r *deletionReconciler) deleteSecondaryObjects(tree *kubebuilderx.ObjectTree) (bool, error) {
	// secondary objects to be deleted
	secondaryObjects := maps.Clone(tree.GetSecondaryObjects())
	for _, obj := range secondaryObjects {
		if err := tree.Delete(obj); err != nil {
			return true, err
		}
	}
	return len(secondaryObjects) > 0, nil
}
