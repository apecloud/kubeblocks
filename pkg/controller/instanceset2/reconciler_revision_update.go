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
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/revisionmap"
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

	updatedReplicas := r.calculateUpdatedReplicas(its, tree.List(&workloads.Instance{}))
	its.Status.UpdatedReplicas = updatedReplicas

	instances := tree.List(&workloads.Instance{})
	instanceList := make([]*workloads.Instance, 0, len(instances))
	for _, object := range instances {
		instance, _ := object.(*workloads.Instance)
		instanceList = append(instanceList, instance)
	}
	pods := tree.List(&corev1.Pod{})
	podList := make([]*corev1.Pod, 0, len(pods))
	for _, object := range pods {
		pod, _ := object.(*corev1.Pod)
		podList = append(podList, pod)
	}
	if err := setInstanceStatus(tree, its, instanceList, podList); err != nil {
		return kubebuilderx.Continue, err
	}

	// ObservedGeneration is advanced only after revisions and the complete instance identity view are published.
	its.Status.ObservedGeneration = its.Generation

	return kubebuilderx.Continue, nil
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
