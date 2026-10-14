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
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func updateInstanceLifecycle(tree *kubebuilderx.ObjectTree, targets []*workloads.Instance) (bool, error) {
	updated := false
	for _, target := range targets {
		object, err := tree.Get(target)
		if err != nil {
			return updated, err
		}
		current := object.(*workloads.Instance)
		if model.IsObjectDeleting(current) || (equality.Semantic.DeepEqual(current.Spec.ScaledDown, target.Spec.ScaledDown) &&
			equality.Semantic.DeepEqual(current.Spec.PersistentVolumeClaimRetentionPolicy, target.Spec.PersistentVolumeClaimRetentionPolicy)) {
			continue
		}
		inst := current.DeepCopy()
		inst.Spec.ScaledDown = target.Spec.ScaledDown
		inst.Spec.PersistentVolumeClaimRetentionPolicy = target.Spec.PersistentVolumeClaimRetentionPolicy
		if err := tree.Update(inst); err != nil {
			return updated, err
		}
		updated = true
	}
	return updated, nil
}

func updateInstances(tree *kubebuilderx.ObjectTree, plan *instancePlan) error {
	if _, err := updateInstanceLifecycle(tree, plan.lifecycle); err != nil {
		return err
	}
	its := tree.GetRoot().(*workloads.InstanceSet)
	newNameSet := sets.New(plan.names...)
	oldNameSet := sets.New[string]()
	var oldInstanceList []*workloads.Instance
	for _, object := range tree.List(&workloads.Instance{}) {
		oldNameSet.Insert(object.GetName())
		inst, _ := object.(*workloads.Instance)
		oldInstanceList = append(oldInstanceList, inst)
	}
	updateNameSet := oldNameSet.Intersection(newNameSet)
	if len(updateNameSet) != len(oldNameSet) || len(updateNameSet) != len(newNameSet) {
		tree.Logger.Info(fmt.Sprintf("InstanceSet %s/%s instances are not aligned", its.Namespace, its.Name))
		return nil
	}

	instanceUpdateStrategy := its.Spec.InstanceUpdateStrategy
	if instanceUpdateStrategy != nil && instanceUpdateStrategy.Type == kbappsv1.OnDeleteStrategyType {
		instanceUpdateStrategy = nil
	}

	// handle 'RollingUpdate'
	replicas, maxUnavailable, err := parseReplicasNMaxUnavailable(instanceUpdateStrategy, len(oldInstanceList))
	if err != nil {
		return err
	}
	desiredInstanceMap := plan.desired

	// Account for existing rolling participants against both admission budgets before sorting candidates.
	// Otherwise traversal order could admit another Instance into a full availability or member-plan window.
	occupiedRollingSlots := 0
	occupiedMemberPlanSlots := 0
	for _, inst := range oldInstanceList {
		specHandedOffButNotConverged := copyAndMergeInstance(inst, desiredInstanceMap[inst.Name]) == nil &&
			!isInstanceUpdated(its, inst)
		if specHandedOffButNotConverged {
			occupiedMemberPlanSlots++
		}
		if !intctrlutil.IsInstanceAvailable(inst) || specHandedOffButNotConverged {
			occupiedRollingSlots++
		}
	}
	availableRollingSlots := maxUnavailable - occupiedRollingSlots

	// if it's a roleful InstanceSet, we use updateCount to represent Pods can be updated according to the spec.memberUpdateStrategy.
	updateCount := len(oldInstanceList)
	if len(its.Spec.Roles) > 0 {
		plan := newUpdatePlan(*its, oldInstanceList)
		instancesToBeUpdated, err := plan.Execute()
		if err != nil {
			return err
		}
		updateCount = max(0, len(instancesToBeUpdated)-occupiedMemberPlanSlots)
	}

	// updatedInstances tracks the positions already covered by the rolling-update
	// window, while updatingInstances tracks actual updates admitted in this round.
	updatedInstances := 0
	updatingInstances := 0
	priorities := composeRolePriorityMap(its.Spec.Roles)
	sortInstanceObjects(oldInstanceList, priorities, false)

	canBeUpdated := func(inst *workloads.Instance) bool {
		if !intctrlutil.IsInstanceReady(inst) {
			tree.Logger.Info(fmt.Sprintf("InstanceSet %s/%s blocks on update as the instance %s is not ready", its.Namespace, its.Name, inst.Name))
			return false
		}
		if !intctrlutil.IsInstanceAvailable(inst) {
			tree.Logger.Info(fmt.Sprintf("InstanceSet %s/%s blocks on update as the instance %s is not available", its.Namespace, its.Name, inst.Name))
			return false
		}
		if !intctrlutil.IsInstanceReadyWithRole(inst) {
			tree.Logger.Info(fmt.Sprintf("InstanceSet %s/%s blocks on update as the role of instance %s is not ready", its.Namespace, its.Name, inst.Name))
			return false
		}
		return true
	}

	for _, inst := range oldInstanceList {
		if updatedInstances >= replicas {
			break
		}
		if updatingInstances >= min(availableRollingSlots, updateCount) {
			break
		}

		if !canBeUpdated(inst) {
			break
		}

		newInst := desiredInstanceMap[inst.Name]
		mergedInst := copyAndMergeInstance(inst, newInst)
		if mergedInst != nil {
			err = tree.Update(mergedInst)
			if err != nil {
				return err
			}
			updatingInstances++
		}
		updatedInstances++
	}
	return nil
}

func parseReplicasNMaxUnavailable(updateStrategy *workloads.InstanceUpdateStrategy, totalReplicas int) (int, int, error) {
	replicas := totalReplicas
	maxUnavailable := 1
	if updateStrategy == nil {
		return replicas, maxUnavailable, nil
	}
	rollingUpdate := updateStrategy.RollingUpdate
	if rollingUpdate == nil {
		return replicas, maxUnavailable, nil
	}
	var err error
	if rollingUpdate.Replicas != nil {
		replicas, err = intstr.GetScaledValueFromIntOrPercent(rollingUpdate.Replicas, totalReplicas, true)
		if err != nil {
			return replicas, maxUnavailable, err
		}
	}
	if rollingUpdate.MaxUnavailable != nil {
		maxUnavailable, err = intstr.GetScaledValueFromIntOrPercent(intstr.ValueOrDefault(rollingUpdate.MaxUnavailable, intstr.FromInt32(1)), totalReplicas, false)
		if err != nil {
			return 0, 0, err
		}
		// maxUnavailable might be zero for small percentage with round down.
		// So we have to enforce it not to be less than 1.
		if maxUnavailable < 1 {
			maxUnavailable = 1
		}
	}
	return replicas, maxUnavailable, nil
}
