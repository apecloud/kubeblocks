/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package rsm2

import (
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// revisionUpdateReconciler is responsible for updating the expected replica names and their corresponding revisions in the status when there are changes in the spec.
type revisionUpdateReconciler struct{}

type replicaRevision struct {
	name     string
	revision string
}

func NewRevisionUpdateReconciler() kubebuilderx.Reconciler {
	return &revisionUpdateReconciler{}
}

func (r *revisionUpdateReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || !model.IsObjectUpdating(tree.GetRoot()) {
		return kubebuilderx.ResultUnsatisfied
	}
	rsm, _ := tree.GetRoot().(*workloads.ReplicatedStateMachine)
	if err := validateSpec(rsm, tree); err != nil {
		return kubebuilderx.CheckResultWithError(err)
	}
	return kubebuilderx.ResultSatisfied
}

func (r *revisionUpdateReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (*kubebuilderx.ObjectTree, error) {
	rsm, _ := tree.GetRoot().(*workloads.ReplicatedStateMachine)

	// 1. build all templates by applying instance template overrides to default pod template
	replicaTemplateGroups := buildReplicaTemplateGroups(rsm, tree)

	// build replica revision list by template groups
	var replicaRevisionList []replicaRevision
	for _, templateList := range replicaTemplateGroups {
		var (
			replicas []replicaRevision
			ordinal  int
			err      error
		)
		for _, template := range templateList {
			replicas, ordinal, err = buildReplicaRevisions(template, ordinal, rsm)
			if err != nil {
				return nil, err
			}
			replicaRevisionList = append(replicaRevisionList, replicas...)
		}
	}
	// validate duplicate pod names
	getNameFunc := func(r replicaRevision) string {
		return r.name
	}
	if err := validateDupReplicaNames(replicaRevisionList, getNameFunc); err != nil {
		return nil, err
	}

	updatedRevisions := make(map[string]string, len(replicaRevisionList))
	for _, r := range replicaRevisionList {
		updatedRevisions[r.name] = r.revision
	}

	// 3. persistent these revisions to status
	revisions, err := buildUpdateRevisions(updatedRevisions)
	if err != nil {
		return nil, err
	}
	rsm.Status.UpdateRevisions = revisions
	updateRevision := ""
	if len(replicaRevisionList) > 0 {
		updateRevision = replicaRevisionList[len(replicaRevisionList)-1].revision
	}
	rsm.Status.UpdateRevision = updateRevision
	// The 'ObservedGeneration' field is used to indicate whether the revisions have been updated.
	// Computing these revisions in each reconciliation loop can be time-consuming, so we optimize it by
	// performing the computation only when the 'spec' is updated.
	rsm.Status.ObservedGeneration = rsm.Generation

	return tree, nil
}

func buildReplicaRevisions(template *podTemplateSpecExt, ordinal int, parent *workloads.ReplicatedStateMachine) ([]replicaRevision, int, error) {
	revision, err := buildPodTemplateRevision(template, parent)
	if err != nil {
		return nil, ordinal, err
	}
	var replicaList []replicaRevision
	var name string
	for i := 0; i < int(template.Replicas); i++ {
		name, ordinal = generatePodName(template.Name, template.GenerateName, ordinal, int(template.OrdinalStart), i)
		replicaList = append(replicaList, replicaRevision{name: name, revision: revision})
	}
	return replicaList, ordinal, nil
}

var _ kubebuilderx.Reconciler = &revisionUpdateReconciler{}
