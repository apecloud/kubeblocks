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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
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
	if err = loadInstancePods(ctx, reader, tree); err != nil {
		return nil, err
	}

	tree.Context = ctx
	tree.EventRecorder = recorder
	tree.Logger = logger
	tree.SetFinalizer(finalizer)

	return tree, err
}

func loadInstancePods(ctx context.Context, reader client.Reader, tree *kubebuilderx.ObjectTree) error {
	its, ok := tree.GetRoot().(*workloads.InstanceSet)
	if !ok || its == nil || model.IsObjectDeleting(its) {
		return nil
	}
	selector, err := metav1.LabelSelectorAsSelector(its.Spec.Selector)
	if err != nil {
		return err
	}
	pods := &corev1.PodList{}
	if err := reader.List(ctx, pods, client.InNamespace(its.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return err
	}
	loaded := make(map[string]struct{}, len(pods.Items))
	for i := range pods.Items {
		if err := tree.AddWithOption(&pods.Items[i], kubebuilderx.SkipToReconcile(true)); err != nil {
			return err
		}
		loaded[pods.Items[i].Name] = struct{}{}
	}
	candidates := make(map[string]struct{}, len(its.Status.InstanceStatus)+len(its.Spec.OfflineInstances))
	for _, status := range its.Status.InstanceStatus {
		candidates[status.PodName] = struct{}{}
	}
	for _, name := range its.Spec.OfflineInstances {
		candidates[name] = struct{}{}
	}
	for _, object := range tree.List(&workloads.Instance{}) {
		candidates[object.GetName()] = struct{}{}
	}
	for name := range candidates {
		if _, ok := loaded[name]; ok {
			continue
		}
		pod := &corev1.Pod{}
		if err := reader.Get(ctx, types.NamespacedName{Namespace: its.Namespace, Name: name}, pod); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		if err := tree.AddWithOption(pod, kubebuilderx.SkipToReconcile(true)); err != nil {
			return err
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
