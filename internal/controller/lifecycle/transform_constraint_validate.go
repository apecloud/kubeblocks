/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package lifecycle

import (
	"fmt"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	graph "github.com/apecloud/kubeblocks/internal/controller/graph"
)

// ConstraintsValidationTransformer validates explicitly specified constraints.
type ConstraintsValidationTransformer struct{}

func (e *ConstraintsValidationTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	clusterDef := transCtx.ClusterDef
	cluster := transCtx.Cluster
	clusterDefMap := cluster.Spec.GetDefNameMappingComponents()

	for _, compDef := range clusterDef.Spec.ComponentDefs {
		clusterComps := clusterDefMap[compDef.Name]
		err := meetsNumOfOccConstraint(&compDef, clusterComps)
		if err != nil {
			return newValidationError(err.Error())
		}

		err = meetsComponentRefConstraint(&compDef, clusterDef)
		if err != nil {
			return newValidationError(err.Error())
		}
	}
	return nil
}

func meetsNumOfOccConstraint(compDef *appsv1alpha1.ClusterComponentDefinition, clusterComps []appsv1alpha1.ClusterComponentSpec) error {
	if compDef.Constraints == nil {
		return nil
	}
	constraint := compDef.Constraints
	var match bool
	switch constraint.NumberOfOccurrence {
	case appsv1alpha1.ZeroOrOnce:
		match = len(clusterComps) <= 1
	case appsv1alpha1.ExactlyOnce:
		match = len(clusterComps) == 1
	case appsv1alpha1.OnceOrMore:
		match = len(clusterComps) >= 1
	case appsv1alpha1.Unlimited:
		match = true
	}

	if !match {
		return fmt.Errorf("components for componentDef: %s, appears %d times, does not meet the numberOfOccurrence constraint: %s", compDef.Name, len(clusterComps), string(constraint.NumberOfOccurrence))
	}

	return nil
}

func meetsComponentRefConstraint(compDef *appsv1alpha1.ClusterComponentDefinition, clusterDef *appsv1alpha1.ClusterDefinition) error {
	if len(compDef.ComponentRef) == 0 {
		return nil
	}
	for _, compref := range compDef.ComponentRef {
		referredCompDefName := compref.ComponentDefName
		referredCompName := compref.ComponentName

		if len(referredCompDefName) == 0 && len(referredCompName) == 0 {
			return fmt.Errorf("ComponentDefinition: %s, field `componentRef` must specify either componentDefName or componentName", compDef.Name)
		}
		// at this very early stage, we only check if the referred componentDef exists and serviceRef is valid.
		// more concrete validations will be done in a later stage when building SynthesizedComponent.
		if len(referredCompDefName) == 0 {
			return nil
		}

		referredCompDef := clusterDef.GetComponentDefByName(referredCompDefName)
		if referredCompDef == nil {
			return fmt.Errorf("ComponentDefinition: %s, field `componentRef` refers to non-existing componentDef: %s", compDef.Name, referredCompDefName)
		}

		serviceSpec := referredCompDef.Service

		findServiceByName := func(serviceName string) bool {
			if serviceSpec == nil {
				return false
			}
			for _, port := range serviceSpec.Ports {
				if port.Name == serviceName {
					return true
				}
			}
			return false
		}

		for _, serviceRef := range compref.ServiceRefs {
			if !findServiceByName(serviceRef.ServiceName) {
				return fmt.Errorf("ComponentDefinition: %s, field `componentRef` refers to componentDef: %s, which does not have a service named: %s", compDef.Name, referredCompDefName, serviceRef)
			}
		}
	}
	return nil
}

var _ graph.Transformer = &ConstraintsValidationTransformer{}
