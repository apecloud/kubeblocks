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
	corev1 "k8s.io/api/core/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func NewHeadlessServiceReconciler() kubebuilderx.Reconciler {
	return &headlessServiceReconciler{}
}

type headlessServiceReconciler struct{}

var _ kubebuilderx.Reconciler = &headlessServiceReconciler{}

func (a *headlessServiceReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	if model.IsReconciliationPaused(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (a *headlessServiceReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	its, _ := tree.GetRoot().(*workloads.InstanceSet)
	var headlessService *corev1.Service
	if !its.Spec.DisableDefaultHeadlessService {
		labels := getMatchLabels(its.Name)
		headlessSelectors := getHeadlessSvcSelector(its)
		headlessService = buildHeadlessSvc(*its, labels, headlessSelectors)
	}
	if headlessService != nil {
		if err := intctrlutil.SetOwnership(its, headlessService, model.GetScheme(), finalizer); err != nil {
			return kubebuilderx.Continue, err
		}
	}

	oldHeadlessService, err := tree.Get(buildHeadlessSvc(*its, nil, nil))
	if err != nil {
		return kubebuilderx.Continue, err
	}

	if oldHeadlessService == nil && headlessService != nil {
		if err := tree.Add(headlessService); err != nil {
			return kubebuilderx.Continue, err
		}
	}
	if oldHeadlessService != nil && headlessService != nil {
		newObj := copyAndMerge(oldHeadlessService, headlessService)
		if err := tree.Update(newObj); err != nil {
			return kubebuilderx.Continue, err
		}
	}
	if oldHeadlessService != nil && headlessService == nil {
		if err := tree.Delete(oldHeadlessService); err != nil {
			return kubebuilderx.Continue, err
		}
	}
	return kubebuilderx.Continue, nil
}
