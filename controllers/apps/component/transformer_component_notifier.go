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
	"context"
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type componentNotifierTransformer struct{}

var _ graph.Transformer = &componentNotifierTransformer{}

func (t *componentNotifierTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	if !model.IsObjectUpdating(transCtx.ComponentOrig) {
		return nil
	}

	synthesizedComp, err := t.init(transCtx)
	if err != nil {
		return err
	}

	dependents, err := t.dependents(transCtx.Context, transCtx.Client, synthesizedComp)
	if err != nil {
		return err
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	for _, compName := range dependents {
		if err = t.notify(transCtx.Context, transCtx.Client, graphCli, dag, synthesizedComp, compName); err != nil {
			return err
		}
	}
	return nil
}

func (t *componentNotifierTransformer) init(transCtx *componentTransformContext) (*component.SynthesizedComponent, error) {
	if transCtx.SynthesizeComponent != nil {
		return transCtx.SynthesizeComponent, nil
	}

	var (
		ctx  = transCtx.Context
		cli  = transCtx.Client
		comp = transCtx.Component
	)
	clusterName, err := component.GetClusterName(comp)
	if err != nil {
		return nil, err
	}
	compName, err := component.ShortName(clusterName, comp.Name)
	if err != nil {
		return nil, err
	}
	comp2CompDef, err := component.BuildComp2CompDefs(ctx, cli, comp.Namespace, clusterName)
	if err != nil {
		return nil, err
	}
	return &component.SynthesizedComponent{
		Namespace:     comp.Namespace,
		ClusterName:   clusterName,
		Comp2CompDefs: comp2CompDef,
		Name:          compName,
		Generation:    strconv.FormatInt(comp.Generation, 10),
		CompDefName:   comp.Spec.CompDef,
	}, nil
}

func (t *componentNotifierTransformer) dependents(ctx context.Context,
	cli client.Reader, synthesizedComp *component.SynthesizedComponent) ([]string, error) {
	dependents := make([]string, 0)
	for compName, compDefName := range synthesizedComp.Comp2CompDefs {
		if compName == synthesizedComp.Name {
			continue // skip self
		}
		depended, err := t.depended(ctx, cli, synthesizedComp, compDefName)
		if err != nil {
			return nil, err
		}
		if depended {
			dependents = append(dependents, compName)
		}
	}
	return dependents, nil
}

func (t *componentNotifierTransformer) depended(ctx context.Context,
	cli client.Reader, synthesizedComp *component.SynthesizedComponent, compDefName string) (bool, error) {
	compDefReferenced := func(v appsv1.EnvVar) string {
		if v.ValueFrom != nil {
			if v.ValueFrom.HostNetworkVarRef != nil {
				return v.ValueFrom.HostNetworkVarRef.CompDef
			}
			if v.ValueFrom.ServiceVarRef != nil {
				return v.ValueFrom.ServiceVarRef.CompDef
			}
			if v.ValueFrom.CredentialVarRef != nil {
				return v.ValueFrom.CredentialVarRef.CompDef
			}
			if v.ValueFrom.TLSVarRef != nil {
				return v.ValueFrom.TLSVarRef.CompDef
			}
			if v.ValueFrom.ServiceRefVarRef != nil {
				return v.ValueFrom.ServiceRefVarRef.CompDef
			}
			if v.ValueFrom.ResourceVarRef != nil {
				return v.ValueFrom.ResourceVarRef.CompDef
			}
			if v.ValueFrom.ComponentVarRef != nil {
				return v.ValueFrom.ComponentVarRef.CompDef
			}
		}
		return ""
	}

	compDef, err := getNCheckCompDefinition(ctx, cli, compDefName)
	if err != nil {
		return false, err
	}

	for _, v := range compDef.Spec.Vars {
		compDefPattern := compDefReferenced(v)
		if len(compDefPattern) > 0 {
			if component.PrefixOrRegexMatched(synthesizedComp.CompDefName, compDefPattern) {
				return true, nil
			}
		}
	}
	return false, nil
}

func (t *componentNotifierTransformer) notify(ctx context.Context, cli client.Reader,
	graphCli model.GraphClient, dag *graph.DAG, synthesizedComp *component.SynthesizedComponent, compName string) error {
	comp := &appsv1.Component{}
	compKey := types.NamespacedName{
		Namespace: synthesizedComp.Namespace,
		Name:      constant.GenerateClusterComponentName(synthesizedComp.ClusterName, compName),
	}
	if err := cli.Get(ctx, compKey, comp); err != nil {
		return client.IgnoreNotFound(err)
	}

	if model.IsObjectUpdating(comp) {
		return nil // skip if the target component is being updated
	}

	compCopy := comp.DeepCopy()
	if compCopy.Annotations == nil {
		compCopy.Annotations = make(map[string]string)
	}
	compCopy.Annotations[constant.ReconcileAnnotationKey] =
		fmt.Sprintf("%s@%s", synthesizedComp.Name, synthesizedComp.Generation)

	graphCli.Patch(dag, comp, compCopy)
	// patch the component object after the changes of other objects are submitted
	for _, obj := range graphCli.FindAll(dag, &appsv1.Component{}, &model.HaveDifferentTypeWithOption{}) {
		graphCli.DependOn(dag, compCopy, obj)
	}

	return nil
}
