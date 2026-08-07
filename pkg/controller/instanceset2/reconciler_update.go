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

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/revisionmap"
	"github.com/apecloud/kubeblocks/pkg/controller/rollingupdate"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func NewUpdateReconciler() kubebuilderx.Reconciler {
	return &updateReconciler{}
}

type updateReconciler struct{}

var _ kubebuilderx.Reconciler = &updateReconciler{}

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
	// OnDelete ends the rolling-update lifecycle even while the instance set is
	// temporarily unaligned (for example, during scaling).
	if its.Spec.InstanceUpdateStrategy != nil && its.Spec.InstanceUpdateStrategy.Type == kbappsv1.OnDeleteStrategyType {
		if rollingupdate.Reset(its) {
			return kubebuilderx.Commit, nil
		}
		return kubebuilderx.Continue, nil
	}
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
		plan := newUpdatePlan(*its, oldInstanceList)
		instancesToBeUpdated, err := plan.Execute()
		if err != nil {
			return kubebuilderx.Continue, err
		}
		updateCount = len(instancesToBeUpdated)
	}

	updatingInstances := 0
	priorities := composeRolePriorityMap(its.Spec.Roles)
	sortInstanceObjects(oldInstanceList, priorities, false)
	orderedNames := make([]string, len(oldInstanceList))
	for i, inst := range oldInstanceList {
		orderedNames[i] = inst.Name
	}
	updateRevisions, err := revisionmap.Decode(its.Status.UpdateRevisions)
	if err != nil {
		return kubebuilderx.Continue, err
	}
	rolloutID := rollingupdate.CurrentRolloutID(its, updateRevisions)
	participants, windowChanged := rollingupdate.Participants(
		its, rolloutID, replicas, orderedNames)
	if windowChanged {
		return kubebuilderx.Commit, nil
	}

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
		if !participants.Has(inst.Name) {
			continue
		}
		if updatingInstances >= min(unavailable, updateCount) {
			break
		}

		if !canBeUpdated(inst) {
			break
		}

		newInst, err := buildInstanceByTemplate(tree, inst.Name, nameToTemplateMap[inst.Name], its)
		if err != nil {
			return kubebuilderx.Continue, err
		}
		mergedInst := copyAndMergeInstance(inst, newInst)
		if mergedInst != nil {
			err = tree.Update(mergedInst)
			if err != nil {
				return kubebuilderx.Continue, err
			}
			updatingInstances++
		}
	}
	return kubebuilderx.Continue, nil
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
