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
	"fmt"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// updateReconciler handles the updates of instances based on the UpdateStrategy.
// Currently, two update strategies are supported: 'OnDelete' and 'RollingUpdate'.
type updateReconciler struct{}

var _ kubebuilderx.Reconciler = &updateReconciler{}

func NewUpdateReconciler() kubebuilderx.Reconciler {
	return &updateReconciler{}
}

func (r *updateReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	if model.IsReconciliationPaused(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (r *updateReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	its, _ := tree.GetRoot().(*workloads.InstanceSet)
	itsExt, err := instancetemplate.BuildInstanceSetExt(its, tree)
	if err != nil {
		return kubebuilderx.Continue, err
	}

	// 1. build desired name to template map
	nameBuilder, err := instancetemplate.NewPodNameBuilder(itsExt, nil)
	if err != nil {
		return kubebuilderx.Continue, err
	}
	nameToTemplateMap, err := nameBuilder.BuildInstanceName2TemplateMap()
	if err != nil {
		return kubebuilderx.Continue, err
	}

	// 2. validate the update set
	newNameSet := sets.New[string]()
	for name := range nameToTemplateMap {
		newNameSet.Insert(name)
	}
	oldNameSet := sets.New[string]()
	oldInstanceMap := make(map[string]*workloads.Instance)
	var oldInstanceList []*workloads.Instance
	for _, object := range tree.List(&workloads.Instance{}) {
		oldNameSet.Insert(object.GetName())
		inst, _ := object.(*workloads.Instance)
		oldInstanceMap[object.GetName()] = inst
		oldInstanceList = append(oldInstanceList, inst)
	}
	updateNameSet := oldNameSet.Intersection(newNameSet)
	if len(updateNameSet) != len(oldNameSet) || len(updateNameSet) != len(newNameSet) {
		tree.Logger.Info(fmt.Sprintf("InstanceSet %s/%s instances are not aligned", its.Namespace, its.Name))
		return kubebuilderx.Continue, nil
	}

	// 3. do update
	instanceUpdateStrategy := its.Spec.InstanceUpdateStrategy
	if instanceUpdateStrategy != nil && instanceUpdateStrategy.Type == kbappsv1.OnDeleteStrategyType {
		instanceUpdateStrategy = nil
	}

	// handle 'RollingUpdate'
	replicas, maxUnavailable, err := parseReplicasNMaxUnavailable(instanceUpdateStrategy, len(oldInstanceList))
	if err != nil {
		return kubebuilderx.Continue, err
	}
	currentUnavailable := 0
	for _, inst := range oldInstanceList {
		if !intctrlutil.IsInstanceAvailable(inst) {
			currentUnavailable++
		}
	}
	unavailable := maxUnavailable - currentUnavailable

	// if it's a roleful InstanceSet, we use updateCount to represent Pods can be updated according to the spec.memberUpdateStrategy.
	updateCount := len(oldInstanceList)
	if len(its.Spec.Roles) > 0 {
		plan := NewUpdatePlan(*its, oldInstanceList, r.isInstanceUpdated(tree))
		instancesToBeUpdated, err := plan.Execute()
		if err != nil {
			return kubebuilderx.Continue, err
		}
		updateCount = len(instancesToBeUpdated)
	}

	updatingInstances := 0
	updatedInstances := 0
	priorities := ComposeRolePriorityMap(its.Spec.Roles)
	sortObjects(oldInstanceList, priorities, false)

	// TODO: ???
	// treat old and Pending pod as a special case, as they can be updated without a consequence
	// PodUpdatePolicy is ignored here since in-place update for a pending pod doesn't make much sense.
	// for _, pod := range oldInstanceList {
	//	updatePolicy, err := getPodUpdatePolicy(its, pod)
	//	if err != nil {
	//		return kubebuilderx.Continue, err
	//	}
	//	if isPodPending(pod) && updatePolicy != NoOpsPolicy {
	//		err = tree.Delete(pod)
	//		// wait another reconciliation, so that the following update process won't be confused
	//		return kubebuilderx.Continue, err
	//	}
	// }

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
		if updatingInstances >= updateCount || updatingInstances >= unavailable {
			break
		}
		if updatedInstances >= replicas {
			break
		}

		if !canBeUpdated(inst) {
			break
		}

		updatePolicy, err := getInstanceUpdatePolicy(tree, its, inst)
		if err != nil {
			return kubebuilderx.Continue, err
		}
		if updatePolicy != NoOpsPolicy {
			newInst, err := buildInstanceByTemplate(tree, inst.Name, nameToTemplateMap[inst.Name], its, getInstanceRevision(inst))
			if err != nil {
				return kubebuilderx.Continue, err
			}
			mergedInst := copyAndMerge(inst, newInst)
			if mergedInst != nil {
				err = tree.Update(mergedInst)
				if err != nil {
					return kubebuilderx.Continue, err
				}
				updatingInstances++
			}
		}
		updatedInstances++
	}
	return kubebuilderx.Continue, nil
}

func (r *updateReconciler) isInstanceUpdated(tree *kubebuilderx.ObjectTree) func(*workloads.InstanceSet, *workloads.Instance) (bool, error) {
	return func(its *workloads.InstanceSet, inst *workloads.Instance) (bool, error) {
		return isInstanceUpdated(tree, its, inst)
	}
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
		replicas, err = intstr.GetScaledValueFromIntOrPercent(rollingUpdate.Replicas, totalReplicas, false)
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
