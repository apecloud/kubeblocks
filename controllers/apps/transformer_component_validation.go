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

package apps

import (
	"fmt"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
)

const (
	defaultMinReplicas         int32 = 0
	defaultMinReplicas4ScaleIn int32 = 1
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

	if err = validateEnabledLogs(comp, transCtx.CompDef); err != nil {
		return newRequeueError(requeueDuration, err.Error())
	}
	if err = validateCompReplicas(comp, transCtx.CompDef); err != nil {
		return newRequeueError(requeueDuration, err.Error())
	}
	// if err = validateSidecarContainers(comp, transCtx.CompDef); err != nil {
	// 	return newRequeueError(requeueDuration, err.Error())
	// }
	return nil
}

func validateEnabledLogs(comp *appsv1alpha1.Component, compDef *appsv1alpha1.ComponentDefinition) error {
	invalidLogNames := validateEnabledLogConfigs(compDef, comp.Spec.EnabledLogs)
	if len(invalidLogNames) > 0 {
		return fmt.Errorf("logs %s are not defined in the definition", invalidLogNames)
	}
	return nil
}

func validateEnabledLogConfigs(compDef *appsv1alpha1.ComponentDefinition, enabledLogs []string) []string {
	invalidLogNames := make([]string, 0, len(enabledLogs))
	logTypes := make(map[string]struct{})

	for _, logConfig := range compDef.Spec.LogConfigs {
		logTypes[logConfig.Name] = struct{}{}
	}

	// imply that all values in enabledLogs config are invalid.
	if len(logTypes) == 0 {
		return enabledLogs
	}
	for _, name := range enabledLogs {
		if _, ok := logTypes[name]; !ok {
			invalidLogNames = append(invalidLogNames, name)
		}
	}
	return invalidLogNames
}

func validateCompReplicas(comp *appsv1alpha1.Component, compDef *appsv1alpha1.ComponentDefinition) error {
	if err := validateCompReplicasGeneral(comp, compDef); err != nil {
		return err
	}
	return validateCompReplicas4Runtime(comp, compDef)
}

func validateCompReplicasGeneral(comp *appsv1alpha1.Component, compDef *appsv1alpha1.ComponentDefinition) error {
	if compDef.Spec.ReplicasLimit == nil {
		return nil
	}
	replicas := comp.Spec.Replicas
	replicasLimit := compDef.Spec.ReplicasLimit
	if replicas >= replicasLimit.MinReplicas && replicas <= replicasLimit.MaxReplicas {
		return nil
	}
	return replicasOutOfLimitError(replicas, *replicasLimit)
}

func validateCompReplicas4Runtime(comp *appsv1alpha1.Component, compDef *appsv1alpha1.ComponentDefinition) error {
	minReplicas := func() int32 {
		// always respect the replicas limit if it is set.
		if compDef.Spec.ReplicasLimit != nil {
			return compDef.Spec.ReplicasLimit.MinReplicas
		}
		// HACK: take observedGeneration == 0 as the provisioning.
		if comp.Status.ObservedGeneration == 0 {
			return defaultMinReplicas
		}
		return defaultMinReplicas4ScaleIn
	}()
	
	replicas := comp.Spec.Replicas
	if replicas < minReplicas {
		return fmt.Errorf("replicas %d is less than required min replicas %d", replicas, minReplicas)
	}
	return nil
}

func replicasOutOfLimitError(replicas int32, replicasLimit appsv1alpha1.ReplicasLimit) error {
	return fmt.Errorf("replicas %d out-of-limit [%d, %d]", replicas, replicasLimit.MinReplicas, replicasLimit.MaxReplicas)
}
