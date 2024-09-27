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

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	viewv1 "github.com/apecloud/kubeblocks/apis/view/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

type resourcesValidator struct {
	ctx    context.Context
	reader client.Reader
}

func (r *resourcesValidator) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree == nil {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (r *resourcesValidator) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	// view object should exist
	if tree.GetRoot() == nil {
		return kubebuilderx.Commit, nil
	}

	// target object should exist
	v, _ := tree.GetRoot().(*viewv1.ReconciliationView)
	objectKey := client.ObjectKeyFromObject(v)
	if v.Spec.TargetObject != nil {
		objectKey.Namespace = v.Spec.TargetObject.Namespace
		objectKey.Name = v.Spec.TargetObject.Name
	}
	if err := r.reader.Get(r.ctx, objectKey, &kbappsv1.Cluster{}); err != nil {
		return kubebuilderx.Commit, err
	}

	return kubebuilderx.Continue, nil
}

func viewResourcesValidation(ctx context.Context, reader client.Reader) kubebuilderx.Reconciler {
	return &resourcesValidator{ctx: ctx, reader: reader}
}

var _ kubebuilderx.Reconciler = &resourcesValidator{}
