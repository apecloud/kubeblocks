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
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	viewv1 "github.com/apecloud/kubeblocks/apis/view/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
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
	v, _ := tree.GetRoot().(*viewv1.ReconciliationView)

	// view definition object should exist
	o, err := tree.Get(builder.NewReconciliationViewDefinitionBuilder(v.Spec.ViewDefinition).GetObject())
	if err != nil {
		return kubebuilderx.Commit, err
	}
	if o == nil {
		return kubebuilderx.Commit, fmt.Errorf("view definition %s for view %s/%s not found", v.Namespace, v.Name, v.Spec.ViewDefinition)
	}

	// i18n resources should exist
	viewDef, _ := o.(*viewv1.ReconciliationViewDefinition)
	if viewDef.Spec.I18nResourceRef != nil {
		_, err = tree.Get(builder.NewConfigMapBuilder(viewDef.Spec.I18nResourceRef.Namespace, viewDef.Spec.I18nResourceRef.Name).GetObject())
		if err != nil {
			return kubebuilderx.Commit, fmt.Errorf("i18n resources %s/%s for view %s/%s, definition %s not found",
				viewDef.Spec.I18nResourceRef.Namespace, viewDef.Spec.I18nResourceRef.Name, v.Namespace, v.Name, viewDef.Name)
		}
	}

	// target object should exist
	objectKey := client.ObjectKeyFromObject(v)
	if v.Spec.TargetObject != nil {
		objectKey.Namespace = v.Spec.TargetObject.Namespace
		objectKey.Name = v.Spec.TargetObject.Name
	}
	if err = r.reader.Get(r.ctx, objectKey, &appsv1alpha1.Cluster{}); err != nil {
		return kubebuilderx.Commit, err
	}

	return kubebuilderx.Continue, nil
}

func viewResourcesValidation(ctx context.Context, reader client.Reader) kubebuilderx.Reconciler {
	return &resourcesValidator{ctx: ctx, reader: reader}
}

var _ kubebuilderx.Reconciler = &resourcesValidator{}
