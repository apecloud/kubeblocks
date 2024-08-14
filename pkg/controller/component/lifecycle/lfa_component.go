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

package lifecycle

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
)

type postProvision struct {
	synthesizedComp *component.SynthesizedComponent
	action          *appsv1alpha1.Action
}

var _ lifecycleAction = &postProvision{}

func (a *postProvision) name() string {
	return "postProvision"
}

func (a *postProvision) precondition(ctx context.Context, cli client.Reader) error {
	return preconditionCheck(ctx, cli, a.synthesizedComp, a.action)
}

func (a *postProvision) parameters(ctx context.Context, cli client.Reader) (map[string]string, error) {
	return nil, nil
}

type preTerminate struct {
	synthesizedComp *component.SynthesizedComponent
	action          *appsv1alpha1.Action
}

var _ lifecycleAction = &preTerminate{}

func (a *preTerminate) name() string {
	return "preTerminate"
}

func (a *preTerminate) precondition(ctx context.Context, cli client.Reader) error {
	return preconditionCheck(ctx, cli, a.synthesizedComp, a.action)
}

func (a *preTerminate) parameters(ctx context.Context, cli client.Reader) (map[string]string, error) {
	return nil, nil
}

func preconditionCheck(ctx context.Context, cli client.Reader, synthesizedComp *component.SynthesizedComponent, action *appsv1alpha1.Action) error {
	if action.PreCondition == nil {
		return nil
	}
	switch *action.PreCondition {
	case appsv1alpha1.ImmediatelyPreConditionType:
		return nil
	case appsv1alpha1.RuntimeReadyPreConditionType:
		return runtimeReadyCheck(ctx, cli, synthesizedComp)
	case appsv1alpha1.ComponentReadyPreConditionType:
		return compReadyCheck(ctx, cli, synthesizedComp)
	case appsv1alpha1.ClusterReadyPreConditionType:
		return clusterReadyCheck(ctx, cli, synthesizedComp)
	default:
		return fmt.Errorf("unknown precondition type %s", *action.PreCondition)
	}
}

func clusterReadyCheck(ctx context.Context, cli client.Reader, synthesizedComp *component.SynthesizedComponent) error {
	ready := func(object client.Object) bool {
		cluster := object.(*appsv1alpha1.Cluster)
		return cluster.Status.Phase == appsv1alpha1.RunningClusterPhase
	}
	return readyCheck(ctx, cli, synthesizedComp, synthesizedComp.ClusterName, "cluster", &appsv1alpha1.Cluster{}, ready)
}

func compReadyCheck(ctx context.Context, cli client.Reader, synthesizedComp *component.SynthesizedComponent) error {
	ready := func(object client.Object) bool {
		comp := object.(*appsv1alpha1.Component)
		return comp.Status.Phase == appsv1alpha1.RunningClusterCompPhase
	}
	return readyCheck(ctx, cli, synthesizedComp, synthesizedComp.FullCompName, "component", &appsv1alpha1.Component{}, ready)
}

func runtimeReadyCheck(ctx context.Context, cli client.Reader, synthesizedComp *component.SynthesizedComponent) error {
	name := constant.GenerateWorkloadNamePattern(synthesizedComp.ClusterName, synthesizedComp.Name)
	ready := func(object client.Object) bool {
		its := object.(*workloads.InstanceSet)
		return instanceset.IsInstancesReady(its)
	}
	return readyCheck(ctx, cli, synthesizedComp, name, "runtime", &workloads.InstanceSet{}, ready)
}

func readyCheck(ctx context.Context, cli client.Reader, synthesizedComp *component.SynthesizedComponent,
	name, kind string, obj client.Object, ready func(object client.Object) bool) error {
	key := types.NamespacedName{
		Namespace: synthesizedComp.Namespace,
		Name:      name,
	}
	if err := cli.Get(ctx, key, obj); err != nil {
		return errors.Wrap(err, fmt.Sprintf("precondition check error for %s ready", kind))
	}
	if !ready(obj) {
		return fmt.Errorf("precondition check error, %s is not ready", kind)
	}
	return nil
}
