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

package view

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	viewv1 "github.com/apecloud/kubeblocks/apis/view/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

type planResourcesValidator struct {
	ctx    context.Context
	reader client.Reader
}

func (r *planResourcesValidator) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree == nil {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (r *planResourcesValidator) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	// plan object should exist
	if tree.GetRoot() == nil {
		return kubebuilderx.Commit, nil
	}

	// target object should exist
	p, _ := tree.GetRoot().(*viewv1.ReconciliationPlan)
	objectKey := client.ObjectKeyFromObject(p)
	if p.Spec.TargetObject != nil {
		objectKey.Namespace = p.Spec.TargetObject.Namespace
		objectKey.Name = p.Spec.TargetObject.Name
	}
	if err := r.reader.Get(r.ctx, objectKey, &appsv1alpha1.Cluster{}); err != nil {
		return kubebuilderx.Commit, err
	}

	return kubebuilderx.Continue, nil
}

func planResourcesValidation(ctx context.Context, reader client.Reader) kubebuilderx.Reconciler {
	return &planResourcesValidator{ctx: ctx, reader: reader}
}

var _ kubebuilderx.Reconciler = &planResourcesValidator{}
