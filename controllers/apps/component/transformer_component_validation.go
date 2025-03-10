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

package component

import (
	"fmt"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

var (
	defaultReplicasLimit = appsv1.ReplicasLimit{
		MinReplicas: 1,
		MaxReplicas: 16384,
	}
)

// componentValidationTransformer validates the consistency between spec & definition.
type componentValidationTransformer struct{}

var _ graph.Transformer = &componentValidationTransformer{}

func (t *componentValidationTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	comp := transCtx.Component

	var err error
	defer func() {
		setProvisioningStartedCondition(&comp.Status.Conditions, comp.Name, comp.Generation, err)
	}()

	if err = validateCompReplicas(comp, transCtx.CompDef); err != nil {
		return intctrlutil.NewRequeueError(appsutil.RequeueDuration, err.Error())
	}
	// if err = validateSidecarContainers(comp, transCtx.CompDef); err != nil {
	// 	return newRequeueError(requeueDuration, err.Error())
	// }
	return nil
}

func validateCompReplicas(comp *appsv1.Component, compDef *appsv1.ComponentDefinition) error {
	replicasLimit := &defaultReplicasLimit
	// always respect the replicas limit if set.
	if compDef.Spec.ReplicasLimit != nil {
		replicasLimit = compDef.Spec.ReplicasLimit
	}

	replicas := comp.Spec.Replicas
	if replicas >= replicasLimit.MinReplicas && replicas <= replicasLimit.MaxReplicas {
		return nil
	}
	return replicasOutOfLimitError(replicas, *replicasLimit)
}

func replicasOutOfLimitError(replicas int32, replicasLimit appsv1.ReplicasLimit) error {
	return fmt.Errorf("replicas %d out-of-limit [%d, %d]", replicas, replicasLimit.MinReplicas, replicasLimit.MaxReplicas)
}
