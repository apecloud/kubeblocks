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

	newNameSet := sets.New[string]()
	for _, obj := range tree.List(&corev1.Pod{}) {
		newNameSet.Insert(obj.GetName())
	}
	oldNameSet := sets.New[string]()
	for _, inst := range its.Status.InstanceStatus {
		oldNameSet.Insert(inst.PodName)
	}
	createNameSet := newNameSet.Difference(oldNameSet)
	deleteNameSet := oldNameSet.Difference(newNameSet)

	loadData := func() *bool {
		if its.Spec.LifecycleActions != nil && its.Spec.LifecycleActions.DataLoad != nil {
			return ptr.To(r.initReplica(its))
		}
		return nil
	}
	joinMember := func() *bool {
		if its.Spec.LifecycleActions != nil && its.Spec.LifecycleActions.MemberJoin != nil {
			return ptr.To(r.initReplica(its))
		}
		return nil
	}

	for name := range createNameSet {
		its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{
			PodName:      name,
			Provisioned:  true,
			DataLoaded:   loadData(),
			MemberJoined: joinMember(),
		})
	}

	for name := range deleteNameSet {
		idx := slices.IndexFunc(its.Status.InstanceStatus, func(inst workloads.InstanceStatus) bool {
			return inst.PodName == name
		})
		if idx < 0 {
			continue
		}
		inst := its.Status.InstanceStatus[idx]
		if ptr.Deref(inst.MemberJoined, false) {
			if err := r.leaveMember(tree, its, nil, nil); err != nil { // TODO: pods & pod
				return kubebuilderx.Continue, err
			}
		}
		its.Status.InstanceStatus = slices.Delete(its.Status.InstanceStatus, idx, idx+1)
	}

	for i, inst := range its.Status.InstanceStatus {
		if createNameSet.Has(inst.PodName) {
			continue
		}
		if !inst.Provisioned {
			continue
		}
		if inst.DataLoaded != nil && !*inst.DataLoaded {
			continue // loading
		}
		if inst.MemberJoined == nil || *inst.MemberJoined {
			continue // joined or not defined
		}
		if err := r.joinMember(tree, its, nil, nil); err != nil { // TODO: pods & pod
			return kubebuilderx.Continue, err
		}
		its.Status.InstanceStatus[i].MemberJoined = ptr.To(true)
	}

	return kubebuilderx.Continue, nil
}

func (r *membershipReconciler) initReplica(its *workloads.InstanceSet) bool {
	if its.Status.InitReplicas == nil || *its.Status.InitReplicas != ptr.Deref(its.Status.ReadyInitReplicas, 0) {
		return true
	}
	return false
}

func (r *membershipReconciler) joinMember(tree *kubebuilderx.ObjectTree,
	its *workloads.InstanceSet, pods []client.Object, pod *corev1.Pod) error {
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

func (r *membershipReconciler) leaveMember(tree *kubebuilderx.ObjectTree,
	its *workloads.InstanceSet, pods []client.Object, pod *corev1.Pod) error {
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
