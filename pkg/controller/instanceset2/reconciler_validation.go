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

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

func NewValidationReconciler() kubebuilderx.Reconciler {
	return &validationReconciler{}
}

type validationReconciler struct{}

var _ kubebuilderx.Reconciler = &apiVersionReconciler{}

func (r *validationReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (r *validationReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	its := tree.GetRoot().(*workloads.InstanceSet)
	if err := validateUnsupportedFeatures(its); err != nil {
		return kubebuilderx.Commit, err
	}
	if err := instancetemplate.ValidateInstanceTemplates(its, tree); err != nil {
		return kubebuilderx.Commit, err
	}
	return kubebuilderx.Continue, nil
}

func validateUnsupportedFeatures(its *workloads.InstanceSet) error {
	if its == nil || its.Annotations == nil {
		return nil
	}
	if _, ok := its.Annotations[constant.NodeSelectorOnceAnnotationKey]; ok {
		return fmt.Errorf("annotation %q is not supported when enableInstanceAPI=true", constant.NodeSelectorOnceAnnotationKey)
	}
	return nil
}
