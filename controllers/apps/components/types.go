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

package components

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/class"
	"github.com/apecloud/kubeblocks/pkg/constant"
	types2 "github.com/apecloud/kubeblocks/pkg/controller/client"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type Component interface {
	GetName() string
	GetNamespace() string
	GetClusterName() string

	GetCluster() *appsv1alpha1.Cluster
	GetClusterVersion() *appsv1alpha1.ClusterVersion
	GetSynthesizedComponent() *component.SynthesizedComponent

	Create(reqCtx intctrlutil.RequestCtx, cli client.Client) error
	Delete(reqCtx intctrlutil.RequestCtx, cli client.Client) error
	Update(reqCtx intctrlutil.RequestCtx, cli client.Client) error
	Status(reqCtx intctrlutil.RequestCtx, cli client.Client) error
}

func NewComponent(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	definition *appsv1alpha1.ClusterDefinition,
	version *appsv1alpha1.ClusterVersion,
	cluster *appsv1alpha1.Cluster,
	compName string,
	dag *graph.DAG) (Component, error) {
	var compDef *appsv1alpha1.ClusterComponentDefinition
	var compVer *appsv1alpha1.ClusterComponentVersion
	compSpec := cluster.Spec.GetComponentByName(compName)
	if compSpec != nil {
		compDef = definition.GetComponentDefByName(compSpec.ComponentDefRef)
		if compDef == nil {
			return nil, fmt.Errorf("referenced component definition does not exist, cluster: %s, component: %s, component definition ref:%s",
				cluster.Name, compSpec.Name, compSpec.ComponentDefRef)
		}
		if version != nil {
			compVer = version.Spec.GetDefNameMappingComponents()[compSpec.ComponentDefRef]
		}
	} else {
		compDef = definition.GetComponentDefByName(compName)
		if version != nil {
			compVer = version.Spec.GetDefNameMappingComponents()[compName]
		}
	}

	if compDef == nil {
		return nil, nil
	}

	clsMgr, err := getClassManager(reqCtx.Ctx, cli, cluster)
	if err != nil {
		return nil, err
	}
	serviceReferences, err := plan.GenServiceReferences(reqCtx, cli, cluster, compDef, compSpec)
	if err != nil {
		return nil, err
	}

	synthesizedComp, err := component.BuildComponent(reqCtx, clsMgr, cluster, definition, compDef, compSpec, serviceReferences, compVer)
	if err != nil {
		return nil, err
	}
	if synthesizedComp == nil {
		return nil, nil
	}

	return newRSMComponent(cli, reqCtx.Recorder, cluster, version, synthesizedComp, dag), nil
}

func getClassManager(ctx context.Context, cli types2.ReadonlyClient, cluster *appsv1alpha1.Cluster) (*class.Manager, error) {
	var classDefinitionList appsv1alpha1.ComponentClassDefinitionList
	ml := []client.ListOption{
		client.MatchingLabels{constant.ClusterDefLabelKey: cluster.Spec.ClusterDefRef},
	}
	if err := cli.List(ctx, &classDefinitionList, ml...); err != nil {
		return nil, err
	}

	var constraintList appsv1alpha1.ComponentResourceConstraintList
	if err := cli.List(ctx, &constraintList); err != nil {
		return nil, err
	}
	return class.NewManager(classDefinitionList, constraintList)
}
