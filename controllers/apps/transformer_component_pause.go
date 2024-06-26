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

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/extensions"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// componentDeletionTransformer handles component deletion
type componentPauseTransformer struct {
	client.Client
}

var _ graph.Transformer = &componentDeletionTransformer{}

func (t *componentPauseTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)

	graphCli, _ := transCtx.Client.(model.GraphClient)
	comp := transCtx.Component
	if model.IsReconciliationPaused(comp) {
		// get instanceSet and set paused
		instanceSet, err := t.getInstanceSet(transCtx, comp)
		if err != nil {
			return err
		}
		if !instanceSet.Spec.Paused {
			instanceSet.Spec.Paused = true
			graphCli.Update(dag, nil, instanceSet)
		}
		// list configmaps and set paused
		configMapList, err := t.listConfigMaps(transCtx, comp)
		if err != nil {
			return err
		}
		if err != nil {
			return err
		}
		for _, configMap := range configMapList.Items {
			annotations := configMap.GetAnnotations()
			if annotations == nil {
				annotations = make(map[string]string)
			}
			annotations[extensions.ControllerPaused] = trueVal
			configMap.SetAnnotations(annotations)
			graphCli.Update(dag, nil, &configMap)
		}

		return graph.ErrPrematureStop
	} else {
		// get instanceSet and cancel paused
		oldInstanceSet, _ := t.getInstanceSet(transCtx, comp)
		if model.IsReconciliationPaused(oldInstanceSet) {
			oldInstanceSet.Spec.Paused = false
			graphCli.Update(dag, nil, oldInstanceSet)
			return nil
		}
		// list configmaps and cancel paused
		configMapList, err := t.listConfigMaps(transCtx, comp)
		if err != nil {
			return err
		}
		for _, configMap := range configMapList.Items {
			if model.IsReconciliationPaused(&configMap) {
				delete(configMap.Annotations, extensions.ControllerPaused)
				graphCli.Update(dag, configMap.DeepCopy(), &configMap)
			}
		}
		return nil
	}
}

func (t *componentPauseTransformer) getInstanceSet(transCtx *componentTransformContext, comp *appsv1alpha1.Component) (*workloads.InstanceSet, error) {
	instanceName := comp.Name
	instanceSet := &workloads.InstanceSet{}
	err := transCtx.Client.Get(transCtx.Context, types.NamespacedName{Name: instanceName, Namespace: comp.Namespace}, instanceSet)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to get instanceSet %s: %v", instanceName, err))
	}
	return instanceSet, nil
}

func (t *componentPauseTransformer) listConfigMaps(transCtx *componentTransformContext, component *appsv1alpha1.Component) (*corev1.ConfigMapList, error) {
	cmList := &corev1.ConfigMapList{}
	ml := constant.GetComponentWellKnownLabels(component.Labels[constant.AppInstanceLabelKey], component.Labels[constant.KBAppComponentLabelKey])

	listOpts := []client.ListOption{
		client.InNamespace(component.Namespace),
		client.MatchingLabels(ml),
	}
	err := t.Client.List(transCtx, cmList, listOpts...)
	if err != nil {
		return nil, err
	}
	return cmList, nil
}
