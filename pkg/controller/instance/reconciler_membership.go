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

package instance

import (
	"errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func NewMembershipReconciler() kubebuilderx.Reconciler {
	return &membershipReconciler{}
}

type membershipReconciler struct{}

var _ kubebuilderx.Reconciler = &membershipReconciler{}

func (r *membershipReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	if model.IsReconciliationPaused(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (r *membershipReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	inst, _ := tree.GetRoot().(*workloads.Instance)
	pods := tree.List(&corev1.Pod{})

	if !inst.Status.Provisioned {
		if len(pods) == 0 {
			return kubebuilderx.Continue, nil // wait provision
		} else {
			inst.Status.Provisioned = true
			inst.Status.DataLoaded = shouldLoadData(inst)
			inst.Status.MemberJoined = shouldJoinMember(inst)
		}
	}

	var err error
	if len(pods) > 0 {
		err = lifecycleCreateInstance(tree, inst, pods[0].(*corev1.Pod))
	}
	return kubebuilderx.Continue, err
}

func lifecycleCreateInstance(tree *kubebuilderx.ObjectTree, inst *workloads.Instance, pod *corev1.Pod) error {
	if !inst.Status.Provisioned {
		return nil
	}
	if inst.Status.DataLoaded != nil && !*inst.Status.DataLoaded {
		return nil // loading
	}
	if inst.Status.MemberJoined == nil || *inst.Status.MemberJoined {
		return nil // not defined or joined
	}
	if err := lifecycleJoinMember(tree, inst, pod); err != nil {
		tree.Logger.Info("failed to join member", "error", err.Error())
	} else {
		inst.Status.MemberJoined = ptr.To(true)
	}
	return nil
}

func lifecycleJoinMember(tree *kubebuilderx.ObjectTree, inst *workloads.Instance, pod *corev1.Pod) error {
	lfa, err := newLifecycleAction(inst, pod)
	if err != nil {
		return err
	}
	if err = lfa.MemberJoin(tree.Context, tree.Reader, nil); err != nil {
		if !errors.Is(err, lifecycle.ErrActionNotDefined) {
			return err
		}
	}
	tree.Logger.Info("succeed to call member join action")
	return nil
}

func lifecycleDeleteInstance(tree *kubebuilderx.ObjectTree, inst *workloads.Instance, pod *corev1.Pod) error {
	if ptr.Deref(inst.Status.MemberJoined, false) {
		if err := lifecycleLeaveMember(tree, inst, pod); err != nil {
			return err
		}
	}
	return nil
}

func lifecycleLeaveMember(tree *kubebuilderx.ObjectTree, inst *workloads.Instance, pod *corev1.Pod) error {
	switchover := func(lfa lifecycle.Lifecycle, pod *corev1.Pod) error {
		if inst.Spec.LifecycleActions.Switchover == nil {
			return nil
		}
		err := lfa.Switchover(tree.Context, tree.Reader, nil, "")
		if err != nil {
			if errors.Is(err, lifecycle.ErrActionNotDefined) {
				return nil
			}
			return err
		}
		tree.Logger.Info("succeed to call switchover action before leave member")
		return nil
	}

	memberLeave := func(lfa lifecycle.Lifecycle, pod *corev1.Pod) error {
		err := lfa.MemberLeave(tree.Context, tree.Reader, nil)
		if err != nil {
			if errors.Is(err, lifecycle.ErrActionNotDefined) {
				return nil
			}
			return err
		}
		tree.Logger.Info("succeed to call leave member action")
		return nil
	}

	lfa, err := newLifecycleAction(inst, pod)
	if err != nil {
		return err

	}
	if err = switchover(lfa, pod); err != nil {
		tree.Logger.Error(err, "failed to call switchover action before leave member, ignore and continue")
	}
	return memberLeave(lfa, pod)
}

func shouldLoadData(inst *workloads.Instance) *bool {
	if inst.Spec.LifecycleActions != nil && inst.Spec.LifecycleActions.DataLoad != nil {
		return ptr.To(false)
	}
	return nil
}

func shouldJoinMember(inst *workloads.Instance) *bool {
	if inst.Spec.LifecycleActions != nil && inst.Spec.LifecycleActions.MemberJoin != nil {
		return ptr.To(false)
	}
	return nil
}
