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

// revisionUpdateReconciler is responsible for updating the expected instance names and their corresponding revisions in the status when there are changes in the spec.
type revisionUpdateReconciler struct{}

type instanceRevision struct {
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
	rsm, _ := tree.GetRoot().(*workloads.InstanceSet)
	if err := validateSpec(rsm, tree); err != nil {
		return kubebuilderx.CheckResultWithError(err)
	}
	return kubebuilderx.ResultSatisfied
}

func (r *revisionUpdateReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (*kubebuilderx.ObjectTree, error) {
	rsm, _ := tree.GetRoot().(*workloads.InstanceSet)
	rsmExt, err := buildRSMExt(rsm, tree)
	if err != nil {
		return nil, err
	}

	// 1. build all templates by applying instance template overrides to default pod template
	instanceTemplateList := buildInstanceTemplateExts(rsmExt)

	// build instance revision list from instance templates
	var instanceRevisionList []instanceRevision
	for _, template := range instanceTemplateList {
		instanceNames := GenerateInstanceNamesFromTemplate(rsm.Name, template.Name, template.Replicas, rsmExt.rsm.Spec.OfflineInstances)
		revision, err := buildInstanceTemplateRevision(template, rsm)
		if err != nil {
			return nil, err
		}
		for _, name := range instanceNames {
			instanceRevisionList = append(instanceRevisionList, instanceRevision{name: name, revision: revision})
		}
	}
	// validate duplicate pod names
	getNameFunc := func(r instanceRevision) string {
		return r.name
	}
	if err := ValidateDupInstanceNames(instanceRevisionList, getNameFunc); err != nil {
		return nil, err
	}

	updatedRevisions := make(map[string]string, len(instanceRevisionList))
	for _, r := range instanceRevisionList {
		updatedRevisions[r.name] = r.revision
	}

	// 3. persistent these revisions to status
	revisions, err := buildUpdateRevisions(updatedRevisions)
	if err != nil {
		return nil, err
	}
	rsm.Status.UpdateRevisions = revisions
	updateRevision := ""
	if len(instanceRevisionList) > 0 {
		updateRevision = instanceRevisionList[len(instanceRevisionList)-1].revision
	}
	rsm.Status.UpdateRevision = updateRevision
	// The 'ObservedGeneration' field is used to indicate whether the revisions have been updated.
	// Computing these revisions in each reconciliation loop can be time-consuming, so we optimize it by
	// performing the computation only when the 'spec' is updated.
	rsm.Status.ObservedGeneration = rsm.Generation

	return tree, nil
}

var _ kubebuilderx.Reconciler = &revisionUpdateReconciler{}
