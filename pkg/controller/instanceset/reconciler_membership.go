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

package instanceset

import (
	"errors"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
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
	its, _ := tree.GetRoot().(*workloads.InstanceSet)
	itsExt, err := instancetemplate.BuildInstanceSetExt(its, tree)
	if err != nil {
		return kubebuilderx.Continue, err
	}

	nameBuilder, err := instancetemplate.NewPodNameBuilder(
		itsExt, &instancetemplate.PodNameBuilderOpts{EventLogger: tree.EventRecorder},
	)
	if err != nil {
		return kubebuilderx.Continue, err
	}
	nameToTemplateMap, err := nameBuilder.BuildInstanceName2TemplateMap()
	if err != nil {
		return kubebuilderx.Continue, err
	}

	newNameSet := sets.New[string]()
	for name := range nameToTemplateMap {
		newNameSet.Insert(name)
	}
	oldNameSet := sets.New[string]()
	pods := tree.List(&corev1.Pod{})
	for _, pod := range pods {
		oldNameSet.Insert(pod.GetName())
	}

	for _, pod := range pods {
		if newNameSet.Has(pod.GetName()) {
			if err = lifecycleCreateInstance(tree, its, pods, pod.(*corev1.Pod)); err != nil {
				return kubebuilderx.Continue, err
			}
		}
	}

	its.Status.InstanceStatus = slices.DeleteFunc(its.Status.InstanceStatus, func(inst workloads.InstanceStatus) bool {
		// The pod has been deleted, but the subsequent update of ITS status failed. Remove it from InstanceStatus directly.
		return !newNameSet.Has(inst.PodName) && !oldNameSet.Has(inst.PodName)
	})

	return kubebuilderx.Continue, nil
}

func lifecycleCreateInstance(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, pods []client.Object, pod *corev1.Pod) error {
	idx := slices.IndexFunc(its.Status.InstanceStatus, func(inst workloads.InstanceStatus) bool {
		return inst.PodName == pod.Name
	})
	if idx < 0 {
		its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{
			PodName:      pod.Name,
			Provisioned:  true,
			DataLoaded:   shouldLoadData(its),
			MemberJoined: shouldJoinMember(its),
		})
		idx = len(its.Status.InstanceStatus) - 1
	}

	inst := its.Status.InstanceStatus[idx]
	if !inst.Provisioned {
		return nil
	}
	if inst.DataLoaded != nil && !*inst.DataLoaded {
		return nil // loading
	}
	if inst.MemberJoined == nil || *inst.MemberJoined {
		return nil // not defined or joined
	}
	if err := lifecycleJoinMember(tree, its, pods, pod); err != nil {
		tree.Logger.Info("failed to join member", "pod", pod.Name, "error", err.Error())
	} else {
		its.Status.InstanceStatus[idx].MemberJoined = ptr.To(true)
	}
	return nil
}

func lifecycleJoinMember(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, pods []client.Object, pod *corev1.Pod) error {
	lfa, err := newLifecycleAction(its, pods, pod)
	if err != nil {
		return err
	}
	if err = lfa.MemberJoin(tree.Context, tree.Reader, nil); err != nil {
		if !errors.Is(err, lifecycle.ErrActionNotDefined) {
			return err
		}
	}
	tree.Logger.Info("succeed to call member join action", "pod", pod.Name)
	return nil
}

func lifecycleDeleteInstance(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, pods []client.Object, pod *corev1.Pod) error {
	idx := slices.IndexFunc(its.Status.InstanceStatus, func(inst workloads.InstanceStatus) bool {
		return inst.PodName == pod.Name
	})
	if idx < 0 {
		return nil
	}
	inst := its.Status.InstanceStatus[idx]
	if ptr.Deref(inst.MemberJoined, false) {
		if err := lifecycleLeaveMember(tree, its, pods, pod); err != nil {
			return err
		}
	}
	its.Status.InstanceStatus = slices.Delete(its.Status.InstanceStatus, idx, idx+1)
	return nil
}

func lifecycleLeaveMember(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, pods []client.Object, pod *corev1.Pod) error {
	switchover := func(lfa lifecycle.Lifecycle, pod *corev1.Pod) error {
		if its.Spec.LifecycleActions.Switchover == nil {
			return nil
		}
		err := lfa.Switchover(tree.Context, tree.Reader, nil, "")
		if err != nil {
			if errors.Is(err, lifecycle.ErrActionNotDefined) {
				return nil
			}
			return err
		}
		tree.Logger.Info("succeed to call switchover action before leave member", "pod", pod.Name)
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
		tree.Logger.Info("succeed to call leave member action", "pod", pod.Name)
		return nil
	}

	lfa, err := newLifecycleAction(its, pods, pod)
	if err != nil {
		return err

	}
	if err = switchover(lfa, pod); err != nil {
		tree.Logger.Error(err, "failed to call switchover action before leave member, ignore and continue", "pod", pod.Name)
	}
	return memberLeave(lfa, pod)
}

func shouldLoadData(its *workloads.InstanceSet) *bool {
	if its.Spec.LifecycleActions != nil && its.Spec.LifecycleActions.DataLoad != nil {
		return ptr.To(its.IsInInitializing())
	}
	return nil
}

func shouldJoinMember(its *workloads.InstanceSet) *bool {
	if its.Spec.LifecycleActions != nil && its.Spec.LifecycleActions.MemberJoin != nil {
		return ptr.To(its.IsInInitializing())
	}
	return nil
}
