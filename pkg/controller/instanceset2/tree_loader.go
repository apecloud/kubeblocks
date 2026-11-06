/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
)

func NewTreeLoader() kubebuilderx.TreeLoader {
	return &treeLoader{}
}

type treeLoader struct{}

var _ kubebuilderx.TreeLoader = &treeLoader{}

func (r *treeLoader) Load(ctx context.Context, reader client.Reader, req ctrl.Request, recorder record.EventRecorder, logger logr.Logger) (*kubebuilderx.ObjectTree, error) {
	ml := getMatchLabels(req.Name)
	kinds := ownedKinds()
	tree, err := kubebuilderx.ReadObjectTree[*workloads.InstanceSet](ctx, reader, req, ml, kinds...)
	if err != nil {
		return nil, err
	}

	// load compressed instance templates if present
	if err = loadCompressedInstanceTemplates(ctx, reader, tree); err != nil {
		return nil, err
	}

	// load assistant objects
	if err = loadInstanceAssistantObjects(ctx, reader, tree); err != nil {
		return nil, err
	}
	if err = loadInstanceRuntimePods(ctx, reader, tree); err != nil {
		return nil, err
	}

	tree.Context = ctx
	tree.EventRecorder = recorder
	tree.Logger = logger
	tree.SetFinalizer(finalizer)

	return tree, err
}

func loadInstanceRuntimePods(ctx context.Context, reader client.Reader, tree *kubebuilderx.ObjectTree) error {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return nil
	}
	its := tree.GetRoot().(*workloads.InstanceSet)
	selector, err := metav1.LabelSelectorAsSelector(its.Spec.Selector)
	if err != nil {
		return err
	}
	seenPlacements := make(map[string]bool)
	for _, object := range tree.List(&workloads.Instance{}) {
		inst := object.(*workloads.Instance)
		placement := inst.Annotations[constant.KBAppMultiClusterPlacementKey]
		if seenPlacements[placement] {
			continue
		}
		seenPlacements[placement] = true
		podContext := multicluster.IntoContext(ctx, placement)
		pods := &corev1.PodList{}
		if err := reader.List(podContext, pods, client.InNamespace(its.Namespace), client.MatchingLabelsSelector{Selector: selector}, multicluster.InDataContext()); err != nil {
			return err
		}
		for i := range pods.Items {
			pod := &pods.Items[i]
			owner := metav1.GetControllerOf(pod)
			if owner == nil || owner.Kind != "Instance" {
				continue
			}
			owned := false
			for _, child := range tree.List(&workloads.Instance{}) {
				candidate := child.(*workloads.Instance)
				owned = owned || candidate.UID != "" && candidate.UID == owner.UID
			}
			if owned {
				if err := tree.AddWithOption(pod, kubebuilderx.SkipToReconcile(true)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func ownedKinds() []client.ObjectList {
	return []client.ObjectList{
		&workloads.InstanceList{},
		&corev1.ServiceList{},
	}
}

func loadCompressedInstanceTemplates(ctx context.Context, reader client.Reader, tree *kubebuilderx.ObjectTree) error {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return nil
	}
	templateMap, err := getInstanceTemplateMap(tree.GetRoot().GetAnnotations())
	if err != nil {
		return err
	}
	ns := tree.GetRoot().GetNamespace()
	for _, templateName := range templateMap {
		template := &corev1.ConfigMap{}
		if err := reader.Get(ctx, types.NamespacedName{Namespace: ns, Name: templateName}, template); err != nil {
			return err
		}
		if err := tree.Add(template); err != nil {
			return err
		}
	}
	return nil
}
