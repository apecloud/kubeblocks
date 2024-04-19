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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/apis/workloads/legacy"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	rsmcore "github.com/apecloud/kubeblocks/pkg/controller/rsm"
)

// upgradeTransformer upgrades the underlying workload from the legacy RSM API to the InstanceSet API.
type upgradeTransformer struct{}

var _ graph.Transformer = &upgradeTransformer{}

func (t *upgradeTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	graphCli, _ := transCtx.Client.(model.GraphClient)
	comp := transCtx.Component
	synthesizeComp := transCtx.SynthesizeComponent

	var parent *model.ObjectVertex
	legacyFound := false
	// remove the RSM object if found
	rsm := &legacy.ReplicatedStateMachine{}
	if err := graphCli.Get(transCtx.Context, client.ObjectKeyFromObject(comp), rsm); err == nil {
		legacyFound = true
		parent = graphCli.Do(dag, nil, rsm, model.ActionDeletePtr(), parent)
	} else if !apierrors.IsNotFound(err) {
		return err
	}

	// remove the StatefulSet object if found
	sts := &appsv1.StatefulSet{}
	if err := graphCli.Get(transCtx.Context, client.ObjectKeyFromObject(comp), sts); err == nil {
		legacyFound = true
		// update spec.selector and replicas to make pods orphans
		sts.Spec.Selector.MatchLabels["apps.kubeblocks.io/upgrade"] = "true"
		sts.Spec.Replicas = func() *int32 { r := int32(0); return &r }()
		parent = graphCli.Do(dag, nil, sts, model.ActionUpdatePtr(), parent)
		parent = graphCli.Do(dag, nil, sts, model.ActionDeletePtr(), parent)
	} else if !apierrors.IsNotFound(err) {
		return err
	}

	// update pod labels, ownerReferences
	podList := &corev1.PodList{}
	ml := client.MatchingLabels{
		constant.AppManagedByLabelKey:   constant.AppName,
		constant.AppInstanceLabelKey:    transCtx.Cluster.Name,
		constant.KBAppComponentLabelKey: comp.Name,
	}
	if err := graphCli.List(transCtx.Context, podList, ml); err == nil {
		if len(podList.Items) > 0 {
			var revision string
			for i := range podList.Items {
				pod := &podList.Items[i]
				if _, ok := pod.Labels[rsmcore.WorkloadsManagedByLabelKey]; ok {
					continue
				}
				legacyFound = true
				pod.OwnerReferences = nil
				pod.Labels[rsmcore.WorkloadsManagedByLabelKey] = rsmcore.KindInstanceSet
				pod.Labels[rsmcore.WorkloadsInstanceLabelKey] = constant.GenerateWorkloadNamePattern(transCtx.Cluster.Name, comp.Name)
				if revision == "" {
					revision, err = buildRevision(synthesizeComp)
					if err != nil {
						return err
					}
				}
				pod.Labels[appsv1.ControllerRevisionHashLabelKey] = revision
				parent = graphCli.Do(dag, nil, pod, model.ActionUpdatePtr(), parent)
			}
		}
	} else if !apierrors.IsNotFound(err) {
		return err
	}

	// remove xxx-rsm-env configmap
	env := &corev1.ConfigMap{}
	key := types.NamespacedName{
		Namespace: comp.Namespace,
		Name:      rsmcore.GetEnvConfigMapName(constant.GenerateWorkloadNamePattern(transCtx.Cluster.Name, comp.Name)),
	}
	if err := graphCli.Get(transCtx.Context, key, env); err == nil {
		legacyFound = true
		parent = graphCli.Do(dag, nil, env, model.ActionDeletePtr(), parent)
	} else if !apierrors.IsNotFound(err) {
		return err
	}

	if legacyFound {
		return graph.ErrPrematureStop
	}
	return nil
}

func buildRevision(synthesizeComp *component.SynthesizedComponent) (string, error) {
	buildPodSpecVolumeMounts(synthesizeComp)
	its, err := factory.BuildInstanceSet(synthesizeComp)
	if err != nil {
		return "", err
	}
	return instanceset.BuildInstanceTemplateRevision(&its.Spec.Template, its)
}
