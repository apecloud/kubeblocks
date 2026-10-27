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

package instancereadiness

import (
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type readinessReconciler struct {
	evaluate          func(*kubebuilderx.ObjectTree) (bool, error)
	retryAvailability bool
}

func NewReconcilers(evaluate func(*kubebuilderx.ObjectTree) (bool, error)) (kubebuilderx.Reconciler, kubebuilderx.Reconciler) {
	readiness := &readinessReconciler{evaluate: evaluate}
	return readiness, &availabilityReconciler{readiness: readiness}
}

func (r *readinessReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (r *readinessReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	its := tree.GetRoot().(*workloads.InstanceSet)
	previous := make(map[string]metav1.Condition, 2)
	for _, kind := range []string{string(workloads.InstanceReady), string(workloads.InstanceAvailable)} {
		if condition := meta.FindStatusCondition(its.Status.Conditions, kind); condition != nil {
			previous[kind] = *condition
		}
	}
	var err error
	r.retryAvailability, err = r.evaluate(tree)
	if err != nil {
		return kubebuilderx.Continue, err
	}
	for _, kind := range []string{string(workloads.InstanceReady), string(workloads.InstanceAvailable)} {
		condition := meta.FindStatusCondition(its.Status.Conditions, kind)
		old, found := previous[kind]
		if condition != nil && (!found || old.Status != condition.Status || old.ObservedGeneration != condition.ObservedGeneration) {
			return kubebuilderx.RetryAfter(time.Millisecond), nil
		}
	}
	return kubebuilderx.Continue, nil
}

type availabilityReconciler struct {
	readiness *readinessReconciler
}

func (r *availabilityReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	return r.readiness.PreCondition(tree)
}

func (r *availabilityReconciler) Reconcile(_ *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	if r.readiness.retryAvailability {
		return kubebuilderx.RetryAfter(time.Second), nil
	}
	return kubebuilderx.Continue, nil
}
