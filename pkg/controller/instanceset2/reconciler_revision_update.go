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
	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/revisionmap"
	"github.com/apecloud/kubeblocks/pkg/controller/workloads/instancestatus"
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

	desiredInstances, names, err := buildDesiredInstancesByName(tree, its)
	if err != nil {
		return kubebuilderx.Continue, err
	}

	updateRevisions := make(map[string]string, len(names))
	for _, name := range names {
		updateRevisions[name] = getInstanceRevision(desiredInstances[name])
	}
	revisions, err := revisionmap.Encode(updateRevisions)
	if err != nil {
		return kubebuilderx.Continue, err
	}
	its.Status.UpdateRevisions = revisions
	if len(names) > 0 {
		its.Status.UpdateRevision = updateRevisions[names[len(names)-1]]
	}

	instances := tree.List(&workloads.Instance{})
	updatedReplicas := r.calculateUpdatedReplicas(its, instances)
	its.Status.UpdatedReplicas = updatedReplicas
	r.invalidateAffectedInstanceStatus(its, desiredInstances, instances)

	its.Status.ObservedGeneration = its.Generation

	return kubebuilderx.Continue, nil
}

// invalidateAffectedInstanceStatus preserves observations for unaffected instances while preventing
// a stale UpToDate=true from crossing an InstanceSet generation that changed that instance's Pod,
// dynamic config, or PVC expansion target. Instance readiness and allocation lifecycle are independent.
func (r *revisionUpdateReconciler) invalidateAffectedInstanceStatus(its *workloads.InstanceSet,
	desiredInstances map[string]*workloads.Instance, instances []client.Object) {
	currentByName := make(map[string]*workloads.Instance, len(instances))
	for _, obj := range instances {
		inst, _ := obj.(*workloads.Instance)
		currentByName[inst.Name] = inst
	}
	for i := range its.Status.InstanceStatus {
		status := &its.Status.InstanceStatus[i]
		if !status.UpToDate || status.EffectiveDesiredState() != workloads.InstanceDesiredStateActive ||
			status.EffectiveCurrentState() != workloads.InstanceCurrentStatePresent {
			continue
		}
		desired, current := desiredInstances[status.PodName], currentByName[status.PodName]
		if desired == nil || current == nil {
			continue
		}
		if current.Generation != current.Status.ObservedGeneration {
			status.UpToDate = false
			continue
		}
		podApplied := equality.Semantic.DeepEqual(current.Spec.Template, desired.Spec.Template)
		configsApplied := instancestatus.ConfigsApplied(desired.Spec.Configs, current.Status.Configs)
		pvcApplied := volumeExpansionTargetsApplied(current, desired) && !current.Status.VolumeExpansion
		if !podApplied || !configsApplied || !pvcApplied {
			status.UpToDate = false
		}
	}
}

func volumeExpansionTargetsApplied(current, desired *workloads.Instance) bool {
	for _, desiredTemplate := range desired.Spec.VolumeClaimTemplates {
		desiredStorage := desiredTemplate.Spec.Resources.Requests.Storage()
		if desiredStorage == nil || desiredStorage.IsZero() {
			continue
		}
		found := false
		for _, currentTemplate := range current.Spec.VolumeClaimTemplates {
			if currentTemplate.Name != desiredTemplate.Name {
				continue
			}
			currentStorage := currentTemplate.Spec.Resources.Requests.Storage()
			if currentStorage != nil && currentStorage.Cmp(*desiredStorage) >= 0 {
				found = true
			}
			break
		}
		if !found {
			return false
		}
	}
	return true
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
